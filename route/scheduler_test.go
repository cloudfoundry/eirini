package route_test

import (
	"errors"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"

	. "code.cloudfoundry.org/eirini/route"
	"code.cloudfoundry.org/eirini/route/routefakes"
	"code.cloudfoundry.org/eirini/util/utilfakes"
)

var _ = Describe("Scheduler", func() {

	var (
		collectorScheduler CollectorScheduler
		collector          *routefakes.FakeCollector
		scheduler          *utilfakes.FakeTaskScheduler
	)

	BeforeEach(func() {
		collector = new(routefakes.FakeCollector)
		scheduler = new(utilfakes.FakeTaskScheduler)

		collectorScheduler = CollectorScheduler{
			Collector: collector,
			Scheduler: scheduler,
		}
	})

	It("should send collected routes on the work channel", func() {
		work := make(chan *Message, 1)
		routes := []Message{
			{Name: "ama"},
		}
		collector.CollectReturns(routes, nil)

		collectorScheduler.Start(work)
		Expect(scheduler.ScheduleCallCount()).To(Equal(1))
		task := scheduler.ScheduleArgsForCall(0)

		Expect(task()).To(Succeed())
		Eventually(work).Should(Receive(PointTo(Equal(Message{Name: "ama"}))))
	})

	It("should propagate errors to the Scheduler", func() {
		work := make(chan *Message, 1)
		collector.CollectReturns(nil, errors.New("collector failure"))

		collectorScheduler.Start(work)
		Expect(scheduler.ScheduleCallCount()).To(Equal(1))
		task := scheduler.ScheduleArgsForCall(0)

		Expect(task()).To(MatchError(Equal("failed to collect routes: collector failure")))
		Expect(work).ToNot(Receive())
	})
})
