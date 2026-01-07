package codegen

import (
	"fmt"

	"github.com/microsoft/go-winmd"
	"github.com/microsoft/go-winmd/flags"
)

// stripGenericInstFromBlob removes GENERICINST from a signature blob and replaces it with the base type
// Also replaces VAR/MVAR (generic type parameters) with OBJECT
// This is a workaround for microsoft/go-winmd not supporting generic types yet
func stripGenericInstFromBlob(blob []byte) []byte {
	if len(blob) == 0 {
		return blob
	}
	
	// Check if blob contains GENERICINST (0x15), VAR (0x13), or MVAR (0x1E)
	needsProcessing := false
	for _, b := range blob {
		if b == byte(flags.ElementType_GENERICINST) || 
		   b == 0x13 ||  // VAR
		   b == 0x1E {   // MVAR
			needsProcessing = true
			break
		}
	}
	
	if !needsProcessing {
		return blob // No modifications needed
	}
	
	// Parse and rebuild the blob, handling VAR/MVAR and GENERICINST
	result := make([]byte, 0, len(blob))
	i := 0
	
	for i < len(blob) {
		b := blob[i]
		
		if b == 0x13 || b == 0x1E { // VAR or MVAR
			// Replace VAR/MVAR with OBJECT (0x1C)
			result = append(result, 0x1C)
			i++
			// Skip the generic parameter index (compressed uint) that follows VAR/MVAR
			if i < len(blob) {
				_, bytesRead := readCompressedUint(blob[i:])
				i += bytesRead
			}
		} else if b == byte(flags.ElementType_GENERICINST) {
			// Skip GENERICINST and keep just the base type
			i++
			if i >= len(blob) {
				break
			}
			
			// Next should be CLASS (0x12) or VALUETYPE (0x11)
			classOrValue := blob[i]
			result = append(result, classOrValue)
			i++
			
			// Next is TypeDefOrRefOrSpec coded index (compressed) - keep it
			if i >= len(blob) {
				break
			}
			_, bytesRead := readCompressedUint(blob[i:])
			result = append(result, blob[i:i+bytesRead]...)
			i += bytesRead
			
			// Next is GenArgCount (compressed) - skip it
			if i >= len(blob) {
				break
			}
			genArgCount, bytesRead := readCompressedUint(blob[i:])
			i += bytesRead
			
			// Skip generic type arguments - each is a full type signature
			// This is recursive and complex, so we use a simple heuristic:
			// skip genArgCount elements, assuming each is 1-2 bytes
			for j := uint32(0); j < genArgCount && i < len(blob); j++ {
				elemType := blob[i]
				i++
				// If it's a complex type, skip its data too
				if elemType == 0x12 || elemType == 0x11 { // CLASS or VALUETYPE
					if i < len(blob) {
						_, bytesRead := readCompressedUint(blob[i:])
						i += bytesRead
					}
				} else if elemType == 0x13 || elemType == 0x1E { // VAR or MVAR
					if i < len(blob) {
						_, bytesRead := readCompressedUint(blob[i:])
						i += bytesRead
					}
				}
			}
		} else {
			// Copy byte as-is
			result = append(result, b)
			i++
		}
	}
	
	return result
}

// readCompressedUint reads a compressed unsigned integer from a blob
// Returns the value and number of bytes read
func readCompressedUint(data []byte) (uint32, int) {
	if len(data) == 0 {
		return 0, 0
	}
	
	b0 := data[0]
	
	// Single byte: 0xxxxxxx (0-127)
	if (b0 & 0x80) == 0 {
		return uint32(b0), 1
	}
	
	// Two bytes: 10xxxxxx xxxxxxxx (128-16383)
	if (b0 & 0xC0) == 0x80 {
		if len(data) < 2 {
			return 0, 0
		}
		return uint32(b0&0x3F)<<8 | uint32(data[1]), 2
	}
	
	// Four bytes: 110xxxxx xxxxxxxx xxxxxxxx xxxxxxxx (16384-536870911)
	if (b0 & 0xE0) == 0xC0 {
		if len(data) < 4 {
			return 0, 0
		}
		return uint32(b0&0x1F)<<24 | uint32(data[1])<<16 | uint32(data[2])<<8 | uint32(data[3]), 4
	}
	
	return 0, 0
}

// safeFieldSignature safely parses a field signature, handling GENERICINST by using base type
func safeFieldSignature(ctx *winmd.Metadata, blob []byte) (winmd.SigField, error) {
	// Try normal parsing first
	sig, err := ctx.FieldSignature(blob)
	if err != nil {
		// Check if it's the generic types error or VAR/MVAR error
		errStr := err.Error()
		if errStr == "generic types are not yet supported" || 
		   (len(errStr) > 24 && errStr[:24] == "unsupported element type") {
			// Strip GENERICINST/VAR/MVAR and try again with base type only
			strippedBlob := stripGenericInstFromBlob(blob)
			sig, err = ctx.FieldSignature(strippedBlob)
			if err != nil {
				return sig, fmt.Errorf("failed to parse field signature even after stripping generics: %w", err)
			}
		} else {
			return sig, err
		}
	}
	return sig, nil
}

