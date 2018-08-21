package k8s

import (
	"code.cloudfoundry.org/eirini/opi"
	"k8s.io/client-go/kubernetes"
)

type Desirer struct {
	IngressManager  IngressManager
	InstanceManager InstanceManager
	ServiceManager  ServiceManager
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
		InstanceManager: NewInstanceManager(clientset, kubeNamespace, option),
		IngressManager:  NewIngressManager(clientset, kubeNamespace),
		ServiceManager:  NewServiceManager(clientset, kubeNamespace),
	}
}

func NewInstanceManager(client kubernetes.Interface, namespace string, option InstanceOptionFunc) InstanceManager {
	return option(namespace, client)
}

func UseStatefulSets(namespace string, client kubernetes.Interface) InstanceManager {
	return NewStatefulSetManager(client, namespace)
}

func (d *Desirer) Desire(lrp *opi.LRP) error {
	exists, err := d.InstanceManager.Exists(lrp.Name)
	if err != nil || exists {
		return err
	}

	if err := d.InstanceManager.Create(lrp); err != nil {
		return err
	}

	if err := d.ServiceManager.Create(lrp); err != nil {
		return err
	}

	return d.IngressManager.Update(lrp)
}

func (d *Desirer) List() ([]*opi.LRP, error) {
	return d.InstanceManager.List()
}

func (d *Desirer) Get(name string) (*opi.LRP, error) {
	return d.InstanceManager.Get(name)
}

func (d *Desirer) Update(lrp *opi.LRP) error {
	if err := d.InstanceManager.Update(lrp); err != nil {
		return err
	}

	if err := d.ServiceManager.Update(lrp); err != nil {
		return err
	}

	return d.IngressManager.Update(lrp)
}

func (d *Desirer) Stop(name string) error {
	if err := d.InstanceManager.Delete(name); err != nil {
		return err
	}

	if err := d.IngressManager.Delete(name); err != nil {
		return err
	}

	return d.ServiceManager.Delete(name)
}
