package patching

import (
	"fmt"

	"code.cloudfoundry.org/eirini/k8s/stset"
	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/types"
)

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
