package winmd

import (
	"fmt"
	"strconv"

	"github.com/go-kit/log"
	"github.com/go-kit/log/level"
	"github.com/microsoft/go-winmd"
	"github.com/microsoft/go-winmd/flags"
)

// TypeDef is a helper struct that wraps winmd.TypeDef and stores the original metadata
// of the typeDef.
type TypeDef struct {
	*winmd.TypeDef
	HasMetadata

	logger log.Logger
}

// TypeNamespace returns the namespace of the type
func (td *TypeDef) TypeNamespace() string {
	ns, err := td.Metadata().Strings.String(td.Namespace.Start)
	if err != nil {
		return ""
	}
	return ns.String()
}

// TypeName returns the name of the type
func (td *TypeDef) TypeName() string {
	name, err := td.Metadata().Strings.String(td.Name.Start)
	if err != nil {
		return ""
	}
	return name.String()
}

// QualifiedID holds the namespace and the name of a qualified element. This may be a type, a static function or a field
type QualifiedID struct {
	Namespace string
	Name      string
}

// GetValueForEnumField returns the value of the requested enum field.
func (typeDef *TypeDef) GetValueForEnumField(fieldIndex winmd.Index) (string, error) {
	// For each Enum value definition, there is a corresponding row in the Constant table to store the integer value for the enum value.
	for i := winmd.Index(1); i <= winmd.Index(typeDef.Metadata().Tables.Constant.Len); i++ {
		constant, err := typeDef.Metadata().Tables.Constant.Record(i)
		if err != nil {
			continue
		}

		// Check if this constant belongs to a Field
		if constant.Parent.Tag != 0 { // 0 is Field in HasConstant coded index
			continue
		}

		// Check if it's the field we're looking for
		if constant.Parent.Index != fieldIndex {
			continue
		}

		// The value is a blob that we need to read as little endian
		valueBytes, err := typeDef.Metadata().Blob.Bytes(uint32(constant.Type))
		if err != nil {
			continue
		}

		// Read as little endian uint32
		var blobIndex uint32
		for i, b := range valueBytes {
			if i >= 4 {
				break
			}
			blobIndex += uint32(b) << (i * 8)
		}
		return strconv.Itoa(int(blobIndex)), nil
	}

	return "", fmt.Errorf("no value found for field %d", fieldIndex)
}

// GetAttributeWithType returns the value of the given attribute type and fails if not found.
func (typeDef *TypeDef) GetAttributeWithType(lookupAttrTypeClass string) ([]byte, error) {
	result := typeDef.GetTypeDefAttributesWithType(lookupAttrTypeClass)
	if len(result) == 0 {
		return nil, fmt.Errorf("type %s has no custom attribute %s", typeDef.TypeNamespace()+"."+typeDef.TypeName(), lookupAttrTypeClass)
	} else if len(result) > 1 {
		_ = level.Warn(typeDef.logger).Log(
			"msg", "type has multiple custom attributes, returning the first one",
			"type", typeDef.TypeNamespace()+"."+typeDef.TypeName(),
			"attr", lookupAttrTypeClass,
		)
	}

	return result[0], nil
}

// GetTypeDefAttributesWithType returns the values of all the attributes that match the given type.
func (typeDef *TypeDef) GetTypeDefAttributesWithType(lookupAttrTypeClass string) [][]byte {
	result := make([][]byte, 0)
	
	// Iterate through CustomAttribute table
	for i := winmd.Index(1); i <= winmd.Index(typeDef.Metadata().Tables.CustomAttribute.Len); i++ {
		cAttr, err := typeDef.Metadata().Tables.CustomAttribute.Record(i)
		if err != nil {
			continue
		}

		// Check if the parent is a TypeDef (tag 3 in HasCustomAttribute)
		if cAttr.Parent.Tag != 3 {
			continue
		}

		// Check if it's the TypeDef we're looking for
		parentTypeDef, err := typeDef.Metadata().Tables.TypeDef.Record(cAttr.Parent.Index)
		if err != nil {
			continue
		}

		parentNs, _ := typeDef.Metadata().Strings.String(parentTypeDef.Namespace.Start)
		parentName, _ := typeDef.Metadata().Strings.String(parentTypeDef.Name.Start)
		if parentNs.String()+"."+parentName.String() != typeDef.TypeNamespace()+"."+typeDef.TypeName() {
			continue
		}

		// Check if the Type matches what we're looking for
		// Type is a MemberRef or MethodDef (CustomAttributeType coded index)
		if cAttr.Type.Tag != 2 { // 2 is MemberRef
			continue
		}

		memberRef, err := typeDef.Metadata().Tables.MemberRef.Record(cAttr.Type.Index)
		if err != nil {
			continue
		}

		// Get the class of the MemberRef (should be a TypeRef)
		if memberRef.Class.Tag != 1 { // 1 is TypeRef in MemberRefParent
			continue
		}

		typeRef, err := typeDef.Metadata().Tables.TypeRef.Record(memberRef.Class.Index)
		if err != nil {
			continue
		}

		typeRefNs, _ := typeDef.Metadata().Strings.String(typeRef.Namespace.Start)
		typeRefName, _ := typeDef.Metadata().Strings.String(typeRef.Name.Start)
		if typeRefNs.String()+"."+typeRefName.String() == lookupAttrTypeClass {
			result = append(result, cAttr.Value)
		}
	}

	return result
}

