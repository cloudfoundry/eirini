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
	Delete(appName string) error
}

type serviceManager struct {
	client    kubernetes.Interface
	namespace string
}

func NewServiceManager(namespace string, client kubernetes.Interface) ServiceManager {
	return &serviceManager{
		client:    client,
		namespace: namespace,
	}
}

func (m *serviceManager) services() types.ServiceInterface {
	return m.client.CoreV1().Services(m.namespace)
}

func (m *serviceManager) Delete(appName string) error {
	serviceName := eirini.GetInternalServiceName(appName)
	return m.services().Delete(serviceName, &meta_v1.DeleteOptions{})
}

func (m *serviceManager) Create(lrp *opi.LRP) error {
	_, err := m.services().Create(toService(lrp))
	return err
}

func toService(lrp *opi.LRP) *v1.Service {
	service := &v1.Service{
		Spec: v1.ServiceSpec{
			Ports: []v1.ServicePort{
				v1.ServicePort{
					Name:     "service",
					Port:     8080,
					Protocol: v1.ProtocolTCP,
				},
			},
			Selector: map[string]string{
				"name": lrp.Name,
			},
			SessionAffinity: "None",
		},
		Status: v1.ServiceStatus{
			LoadBalancer: v1.LoadBalancerStatus{},
		},
	}

	service.APIVersion = "v1"
	service.Kind = "Service"
	service.Name = eirini.GetInternalServiceName(lrp.Name)
	service.Labels = map[string]string{
		"eirini": "eirini",
		"name":   lrp.Name,
	}

	service.Annotations = map[string]string{
		"routes": lrp.Metadata[cf.VcapAppUris],
	}

	return service
}
