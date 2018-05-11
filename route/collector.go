package route

import (
	"encoding/json"
	"errors"

	ext "k8s.io/api/extensions/v1beta1"
	av1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

const (
	httpPort = 80
	tlsPort  = 443
)

type RouteCollector struct {
	Client        kubernetes.Interface
	Scheduler     TaskScheduler
	Work          chan<- []RegistryMessage
	KubeNamespace string
}

func (r *RouteCollector) Start() {
	r.Scheduler.Schedule(r.collectRoutes)
}

func (r *RouteCollector) collectRoutes() error {
	ingressList, err := r.Client.ExtensionsV1beta1().Ingresses(r.KubeNamespace).List(av1.ListOptions{})
	if err != nil {
		return err
	}

	var messages []RegistryMessage
	for _, ingress := range ingressList.Items {
		for _, rule := range ingress.Spec.Rules {
			message, err := r.createRegistryMessage(&rule)
			if err != nil {
				return err
			}
			messages = append(messages, message)
		}
	}

	r.Work <- messages
	return nil
}

func (r *RouteCollector) createRegistryMessage(rule *ext.IngressRule) (RegistryMessage, error) {
	if len(rule.HTTP.Paths) == 0 {
		return RegistryMessage{}, errors.New("paths must not be empty slice")
	}
	host := rule.Host
	serviceName := rule.HTTP.Paths[0].Backend.ServiceName

	routes, err := r.getRoutes(serviceName)
	if err != nil {
		return RegistryMessage{}, err
	}

	return RegistryMessage{
		Host:    host,
		Port:    httpPort,
		TlsPort: tlsPort,
		URIs:    routes,
		App:     serviceName,
	}, nil
}

func (r *RouteCollector) getRoutes(serviceName string) ([]string, error) {
	service, err := r.Client.CoreV1().Services(r.KubeNamespace).Get(serviceName, av1.GetOptions{})
	if err != nil {
		return []string{}, err
	}
	return decode(service.Annotations["routes"])
}

func decode(s string) ([]string, error) {
	uris := []string{}
	err := json.Unmarshal([]byte(s), &uris)

	return uris, err
}