// GetImplementedInterfaces returns the interfaces implemented by the type.
func (typeDef *TypeDef) GetImplementedInterfaces() ([]QualifiedID, error) {
	interfaces := make([]QualifiedID, 0)

	// Iterate through InterfaceImpl table
	for i := winmd.Index(1); i <= winmd.Index(typeDef.Metadata().Tables.InterfaceImpl.Len); i++ {
		interfaceImpl, err := typeDef.Metadata().Tables.InterfaceImpl.Record(i)
		if err != nil {
			continue
		}

		// Check if this InterfaceImpl belongs to our TypeDef
		classTd, err := typeDef.Metadata().Tables.TypeDef.Record(interfaceImpl.Class)
		if err != nil {
			continue
		}

		classNs, _ := typeDef.Metadata().Strings.String(classTd.Namespace.Start)
		className, _ := typeDef.Metadata().Strings.String(classTd.Name.Start)
		if classNs.String()+"."+className.String() != typeDef.TypeNamespace()+"."+typeDef.TypeName() {
			continue
		}

		// Get the interface
		// Interface is a TypeDefOrRef coded index
		var ifaceNs, ifaceName string
		switch interfaceImpl.Interface.Tag {
		case 0: // TypeDef
			ifaceTd, err := typeDef.Metadata().Tables.TypeDef.Record(interfaceImpl.Interface.Index)
			if err != nil {
				continue
			}
			ns, _ := typeDef.Metadata().Strings.String(ifaceTd.Namespace.Start)
			name, _ := typeDef.Metadata().Strings.String(ifaceTd.Name.Start)
			ifaceNs = ns.String()
			ifaceName = name.String()
		case 1: // TypeRef
			ifaceTr, err := typeDef.Metadata().Tables.TypeRef.Record(interfaceImpl.Interface.Index)
			if err != nil {
				continue
			}
			ns, _ := typeDef.Metadata().Strings.String(ifaceTr.Namespace.Start)
			name, _ := typeDef.Metadata().Strings.String(ifaceTr.Name.Start)
			ifaceNs = ns.String()
			ifaceName = name.String()
		case 2: // TypeSpec
			// Skip TypeSpec for now (generic instantiations)
			continue
		}

		interfaces = append(interfaces, QualifiedID{
			Namespace: ifaceNs,
			Name:      ifaceName,
		})
	}

	return interfaces, nil
}

// Extends checks if the type extends the given class.
func (typeDef *TypeDef) Extends(class string) (bool, error) {
	// Check the Extends field
	if typeDef.TypeDef.Extends.Tag == 0 { // No parent
		return false, nil
	}

	var parentNs, parentName string
	switch typeDef.TypeDef.Extends.Tag {
	case 0: // TypeDef
		parentTd, err := typeDef.Metadata().Tables.TypeDef.Record(typeDef.TypeDef.Extends.Index)
		if err != nil {
			return false, err
		}
		ns, _ := typeDef.Metadata().Strings.String(parentTd.Namespace.Start)
		name, _ := typeDef.Metadata().Strings.String(parentTd.Name.Start)
		parentNs = ns.String()
		parentName = name.String()
	case 1: // TypeRef
		parentTr, err := typeDef.Metadata().Tables.TypeRef.Record(typeDef.TypeDef.Extends.Index)
		if err != nil {
			return false, err
		}
		ns, _ := typeDef.Metadata().Strings.String(parentTr.Namespace.Start)
		name, _ := typeDef.Metadata().Strings.String(parentTr.Name.Start)
		parentNs = ns.String()
		parentName = name.String()
	default:
		return false, nil
	}

	return parentNs+"."+parentName == class, nil
}

