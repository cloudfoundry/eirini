package pulseemitter

import (
	"fmt"
	"sync/atomic"

	"code.cloudfoundry.org/go-loggregator/rpc/loggregator_v2"
	"github.com/golang/protobuf/proto"

	loggregator "code.cloudfoundry.org/go-loggregator"
)

// MetricOption defines a function type that can be used to configure tags for
// many types of metrics.
type MetricOption func(map[string]string)

// WithVersion will apply a `metric_version` tag to all envelopes sent about
// the metric.
func WithVersion(major, minor uint) MetricOption {
	return WithTags(map[string]string{
		"metric_version": fmt.Sprintf("%d.%d", major, minor),
	})
}

// WithTags will set the tags to apply to every envelopes sent about the
// metric..
func WithTags(tags map[string]string) MetricOption {
	return func(c map[string]string) {
		for k, v := range tags {
			c[k] = v
		}
	}
}

// counterMetric is used by the pulse emitter to emit counter metrics to the
// LogClient.
type counterMetric struct {
	name     string
	sourceID string
	delta    uint64
	tags     map[string]string
}

// CounterMetric is used by the pulse emitter to emit counter metrics to the
// LogClient.
type CounterMetric interface {
	// Increment increases the counter's delta by the given value
	Increment(c uint64)

	// Emit sends the counter values to the LogClient.
	Emit(c LogClient)
}

// NewCounterMetric returns a new counterMetric that can be incremented and
// emitted via a LogClient.
func NewCounterMetric(name, sourceID string, opts ...MetricOption) CounterMetric {
	m := &counterMetric{
		name:     name,
		sourceID: sourceID,
		tags:     make(map[string]string),
	}

	for _, opt := range opts {
		opt(m.tags)
	}

	return m
}

// Increment will add the given uint64 to the current delta.
func (m *counterMetric) Increment(c uint64) {
	atomic.AddUint64(&m.delta, c)
}

// Emit will send the current delta and tagging options to the LogClient to
// be emitted. The delta on the counterMetric will be reset to 0.
func (m *counterMetric) Emit(c LogClient) {
	d := atomic.SwapUint64(&m.delta, 0)
	options := []loggregator.EmitCounterOption{
		loggregator.WithDelta(d),
		m.sourceIDOption,
	}

	for k, v := range m.tags {
		options = append(options, loggregator.WithEnvelopeTag(k, v))
	}

	c.EmitCounter(m.name, options...)
}

func (m *counterMetric) sourceIDOption(p proto.Message) {
	env, ok := p.(*loggregator_v2.Envelope)
	if ok {
		env.SourceId = m.sourceID
	}
}
