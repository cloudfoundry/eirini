package route_test

import (
	"errors"

	. "code.cloudfoundry.org/eirini/route"
	"code.cloudfoundry.org/eirini/route/routefakes"
	"code.cloudfoundry.org/eirini/util/utilfakes"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Scheduler", func() {
	var (
		collectorScheduler CollectorScheduler
		collector          *routefakes.FakeCollector
		scheduler          *utilfakes.FakeTaskScheduler
		emitter            *routefakes.FakeEmitter
	)

	BeforeEach(func() {
		collector = new(routefakes.FakeCollector)
		scheduler = new(utilfakes.FakeTaskScheduler)
		emitter = new(routefakes.FakeEmitter)

		collectorScheduler = CollectorScheduler{
			Collector: collector,
			Scheduler: scheduler,
			Emitter:   emitter,
		}
	})

	It("should send collected routes on the work channel", func() {
		routes := []Message{
			{Name: "ama"},
			{Name: "zashto"},
		}
		collector.CollectReturns(routes, nil)

		collectorScheduler.Start()
		Expect(scheduler.ScheduleCallCount()).To(Equal(1))
		task := scheduler.ScheduleArgsForCall(0)

		Expect(task()).To(Succeed())
		Expect(emitter.EmitCallCount()).To(Equal(2))
		_, actualMsg0 := emitter.EmitArgsForCall(0)
		_, actualMsg1 := emitter.EmitArgsForCall(1)
		Expect(actualMsg0).To(Equal(Message{Name: "ama"}))
		Expect(actualMsg1).To(Equal(Message{Name: "zashto"}))
	})

	It("should propagate errors to the Scheduler", func() {
		collector.CollectReturns(nil, errors.New("collector failure"))

		collectorScheduler.Start()
		Expect(scheduler.ScheduleCallCount()).To(Equal(1))
		task := scheduler.ScheduleArgsForCall(0)

		Expect(task()).To(MatchError(Equal("failed to collect routes: collector failure")))
	})
})
