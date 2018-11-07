package route

import (
	"encoding/json"
	"fmt"

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
	work      <-chan *Message
}

func NewEmitter(publisher Publisher, workChannel <-chan *Message, scheduler TaskScheduler) *Emitter {
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

func (e *Emitter) emit(route *Message) {
	e.registerRoutes(route)
	e.unregisterRoutes(route)
}

func (e *Emitter) registerRoutes(route *Message) {
	if len(route.Routes) == 0 {
		return
	}

	err := e.publish(registerSubject, route)
	if err != nil {
		fmt.Println("failed to publish registered route:", err.Error())
	}
}
func (e *Emitter) unregisterRoutes(route *Message) {
	if len(route.UnregisteredRoutes) == 0 {
		return
	}

	err := e.publish(unregisterSubject, route)
	if err != nil {
		fmt.Println("failed to publish unregistered route:", err.Error())
	}
}

func (e *Emitter) publish(subject string, route *Message) error {
	message := RegistryMessage{
		Host:              route.Address,
		Port:              route.Port,
		TLSPort:           route.TLSPort,
		URIs:              route.Routes,
		App:               route.Name,
		PrivateInstanceID: route.InstanceID,
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
