package k8s

import (
	"code.cloudfoundry.org/eirini/opi"
	"k8s.io/client-go/kubernetes"
)

type Desirer struct {
	Client          *kubernetes.Clientset
	ingressManager  IngressManager
	instanceManager InstanceManager
	serviceManager  ServiceManager
}

type InstanceOptionFunc func(string, kubernetes.Interface) InstanceManager

//go:generate counterfeiter . InstanceManager
type InstanceManager interface {
	List() ([]*opi.LRP, error)
	Exists(name string) (bool, error)
	Get(name string) (*opi.LRP, error)
	Create(lrp *opi.LRP) error
	Delete(name string) error
	Update(lrp *opi.LRP) error
}

func NewDesirer(kubeNamespace string, clientset kubernetes.Interface, option InstanceOptionFunc) *Desirer {
	return &Desirer{
		instanceManager: NewInstanceManager(clientset, kubeNamespace, option),
		ingressManager:  NewIngressManager(clientset, kubeNamespace),
		serviceManager:  NewServiceManager(clientset, kubeNamespace),
	}
}

func NewTestDesirer(
	instanceManager InstanceManager,
	ingressManager IngressManager,
	serviceManager ServiceManager,
) *Desirer {
	return &Desirer{
		instanceManager: instanceManager,
		ingressManager:  ingressManager,
		serviceManager:  serviceManager,
	}
}

func NewInstanceManager(client kubernetes.Interface, namespace string, option InstanceOptionFunc) InstanceManager {
	return option(namespace, client)
}

func UseStatefulSets(namespace string, client kubernetes.Interface) InstanceManager {
	return NewStatefulSetManager(client, namespace)
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
