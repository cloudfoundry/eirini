package prometheus

import (
	"context"
	"errors"

	"code.cloudfoundry.org/eirini/api"
	"code.cloudfoundry.org/eirini/k8s/shared"
	"code.cloudfoundry.org/lager"
	prometheus_api "github.com/prometheus/client_golang/prometheus"
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
	Desire(ctx context.Context, namespace string, lrp *api.LRP, opts ...shared.Option) error
	Get(ctx context.Context, identifier api.LRPIdentifier) (*api.LRP, error)
	Update(ctx context.Context, lrp *api.LRP) error
}

type LRPClientDecorator struct {
	LRPClient
	logger            lager.Logger
	creations         prometheus_api.Counter
	creationDurations prometheus_api.Histogram
	clock             clock.PassiveClock
}

func NewLRPClientDecorator(
	logger lager.Logger,
	lrpClient LRPClient,
	registry prometheus_api.Registerer,
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

func (d *LRPClientDecorator) Desire(ctx context.Context, namespace string, lrp *api.LRP, opts ...shared.Option) error {
	start := d.clock.Now()

	err := d.LRPClient.Desire(ctx, namespace, lrp, opts...)
	if err == nil {
		d.creations.Inc()
		d.creationDurations.Observe(float64(d.clock.Since(start).Milliseconds()))
	}

	return err
}

func registerCounter(registry prometheus_api.Registerer, name, help string) (prometheus_api.Counter, error) {
	c := prometheus_api.NewCounter(prometheus_api.CounterOpts{
		Name: name,
		Help: help,
	})

	err := registry.Register(c)
	if err == nil {
		return c, nil
	}

	var are prometheus_api.AlreadyRegisteredError
	if errors.As(err, &are) {
		return are.ExistingCollector.(prometheus_api.Counter), nil //nolint:forcetypeassert
	}

	return nil, err
}

func registerHistogram(registry prometheus_api.Registerer, name, help string) (prometheus_api.Histogram, error) {
	h := prometheus_api.NewHistogram(prometheus_api.HistogramOpts{
		Name: name,
		Help: help,
	})

	err := registry.Register(h)
	if err == nil {
		return h, nil
	}

	var are prometheus_api.AlreadyRegisteredError
	if errors.As(err, &are) {
		return are.ExistingCollector.(prometheus_api.Histogram), nil //nolint:forcetypeassert
	}

	return nil, err
}
