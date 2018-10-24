package route

import (
	"encoding/json"
	"fmt"

	"code.cloudfoundry.org/eirini"
	nats "github.com/nats-io/go-nats"
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
	publisher Publisher
	scheduler TaskScheduler
	work      <-chan []*eirini.Routes
}

func NewEmitter(publisher Publisher, workChannel chan []*eirini.Routes, scheduler TaskScheduler) *Emitter {
	return &Emitter{
		publisher: publisher,
		scheduler: scheduler,
		work:      workChannel,
	}
}

func (e *Emitter) Start() {
	e.scheduler.Schedule(func() error {
		e.emit(<-e.work)
		return nil
	})
}

func (e *Emitter) emit(batch []*eirini.Routes) {
	for _, route := range batch {
		if len(route.Routes) != 0 {
			err := e.publish(registerSubject, route)
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
	err := e.publish(unregisterSubject, route)
	if err != nil {
		fmt.Println("failed to publish unregistered route:", err.Error())
	}
}

func (e *Emitter) publish(subject string, route *eirini.Routes) error {
	message := RegistryMessage{
		Host:    route.ServiceAddress,
		Port:    route.ServicePort,
		TLSPort: route.ServiceTLSPort,
		URIs:    route.Routes,
		App:     route.Name,
	}

	if subject == unregisterSubject {
		message.URIs = route.UnregisteredRoutes
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