// GetGenericParams returns the generic parameters of the type.
// Note: Returning empty list for now as generics handling is complex
func (typeDef *TypeDef) GetGenericParams() ([]interface{}, error) {
	// TODO: Implement using microsoft/go-winmd GenericParam table
	return []interface{}{}, nil
}

// IsInterface returns true if the type is an interface
func (typeDef *TypeDef) IsInterface() bool {
	return typeDef.Flags&flags.TypeAttributes_Interface != 0
}

// IsEnum returns true if the type is an enum
func (typeDef *TypeDef) IsEnum() bool {
	ok, err := typeDef.Extends("System.Enum")
	if err != nil {
		_ = level.Error(typeDef.logger).Log("msg", "error resolving type extends, all classes should extend at least System.Object", "err", err)
		return false
	}
	return ok
}

// IsDelegate returns true if the type is a delegate
func (typeDef *TypeDef) IsDelegate() bool {
	// Check flags: must be public and sealed
	if typeDef.Flags&flags.TypeAttributes_Public == 0 || typeDef.Flags&flags.TypeAttributes_Sealed == 0 {
		return false
	}

	ok, err := typeDef.Extends("System.MulticastDelegate")
	if err != nil {
		_ = level.Error(typeDef.logger).Log("msg", "error resolving type extends, all classes should extend at least System.Object", "err", err)
		return false
	}

	return ok
}

// IsStruct returns true if the type is a struct
func (typeDef *TypeDef) IsStruct() bool {
	ok, err := typeDef.Extends("System.ValueType")
	if err != nil {
		_ = level.Error(typeDef.logger).Log("msg", "error resolving type extends, all classes should extend at least System.Object", "err", err)
		return false
	}
	return ok
}

// IsRuntimeClass returns true if the type is a runtime class
func (typeDef *TypeDef) IsRuntimeClass() bool {
	// Flags: all runtime classes must carry the public, auto layout, class, and tdWindowsRuntime flags.
	return typeDef.Flags&flags.TypeAttributes_Public != 0 && 
		typeDef.Flags&flags.TypeAttributes_AutoLayout != 0 && 
		typeDef.Flags&flags.TypeAttributes_Interface == 0 && 
		typeDef.Flags&0x4000 != 0
}

// GUID returns the GUID of the type.
func (typeDef *TypeDef) GUID() (string, error) {
	blob, err := typeDef.GetAttributeWithType(AttributeTypeGUID)
	if err != nil {
		return "", err
	}
	return guidBlobToString(blob)
}

// ResolveMethodList returns the methods defined in this type.
func (typeDef *TypeDef) ResolveMethodList() ([]*winmd.MethodDef, error) {
	methods := make([]*winmd.MethodDef, 0)
	for i := typeDef.MethodList.Start; i < typeDef.MethodList.End; i++ {
		method, err := typeDef.Metadata().Tables.MethodDef.Record(i)
		if err != nil {
			return nil, err
		}
		methods = append(methods, method)
	}
	return methods, nil
}

// ResolveFieldList returns the fields defined in this type.
func (typeDef *TypeDef) ResolveFieldList() ([]*winmd.Field, error) {
	fields := make([]*winmd.Field, 0)
	for i := typeDef.FieldList.Start; i < typeDef.FieldList.End; i++ {
		field, err := typeDef.Metadata().Tables.Field.Record(i)
		if err != nil {
			return nil, err
		}
		fields = append(fields, field)
	}
	return fields, nil
}

func guidBlobToString(b []byte) (string, error) {
	// Custom attribute blob format: prolog (2 bytes) + guid (16 bytes)
	if len(b) < 18 {
		return "", fmt.Errorf("invalid GUID blob length: %d", len(b))
	}

	// Skip the prolog (0x01 0x00)
	guidBytes := b[2:18]

	return fmt.Sprintf("%08X-%04X-%04X-%04X-%012X",
		uint32(guidBytes[0])|uint32(guidBytes[1])<<8|uint32(guidBytes[2])<<16|uint32(guidBytes[3])<<24,
		uint16(guidBytes[4])|uint16(guidBytes[5])<<8,
		uint16(guidBytes[6])|uint16(guidBytes[7])<<8,
		uint16(guidBytes[8])|uint16(guidBytes[9])<<8,
		uint64(guidBytes[10])|uint64(guidBytes[11])<<8|uint64(guidBytes[12])<<16|uint64(guidBytes[13])<<24|
			uint64(guidBytes[14])<<32|uint64(guidBytes[15])<<40,
	), nil
}
