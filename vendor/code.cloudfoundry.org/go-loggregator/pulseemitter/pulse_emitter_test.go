package pulseemitter_test

import (
	"time"

	"code.cloudfoundry.org/go-loggregator/pulseemitter"
	"code.cloudfoundry.org/go-loggregator/rpc/loggregator_v2"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Pulse EmitterClient", func() {
	It("emits a counter with a zero delta", func() {
		spyLogClient := newSpyLogClient()
		client := pulseemitter.New(
			spyLogClient,
			pulseemitter.WithPulseInterval(50*time.Millisecond),
			pulseemitter.WithSourceID("my-source-id"),
		)

		client.NewCounterMetric("some-name")
		Eventually(spyLogClient.CounterName).Should(Equal("some-name"))

		e := &loggregator_v2.Envelope{
			Message: &loggregator_v2.Envelope_Counter{
				Counter: &loggregator_v2.Counter{},
			},
		}
		for _, o := range spyLogClient.CounterOpts() {
			o(e)
		}
		Expect(e.GetCounter().GetDelta()).To(Equal(uint64(0)))
		Expect(e.GetSourceId()).To(Equal("my-source-id"))
	})

	It("emits a gauge with a zero value", func() {
		spyLogClient := newSpyLogClient()
		client := pulseemitter.New(
			spyLogClient,
			pulseemitter.WithPulseInterval(50*time.Millisecond),
			pulseemitter.WithSourceID("my-source-id"),
		)

		client.NewGaugeMetric("some-name", "some-unit")
		Eventually(spyLogClient.GaugeOpts).Should(HaveLen(2))

		e := &loggregator_v2.Envelope{
			Message: &loggregator_v2.Envelope_Gauge{
				Gauge: &loggregator_v2.Gauge{
					Metrics: make(map[string]*loggregator_v2.GaugeValue),
				},
			},
		}
		for _, o := range spyLogClient.GaugeOpts() {
			o(e)
		}
		Expect(e.GetGauge().GetMetrics()).To(HaveLen(1))
		Expect(e.GetGauge().GetMetrics()).To(HaveKey("some-name"))
		Expect(e.GetGauge().GetMetrics()["some-name"].GetUnit()).To(Equal("some-unit"))
		Expect(e.GetGauge().GetMetrics()["some-name"].GetValue()).To(Equal(0.0))
		Expect(e.GetSourceId()).To(Equal("my-source-id"))
	})

	It("pulses", func() {
		spyLogClient := newSpyLogClient()
		client := pulseemitter.New(
			spyLogClient,
			pulseemitter.WithPulseInterval(time.Millisecond),
		)

		client.NewGaugeMetric("some-name", "some-unit")
		Eventually(spyLogClient.GaugeCallCount).Should(BeNumerically(">", 1))
	})
})
