package codegen

import (
	"fmt"

	"github.com/microsoft/go-winmd"
	"github.com/microsoft/go-winmd/flags"
)

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
