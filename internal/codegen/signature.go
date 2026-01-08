package codegen

import (
	"fmt"

	"github.com/microsoft/go-winmd"
	"github.com/microsoft/go-winmd/flags"
)

// elementTypeFromSig converts a SigType to a genParamType
func (g *generator) elementTypeFromSig(metadata *winmd.Metadata, sigType winmd.SigType) (*genParamType, error) {
	switch sigType.Kind {
	case flags.ElementType_BOOLEAN:
		return &genParamType{
			namespace:    "",
			name:         "bool",
			IsPointer:    false,
			IsPrimitive:  true,
			IsArray:      false,
			defaultValue: g.elementDefaultValueFromSig(sigType),
		}, nil
	case flags.ElementType_CHAR:
		return &genParamType{
			namespace:    "",
			name:         "byte",
			IsPointer:    false,
			IsPrimitive:  true,
			IsArray:      false,
			defaultValue: g.elementDefaultValueFromSig(sigType),
		}, nil
	case flags.ElementType_I1:
		return &genParamType{
			namespace:    "",
			name:         "int8",
			IsPointer:    false,
			IsPrimitive:  true,
			IsArray:      false,
			defaultValue: g.elementDefaultValueFromSig(sigType),
		}, nil
	case flags.ElementType_U1:
		return &genParamType{
			namespace:    "",
			name:         "uint8",
			IsPointer:    false,
			IsPrimitive:  true,
			IsArray:      false,
			defaultValue: g.elementDefaultValueFromSig(sigType),
		}, nil
	case flags.ElementType_I2:
		return &genParamType{
			namespace:    "",
			name:         "int16",
			IsPointer:    false,
			IsPrimitive:  true,
			IsArray:      false,
			defaultValue: g.elementDefaultValueFromSig(sigType),
		}, nil
	case flags.ElementType_U2:
		return &genParamType{
			namespace:    "",
			name:         "uint16",
			IsPointer:    false,
			IsPrimitive:  true,
			IsArray:      false,
			defaultValue: g.elementDefaultValueFromSig(sigType),
		}, nil
	case flags.ElementType_I4:
		return &genParamType{
			namespace:    "",
			name:         "int32",
			IsPointer:    false,
			IsPrimitive:  true,
			IsArray:      false,
			defaultValue: g.elementDefaultValueFromSig(sigType),
		}, nil
	case flags.ElementType_U4:
		return &genParamType{
			namespace:    "",
			name:         "uint32",
			IsPointer:    false,
			IsPrimitive:  true,
			IsArray:      false,
			defaultValue: g.elementDefaultValueFromSig(sigType),
		}, nil
	case flags.ElementType_I8:
		return &genParamType{
			namespace:    "",
			name:         "int64",
			IsPointer:    false,
			IsPrimitive:  true,
			IsArray:      false,
			defaultValue: g.elementDefaultValueFromSig(sigType),
		}, nil
	case flags.ElementType_U8:
		return &genParamType{
			namespace:    "",
			name:         "uint64",
			IsPointer:    false,
			IsPrimitive:  true,
			IsArray:      false,
			defaultValue: g.elementDefaultValueFromSig(sigType),
		}, nil
	case flags.ElementType_R4:
		return &genParamType{
			namespace:    "",
			name:         "float32",
			IsPointer:    false,
			IsPrimitive:  true,
			IsArray:      false,
			defaultValue: g.elementDefaultValueFromSig(sigType),
		}, nil
	case flags.ElementType_R8:
		return &genParamType{
			namespace:    "",
			name:         "float64",
			IsPointer:    false,
			IsPrimitive:  true,
			IsArray:      false,
			defaultValue: g.elementDefaultValueFromSig(sigType),
		}, nil
	case flags.ElementType_STRING:
		return &genParamType{
			namespace:    "",
			name:         "string",
			IsPointer:    false,
			IsPrimitive:  true,
			IsArray:      false,
			defaultValue: g.elementDefaultValueFromSig(sigType),
		}, nil
	case flags.ElementType_GENERICINST:
		// CRITICAL: go-winmd doesn't support generics - return unsafe.Pointer
		// This is the key requirement from the problem statement
		return &genParamType{
			namespace:    "unsafe",
			name:         "Pointer",
			IsGeneric:    true,
			IsPointer:    false,
			IsPrimitive:  false,
			IsArray:      false,
			defaultValue: genDefaultValue{"nil", true},
		}, nil
	case flags.ElementType_CLASS:
		// Get the class type from the Value field
		if sigType.Value == nil {
			return nil, fmt.Errorf("CLASS type with no value")
		}
		
		// Value should be a CodedIndex for TypeDefOrRef
		codedIdx, ok := sigType.Value.(winmd.CodedIndex)
		if !ok {
			return nil, fmt.Errorf("CLASS type value is not a CodedIndex")
		}

		namespace, name, err := resolveTypeDefOrRef(metadata, codedIdx)
		if err != nil {
			return nil, err
		}
		
		return &genParamType{
			namespace:    namespace,
			name:         name,
			IsPointer:    true,
			IsPrimitive:  false,
			IsArray:      false,
			defaultValue: g.elementDefaultValueFromSig(sigType),
		}, nil
	case flags.ElementType_VALUETYPE:
		// Get the valuetype from the Value field
		if sigType.Value == nil {
			return nil, fmt.Errorf("VALUETYPE with no value")
		}
		
		codedIdx, ok := sigType.Value.(winmd.CodedIndex)
		if !ok {
			return nil, fmt.Errorf("VALUETYPE value is not a CodedIndex")
		}

		namespace, name, err := resolveTypeDefOrRef(metadata, codedIdx)
		if err != nil {
			return nil, err
		}

		// Check for system types
		if t, ok := isSystemType(namespace, name); ok {
			return t, nil
		}

		// TODO: Check if it's an enum and get the underlying type
		// For now, treat as a regular struct
		return &genParamType{
			namespace:    namespace,
			name:         name,
			IsPointer:    false,
			IsPrimitive:  false,
			IsArray:      false,
			defaultValue: g.elementDefaultValueFromSig(sigType),
		}, nil
	case flags.ElementType_VAR:
		// Generic types are not fully supported yet,
		// so we will just pass the raw unsafe.Pointer up to the user.
		return &genParamType{
			namespace:    "unsafe",
			name:         "Pointer",
			IsGeneric:    true,
			IsPointer:    false,
			IsPrimitive:  false,
			IsArray:      false,
			defaultValue: genDefaultValue{"nil", true},
		}, nil
	case flags.ElementType_SZARRAY:
		// A single-dimensional, zero lower-bound array type modifier
		if sigType.Value == nil {
			return nil, fmt.Errorf("SZARRAY with no element type")
		}

		elemType, ok := sigType.Value.(*winmd.SigType)
		if !ok {
			return nil, fmt.Errorf("SZARRAY value is not a SigType")
		}

		param, err := g.elementTypeFromSig(metadata, *elemType)
		if err != nil {
			return nil, err
		}

		param.IsArray = true
		// override default val
		param.defaultValue = genDefaultValue{"nil", true}

		return param, nil
	case flags.ElementType_OBJECT:
		// This represents System.Object, so just use a pointer
		return &genParamType{
			namespace:    "unsafe",
			name:         "Pointer",
			IsPointer:    false,
			IsPrimitive:  false,
			IsArray:      false,
			defaultValue: genDefaultValue{"nil", true},
		}, nil
	default:
		return nil, fmt.Errorf("unsupported element type: %v", sigType.Kind)
	}
}

