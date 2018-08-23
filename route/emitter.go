package route

import (
	"encoding/json"
	"errors"
	"fmt"

	"code.cloudfoundry.org/eirini"
	nats "github.com/nats-io/go-nats"
	"k8s.io/api/core/v1"
	ext "k8s.io/api/extensions/v1beta1"
)

const (
	registerSubject   = "router.register"
	unregisterSubject = "router.unregister"
)

type Publisher interface {
	Publish(subj string, data []byte) error
}

type NATSPublisher struct {
	NatsClient *nats.Conn
}

func (p *NATSPublisher) Publish(subj string, data []byte) error {
	return p.NatsClient.Publish(subj, data)
}

type Emitter struct {
	publisher    Publisher
	scheduler    TaskScheduler
	kubeEndpoint string
	work         <-chan []*eirini.Routes
}

func NewEmitter(publisher Publisher, workChannel chan []*eirini.Routes, scheduler TaskScheduler, kubeEndpoint string) *Emitter {
	return &Emitter{
		publisher:    publisher,
		scheduler:    scheduler,
		work:         workChannel,
		kubeEndpoint: kubeEndpoint,
	}
}

func (e *Emitter) Start() {
	e.scheduler.Schedule(func() error {
		select {
		case batch := <-e.work:
			e.emit(batch)
		}
		return nil
	})
}

func (e *Emitter) emit(batch []*eirini.Routes) {
	for _, route := range batch {
		if len(route.Routes) != 0 {
			e.publish(registerSubject, route.Routes, route.Name)
		}

		if len(route.UnregisteredRoutes) != 0 {
			e.unregisterRoute(route)
		}
	}
}

func (e *Emitter) unregisterRoute(route *eirini.Routes) {
	err := e.publish(unregisterSubject, route.UnregisteredRoutes, route.Name)
	if err != nil {
		fmt.Println("failed to publish route:", err.Error())
		return
	}
	route.Pop()
}

func (e *Emitter) publish(subject string, routes []string, name string) error {
	message := RegistryMessage{
		NatsMessage: NatsMessage{
			Host:    e.kubeEndpoint,
			Port:    httpPort,
			TLSPort: tlsPort,
			URIs:    routes,
		},
		App: name,
	}

	routeJSON, err := json.Marshal(message)
	if err != nil {
		fmt.Println("Faild to marshal route message:", err.Error())
		return err
	}

	if err = e.publisher.Publish(subject, routeJSON); err != nil {
		return err
	}
	return nil
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
		NatsMessage: NatsMessage{
			Host:    r.KubeEndpoint,
			Port:    httpPort,
			TLSPort: tlsPort,
			URIs:    routes,
		},
		App: service.Name,
	}, nil
}

func decodeRoutes(s *v1.Service) ([]string, error) {
	routes := s.Annotations["routes"]
	uris := []string{}
	err := json.Unmarshal([]byte(routes), &uris)

	return uris, err
}
