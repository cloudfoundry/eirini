package metrics_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "code.cloudfoundry.org/eirini/metrics"
	"code.cloudfoundry.org/eirini/metrics/metricsfakes"
	"code.cloudfoundry.org/eirini/route/routefakes"
)

var _ = Describe("emitter", func() {

	var (
		emitter   *Emitter
		work      chan []Message
		scheduler *routefakes.FakeTaskScheduler
		forwarder *metricsfakes.FakeForwarder
		err       error
	)

	BeforeEach(func() {
		work = make(chan []Message, 5)
		scheduler = new(routefakes.FakeTaskScheduler)
		forwarder = new(metricsfakes.FakeForwarder)
		emitter = NewEmitter(work, scheduler, forwarder)
	})

	Context("when metrics are send to the channel", func() {

		BeforeEach(func() {
			emitter.Start()

			work <- []Message{
				{
					AppID:       "appid",
					IndexID:     "0",
					CPU:         123.4,
					Memory:      123.4,
					MemoryUnit:  "Mb",
					MemoryQuota: 1000.4,
					Disk:        10.1,
					DiskUnit:    "Gb",
					DiskQuota:   250.5,
				},
				{
					AppID:       "appid",
					IndexID:     "1",
					CPU:         234.1,
					Memory:      675.4,
					MemoryUnit:  "Mb",
					MemoryQuota: 1000.4,
					Disk:        10.1,
					DiskUnit:    "Gb",
					DiskQuota:   250.5,
				},
			}
		})

		JustBeforeEach(func() {
			task := scheduler.ScheduleArgsForCall(0)
			err = task()
		})

		It("should not return an error", func() {
			Expect(err).ToNot(HaveOccurred())
		})

		It("should call the forwarder", func() {
			callCount := forwarder.ForwardCallCount()
			Expect(callCount).To(Equal(2))
		})

		It("should forward the first message", func() {
			message := forwarder.ForwardArgsForCall(0)
			Expect(message).To(Equal(Message{
				AppID:       "appid",
				IndexID:     "0",
				CPU:         123.4,
				Memory:      123.4,
				MemoryUnit:  "Mb",
				MemoryQuota: 1000.4,
				Disk:        10.1,
				DiskUnit:    "Gb",
				DiskQuota:   250.5,
			}))
		})

		It("should forward the second message", func() {
			message := forwarder.ForwardArgsForCall(1)
			Expect(message).To(Equal(Message{
				AppID:       "appid",
				IndexID:     "1",
				CPU:         234.1,
				Memory:      675.4,
				MemoryUnit:  "Mb",
				MemoryQuota: 1000.4,
				Disk:        10.1,
				DiskUnit:    "Gb",
				DiskQuota:   250.5,
			}))
		})
	})
})
