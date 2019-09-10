package route

import (
	"encoding/json"

	"code.cloudfoundry.org/eirini/util"
	"code.cloudfoundry.org/lager"
	nats "github.com/nats-io/nats.go"
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
	scheduler util.TaskScheduler
	work      <-chan *Message
	logger    lager.Logger
}

func NewEmitter(publisher Publisher, workChannel <-chan *Message, scheduler util.TaskScheduler, logger lager.Logger) *Emitter {
	return &Emitter{
		publisher: publisher,
		scheduler: scheduler,
		work:      workChannel,
		logger:    logger,
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
	if len(route.RegisteredRoutes) == 0 {
		return
	}

	err := e.publish(registerSubject, route)
	if err != nil {
		e.logger.Error("failed-to-publish-registered-route", err, lager.Data{"routes": route.Routes})
	}
}

func (e *Emitter) unregisterRoutes(route *Message) {
	if len(route.UnregisteredRoutes) == 0 {
		return
	}

	err := e.publish(unregisterSubject, route)
	if err != nil {
		e.logger.Error("failed-to-publish-unregistered-route", err, lager.Data{"routes": route.UnregisteredRoutes})
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
		URIs:              route.RegisteredRoutes,
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
