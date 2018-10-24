package k8s

import (
	"code.cloudfoundry.org/eirini/opi"
)

type KubeIngressManagerNoOp struct {
}

func NewNoOpIngressManager() IngressManager {
	return &KubeIngressManagerNoOp{}
}

func (i *KubeIngressManagerNoOp) Delete(lrpName string) error {
	return nil
}

func (i *KubeIngressManagerNoOp) Update(lrp *opi.LRP) error {
	return nil
}
