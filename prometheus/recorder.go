package prometheus

import (
	"errors"

	"code.cloudfoundry.org/lager"
	api "github.com/prometheus/client_golang/prometheus"
)

//counterfeiter:generate . Recorder

const (
	LRPCreations = "eirini_lrp_creations"
)

type Recorder interface {
	Increment(counterName string)
}

type prometheusRecorder struct {
	logger   lager.Logger
	counters map[string]api.Counter
}

func NewRecorder(logger lager.Logger, registry api.Registerer) (Recorder, error) {
	lrpCreations, err := registerCounter(registry, LRPCreations, "The total number of created lrps")
	if err != nil {
		return nil, err
	}

	return &prometheusRecorder{
		logger: logger,
		counters: map[string]api.Counter{
			LRPCreations: lrpCreations,
		},
	}, nil
}

func (p *prometheusRecorder) Increment(counterName string) {
	logger := p.logger.Session("increment-counter", lager.Data{"counter-name": counterName})

	counter, ok := p.counters[counterName]
	if !ok {
		logger.Error("unknown-counter", nil)

		return
	}

	counter.Inc()
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
