package route_test

import (
	"errors"
	"fmt"

	. "code.cloudfoundry.org/eirini/route"
	"code.cloudfoundry.org/eirini/route/routefakes"
	"code.cloudfoundry.org/lager/lagertest"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
)

var _ = Describe("MessageEmitter", func() {
	var (
		publisher *routefakes.FakePublisher
		logger    *lagertest.TestLogger

		messageEmitter Emitter
		routes         Message
		publishCount   int
	)

	assertPublishedRoutes := func(expectedSubject, route string, callIndex int) {
		Expect(publisher.PublishCallCount()).To(Equal(publishCount))

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
		publisher = new(routefakes.FakePublisher)
		publishCount = 2

		routes = Message{
			Routes: Routes{
				RegisteredRoutes:   []string{"route1.my.app.com"},
				UnregisteredRoutes: []string{"removed.route1.my.app.com"},
			},
			Name:       "app1",
			InstanceID: "instance-id",
			Address:    "203.0.113.2",
			Port:       8080,
			TLSPort:    8443,
		}

		logger = lagertest.NewTestLogger("test-logger")
		messageEmitter = NewMessageEmitter(publisher, logger)
	})

	Context("When MessageEmitter is started", func() {
		It("should publish the registered routes", func() {
			messageEmitter.Emit(ctx, routes)
			assertPublishedRoutes("router.register", "route1.my.app.com", 0)
		})

		It("should publish the unregistered routes", func() {
			messageEmitter.Emit(ctx, routes)
			assertPublishedRoutes("router.unregister", "removed.route1.my.app.com", 1)
		})

		Context("When there are no unregistered routes", func() {
			BeforeEach(func() {
				routes.UnregisteredRoutes = []string{}
				publishCount = 1
			})

			It("should only publish the registered routes", func() {
				messageEmitter.Emit(ctx, routes)
				assertPublishedRoutes("router.register", "route1.my.app.com", 0)
			})
		})

		Context("When there are no registered routes", func() {
			BeforeEach(func() {
				routes.RegisteredRoutes = []string{}
				publishCount = 1
			})

			It("should only publish the unregistered routes", func() {
				messageEmitter.Emit(ctx, routes)
				assertPublishedRoutes("router.unregister", "removed.route1.my.app.com", 0)
			})
		})

		Context("When the publisher returns an error", func() {
			BeforeEach(func() {
				publisher.PublishReturns(errors.New("Failed to publish message"))
			})

			It("should publish the registered routes", func() {
				messageEmitter.Emit(ctx, routes)
				assertPublishedRoutes("router.register", "route1.my.app.com", 0)
			})

			It("prints an informative message that registration failed", func() {
				messageEmitter.Emit(ctx, routes)
				Expect(logger.Buffer()).To(gbytes.Say(`"message":"test-logger.failed-to-publish-registered-route"`))
				Expect(logger.Buffer()).To(gbytes.Say(`"error":".*Failed to publish message"`))
			})

			It("should publish the unregistered routes", func() {
				messageEmitter.Emit(ctx, routes)
				assertPublishedRoutes("router.unregister", "removed.route1.my.app.com", 1)
			})

			It("prints an informative message that unregistration failed", func() {
				messageEmitter.Emit(ctx, routes)
				Expect(logger.Buffer()).To(gbytes.Say(`"message":"test-logger.failed-to-publish-unregistered-route"`))
				Expect(logger.Buffer()).To(gbytes.Say(`"error":".*Failed to publish message"`))
			})
		})
	})

	Context("When the route message is missing an address", func() {
		BeforeEach(func() {
			routes.Address = ""
		})

		It("should not publish a route", func() {
			messageEmitter.Emit(ctx, routes)
			Expect(publisher.PublishCallCount()).To(Equal(0))
		})

		It("should log the error", func() {
			messageEmitter.Emit(ctx, routes)

			logs := logger.Logs()
			Expect(logs).To(HaveLen(1))
			log := logs[0]
			Expect(log.Message).To(Equal("test-logger.route-address-missing"))
			Expect(log.Data).To(HaveKeyWithValue("app-name", "app1"))
			Expect(log.Data).To(HaveKeyWithValue("instance-id", "instance-id"))
		})
	})
})
