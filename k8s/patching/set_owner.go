package patching

import (
	"encoding/json"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

type SetOwnerPatch struct {
	ownerReference metav1.OwnerReference
}

func NewSetOwner(ownerReference metav1.OwnerReference) SetOwnerPatch {
	return SetOwnerPatch{
		ownerReference: ownerReference,
	}
}

func (p SetOwnerPatch) Type() types.PatchType {
	return types.MergePatchType
}

func (p SetOwnerPatch) GetPatchBytes() []byte {
	m := map[string]interface{}{
		"metadata": map[string]interface{}{
			"ownerReferences": []map[string]interface{}{
				{
					"apiVersion": p.ownerReference.APIVersion,
					"kind":       p.ownerReference.Kind,
					"name":       p.ownerReference.Name,
					"uid":        p.ownerReference.UID,
				},
			},
		},
	}

	bytes, _ := json.Marshal(m)

	return bytes
}
