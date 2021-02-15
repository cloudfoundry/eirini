package prometheus

import (
	"errors"
	"time"

	"code.cloudfoundry.org/eirini/k8s/shared"
	"code.cloudfoundry.org/eirini/opi"
	"code.cloudfoundry.org/lager"
	api "github.com/prometheus/client_golang/prometheus"
)

const (
	LRPCreations             = "eirini_lrp_creations"
	LRPCreationsHelp         = "The total number of created lrps"
	LRPCreationDurations     = "eirini_lrp_creation_durations"
	LRPCreationDurationsHelp = "The duration of lrp creations"
)

//counterfeiter:generate . LRPClient

type LRPClient interface {
	Desire(namespace string, lrp *opi.LRP, opts ...shared.Option) error
	Get(identifier opi.LRPIdentifier) (*opi.LRP, error)
	Update(lrp *opi.LRP) error
}

type LRPClientDecorator struct {
	LRPClient
	logger            lager.Logger
	creations         api.Counter
	creationDurations api.Histogram
}

func NewLRPClientDecorator(
	logger lager.Logger,
	lrpClient LRPClient,
	registry api.Registerer,
) (*LRPClientDecorator, error) {
	creations, err := registerCounter(registry, LRPCreations, "The total number of created lrps")
	if err != nil {
		return nil, err
	}
	creationDurations, err := registerHistogram(registry, LRPCreationDurations, LRPCreationDurationsHelp)
	if err != nil {
		return nil, err
	}

	return &LRPClientDecorator{
		LRPClient:         lrpClient,
		logger:            logger,
		creations:         creations,
		creationDurations: creationDurations,
	}, nil
}

func (d *LRPClientDecorator) Desire(namespace string, lrp *opi.LRP, opts ...shared.Option) error {
	start := time.Now()
	err := d.LRPClient.Desire(namespace, lrp, opts...)
	if err == nil {
		d.creations.Inc()
		d.creationDurations.Observe(float64(time.Since(start).Milliseconds()))
	}

	return err
}

func registerCounter(registry api.Registerer, name, help string) (api.Counter, error) {
	c := api.NewCounter(api.CounterOpts{
		Name: name,
		Help: help,
	})

	err := registry.Register(c)
	if err == nil {
		return c, nil
	}

	var are api.AlreadyRegisteredError
	if errors.As(err, &are) {
		return are.ExistingCollector.(api.Counter), nil
	}

	return nil, err
}

func registerHistogram(registry api.Registerer, name, help string) (api.Histogram, error) {
	h := api.NewHistogram(api.HistogramOpts{
		Name: name,
		Help: help,
	})

	err := registry.Register(h)
	if err == nil {
		return h, nil
	}

	var are api.AlreadyRegisteredError
	if errors.As(err, &are) {
		return are.ExistingCollector.(api.Histogram), nil
	}

	return nil, err
}