// safeMethodDefSignature safely parses a method signature, handling GENERICINST by using base type
func safeMethodDefSignature(ctx *winmd.Metadata, blob []byte) (winmd.SigMethodDef, error) {
	// Try normal parsing first
	sig, err := ctx.MethodDefSignature(blob)
	if err != nil {
		// Check if it's the generic types error or VAR/MVAR error
		errStr := err.Error()
		if errStr == "generic types are not yet supported" || 
		   (len(errStr) > 24 && errStr[:24] == "unsupported element type") {
			// Strip GENERICINST/VAR/MVAR and try again with base type only
			strippedBlob := stripGenericInstFromBlob(blob)
			sig, err = ctx.MethodDefSignature(strippedBlob)
			if err != nil {
				return sig, fmt.Errorf("failed to parse method signature even after stripping generics: %w", err)
			}
		} else {
			return sig, err
		}
	}
	return sig, nil
}

// ResolveParamList resolves the ParamList for a MethodDef
func ResolveParamList(ctx *winmd.Metadata, methodDef *winmd.MethodDef) ([]*winmd.Param, error) {
	if methodDef.ParamList.Start == methodDef.ParamList.End {
		return nil, nil
	}

	result := make([]*winmd.Param, 0, methodDef.ParamList.End-methodDef.ParamList.Start)
	for i := methodDef.ParamList.Start; i < methodDef.ParamList.End; i++ {
		param, err := ctx.Tables.Param.Record(i)
		if err != nil {
			return nil, err
		}
		result = append(result, param)
	}
	return result, nil
}

