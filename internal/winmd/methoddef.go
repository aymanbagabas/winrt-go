package winmd

import (
	"github.com/microsoft/go-winmd"
)

// GetMethodOverloadName finds and returns the overload attribute for the given method
func GetMethodOverloadName(ctx *winmd.Metadata, methodDef *winmd.MethodDef) string {
	cAttrTable := ctx.Tables.CustomAttribute
	for i := winmd.Index(0); i < winmd.Index(cAttrTable.Len); i++ {
		cAttr, err := cAttrTable.Record(i)
		if err != nil {
			continue
		}

		// Parent: The owner of the Attribute must be the given func
		// For HasCustomAttribute coded index, Tag 0 = MethodDef
		if cAttr.Parent.Tag != 0 {
			continue
		}

		parentMethodDef, err := ctx.Tables.MethodDef.Record(cAttr.Parent.Index)
		if err != nil {
			continue
		}

		// does the blob belong to the method we're looking for?
		// Compare name and signature
		if parentMethodDef.Name.String() != methodDef.Name.String() {
			continue
		}
		
		// Compare signatures (Signature is already a []byte, not a blob index)
		if string(parentMethodDef.Signature) != string(methodDef.Signature) {
			continue
		}

		// Type: the attribute type must be the given type
		// For CustomAttributeType coded index, Tag 3 = MemberRef
		if cAttr.Type.Tag != 3 {
			continue
		}

		attrTypeMemberRef, err := ctx.Tables.MemberRef.Record(cAttr.Type.Index)
		if err != nil {
			continue
		}

		// Check the MemberRef Class
		// For MemberRefParent coded index, Tag 1 = TypeRef
		if attrTypeMemberRef.Class.Tag != 1 {
			continue
		}

		attrTypeRef, err := ctx.Tables.TypeRef.Record(attrTypeMemberRef.Class.Index)
		if err != nil {
			continue
		}

		if attrTypeRef.Namespace.String()+"."+attrTypeRef.Name.String() == AttributeTypeOverloadAttribute {
			// cAttr.Value is already a []byte, not a blob index
			valueBlob := cAttr.Value
			
			// Metadata values start with 0x01 0x00 and ends with 0x00 0x00
			if len(valueBlob) < 4 {
				continue
			}
			mdVal := valueBlob[2 : len(valueBlob)-2]
			// the next value is the length of the string
			if len(mdVal) < 1 {
				continue
			}
			mdVal = mdVal[1:]
			return string(mdVal)
		}
	}
	return methodDef.Name.String()
}
