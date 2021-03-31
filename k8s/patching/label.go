package patching

import (
	"encoding/json"

	"k8s.io/apimachinery/pkg/types"
)

type LabelPatch struct {
	name  string
	value string
}

func NewLabel(name, value string) LabelPatch {
	return LabelPatch{
		name:  name,
		value: value,
	}
}

func (p LabelPatch) Type() types.PatchType {
	return types.MergePatchType
}

func (p LabelPatch) GetPatchBytes() []byte {
	m := map[string]interface{}{
		"metadata": map[string]interface{}{
			"labels": map[string]string{
				p.name: p.value,
			},
		},
	}

	bytes, _ := json.Marshal(m)

	return bytes
}
