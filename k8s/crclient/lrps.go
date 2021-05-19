package crclient

import (
	"context"

	eiriniv1 "code.cloudfoundry.org/eirini/pkg/apis/eirini/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type LRPs struct {
	controllerClient client.Client
}

func NewLRPs(controllerClient client.Client) *LRPs {
	return &LRPs{controllerClient: controllerClient}
}

func (t *LRPs) GetLRP(ctx context.Context, namespace, name string) (*eiriniv1.LRP, error) {
	lrp := &eiriniv1.LRP{}
	err := t.controllerClient.Get(ctx, types.NamespacedName{
		Namespace: namespace,
		Name:      name,
	}, lrp)

	return lrp, err
}

func (t *LRPs) UpdateLRPStatus(ctx context.Context, lrp *eiriniv1.LRP, newStatus eiriniv1.LRPStatus) error {
	newLRP := lrp.DeepCopy()
	newLRP.Status = newStatus

	return t.controllerClient.Status().Patch(ctx, newLRP, client.MergeFrom(lrp))
}
