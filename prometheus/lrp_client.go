package prometheus

import (
	"context"
	"errors"

	"code.cloudfoundry.org/eirini/k8s/shared"
	"code.cloudfoundry.org/eirini/opi"
	"code.cloudfoundry.org/lager"
	api "github.com/prometheus/client_golang/prometheus"
	"k8s.io/apimachinery/pkg/util/clock"
)

const (
	LRPCreations             = "eirini_lrp_creations"
	LRPCreationsHelp         = "The total number of created lrps"
	LRPCreationDurations     = "eirini_lrp_creation_durations"
	LRPCreationDurationsHelp = "The duration of lrp creations"
)

//counterfeiter:generate . LRPClient

type LRPClient interface {
	Desire(ctx context.Context, namespace string, lrp *opi.LRP, opts ...shared.Option) error
	Get(ctx context.Context, identifier opi.LRPIdentifier) (*opi.LRP, error)
	Update(ctx context.Context, lrp *opi.LRP) error
}

type LRPClientDecorator struct {
	LRPClient
	logger            lager.Logger
	creations         api.Counter
	creationDurations api.Histogram
	clock             clock.PassiveClock
}

func NewLRPClientDecorator(
	logger lager.Logger,
	lrpClient LRPClient,
	registry api.Registerer,
	clck clock.PassiveClock,
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
		clock:             clck,
	}, nil
}

func (d *LRPClientDecorator) Desire(ctx context.Context, namespace string, lrp *opi.LRP, opts ...shared.Option) error {
	start := d.clock.Now()

	err := d.LRPClient.Desire(ctx, namespace, lrp, opts...)
	if err == nil {
		d.creations.Inc()
		d.creationDurations.Observe(float64(d.clock.Since(start).Milliseconds()))
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
