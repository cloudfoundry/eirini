package route_test

import (
	"errors"
	"fmt"
	"time"

	"code.cloudfoundry.org/eirini/route/routefakes"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "code.cloudfoundry.org/eirini/route"
)

var _ = Describe("Emitter", func() {

	const timeout = 500 * time.Millisecond

	var (
		scheduler    *routefakes.FakeTaskScheduler
		publisher    *routefakes.FakePublisher
		workChannel  chan *Message
		emitter      *Emitter
		routes       *Message
		publishCount int
	)

	assertPublishedRoutes := func(subject, route string, callIndex int) {
		Eventually(func() int {
			return publisher.PublishCallCount()
		}, timeout).Should(Equal(publishCount))

		subject, routeJSON := publisher.PublishArgsForCall(callIndex)
		Expect(subject).To(Equal(subject))
		Expect(routeJSON).To(MatchJSON(
			fmt.Sprintf(`
			{
				"host": "203.0.113.2",
				"port": 8080,
				"tls_port": 8443,
				"uris": ["%s"],
				"app": "app1",
				"private_instance_id": "instance-id"
			}`, route)))
	}

	BeforeEach(func() {
		scheduler = new(routefakes.FakeTaskScheduler)
		publisher = new(routefakes.FakePublisher)
		workChannel = make(chan *Message, 1)
		publishCount = 2

		routes = &Message{
			Routes:             []string{"route1.my.app.com"},
			UnregisteredRoutes: []string{"removed.route1.my.app.com"},
			Name:               "app1",
			InstanceID:         "instance-id",
			Address:            "203.0.113.2",
			Port:               8080,
			TLSPort:            8443,
		}

		emitter = NewEmitter(publisher, workChannel, scheduler)
		emitter.Start()
	})

	AfterEach(func() {
		close(workChannel)
	})

	JustBeforeEach(func() {
		task := scheduler.ScheduleArgsForCall(0)
		workChannel <- routes

		err := task()
		Expect(err).ToNot(HaveOccurred())
	})

	Context("When emitter is started", func() {
		It("should publish the registered routes", func() {
			assertPublishedRoutes("router.register", "route1.my.app.com", 0)
		})
		It("should publish the unregistered routes", func() {
			assertPublishedRoutes("router.unregister", "removed.route1.my.app.com", 1)
		})
	})

	Context("When there are no unregistered routes", func() {

		BeforeEach(func() {
			routes.UnregisteredRoutes = []string{}
			publishCount = 1
		})

		It("should only publish the registered routes", func() {
			assertPublishedRoutes("router.register", "route1.my.app.com", 0)
		})
	})

	Context("When there are no registered routes", func() {

		BeforeEach(func() {
			routes.Routes = []string{}
			publishCount = 1
		})

		It("should only publish the unregistered routes", func() {
			assertPublishedRoutes("router.unregister", "removed.route1.my.app.com", 0)
		})
	})

	Context("When the publisher returns an error", func() {

		BeforeEach(func() {
			publisher.PublishReturns(errors.New("Failed to publish message"))
		})

		It("should publish the registered routes", func() {
			assertPublishedRoutes("router.register", "route1.my.app.com", 0)
		})
		It("should publish the unregistered routes", func() {
			assertPublishedRoutes("router.unregister", "removed.route1.my.app.com", 1)
		})
	})
})
