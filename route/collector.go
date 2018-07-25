package route

import (
	"encoding/json"
	"errors"

	"k8s.io/api/core/v1"
	ext "k8s.io/api/extensions/v1beta1"
	av1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

const (
	httpPort = 80
	tlsPort  = 443
)

type Collector struct {
	Client        kubernetes.Interface
	Scheduler     TaskScheduler
	Work          chan<- []RegistryMessage
	KubeNamespace string
	KubeEndpoint  string
}

func (r *Collector) Start() {
	r.Scheduler.Schedule(r.collectRoutes)
}

func (r *Collector) collectRoutes() error {
	ingressList, err := r.Client.ExtensionsV1beta1().Ingresses(r.KubeNamespace).List(av1.ListOptions{})
	if err != nil {
		return err
	}

	var messages []RegistryMessage
	for _, ingress := range ingressList.Items {
		for _, rule := range ingress.Spec.Rules {
			service, err := r.getService(&rule)
			if err != nil {
				return err
			}
			if !r.hasRoutes(service) {
				continue
			}
			message, err := r.createRegistryMessage(&rule, service)
			if err != nil {
				return err
			}
			messages = append(messages, message)
		}
	}

	r.Work <- messages
	return nil
}

func (r *Collector) getService(rule *ext.IngressRule) (*v1.Service, error) {
	serviceName := rule.HTTP.Paths[0].Backend.ServiceName
	return r.Client.CoreV1().Services(r.KubeNamespace).Get(serviceName, av1.GetOptions{})
}

func (r *Collector) hasRoutes(service *v1.Service) bool {
	return service.Annotations != nil && service.Annotations["routes"] != ""
}

func (r *Collector) createRegistryMessage(rule *ext.IngressRule, service *v1.Service) (RegistryMessage, error) {
	if len(rule.HTTP.Paths) == 0 {
		return RegistryMessage{}, errors.New("paths must not be empty slice")
	}

	routes, err := decodeRoutes(service)
	if err != nil {
		return RegistryMessage{}, err
	}

	return RegistryMessage{
		Host:    r.KubeEndpoint,
		Port:    httpPort,
		TLSPort: tlsPort,
		URIs:    routes,
		App:     service.Name,
	}, nil
}

func decodeRoutes(s *v1.Service) ([]string, error) {
	routes := s.Annotations["routes"]
	uris := []string{}
	err := json.Unmarshal([]byte(routes), &uris)

	return uris, err
}
