package metrics_test

import (
	. "github.com/apoydence/eachers"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/cloudfoundry/dropsonde/metrics"
	"github.com/cloudfoundry/sonde-go/events"
)

var _ = Describe("Metrics", func() {
	var (
		metricSender  *mockMetricSender
		metricBatcher *mockMetricBatcher
	)

	BeforeEach(func() {
		metricSender = newMockMetricSender()
		metricBatcher = newMockMetricBatcher()
		metrics.Initialize(metricSender, metricBatcher)
	})

	It("delegates Send", func() {
		metricSender.SendOutput.Ret0 <- nil
		event := &events.ValueMetric{}
		metrics.Send(event)
		Expect(metricSender.SendInput).To(BeCalled(With(event)))
	})

	It("delegates Value", func() {
		metricSender.ValueOutput.Ret0 <- nil
		metrics.Value("metric", 42.42, "answers")
		Expect(metricSender.ValueInput).To(BeCalled(With("metric", 42.42, "answers")))
	})

	It("delegates ContainerMetric", func() {
		metricSender.ContainerMetricOutput.Ret0 <- nil
		appGuid := "some_app_guid"
		metrics.ContainerMetric(appGuid, 7, 42.42, 1234, 123412341234)
		Expect(metricSender.ContainerMetricInput).To(BeCalled(
			With(appGuid, int32(7), 42.42, uint64(1234), uint64(123412341234)),
		))
	})

	It("delegates Counter", func() {
		metricSender.CounterOutput.Ret0 <- nil
		metrics.Counter("requests")
		Expect(metricSender.CounterInput).To(BeCalled(With("requests")))
	})

	It("delegates SendValue", func() {
		metricSender.SendValueOutput.Ret0 <- nil
		metrics.SendValue("metric", 42.42, "answers")
		Eventually(metricSender.SendValueInput).Should(BeCalled(With("metric", 42.42, "answers")))
	})

	It("delegates IncrementCounter", func() {
		metricSender.IncrementCounterOutput.Ret0 <- nil
		metrics.IncrementCounter("count")
		Eventually(metricSender.IncrementCounterInput).Should(BeCalled(With("count")))
	})

	It("delegates BatchIncrementCounter", func() {
		metrics.BatchIncrementCounter("count")
		Eventually(metricBatcher.BatchIncrementCounterInput).Should(BeCalled(With("count")))
	})

	It("delegates AddToCounter", func() {
		metricSender.AddToCounterOutput.Ret0 <- nil
		metrics.AddToCounter("count", 5)
		Eventually(metricSender.AddToCounterInput).Should(BeCalled(With("count", uint64(5))))
	})

	It("delegates BatchAddCounter", func() {
		metrics.BatchAddCounter("count", 3)
		Eventually(metricBatcher.BatchAddCounterInput).Should(BeCalled(With("count", uint64(3))))
	})

	It("delegates SendContainerMetric", func() {
		metricSender.SendContainerMetricOutput.Ret0 <- nil
		appGuid := "some_app_guid"
		metrics.SendContainerMetric(appGuid, 7, 42.42, 1234, 123412341234)
		Eventually(metricSender.SendContainerMetricInput).Should(
			BeCalled(With(appGuid, int32(7), 42.42, uint64(1234), uint64(123412341234))),
		)
	})

	Context("with a metrics package that is not initialized", func() {
		BeforeEach(func() {
			metrics.Initialize(nil, nil)
		})

		It("SendValue is a no-op", func() {
			err := metrics.SendValue("metric", 42.42, "answers")
			Expect(err).ToNot(HaveOccurred())
		})

		It("IncrementCounter is a no-op", func() {
			err := metrics.IncrementCounter("count")
			Expect(err).ToNot(HaveOccurred())
		})

		It("AddToCounter is a no-op", func() {
			err := metrics.AddToCounter("count", 10)
			Expect(err).ToNot(HaveOccurred())
		})

		It("SendContainerMetric is a no-op", func() {
			appGuid := "some_app_guid"
			err := metrics.SendContainerMetric(appGuid, 0, 42.42, 1234, 123412341234)
			Expect(err).ToNot(HaveOccurred())
		})

		It("Value is a no-op", func() {
			value := metrics.Value("metric", 42.42, "answers")
			Expect(value).To(BeNil())
		})

		It("ContainerMetric is a no-op", func() {
			appGuid := "some_app_guid"
			containerMetric := metrics.ContainerMetric(appGuid, 0, 42.42, 1234, 123412341234)
			Expect(containerMetric).To(BeNil())
		})

		It("Counter is a no-op", func() {
			counter := metrics.Counter("requests")
			Expect(counter).To(BeNil())
		})
	})

	Context("Close", func() {
		It("closes metric batcher", func() {
			metrics.Close()
			Eventually(metricBatcher.CloseCalled).Should(BeCalled())
		})

		It("calls close on previous batcher when initializing with a new one", func() {
			newMetricBatcher := newMockMetricBatcher()
			metrics.Initialize(nil, newMetricBatcher)
			Eventually(metricBatcher.CloseCalled).Should(BeCalled())
			Consistently(newMetricBatcher.CloseCalled).ShouldNot(BeCalled())
		})
	})
})
