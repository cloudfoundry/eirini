package route_test

import (
	"errors"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"code.cloudfoundry.org/eirini"
	"code.cloudfoundry.org/eirini/eirinifakes"
	. "code.cloudfoundry.org/eirini/route"
	"code.cloudfoundry.org/eirini/route/routefakes"
)

var _ = Describe("Collector", func() {

	var (
		collector              *Collector
		scheduler              *routefakes.FakeTaskScheduler
		fakeRouteLister        *routefakes.FakeLister
		fakeRemoveCallbackFunc *eirinifakes.FakeRemoveCallbackFunc
		workChannel            chan []*eirini.Routes
		routes                 []*eirini.Routes
		registeredRoutes       []string
		removedRoutes          []string
		err                    error
	)

	const (
		appName  = "dora"
		httpPort = 80
		tlsPort  = 443
	)

	BeforeEach(func() {
		registeredRoutes = []string{"route1.app.com", "route2.app.com"}
		removedRoutes = []string{"removed.route1.app.com", "removed.route2.app.com"}
		scheduler = new(routefakes.FakeTaskScheduler)
		workChannel = make(chan []*eirini.Routes, 1)
		fakeRouteLister = new(routefakes.FakeLister)
		fakeRemoveCallbackFunc = new(eirinifakes.FakeRemoveCallbackFunc)
	})

	JustBeforeEach(func() {
		collector = &Collector{
			RouteLister: fakeRouteLister,
			Scheduler:   scheduler,
			Work:        workChannel,
		}

		collector.Start()
		task := scheduler.ScheduleArgsForCall(0)
		err = task()
	})

	Context("Start collecting routes", func() {

		BeforeEach(func() {
			route := &eirini.Routes{
				Routes:             registeredRoutes,
				UnregisteredRoutes: removedRoutes,
				Name:               appName,
			}
			routes = []*eirini.Routes{route}
			fakeRouteLister.ListRoutesReturns(routes, nil)
		})

		It("should not return an error", func() {
			Expect(err).ToNot(HaveOccurred())
		})

		It("should use the scheduler to collect routes", func() {
			Expect(scheduler.ScheduleCallCount()).To(Equal(1))
		})

		It("should use the route lister to get the routes", func() {
			Expect(fakeRouteLister.ListRoutesCallCount()).To(Equal(1))
		})

		It("should send the correct RegistryMessage in the work channel", func() {
			actualMessages := <-workChannel
			Expect(actualMessages).To(Equal(routes))
		})

		Context("When the RouteLister fails", func() {
			BeforeEach(func() {
				listErr := errors.New("failed-to-list-routes")
				fakeRouteLister.ListRoutesReturns([]*eirini.Routes{}, listErr)
			})

			It("should return an error", func() {
				Expect(err).To(HaveOccurred())
			})
		})
	})
})
