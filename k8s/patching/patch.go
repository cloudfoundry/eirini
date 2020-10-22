package patching

import (
	"encoding/json"
	"strings"
)

type jsonPatchString struct {
	Op    string `json:"op"`
	Path  string `json:"path"`
	Value string `json:"value"`
}

type Patch struct {
	name  string
	value string
	path  string
}

func NewLabel(name, value string) Patch {
	return Patch{
		name:  name,
		value: value,
		path:  "/metadata/labels/",
	}
}

func NewAnnotation(name, value string) Patch {
	return Patch{
		name:  name,
		value: value,
		path:  "/metadata/annotations/",
	}
}

func (p Patch) GetJSONPatchBytes() []byte {
	list := []jsonPatchString{
		{
			Op:    "add",
			Path:  p.path + sanitize(p.name),
			Value: p.value,
		},
	}

	bytes, _ := json.Marshal(list)

	return bytes
}

// sanitize replaces ~ and / in labels as documented in
// http://jsonpatch.com/#json-pointer
func sanitize(name string) string {
	noTilde := strings.ReplaceAll(name, "~", "~0")
	noSlash := strings.ReplaceAll(noTilde, "/", "~1")

	return noSlash
}
