package route

import (
	"encoding/json"
	"fmt"

	"code.cloudfoundry.org/eirini"
	nats "github.com/nats-io/go-nats"
)

const (
	httpPort          = 80
	tlsPort           = 443
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
			err := e.publish(registerSubject, route.Routes, route.Name)
			if err != nil {
				fmt.Println("failed to publish registered route:", err.Error())
			}
		}

		if len(route.UnregisteredRoutes) != 0 {
			e.unregisterRoute(route)
		}
	}
}

func (e *Emitter) unregisterRoute(route *eirini.Routes) {
	err := e.publish(unregisterSubject, route.UnregisteredRoutes, route.Name)
	if err != nil {
		fmt.Println("failed to publish unregistered route:", err.Error())
		return
	}
	err = route.PopUnregisteredRoutes()
	if err != nil {
		fmt.Println("failed to remove unregistered routes")
	}
}

func (e *Emitter) publish(subject string, routes []string, name string) error {
	message := RegistryMessage{
		Host:    e.kubeEndpoint,
		Port:    httpPort,
		TLSPort: tlsPort,
		URIs:    routes,
		App:     name,
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
