package k8s

import (
	"code.cloudfoundry.org/eirini"
	"code.cloudfoundry.org/eirini/route"
	meta_v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

const (
	IngressHTTPPort = uint32(80)
	IngressTLSPort  = uint32(443)
)

type ServiceRouteLister struct {
	client         kubernetes.Interface
	namespace      string
	useIngress     bool
	ingressAddress string
}

func NewServiceRouteLister(client kubernetes.Interface, namespace string, useIngress bool, ingressAddress string) route.Lister {
	return &ServiceRouteLister{
		client:         client,
		namespace:      namespace,
		useIngress:     useIngress,
		ingressAddress: ingressAddress,
	}
}

func (r *ServiceRouteLister) ListRoutes() ([]*eirini.Routes, error) {
	services, err := r.client.CoreV1().Services(r.namespace).List(meta_v1.ListOptions{})
	if err != nil {
		return []*eirini.Routes{}, err
	}

	routes := []*eirini.Routes{}
	for _, s := range services.Items {
		if !isCFService(s.Name) || isHeadless(s.Name) {
			continue
		}

		registered, err := decodeRoutes(s.Annotations[eirini.RegisteredRoutes])
		if err != nil {
			return []*eirini.Routes{}, err
		}

		route := eirini.Routes{
			Routes:         registered,
			Name:           s.Name,
			ServiceAddress: s.Spec.ClusterIP,
			ServicePort:    uint32(s.Spec.Ports[0].Port),
			ServiceTLSPort: 0,
		}

		if r.useIngress {
			route.ServiceAddress = r.ingressAddress
			route.ServicePort = IngressHTTPPort
			route.ServiceTLSPort = IngressTLSPort
		}

		routes = append(routes, &route)
	}

	return routes, nil
}
