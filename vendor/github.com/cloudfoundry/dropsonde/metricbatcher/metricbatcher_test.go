package metricbatcher_test

import (
	"time"

	. "github.com/apoydence/eachers"
	"github.com/apoydence/eachers/testhelpers"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/cloudfoundry/dropsonde/metricbatcher"
)

var _ = Describe("MetricBatcher", func() {
	var (
		mockMetricSender *mockMetricSender
		metricBatcher    *metricbatcher.MetricBatcher
		mockChainer      *mockCounterChainer
	)

	BeforeEach(func() {
		mockMetricSender = newMockMetricSender()
		mockChainer = newMockCounterChainer()
		testhelpers.AlwaysReturn(mockMetricSender.CounterOutput, mockChainer)
		testhelpers.AlwaysReturn(mockChainer.SetTagOutput, mockChainer)

		metricBatcher = metricbatcher.New(mockMetricSender, 50*time.Millisecond)
	})

	Describe("BatchCounter", func() {
		It("batches and sends with a chaining API", func() {
			metricBatcher.BatchCounter("count").Increment()
			Expect(mockMetricSender.CounterInput).ToNot(BeCalled())
			Eventually(mockMetricSender.CounterInput).Should(BeCalled(With("count")))
			Eventually(mockChainer.AddInput).Should(BeCalled(With(uint64(1))))
			mockChainer.AddOutput.Ret0 <- nil
		})

		It("batches events with different tags separately", func() {
			metricBatcher.BatchCounter("count").
				SetTag("foo", "bar").
				Increment()
			metricBatcher.BatchCounter("count").
				SetTag("baz", "qux").
				Increment()
			metricBatcher.BatchCounter("count").
				SetTag("baz", "qux").
				Add(2)

			Eventually(mockMetricSender.CounterInput).Should(BeCalled(With("count")))
			Eventually(mockChainer.SetTagInput).Should(BeCalled(With("foo", "bar")))
			Eventually(mockChainer.AddInput).Should(BeCalled(With(uint64(1))))

			mockChainer.AddOutput.Ret0 <- nil

			Eventually(mockMetricSender.CounterInput).Should(BeCalled(With("count")))

			Eventually(mockChainer.SetTagInput).Should(BeCalled(With("baz", "qux")))
			Eventually(mockChainer.AddInput).Should(BeCalled(With(uint64(3))))

			mockChainer.AddOutput.Ret0 <- nil
		})

		It("can add while it flushes without a data race", func() {
			close(mockChainer.AddOutput.Ret0)

			counter := metricBatcher.BatchCounter("count").SetTag("foo", "bar")
			after := time.After(100 * time.Millisecond)
			for {
				select {
				case <-after:
					return
				default:
					counter.Increment()
					counter.Add(2)
				}
			}
		})
	})

	Describe("BatchIncrementCounter", func() {
		It("accepts metrics while it's flushing metrics", func() {
			metricBatcher.BatchIncrementCounter("count")
			Eventually(mockMetricSender.CounterInput).Should(BeCalled(With("count")))
			Eventually(mockChainer.AddInput).Should(BeCalled(With(uint64(1))))

			done := make(chan struct{})
			go func() {
				defer close(done)
				metricBatcher.BatchIncrementCounter("count")
				metricBatcher.BatchIncrementCounter("count")
			}()
			Eventually(done).Should(BeClosed())

			mockChainer.AddOutput.Ret0 <- nil

			Eventually(mockMetricSender.CounterInput).Should(BeCalled(With("count")))
			Eventually(mockChainer.AddInput).Should(BeCalled(With(uint64(2))))

			mockChainer.AddOutput.Ret0 <- nil
		})

		It("batches and then sends a single metric", func() {
			metricBatcher.BatchIncrementCounter("count")
			Expect(mockChainer.AddInput).ToNot(BeCalled())

			metricBatcher.BatchIncrementCounter("count")
			metricBatcher.BatchIncrementCounter("count")
			Eventually(mockChainer.AddInput).Should(BeCalled(With(uint64(3))))

			mockChainer.AddOutput.Ret0 <- nil

			metricBatcher.BatchIncrementCounter("count")
			Expect(mockChainer.AddInput).ToNot(BeCalled())

			metricBatcher.BatchIncrementCounter("count")

			Eventually(mockChainer.AddInput).Should(BeCalled(With(uint64(2))))

			mockChainer.AddOutput.Ret0 <- nil
		})

		It("batches and then sends multiple metrics", func() {
			close(mockChainer.AddOutput.Ret0)

			metricBatcher.BatchIncrementCounter("count1")
			metricBatcher.BatchIncrementCounter("count2")
			metricBatcher.BatchIncrementCounter("count2")
			Expect(mockChainer.AddInput).ToNot(BeCalled())
			Eventually(mockMetricSender.CounterInput).Should(BeCalled(With("count1")))
			Eventually(mockMetricSender.CounterInput).Should(BeCalled(With("count2")))
			Eventually(mockChainer.AddInput).Should(BeCalled(With(uint64(1))))
			Eventually(mockChainer.AddInput).Should(BeCalled(With(uint64(2))))

			metricBatcher.BatchIncrementCounter("count1")
			metricBatcher.BatchIncrementCounter("count2")
			Expect(mockChainer.AddInput).ToNot(BeCalled())
			Eventually(mockMetricSender.CounterInput).Should(BeCalled(With("count1")))
			Eventually(mockMetricSender.CounterInput).Should(BeCalled(With("count2")))
			Eventually(mockChainer.AddInput).Should(BeCalled(With(uint64(1))))
			Eventually(mockChainer.AddInput).Should(BeCalled(With(uint64(1))))
		})
	})

	Describe("BatchAddCounter", func() {
		It("batches and then sends a single metric", func() {
			close(mockChainer.AddOutput.Ret0)

			metricBatcher.BatchAddCounter("count", 2)
			Expect(mockChainer.AddInput).ToNot(BeCalled())

			metricBatcher.BatchAddCounter("count", 2)
			metricBatcher.BatchAddCounter("count", 3)

			Eventually(mockChainer.AddInput).Should(BeCalled(With(uint64(7))))

			metricBatcher.BatchAddCounter("count", 1)
			metricBatcher.BatchAddCounter("count", 2)
			Eventually(mockChainer.AddInput).Should(BeCalled(With(uint64(3))))
		})

		It("batches and then sends multiple metrics", func() {
			close(mockChainer.AddOutput.Ret0)

			metricBatcher.BatchAddCounter("count1", 2)
			metricBatcher.BatchAddCounter("count2", 1)
			metricBatcher.BatchAddCounter("count2", 2)
			Expect(mockChainer.AddInput).ToNot(BeCalled())
			Eventually(mockMetricSender.CounterInput).Should(BeCalled(With("count1")))
			Eventually(mockMetricSender.CounterInput).Should(BeCalled(With("count2")))
			Eventually(mockChainer.AddInput).Should(BeCalled(With(uint64(2))))
			Eventually(mockChainer.AddInput).Should(BeCalled(With(uint64(3))))

			metricBatcher.BatchAddCounter("count1", 2)
			metricBatcher.BatchAddCounter("count2", 2)
			Expect(mockChainer.AddInput).ToNot(BeCalled())
			Eventually(mockMetricSender.CounterInput).Should(BeCalled(With("count1")))
			Eventually(mockMetricSender.CounterInput).Should(BeCalled(With("count2")))
			Eventually(mockChainer.AddInput).Should(BeCalled(With(uint64(2))))
			Eventually(mockChainer.AddInput).Should(BeCalled(With(uint64(2))))
		})
	})

	Describe("Reset", func() {
		It("cancels any scheduled counter emission", func() {
			metricBatcher.BatchAddCounter("count1", 2)
			metricBatcher.BatchIncrementCounter("count1")

			metricBatcher.Reset()

			Consistently(mockChainer.AddInput).ShouldNot(BeCalled())
		})
	})

	Describe("Close", func() {
		BeforeEach(func() {
			// Sets ticker to a longer time so that the Flush isn't called automatically from the go routine
			metricBatcher = metricbatcher.New(mockMetricSender, 5*time.Second)
		})

		It("flushes remaining metrics", func() {
			close(mockChainer.AddOutput.Ret0)

			metricBatcher.BatchAddCounter("count2", 1)
			metricBatcher.Close()

			Eventually(mockMetricSender.CounterInput).Should(BeCalled(With("count2")))
			Eventually(mockChainer.AddInput).Should(BeCalled(With(uint64(1))))
		})

		It("panics when sending metrics after closing", func() {
			metricBatcher.Close()
			Expect(func() {
				metricBatcher.BatchAddCounter("count3", 3)
			}).To(Panic())
		})
	})
})
