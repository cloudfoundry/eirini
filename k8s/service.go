package k8s

import (
	"code.cloudfoundry.org/eirini"
	"code.cloudfoundry.org/eirini/models/cf"
	"code.cloudfoundry.org/eirini/opi"
	"k8s.io/api/core/v1"
	meta_v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	types "k8s.io/client-go/kubernetes/typed/core/v1"
)

//go:generate counterfeiter . ServiceManager
type ServiceManager interface {
	Create(lrp *opi.LRP) error
	CreateHeadless(lrp *opi.LRP) error
	Update(lrp *opi.LRP) error
	Delete(appName string) error
	DeleteHeadless(appName string) error
}

type serviceManager struct {
	client    kubernetes.Interface
	namespace string
}

func NewServiceManager(client kubernetes.Interface, namespace string) ServiceManager {
	return &serviceManager{
		client:    client,
		namespace: namespace,
	}
}

func (m *serviceManager) services() types.ServiceInterface {
	return m.client.CoreV1().Services(m.namespace)
}

func (m *serviceManager) Create(lrp *opi.LRP) error {
	_, err := m.services().Create(toService(lrp))
	return err
}

func (m *serviceManager) CreateHeadless(lrp *opi.LRP) error {
	_, err := m.services().Create(toHeadlessService(lrp))
	return err
}

func (m *serviceManager) Update(lrp *opi.LRP) error {
	service, err := m.services().Get(eirini.GetInternalServiceName(lrp.Name), meta_v1.GetOptions{})
	if err != nil {
		return err
	}

	service.Annotations["routes"] = lrp.Metadata[cf.VcapAppUris]
	_, err = m.services().Update(service)
	return err
}

func (m *serviceManager) Delete(appName string) error {
	serviceName := eirini.GetInternalServiceName(appName)
	return m.services().Delete(serviceName, &meta_v1.DeleteOptions{})
}

func (m *serviceManager) DeleteHeadless(appName string) error {
	serviceName := eirini.GetInternalHeadlessServiceName(appName)
	return m.services().Delete(serviceName, &meta_v1.DeleteOptions{})
}

func toService(lrp *opi.LRP) *v1.Service {
	service := &v1.Service{
		Spec: v1.ServiceSpec{
			Ports: []v1.ServicePort{
				{
					Name: "service",
					Port: 8080,
				},
			},
			Selector: map[string]string{
				"name": lrp.Name,
			},
		},
	}

	service.Name = eirini.GetInternalServiceName(lrp.Name)
	service.Labels = map[string]string{
		"name": lrp.Name,
	}

	service.Annotations = map[string]string{
		"routes": lrp.Metadata[cf.VcapAppUris],
	}

	return service
}

func toHeadlessService(lrp *opi.LRP) *v1.Service {
	service := &v1.Service{
		Spec: v1.ServiceSpec{
			ClusterIP: "None",
			Ports: []v1.ServicePort{
				{
					Name: "service",
					Port: 8080,
				},
			},
			Selector: map[string]string{
				"name": lrp.Name,
			},
		},
	}

	service.Name = eirini.GetInternalHeadlessServiceName(lrp.Name)
	service.Labels = map[string]string{
		"name": lrp.Name,
	}

	return service
}
