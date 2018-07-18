package k8s

import (
	"code.cloudfoundry.org/eirini"
	meta_v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

//go:generate counterfeiter . ServiceManager
type ServiceManager interface {
	Delete(appName, namespace string) error
}

type serviceManager struct {
	client   kubernetes.Interface
	endpoint string
}

func NewServiceManager(client kubernetes.Interface) ServiceManager {
	return &serviceManager{
		client: client,
	}
}

func (m *serviceManager) Delete(appName, namespace string) error {
	serviceName := eirini.GetInternalServiceName(appName)
	return m.client.CoreV1().Services(namespace).Delete(serviceName, &meta_v1.DeleteOptions{})
}
