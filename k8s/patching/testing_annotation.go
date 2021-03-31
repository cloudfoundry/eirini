package patching

import (
	"fmt"
	"strings"

	"k8s.io/apimachinery/pkg/types"
)

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

// sanitize replaces ~ and / in labels as documented in
// http://jsonpatch.com/#json-pointer
func sanitize(name string) string {
	noTilde := strings.ReplaceAll(name, "~", "~0")
	noSlash := strings.ReplaceAll(noTilde, "/", "~1")

	return noSlash
}
