package route_test

import (
	"errors"
	"fmt"
	"io"
	"time"

	"code.cloudfoundry.org/eirini/route/routefakes"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"

	. "code.cloudfoundry.org/eirini/route"
)

var _ = Describe("Emitter", func() {

	const timeout = 500 * time.Millisecond

	var (
		scheduler   *routefakes.FakeTaskScheduler
		publisher   *routefakes.FakePublisher
		workChannel chan *Message
		log         *gbytes.Buffer

		emitter      *Emitter
		routes       *Message
		publishCount int
	)

	assertPublishedRoutes := func(expectedSubject, route string, callIndex int) {
		Eventually(publisher.PublishCallCount, timeout).Should(Equal(publishCount))

		actualSubject, routeJSON := publisher.PublishArgsForCall(callIndex)
		Expect(expectedSubject).To(Equal(actualSubject))
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

		log = gbytes.NewBuffer()
		emitter = NewEmitter(publisher, workChannel, scheduler, io.MultiWriter(GinkgoWriter, log))
		emitter.Start()
	})

	AfterEach(func() {
		close(workChannel)
	})

	Context("When emitter is started", func() {

		JustBeforeEach(func() {
			task := scheduler.ScheduleArgsForCall(0)
			workChannel <- routes

			err := task()
			Expect(err).ToNot(HaveOccurred())
		})

		It("should publish the registered routes", func() {
			assertPublishedRoutes("router.register", "route1.my.app.com", 0)
		})

		It("should publish the unregistered routes", func() {
			assertPublishedRoutes("router.unregister", "removed.route1.my.app.com", 1)
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

			It("prints an informative message that registration failed", func() {
				Eventually(log, timeout).Should(gbytes.Say("failed to publish registered route: Failed to publish message"))
			})

			It("should publish the unregistered routes", func() {
				assertPublishedRoutes("router.unregister", "removed.route1.my.app.com", 1)
			})

			It("prints an informative message that unregistration failed", func() {
				Eventually(log, timeout).Should(gbytes.Say("failed to publish unregistered route: Failed to publish message"))
			})
		})
	})

	Context("When the route message is invalid", func() {

		BeforeEach(func() {
			routes.Address = ""
		})

		It("should not publish a route", func() {
			task := scheduler.ScheduleArgsForCall(0)
			workChannel <- routes

			Expect(func() { _ = task() /*#nosec*/ }).To(Panic())
			Expect(publisher.PublishCallCount()).To(Equal(0))
		})
	})
})