// resolveTypeDefOrRef resolves a TypeDefOrRef coded index to namespace and name
func resolveTypeDefOrRef(metadata *winmd.Metadata, codedIdx winmd.CodedIndex) (string, string, error) {
	switch codedIdx.Tag {
	case 0: // TypeDef
		typeDef, err := metadata.Tables.TypeDef.Record(codedIdx.Index)
		if err != nil {
			return "", "", err
		}
		ns, _ := metadata.Strings.String(typeDef.Namespace.Start)
		name, _ := metadata.Strings.String(typeDef.Name.Start)
		return ns.String(), name.String(), nil
	case 1: // TypeRef
		typeRef, err := metadata.Tables.TypeRef.Record(codedIdx.Index)
		if err != nil {
			return "", "", err
		}
		ns, _ := metadata.Strings.String(typeRef.Namespace.Start)
		name, _ := metadata.Strings.String(typeRef.Name.Start)
		return ns.String(), name.String(), nil
	case 2: // TypeSpec
		// TypeSpec is used for generic instantiations - return unsafe.Pointer
		return "unsafe", "Pointer", nil
	default:
		return "", "", fmt.Errorf("unknown TypeDefOrRef tag: %d", codedIdx.Tag)
	}
}

// elementDefaultValueFromSig returns the default value for a SigType
func (g *generator) elementDefaultValueFromSig(sigType winmd.SigType) genDefaultValue {
	switch sigType.Kind {
	case flags.ElementType_BOOLEAN:
		return genDefaultValue{"false", true}
	case flags.ElementType_CHAR,
		flags.ElementType_I1, flags.ElementType_U1,
		flags.ElementType_I2, flags.ElementType_U2,
		flags.ElementType_I4, flags.ElementType_U4,
		flags.ElementType_I8, flags.ElementType_U8:
		return genDefaultValue{"0", true}
	case flags.ElementType_R4, flags.ElementType_R8:
		return genDefaultValue{"0.0", true}
	case flags.ElementType_STRING:
		return genDefaultValue{`""`, true}
	case flags.ElementType_CLASS,
		flags.ElementType_GENERICINST, flags.ElementType_SZARRAY:
		return genDefaultValue{"nil", true}
	case flags.ElementType_VALUETYPE:
		// For value types, we need to return the zero value
		// This would require looking up the type, so for now return empty struct
		return genDefaultValue{"{}", false}
	case flags.ElementType_VAR:
		return genDefaultValue{"nil", true}
	case flags.ElementType_OBJECT:
		return genDefaultValue{"nil", true}
	default:
		return genDefaultValue{"nil", true}
	}
}
