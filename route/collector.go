package route

import (
	"fmt"
	"time"

	"k8s.io/api/core/v1"
	av1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

type RouteCollector struct {
	Client *kubernetes.Clientset
	Work   chan []RegistryMessage
	Host   string
}

func (r *RouteCollector) Start(interval int) {
	ticker := time.NewTicker(time.Second * time.Duration(interval))
	for range ticker.C {
		serviceList, err := r.Client.CoreV1().Services("default").List(av1.ListOptions{})
		if err != nil {
			fmt.Println("could not list services:", err.Error())
		}

		var messages []RegistryMessage
		for _, service := range serviceList.Items {
			if service.Name != "kubernetes" {
				msg := createRegistryMessage(service, r.Host)
				messages = append(messages, msg)
			}
		}

		r.Work <- messages
	}
}

func createRegistryMessage(service v1.Service, host string) RegistryMessage {
	return RegistryMessage{
		Host:    host,
		Port:    uint32(service.Spec.Ports[0].NodePort),
		TlsPort: uint32(service.Spec.Ports[0].NodePort),
		URIs:    []string{service.Labels["routes"]},
		App:     service.Name,
	}
}
