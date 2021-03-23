package migrations

import (
	"context"
	"encoding/json"
	"fmt"

	"code.cloudfoundry.org/eirini/k8s/stset"
	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/runtime"
)

//counterfeiter:generate . CPURequestSetter

type CPURequestSetter interface {
	SetCPURequest(ctx context.Context, stSet *appsv1.StatefulSet, cpuRequest *resource.Quantity) (*appsv1.StatefulSet, error)
}

func NewAdjustCPURequest(cpuRequestSetter CPURequestSetter) AdjustCPURequest {
	return AdjustCPURequest{
		cpuRequestSetter: cpuRequestSetter,
	}
}

type AdjustCPURequest struct {
	cpuRequestSetter CPURequestSetter
}

func (m AdjustCPURequest) SequenceID() int {
	return 1
}

type originalRequest struct {
	CPUWeight int64 `json:"cpu_weight"`
}

func (m AdjustCPURequest) Apply(ctx context.Context, o runtime.Object) error {
	stSet, ok := o.(*appsv1.StatefulSet)
	if !ok {
		return fmt.Errorf("expected *v1.StatefulSet, got: %T", o)
	}

	reqJSON := stSet.Annotations[stset.AnnotationOriginalRequest]

	var req originalRequest

	err := json.Unmarshal([]byte(reqJSON), &req)
	if err != nil {
		return fmt.Errorf("%q is not valid json: %w", reqJSON, err)
	}

	_, err = m.cpuRequestSetter.SetCPURequest(ctx, stSet, resource.NewMilliQuantity(req.CPUWeight, resource.DecimalSI))
	if err != nil {
		return fmt.Errorf("failed to set cpu request: %w", err)
	}

	return nil
}
