package pulseemitter

import (
	"math"
	"sync/atomic"

	loggregator "code.cloudfoundry.org/go-loggregator"
	"code.cloudfoundry.org/go-loggregator/rpc/loggregator_v2"
	"github.com/golang/protobuf/proto"
)

// GaugeMetric is used by the pulse emitter to emit gauge metrics to the
// LogClient.
type GaugeMetric interface {
	// Set sets the current value of the gauge metric.
	Set(n float64)

	// Emit sends the counter values to the LogClient.
	Emit(c LogClient)
}

// gaugeMetric is used by the pulse emitter to emit gauge metrics to the
// LogClient.
type gaugeMetric struct {
	name     string
	unit     string
	sourceID string
	value    uint64
	tags     map[string]string
}

// NewGaugeMetric returns a new gaugeMetric that has a value that can be set
// and emitted via a LogClient.
func NewGaugeMetric(name, unit, sourceID string, opts ...MetricOption) GaugeMetric {
	g := &gaugeMetric{
		name:     name,
		unit:     unit,
		sourceID: sourceID,
		tags:     make(map[string]string),
	}

	for _, opt := range opts {
		opt(g.tags)
	}

	return g
}

// Set will set the current value of the gauge metric to the given number.
func (g *gaugeMetric) Set(n float64) {
	atomic.StoreUint64(&g.value, toUint64(n, 2))
}

// Emit will send the current value and tagging options to the LogClient to
// be emitted.
func (g *gaugeMetric) Emit(c LogClient) {
	options := []loggregator.EmitGaugeOption{
		loggregator.WithGaugeValue(
			g.name,
			toFloat64(atomic.LoadUint64(&g.value), 2),
			g.unit,
		),
		g.sourceIDOption,
	}

	for k, v := range g.tags {
		options = append(options, loggregator.WithEnvelopeTag(k, v))
	}

	c.EmitGauge(options...)
}

func (g *gaugeMetric) sourceIDOption(p proto.Message) {
	env, ok := p.(*loggregator_v2.Envelope)
	if ok {
		env.SourceId = g.sourceID
	}
}

func toFloat64(v uint64, precision int) float64 {
	return float64(v) / math.Pow(10.0, float64(precision))
}

func toUint64(v float64, precision int) uint64 {
	return uint64(v * math.Pow(10.0, float64(precision)))
}
