package winmd

import (
	"fmt"
	"strconv"

	"github.com/go-kit/log"
	"github.com/go-kit/log/level"
	"github.com/microsoft/go-winmd"
)

// TypeDef is a helper struct that wraps winmd.TypeDef and stores the original context
// of the typeDef.
type TypeDef struct {
	*winmd.TypeDef
	HasContext

	logger log.Logger
}

// QualifiedID holds the namespace and the name of a qualified element. This may be a type, a static function or a field
type QualifiedID struct {
	Namespace string
	Name      string
}

// GetValueForEnumField returns the value of the requested enum field.
func (typeDef *TypeDef) GetValueForEnumField(fieldIndex uint32) (string, error) {
	// For each Enum value definition, there is a corresponding row in the Constant table to store the integer value for the enum value.
	tableConstants := typeDef.Ctx().Tables.Constant
	for i := winmd.Index(0); i < winmd.Index(tableConstants.Len); i++ {
		constant, err := tableConstants.Record(i)
		if err != nil {
			return "", err
		}

		// Check if parent is a Field and matches our field index
		// In the new API, we need to check the CodedIndex Tag to determine the table type
		// Tag values for HasConstant: Field=0, Param=1, Property=2 (from coded.go)
		if constant.Parent.Tag != 0 { // 0 = Field
			continue
		}

		// does the blob belong to the field we're looking for?
		if uint32(constant.Parent.Index) != fieldIndex {
			continue
		}

		// The value is already a blob ([]byte), we need to read as little endian
		valueBlob := constant.Value
		
		var blobIndex uint32
		for i, b := range valueBlob {
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
		return nil, fmt.Errorf("type %s has no custom attribute %s", typeDef.Namespace.String()+"."+typeDef.Name.String(), lookupAttrTypeClass)
	} else if len(result) > 1 {
		_ = level.Warn(typeDef.logger).Log(
			"msg", "type has multiple custom attributes, returning the first one",
			"type", typeDef.Namespace.String()+"."+typeDef.Name.String(),
			"attr", lookupAttrTypeClass,
		)
	}

	return result[0], nil
}

// Helper function to get CodedIndex table type
// Based on the codedHasCustomAttribute mapping in the new API
func getTableTypeFromCodedIndex(codedIndex winmd.CodedIndex) int8 {
	// For HasCustomAttribute, the table mapping is:
	// Tag 0 = MethodDef, 1 = Field, 2 = TypeRef, 3 = TypeDef, 4 = Param, etc.
	// We specifically need Tag 3 for TypeDef
	return codedIndex.Tag
}

// GetTypeDefAttributesWithType returns the values of all the attributes that match the given type.
func (typeDef *TypeDef) GetTypeDefAttributesWithType(lookupAttrTypeClass string) [][]byte {
	result := make([][]byte, 0)
	cAttrTable := typeDef.Ctx().Tables.CustomAttribute
	
	for i := winmd.Index(0); i < winmd.Index(cAttrTable.Len); i++ {
		cAttr, err := cAttrTable.Record(i)
		if err != nil {
			continue
		}

		// Parent: The owner of the Attribute must be the given typeDef (Tag 3 = TypeDef in HasCustomAttribute)
		if cAttr.Parent.Tag != 3 {
			continue
		}

		// Get the parent TypeDef
		parentTypeDef, err := typeDef.Ctx().Tables.TypeDef.Record(cAttr.Parent.Index)
		if err != nil {
			continue
		}

		// does the blob belong to the type we're looking for?
		if parentTypeDef.Namespace.String() != typeDef.Namespace.String() || 
		   parentTypeDef.Name.String() != typeDef.Name.String() {
			continue
		}

		// Type: the attribute type must be the given type
		// cAttr.Type is CustomAttributeType coded index
		// Tag values: 0,1 = none, 2 = MethodDef, 3 = MemberRef, 4 = none
		// We want MemberRef (Tag 3)
		if cAttr.Type.Tag != 3 {
			continue
		}

		attrTypeMemberRef, err := typeDef.Ctx().Tables.MemberRef.Record(cAttr.Type.Index)
		if err != nil {
			continue
		}

		// Check the MemberRef Class
		// For MemberRefParent: Tag 0 = TypeDef, 1 = TypeRef, 2 = ModuleRef, 3 = MethodDef, 4 = TypeSpec
		// We want TypeRef (Tag 1)
		if attrTypeMemberRef.Class.Tag != 1 {
			continue
		}

		attrTypeRef, err := typeDef.Ctx().Tables.TypeRef.Record(attrTypeMemberRef.Class.Index)
		if err != nil {
			continue
		}

		if attrTypeRef.Namespace.String()+"."+attrTypeRef.Name.String() == lookupAttrTypeClass {
			// cAttr.Value is already a []byte
			result = append(result, cAttr.Value)
		}
	}

	return result
}

// GetImplementedInterfaces returns the interfaces implemented by the type.
func (typeDef *TypeDef) GetImplementedInterfaces() ([]QualifiedID, error) {
	interfaces := make([]QualifiedID, 0)

	tableInterfaceImpl := typeDef.Ctx().Tables.InterfaceImpl
	for i := winmd.Index(0); i < winmd.Index(tableInterfaceImpl.Len); i++ {
		interfaceImpl, err := tableInterfaceImpl.Record(i)
		if err != nil {
			return nil, err
		}

		// Get the class TypeDef
		classTd, err := typeDef.Ctx().Tables.TypeDef.Record(interfaceImpl.Class)
		if err != nil {
			return nil, err
		}

		if classTd.Namespace.String()+"."+classTd.Name.String() != 
		   typeDef.Namespace.String()+"."+typeDef.Name.String() {
			// not the class we are looking for
			continue
		}

		// Interface is TypeDefOrRef coded index
		// Tag 0 = TypeDef, 1 = TypeRef, 2 = TypeSpec
		// Ignore TypeSpec (Tag 2)
		if interfaceImpl.Interface.Tag == 2 {
			continue
		}

		var ifaceNS, ifaceName string
		if interfaceImpl.Interface.Tag == 0 {
			// TypeDef
			iface, err := typeDef.Ctx().Tables.TypeDef.Record(interfaceImpl.Interface.Index)
			if err != nil {
				return nil, err
			}
			ifaceNS = iface.Namespace.String()
			ifaceName = iface.Name.String()
		} else if interfaceImpl.Interface.Tag == 1 {
			// TypeRef
			iface, err := typeDef.Ctx().Tables.TypeRef.Record(interfaceImpl.Interface.Index)
			if err != nil {
				return nil, err
			}
			ifaceNS = iface.Namespace.String()
			ifaceName = iface.Name.String()
		}

		interfaces = append(interfaces, QualifiedID{Namespace: ifaceNS, Name: ifaceName})
	}

	return interfaces, nil
}

// Extends returns true if the type extends the given class
func (typeDef *TypeDef) Extends(class string) (bool, error) {
	// Extends is a TypeDefOrRef coded index
	// Tag 0 = TypeDef, 1 = TypeRef, 2 = TypeSpec
	var ns, name string
	
	if typeDef.TypeDef.Extends.Tag == 0 {
		// TypeDef
		extends, err := typeDef.Ctx().Tables.TypeDef.Record(typeDef.TypeDef.Extends.Index)
		if err != nil {
			return false, err
		}
		ns = extends.Namespace.String()
		name = extends.Name.String()
	} else if typeDef.TypeDef.Extends.Tag == 1 {
		// TypeRef
		extends, err := typeDef.Ctx().Tables.TypeRef.Record(typeDef.TypeDef.Extends.Index)
		if err != nil {
			return false, err
		}
		ns = extends.Namespace.String()
		name = extends.Name.String()
	} else {
		// TypeSpec or invalid
		return false, nil
	}
	
	return ns+"."+name == class, nil
}

// GetGenericParams returns the generic parameters of the type.
func (typeDef *TypeDef) GetGenericParams() ([]*winmd.GenericParam, error) {
	params := make([]*winmd.GenericParam, 0)
	tableGenericParam := typeDef.Ctx().Tables.GenericParam
	
	for i := winmd.Index(0); i < winmd.Index(tableGenericParam.Len); i++ {
		genericParam, err := tableGenericParam.Record(i)
		if err != nil {
			continue
		}

		// Owner is TypeOrMethodDef coded index
		// Tag 0 = TypeDef, 1 = MethodDef
		// We want TypeDef (Tag 0)
		if genericParam.Owner.Tag != 0 {
			continue
		}

		ownerTypeDef, err := typeDef.Ctx().Tables.TypeDef.Record(genericParam.Owner.Index)
		if err != nil {
			continue
		}

		// does the param belong to the type we're looking for?
		if ownerTypeDef.Namespace.String() != typeDef.Namespace.String() || 
		   ownerTypeDef.Name.String() != typeDef.Name.String() {
			continue
		}

		params = append(params, genericParam)
	}
	
	if len(params) == 0 {
		return nil, fmt.Errorf("could not find generic params for type %s.%s", 
			typeDef.Namespace.String(), typeDef.Name.String())
	}

	return params, nil
}

// IsInterface returns true if the type is an interface
func (typeDef *TypeDef) IsInterface() bool {
	// Check Flags field for Interface flag
	// TypeAttributes.Interface = 0x00000020
	return typeDef.Flags&0x00000020 != 0
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
	// Check for Public and Sealed flags
	// TypeAttributes.Public = 0x00000001
	// TypeAttributes.Sealed = 0x00000100
	isPublic := typeDef.Flags&0x00000007 == 0x00000001 // Visibility mask
	isSealed := typeDef.Flags&0x00000100 != 0
	
	if !(isPublic && isSealed) {
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
	// TypeAttributes.Public = 0x00000001 (visibility mask 0x00000007)
	// TypeAttributes.AutoLayout = 0x00000000 (layout mask 0x00000018)
	// TypeAttributes.Class = 0x00000000 (not interface, so bit 0x00000020 is not set)
	// TypeAttributes.WindowsRuntime = 0x00004000
	isPublic := typeDef.Flags&0x00000007 == 0x00000001
	isAutoLayout := typeDef.Flags&0x00000018 == 0x00000000
	isClass := typeDef.Flags&0x00000020 == 0x00000000
	isWindowsRuntime := typeDef.Flags&0x00004000 != 0
	
	return isPublic && isAutoLayout && isClass && isWindowsRuntime
}

// GUID returns the GUID of the type.
func (typeDef *TypeDef) GUID() (string, error) {
	blob, err := typeDef.GetAttributeWithType(AttributeTypeGUID)
	if err != nil {
		return "", err
	}
	return guidBlobToString(blob)
}

// guidBlobToString converts an array into the textual representation of a GUID
func guidBlobToString(b []byte) (string, error) {
	// the guid is a blob of 20 bytes
	if len(b) != 20 {
		return "", fmt.Errorf("invalid GUID blob length: %d", len(b))
	}

	// that starts with 0100
	if b[0] != 0x01 || b[1] != 0x00 {
		return "", fmt.Errorf("invalid GUID blob header, expected '0x01 0x00' but found '0x%02x 0x%02x'", b[0], b[1])
	}

	// and ends with 0000
	if b[18] != 0x00 || b[19] != 0x00 {
		return "", fmt.Errorf("invalid GUID blob footer, expected '0x00 0x00' but found '0x%02x 0x%02x'", b[18], b[19])
	}

	guid := b[2 : len(b)-2]
	// the string version has 5 parts separated by '-'
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%04x%08x",
		// The first 3 are encoded as little endian
		uint32(guid[0])|uint32(guid[1])<<8|uint32(guid[2])<<16|uint32(guid[3])<<24,
		uint16(guid[4])|uint16(guid[5])<<8,
		uint16(guid[6])|uint16(guid[7])<<8,
		//the rest is not
		uint16(guid[8])<<8|uint16(guid[9]),
		uint16(guid[10])<<8|uint16(guid[11]),
		uint32(guid[12])<<24|uint32(guid[13])<<16|uint32(guid[14])<<8|uint32(guid[15])), nil
}

// ResolveMethodList resolves the MethodList and returns all methods for this TypeDef
func (typeDef *TypeDef) ResolveMethodList(ctx *winmd.Metadata) ([]*winmd.MethodDef, error) {
	if typeDef.MethodList.Start == typeDef.MethodList.End {
		return nil, nil
	}

	result := make([]*winmd.MethodDef, 0, typeDef.MethodList.End-typeDef.MethodList.Start)
	for i := typeDef.MethodList.Start; i < typeDef.MethodList.End; i++ {
		methodDef, err := ctx.Tables.MethodDef.Record(i)
		if err != nil {
			return nil, err
		}
		result = append(result, methodDef)
	}
	return result, nil
}

// ResolveFieldList resolves the FieldList and returns all fields for this TypeDef
func (typeDef *TypeDef) ResolveFieldList(ctx *winmd.Metadata) ([]*winmd.Field, error) {
	if typeDef.FieldList.Start == typeDef.FieldList.End {
		return nil, nil
	}

	result := make([]*winmd.Field, 0, typeDef.FieldList.End-typeDef.FieldList.Start)
	for i := typeDef.FieldList.Start; i < typeDef.FieldList.End; i++ {
		field, err := ctx.Tables.Field.Record(i)
		if err != nil {
			return nil, err
		}
		result = append(result, field)
	}
	return result, nil
}
