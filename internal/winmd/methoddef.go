package winmd

import (
	"github.com/microsoft/go-winmd"
)

// GetMethodOverloadName finds and returns the overload attribute for the given method
func GetMethodOverloadName(metadata *winmd.Metadata, methodDef *winmd.MethodDef) string {
	methodName, _ := metadata.Strings.String(methodDef.Name.Start)
	
	// Iterate through CustomAttribute table
	for i := winmd.Index(1); i <= winmd.Index(metadata.Tables.CustomAttribute.Len); i++ {
		cAttr, err := metadata.Tables.CustomAttribute.Record(i)
		if err != nil {
			continue
		}

		// Check if the parent is a MethodDef (tag 0 in HasCustomAttribute)
		if cAttr.Parent.Tag != 0 {
			continue
		}

		// Get the parent MethodDef
		parentMethodDef, err := metadata.Tables.MethodDef.Record(cAttr.Parent.Index)
		if err != nil {
			continue
		}

		// Check if it's the method we're looking for
		parentMethodName, _ := metadata.Strings.String(parentMethodDef.Name.Start)
		if parentMethodName.String() != methodName.String() {
			continue
		}

		// Check if the Type is OverloadAttribute (should be a MemberRef)
		if cAttr.Type.Tag != 2 { // 2 is MemberRef in CustomAttributeType
			continue
		}

		memberRef, err := metadata.Tables.MemberRef.Record(cAttr.Type.Index)
		if err != nil {
			continue
		}

		// Get the class of the MemberRef (should be a TypeRef)
		if memberRef.Class.Tag != 1 { // 1 is TypeRef in MemberRefParent
			continue
		}

		typeRef, err := metadata.Tables.TypeRef.Record(memberRef.Class.Index)
		if err != nil {
			continue
		}

		typeRefNs, _ := metadata.Strings.String(typeRef.Namespace.Start)
		typeRefName, _ := metadata.Strings.String(typeRef.Name.Start)
		if typeRefNs.String()+"."+typeRefName.String() == AttributeTypeOverloadAttribute {
			// Metadata values start with 0x01 0x00 and ends with 0x00 0x00
			if len(cAttr.Value) > 3 {
				mdVal := cAttr.Value[2 : len(cAttr.Value)-2]
				// the next value is the length of the string
				if len(mdVal) > 1 {
					mdVal = mdVal[1:]
					return string(mdVal)
				}
			}
		}
	}
	return methodName.String()
}