// sigTypeToGenParamType converts a winmd.SigType to a genParamType
func (g *generator) sigTypeToGenParamType(ctx *winmd.Metadata, sigType winmd.SigType, curPackage string) (*genParamType, error) {
	switch sigType.Kind {
	case flags.ElementType_BOOLEAN:
		return &genParamType{
			namespace:    "",
			name:         "bool",
			IsPointer:    false,
			IsPrimitive:  true,
			IsArray:      false,
			defaultValue: genDefaultValue{"false", true},
		}, nil
	case flags.ElementType_CHAR:
		return &genParamType{
			namespace:    "",
			name:         "byte",
			IsPointer:    false,
			IsPrimitive:  true,
			IsArray:      false,
			defaultValue: genDefaultValue{"0", true},
		}, nil
	case flags.ElementType_I1:
		return &genParamType{
			namespace:    "",
			name:         "int8",
			IsPointer:    false,
			IsPrimitive:  true,
			IsArray:      false,
			defaultValue: genDefaultValue{"0", true},
		}, nil
	case flags.ElementType_U1:
		return &genParamType{
			namespace:    "",
			name:         "uint8",
			IsPointer:    false,
			IsPrimitive:  true,
			IsArray:      false,
			defaultValue: genDefaultValue{"0", true},
		}, nil
	case flags.ElementType_I2:
		return &genParamType{
			namespace:    "",
			name:         "int16",
			IsPointer:    false,
			IsPrimitive:  true,
			IsArray:      false,
			defaultValue: genDefaultValue{"0", true},
		}, nil
	case flags.ElementType_U2:
		return &genParamType{
			namespace:    "",
			name:         "uint16",
			IsPointer:    false,
			IsPrimitive:  true,
			IsArray:      false,
			defaultValue: genDefaultValue{"0", true},
		}, nil
	case flags.ElementType_I4:
		return &genParamType{
			namespace:    "",
			name:         "int32",
			IsPointer:    false,
			IsPrimitive:  true,
			IsArray:      false,
			defaultValue: genDefaultValue{"0", true},
		}, nil
	case flags.ElementType_U4:
		return &genParamType{
			namespace:    "",
			name:         "uint32",
			IsPointer:    false,
			IsPrimitive:  true,
			IsArray:      false,
			defaultValue: genDefaultValue{"0", true},
		}, nil
	case flags.ElementType_I8:
		return &genParamType{
			namespace:    "",
			name:         "int64",
			IsPointer:    false,
			IsPrimitive:  true,
			IsArray:      false,
			defaultValue: genDefaultValue{"0", true},
		}, nil
	case flags.ElementType_U8:
		return &genParamType{
			namespace:    "",
			name:         "uint64",
			IsPointer:    false,
			IsPrimitive:  true,
			IsArray:      false,
			defaultValue: genDefaultValue{"0", true},
		}, nil
	case flags.ElementType_R4:
		return &genParamType{
			namespace:    "",
			name:         "float32",
			IsPointer:    false,
			IsPrimitive:  true,
			IsArray:      false,
			defaultValue: genDefaultValue{"0", true},
		}, nil
	case flags.ElementType_R8:
		return &genParamType{
			namespace:    "",
			name:         "float64",
			IsPointer:    false,
			IsPrimitive:  true,
			IsArray:      false,
			defaultValue: genDefaultValue{"0", true},
		}, nil
	case flags.ElementType_STRING:
		return &genParamType{
			namespace:    "",
			name:         "string",
			IsPointer:    false,
			IsPrimitive:  true,
			IsArray:      false,
			defaultValue: genDefaultValue{`""`, true},
		}, nil
	case flags.ElementType_OBJECT:
		// OBJECT type - generic placeholder or System.Object
		return &genParamType{
			namespace:    "",
			name:         "interface{}",
			IsPointer:    false,
			IsPrimitive:  false,
			IsArray:      false,
			defaultValue: genDefaultValue{"nil", true},
		}, nil
	case flags.ElementType_VALUETYPE, flags.ElementType_CLASS:
		// These require looking up the type by CodedIndex in Value
		if codedIdx, ok := sigType.Value.(winmd.CodedIndex); ok {
			return g.resolveCodedIndexType(ctx, codedIdx, curPackage)
		}
		return nil, fmt.Errorf("VALUETYPE/CLASS without CodedIndex")
	case flags.ElementType_SZARRAY:
		// Single-dimension array with zero lower bound
		if arrayType, ok := sigType.Value.(winmd.SigType); ok {
			elemType, err := g.sigTypeToGenParamType(ctx, arrayType, curPackage)
			if err != nil {
				return nil, err
			}
			elemType.IsArray = true
			return elemType, nil
		}
		return nil, fmt.Errorf("SZARRAY without element type")
	case flags.ElementType_GENERICINST:
		// Generic instantiation
		if genInst, ok := sigType.Value.(winmd.SigGenericInst); ok {
			return g.resolveGenericInst(ctx, genInst, curPackage)
		}
		return nil, fmt.Errorf("GENERICINST without SigGenericInst")
	case flags.ElementType_VAR, flags.ElementType_MVAR:
		// Generic type parameter (VAR for type, MVAR for method)
		// Treat as object/interface{} since we don't track generic parameters
		return &genParamType{
			namespace:    "",
			name:         "interface{}",
			IsPointer:    false,
			IsPrimitive:  false,
			IsArray:      false,
			defaultValue: genDefaultValue{"nil", true},
		}, nil
	default:
		return nil, fmt.Errorf("unsupported element type: 0x%x", sigType.Kind)
	}
}

// resolveCodedIndexType resolves a CodedIndex to a type
func (g *generator) resolveCodedIndexType(ctx *winmd.Metadata, codedIdx winmd.CodedIndex, curPackage string) (*genParamType, error) {
	// TypeDefOrRefOrSpec coded index: Tag 0=TypeDef, 1=TypeRef, 2=TypeSpec
	switch codedIdx.Tag {
	case 0: // TypeDef
		typeDef, err := ctx.Tables.TypeDef.Record(codedIdx.Index)
		if err != nil {
			return nil, err
		}
		return &genParamType{
			namespace:   typeDef.Namespace.String(),
			name:        typeDef.Name.String(),
			IsPointer:   false,
			IsPrimitive: false,
			IsArray:     false,
		}, nil
	case 1: // TypeRef
		typeRef, err := ctx.Tables.TypeRef.Record(codedIdx.Index)
		if err != nil {
			return nil, err
		}
		return &genParamType{
			namespace:   typeRef.Namespace.String(),
			name:        typeRef.Name.String(),
			IsPointer:   false,
			IsPrimitive: false,
			IsArray:     false,
		}, nil
	case 2: // TypeSpec
		// TypeSpec is more complex, skip for now
		return nil, fmt.Errorf("TypeSpec not yet supported")
	default:
		return nil, fmt.Errorf("unknown coded index tag: %d", codedIdx.Tag)
	}
}

// resolveGenericInst resolves a generic instantiation
func (g *generator) resolveGenericInst(ctx *winmd.Metadata, genInst winmd.SigGenericInst, curPackage string) (*genParamType, error) {
	// Resolve the base generic type
	baseType, err := g.resolveCodedIndexType(ctx, genInst.Index, curPackage)
	if err != nil {
		return nil, err
	}
	
	// For now, just return the base type without generic parameters
	// Full generic support would require tracking the type parameters
	return baseType, nil
}
