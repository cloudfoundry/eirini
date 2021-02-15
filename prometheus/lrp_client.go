package prometheus

import (
	"code.cloudfoundry.org/eirini/k8s/shared"
	"code.cloudfoundry.org/eirini/opi"
)

//counterfeiter:generate . LRPClient

type LRPClient interface {
	Desire(namespace string, lrp *opi.LRP, opts ...shared.Option) error
	Get(identifier opi.LRPIdentifier) (*opi.LRP, error)
	Update(lrp *opi.LRP) error
}

type LRPClientDecorator struct {
	LRPClient
	metricsRecorder Recorder
}

func NewLRPClientDecorator(
	lrpClient LRPClient,
	metricsRecorder Recorder,

) *LRPClientDecorator {
	return &LRPClientDecorator{
		LRPClient:       lrpClient,
		metricsRecorder: metricsRecorder,
	}
}

func (d *LRPClientDecorator) Desire(namespace string, lrp *opi.LRP, opts ...shared.Option) error {
	err := d.LRPClient.Desire(namespace, lrp, opts...)
	if err == nil {
		d.metricsRecorder.Increment(LRPCreations)
	}

	return err
}
