package eats_test

import (
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"

	"code.cloudfoundry.org/eirini"
	"code.cloudfoundry.org/eirini/integration/util"
	"code.cloudfoundry.org/eirini/models/cf"
	"code.cloudfoundry.org/eirini/route"
	"github.com/nats-io/nats-server/v2/server"
	natstest "github.com/nats-io/nats-server/v2/test"
	"github.com/nats-io/nats.go"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"
	. "github.com/onsi/gomega/gstruct"
)

var _ = Describe("Routes", func() {
	var (
		collectorSession        *gexec.Session
		collectorConfig         string
		uriInformerSession      *gexec.Session
		uriInformerConfig       string
		instanceInformerSession *gexec.Session
		instanceInformerConfig  string

		natsConfig *server.Options
		natsServer *server.Server
		natsClient *nats.Conn

		registerChan   chan *nats.Msg
		unregisterChan chan *nats.Msg

		lrp cf.DesireLRPRequest
	)

	BeforeEach(func() {
		registerChan = make(chan *nats.Msg)
		unregisterChan = make(chan *nats.Msg)

		natsConfig = getNatsServerConfig()
		// natstest.RunServer will panic after 10 seconds and that can't be changed
		Eventually(func() error {
			var err error
			natsServer, err = runNatsTestServer(natsConfig)
			return err
		}, "1m").Should(Succeed())
		natsClient = subscribeToNats(natsConfig, registerChan, unregisterChan)

		eiriniRouteConfig := eirini.RouteEmitterConfig{
			NatsPassword:        natsConfig.Password,
			NatsIP:              natsConfig.Host,
			NatsPort:            natsConfig.Port,
			EmitPeriodInSeconds: 1,
			KubeConfig: eirini.KubeConfig{
				ConfigPath: fixture.KubeConfigPath,
				Namespace:  fixture.DefaultNamespace,
			},
		}
		collectorSession, collectorConfig = eiriniBins.RouteCollector.Run(eiriniRouteConfig)
		uriInformerSession, uriInformerConfig = eiriniBins.RouteStatefulsetInformer.Run(eiriniRouteConfig)
		instanceInformerSession, instanceInformerConfig = eiriniBins.RoutePodInformer.Run(eiriniRouteConfig)

		lrp = cf.DesireLRPRequest{
			GUID:         util.GenerateGUID(),
			Version:      util.GenerateGUID(),
			NumInstances: 1,
			Routes: map[string]json.RawMessage{
				"cf-router": marshalRoutes([]routeInfo{
					{Hostname: "app-hostname-1", Port: 8080},
				}),
			},
			Ports: []int32{8080},
			Lifecycle: cf.Lifecycle{
				DockerLifecycle: &cf.DockerLifecycle{
					Image: "eirini/dorini",
				},
			},
		}
	})

	AfterEach(func() {
		if collectorSession != nil {
			collectorSession.Kill()
		}
		if uriInformerSession != nil {
			uriInformerSession.Kill()
		}
		if instanceInformerSession != nil {
			instanceInformerSession.Kill()
		}
		if natsServer != nil {
			natsServer.Shutdown()
		}
		if natsClient != nil {
			natsClient.Close()
		}
		Expect(os.Remove(collectorConfig)).To(Succeed())
		Expect(os.Remove(uriInformerConfig)).To(Succeed())
		Expect(os.Remove(instanceInformerConfig)).To(Succeed())
	})

	Describe("Desiring an app", func() {
		JustBeforeEach(func() {
			Expect(desireLRP(lrp).StatusCode).To(Equal(http.StatusAccepted))
		})

		It("continuously registers its routes", func() {
			var msg *nats.Msg

			for i := 0; i < 5; i++ {
				Eventually(registerChan).Should(Receive(&msg))
				var actualMessage route.RegistryMessage
				Expect(json.Unmarshal(msg.Data, &actualMessage)).To(Succeed())
				Expect(net.ParseIP(actualMessage.Host).IsUnspecified()).To(BeFalse())
				Expect(actualMessage.Port).To(BeNumerically("==", 8080))
				Expect(actualMessage.URIs).To(ConsistOf("app-hostname-1"))
				Expect(actualMessage.App).To(Equal(lrp.GUID))
				Expect(actualMessage.PrivateInstanceID).To(ContainSubstring(lrp.GUID))
			}
		})

		When("the app fails to start", func() {
			BeforeEach(func() {
				lrp.Lifecycle.DockerLifecycle.Image = "eirini/does-not-exist"
			})

			It("does not register routes", func() {
				Consistently(registerChan).ShouldNot(Receive())
			})
		})
	})

	Describe("Updating an app", func() {
		var (
			desiredRoutes []routeInfo
			emittedRoutes []string
			instances     int
		)

		appRoutes := func() []string {
			var (
				msg           *nats.Msg
				actualMessage route.RegistryMessage
			)

			Eventually(registerChan).Should(Receive(&msg))
			Expect(json.Unmarshal(msg.Data, &actualMessage)).To(Succeed())
			emittedRoutes = append(emittedRoutes, actualMessage.URIs...)
			return emittedRoutes
		}

		JustBeforeEach(func() {
			Expect(desireLRP(lrp).StatusCode).To(Equal(http.StatusAccepted))
			Eventually(registerChan).Should(Receive())

			resp, err := updateLRP(cf.UpdateDesiredLRPRequest{
				GUID:    lrp.GUID,
				Version: lrp.Version,
				Update: cf.DesiredLRPUpdate{
					Instances: instances,
					Routes: map[string]json.RawMessage{
						"cf-router": marshalRoutes(desiredRoutes),
					},
				},
			})
			Expect(err).NotTo(HaveOccurred())
			Expect(resp.StatusCode).To(Equal(http.StatusOK))
		})

		When("a new route is added to the app", func() {
			BeforeEach(func() {
				instances = lrp.NumInstances
				desiredRoutes = []routeInfo{
					{Hostname: "app-hostname-1", Port: 8080},
					{Hostname: "app-hostname-2", Port: 8080},
				}
			})

			It("registers the new route", func() {
				Eventually(appRoutes).Should(ContainElements("app-hostname-1", "app-hostname-2"))
			})
		})

		When("a route is removed from the app", func() {
			BeforeEach(func() {
				instances = lrp.NumInstances
				desiredRoutes = []routeInfo{}
			})

			It("unregisters the route", func() {
				Eventually(func() []string {
					var (
						msg           *nats.Msg
						actualMessage route.RegistryMessage
					)

					Eventually(unregisterChan).Should(Receive(&msg))
					Expect(json.Unmarshal(msg.Data, &actualMessage)).To(Succeed())
					return actualMessage.URIs
				}).Should(ConsistOf("app-hostname-1"))
			})
		})

		When("an app is scaled up", func() {

			BeforeEach(func() {
				instances = lrp.NumInstances + 1
				desiredRoutes = []routeInfo{
					{Hostname: "app-hostname-1", Port: 8080},
				}
			})

			It("registers the route for new instance", func() {
				Eventually(func() route.RegistryMessage {
					return receivedMessage(registerChan)
				}).Should(MatchFields(IgnoreExtras, Fields{
					"URIs":              ConsistOf("app-hostname-1"),
					"PrivateInstanceID": SatisfyAll(ContainSubstring(lrp.GUID), MatchRegexp("-1$")),
				}))

			})
		})

		When("an app is scaled down", func() {

			BeforeEach(func() {
				instances = 0
				desiredRoutes = []routeInfo{
					{Hostname: "app-hostname-1", Port: 8080},
				}
			})

			It("registers the route for new instance", func() {
				Eventually(func() route.RegistryMessage {
					return receivedMessage(unregisterChan)
				}).Should(MatchFields(IgnoreExtras, Fields{
					"URIs":              ConsistOf("app-hostname-1"),
					"PrivateInstanceID": SatisfyAll(ContainSubstring(lrp.GUID), MatchRegexp("-0$")),
				}))

			})
		})
	})

	Describe("Stopping an app", func() {

		JustBeforeEach(func() {
			Expect(desireLRP(lrp).StatusCode).To(Equal(http.StatusAccepted))
			Eventually(registerChan).Should(Receive())

			resp, err := stopLRP(httpClient, opiURL, lrp.GUID, lrp.Version)
			Expect(err).NotTo(HaveOccurred())
			Expect(resp.StatusCode).To(Equal(http.StatusOK))
		})

		It("unregisteres the app route", func() {
			var msg *nats.Msg
			Eventually(unregisterChan).Should(Receive(&msg))
			var actualMessage route.RegistryMessage
			Expect(json.Unmarshal(msg.Data, &actualMessage)).To(Succeed())
			Expect(actualMessage.URIs).To(ConsistOf("app-hostname-1"))
		})
	})

	Describe("Stopping an app instance", func() {

		JustBeforeEach(func() {
			Expect(desireLRP(lrp).StatusCode).To(Equal(http.StatusAccepted))
			Eventually(registerChan).Should(Receive())

			resp, err := stopLRPInstance(lrp.GUID, lrp.Version, 0)
			Expect(err).NotTo(HaveOccurred())
			Expect(resp.StatusCode).To(Equal(http.StatusOK))
		})

		It("unregisteres the app route", func() {
			Eventually(func() route.RegistryMessage {
				return receivedMessage(unregisterChan)
			}).Should(MatchFields(IgnoreExtras, Fields{
				"URIs":              ConsistOf("app-hostname-1"),
				"PrivateInstanceID": SatisfyAll(ContainSubstring(lrp.GUID), MatchRegexp("-0$")),
			}))
			Eventually(func() route.RegistryMessage {
				return receivedMessage(registerChan)
			}).Should(MatchFields(IgnoreExtras, Fields{
				"URIs":              ConsistOf("app-hostname-1"),
				"PrivateInstanceID": SatisfyAll(ContainSubstring(lrp.GUID), MatchRegexp("-0$")),
			}))
		})
	})

})

