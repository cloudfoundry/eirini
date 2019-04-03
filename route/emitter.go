package route

import (
	"encoding/json"
	"fmt"
	"io"

	nats "github.com/nats-io/go-nats"
	"github.com/pkg/errors"
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
	log       io.Writer
}

func NewEmitter(publisher Publisher, workChannel <-chan *Message, scheduler TaskScheduler, log io.Writer) *Emitter {
	return &Emitter{
		publisher: publisher,
		scheduler: scheduler,
		work:      workChannel,
		log:       log,
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
		fmt.Fprintln(e.log, "failed to publish registered route:", err.Error())
	}
}

func (e *Emitter) unregisterRoutes(route *Message) {
	if len(route.UnregisteredRoutes) == 0 {
		return
	}

	err := e.publish(unregisterSubject, route)
	if err != nil {
		fmt.Fprintln(e.log, "failed to publish unregistered route:", err.Error())
	}
}

func (e *Emitter) publish(subject string, route *Message) error {
	if len(route.Address) == 0 {
		panic(errors.New("route address missing"))
	}

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
		return errors.Wrap(err, "failed to marshal route message:")
	}

	if err = e.publisher.Publish(subject, routeJSON); err != nil {
		return err
	}
	return nil
}
