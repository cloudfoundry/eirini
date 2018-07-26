package k8s

import (
	"code.cloudfoundry.org/eirini/opi"
	"k8s.io/client-go/kubernetes"
)

type Desirer struct {
	KubeNamespace   string
	Client          *kubernetes.Clientset
	ingressManager  IngressManager
	instanceManager InstanceManager
	serviceManager  ServiceManager
}

//go:generate counterfeiter . InstanceManager
type InstanceManager interface {
	List() ([]*opi.LRP, error)
	Exists(name string) (bool, error)
	Get(name string) (*opi.LRP, error)
	Create(lrp *opi.LRP) error
	Delete(name string) error
	Update(lrp *opi.LRP) error
}

func NewDesirer(kubeNamespace string, instanceManager InstanceManager, ingressManager IngressManager, serviceManager ServiceManager) *Desirer {

	return &Desirer{
		KubeNamespace:   kubeNamespace,
		instanceManager: instanceManager,
		ingressManager:  ingressManager,
		serviceManager:  serviceManager,
	}
}

func (d *Desirer) Desire(lrp *opi.LRP) error {
	exists, err := d.instanceManager.Exists(lrp.Name)
	if err != nil || exists {
		return err
	}

	if err := d.instanceManager.Create(lrp); err != nil {
		// fixme: this should be a multi-error and deferred
		return err
	}

	if err := d.serviceManager.Create(lrp); err != nil {
		return err
	}

	return d.ingressManager.Update(lrp)
}

func (d *Desirer) List() ([]*opi.LRP, error) {
	return d.instanceManager.List()
}

func (d *Desirer) Get(name string) (*opi.LRP, error) {
	return d.instanceManager.Get(name)
}

func (d *Desirer) Update(lrp *opi.LRP) error {
	return d.instanceManager.Update(lrp)
}

func (d *Desirer) Stop(name string) error {
	if err := d.instanceManager.Delete(name); err != nil {
		return err
	}

	if err := d.ingressManager.Delete(name); err != nil {
		return err
	}

	return d.serviceManager.Delete(name)
}
