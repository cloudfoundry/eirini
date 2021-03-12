package patching

import (
	"encoding/json"
	"fmt"
	"strings"

	"code.cloudfoundry.org/eirini/k8s/stset"
	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/api/resource"
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

type TestingAnnotationPatch struct {
	name     string
	value    string
	oldValue *string
}

func NewTestingAnnotation(name, value string, oldValue *string) TestingAnnotationPatch {
	return TestingAnnotationPatch{
		name:     name,
		value:    value,
		oldValue: oldValue,
	}
}

func (p TestingAnnotationPatch) Type() types.PatchType {
	return types.JSONPatchType
}

func (p TestingAnnotationPatch) GetPatchBytes() []byte {
	prevValue := "null"
	if p.oldValue != nil {
		prevValue = `"` + *p.oldValue + `"`
	}

	jsonPatch := fmt.Sprintf(`
	[
	  {"op":"test", "path":"/metadata/annotations/%[1]s", "value": %[2]s},
	  {"op":"replace", "path":"/metadata/annotations/%[1]s", "value": "%[3]s"}
	]`, sanitize(p.name), prevValue, p.value)

	return []byte(jsonPatch)
}

type CPURequestPatch struct {
	stSet *appsv1.StatefulSet
	value *resource.Quantity
}

func NewCPURequestPatch(stSet *appsv1.StatefulSet, value *resource.Quantity) CPURequestPatch {
	return CPURequestPatch{
		stSet: stSet,
		value: value,
	}
}

func (p CPURequestPatch) Type() types.PatchType {
	return types.JSONPatchType
}

func (p CPURequestPatch) GetPatchBytes() []byte {
	containerIdx := -1

	for i, c := range p.stSet.Spec.Template.Spec.Containers {
		if c.Name == stset.OPIContainerName {
			containerIdx = i

			break
		}
	}

	if containerIdx == -1 {
		return []byte("[]")
	}

	jsonPatch := fmt.Sprintf(`
	[
	  {"op": "replace", "path": "/spec/template/spec/containers/%d/resources/requests/cpu", "value": "%dm"}
	]`, containerIdx, p.value.MilliValue())

	return []byte(jsonPatch)
}

// sanitize replaces ~ and / in labels as documented in
// http://jsonpatch.com/#json-pointer
func sanitize(name string) string {
	noTilde := strings.ReplaceAll(name, "~", "~0")
	noSlash := strings.ReplaceAll(noTilde, "/", "~1")

	return noSlash
}
