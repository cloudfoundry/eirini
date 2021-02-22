package patching

import (
	"encoding/json"
)

type LabelPatch struct {
	name            string
	value           string
	resourceVersion string
}

func NewLabel(name, value, resourceVersion string) LabelPatch {
	return LabelPatch{
		name:            name,
		value:           value,
		resourceVersion: resourceVersion,
	}
}

func (p LabelPatch) GetPatchBytes() []byte {
	m := map[string]interface{}{
		"metadata": map[string]interface{}{
			"labels": map[string]string{
				p.name: p.value,
			},
			"resourceVersion": p.resourceVersion,
		},
	}

	bytes, _ := json.Marshal(m)

	return bytes
}

type AnnotationPatch struct {
	name            string
	value           string
	resourceVersion string
}

func NewAnnotation(name, value, resourceVersion string) AnnotationPatch {
	return AnnotationPatch{
		name:            name,
		value:           value,
		resourceVersion: resourceVersion,
	}
}

func (p AnnotationPatch) GetPatchBytes() []byte {
	m := map[string]interface{}{
		"metadata": map[string]interface{}{
			"annotations": map[string]string{
				p.name: p.value,
			},
			"resourceVersion": p.resourceVersion,
		},
	}

	bytes, _ := json.Marshal(m)

	return bytes
}
