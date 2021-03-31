package patching

import (
	"encoding/json"

	"k8s.io/apimachinery/pkg/types"
)

type AnnotationPatch struct {
	name  string
	value string
}

func NewAnnotation(name, value string) AnnotationPatch {
	return AnnotationPatch{
		name:  name,
		value: value,
	}
}

func (p AnnotationPatch) Type() types.PatchType {
	return types.MergePatchType
}

func (p AnnotationPatch) GetPatchBytes() []byte {
	m := map[string]interface{}{
		"metadata": map[string]interface{}{
			"annotations": map[string]string{
				p.name: p.value,
			},
		},
	}

	bytes, _ := json.Marshal(m)

	return bytes
}
