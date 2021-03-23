package route

import (
	"context"
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

//counterfeiter:generate . Publisher

type Publisher interface {
	Publish(subj string, data []byte) error
}

type NATSPublisher struct {
	NatsClient *nats.Conn
}

func (p *NATSPublisher) Publish(subj string, data []byte) error {
	return p.NatsClient.Publish(subj, data)
}

type MessageEmitter struct {
	publisher Publisher
	logger    lager.Logger
}

func NewMessageEmitter(publisher Publisher, logger lager.Logger) MessageEmitter {
	return MessageEmitter{
		publisher: publisher,
		logger:    logger,
	}
}

func NewEmitterFromConfig(natsIP string, natsPort int, natsPassword string, logger lager.Logger) (Emitter, error) {
	nc, err := nats.Connect(util.GenerateNatsURL(natsPassword, natsIP, natsPort), nats.MaxReconnects(-1))
	if err != nil {
		return nil, errors.Wrap(err, "failed to connect to nats")
	}

	emitterLogger := logger.Session("emitter")

	return NewMessageEmitter(&NATSPublisher{NatsClient: nc}, emitterLogger), nil
}

func (e MessageEmitter) Emit(ctx context.Context, route Message) {
	if len(route.Address) == 0 {
		e.logger.Debug("route-address-missing", lager.Data{"app-name": route.Name, "instance-id": route.InstanceID})

		return
	}

	e.registerRoutes(route)
	e.unregisterRoutes(route)
}

func (e MessageEmitter) registerRoutes(route Message) {
	if len(route.RegisteredRoutes) == 0 {
		return
	}

	err := e.publish(registerSubject, route)
	if err != nil {
		e.logger.Error("failed-to-publish-registered-route", err, lager.Data{"routes": route.Routes})
	}
}

func (e MessageEmitter) unregisterRoutes(route Message) {
	if len(route.UnregisteredRoutes) == 0 {
		return
	}

	err := e.publish(unregisterSubject, route)
	if err != nil {
		e.logger.Error("failed-to-publish-unregistered-route", err, lager.Data{"routes": route.UnregisteredRoutes})
	}
}

func (e MessageEmitter) publish(subject string, route Message) error {
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
		return errors.Wrap(err, "failed to publish route message")
	}

	return nil
}