func receivedMessage(channel <-chan *nats.Msg) route.RegistryMessage {
	var (
		msg           *nats.Msg
		actualMessage route.RegistryMessage
	)

	Eventually(channel).Should(Receive(&msg))
	Expect(json.Unmarshal(msg.Data, &actualMessage)).To(Succeed())
	return actualMessage
}

func getNatsServerConfig() *server.Options {
	return &server.Options{
		Host:           "127.0.0.1",
		Port:           fixture.NextAvailablePort(),
		NoLog:          true,
		NoSigs:         true,
		MaxControlLine: 2048,
		Username:       "nats",
		Password:       "s3cr3t",
	}
}

func marshalRoutes(routes []routeInfo) json.RawMessage {
	bytes, err := json.Marshal(routes)
	Expect(err).NotTo(HaveOccurred())

	rawMessage := json.RawMessage{}
	Expect(rawMessage.UnmarshalJSON(bytes)).To(Succeed())
	return rawMessage
}

func subscribeToNats(natsConfig *server.Options, registerChan, unregisterChan chan *nats.Msg) *nats.Conn {
	natsClientConfig := nats.GetDefaultOptions()
	natsClientConfig.Servers = []string{fmt.Sprintf("%s:%d", natsConfig.Host, natsConfig.Port)}
	natsClientConfig.User = natsConfig.Username
	natsClientConfig.Password = natsConfig.Password
	natsClient, err := natsClientConfig.Connect()
	Expect(err).ToNot(HaveOccurred())

	_, err = natsClient.Subscribe("router.register", func(msg *nats.Msg) {
		registerChan <- msg
	})
	Expect(err).NotTo(HaveOccurred())

	_, err = natsClient.Subscribe("router.unregister", func(msg *nats.Msg) {
		unregisterChan <- msg
	})
	Expect(err).NotTo(HaveOccurred())

	return natsClient
}

func runNatsTestServer(opts *server.Options) (server *server.Server, err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("Failed to start test NATS server: %s", r)
		}
	}()

	server = natstest.RunServer(opts)
	return server, nil
}
