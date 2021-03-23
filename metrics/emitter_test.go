package metrics_test

import (
	"context"
	"time"

	"code.cloudfoundry.org/eirini/metrics"
	"code.cloudfoundry.org/eirini/metrics/metricsfakes"
	"code.cloudfoundry.org/go-loggregator/rpc/loggregator_v2"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("emitter", func() {
	var ctx context.Context

	BeforeEach(func() {
		ctx = context.Background()
	})

	It("should forward source info to loggregator", func() {
		fakeClient := new(metricsfakes.FakeLoggregatorClient)
		emitter := metrics.NewLoggregatorEmitter(fakeClient)

		envelope := newEnvelope()

		msg := metrics.Message{
			AppID:       "amazing-app-id",
			IndexID:     "best-index-id",
			CPU:         100,
			Memory:      320,
			MemoryQuota: 500,
			Disk:        645,
			DiskQuota:   1001,
		}
		emitter.Emit(ctx, msg)
		Expect(fakeClient.EmitGaugeCallCount()).To(Equal(1))

		emitGaugeOpts := fakeClient.EmitGaugeArgsForCall(0)
		for _, g := range emitGaugeOpts {
			g(envelope)
		}

		Expect(envelope.SourceId).To(Equal(msg.AppID))
		Expect(envelope.InstanceId).To(Equal(msg.IndexID))
		expectedMetrics := map[string]*loggregator_v2.GaugeValue{
			"cpu": {
				Unit:  metrics.CPUUnit,
				Value: 100,
			},
			"memory": {
				Unit:  metrics.MemoryUnit,
				Value: 320,
			},
			"memory_quota": {
				Unit:  metrics.MemoryUnit,
				Value: 500,
			},
			"disk": {
				Unit:  metrics.DiskUnit,
				Value: 645,
			},
			"disk_quota": {
				Unit:  metrics.DiskUnit,
				Value: 1001,
			},
		}

		Expect(envelope.GetGauge().Metrics).To(Equal(expectedMetrics))
	})
})

func newEnvelope() *loggregator_v2.Envelope {
	return &loggregator_v2.Envelope{
		Timestamp: time.Now().UnixNano(),
		Message: &loggregator_v2.Envelope_Gauge{
			Gauge: &loggregator_v2.Gauge{
				Metrics: make(map[string]*loggregator_v2.GaugeValue),
			},
		},
		Tags: make(map[string]string),
	}
}
