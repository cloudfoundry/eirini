package eats_test

import (
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"time"

	"code.cloudfoundry.org/eirini/models/cf"
	"code.cloudfoundry.org/eirini/route"
	"code.cloudfoundry.org/eirini/tests"
	"github.com/nats-io/nats-server/v2/server"
	"github.com/nats-io/nats.go"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
)

var _ = Describe("Routes", func() {
	var (
		natsConfig *server.Options
		natsClient *nats.Conn

		registerChan   chan *nats.Msg
		unregisterChan chan *nats.Msg

		lrp cf.DesireLRPRequest
	)

	BeforeEach(func() {
		registerChan = make(chan *nats.Msg)
		unregisterChan = make(chan *nats.Msg)

		natsConfig = getNatsServerConfig()
		natsClient = subscribeToNats(natsConfig, registerChan, unregisterChan)

		lrp = cf.DesireLRPRequest{
			GUID:         tests.GenerateGUID(),
			Version:      tests.GenerateGUID(),
			NumInstances: 1,
			Namespace:    fixture.Namespace,
			DiskMB:       400,
			Routes: map[string]json.RawMessage{
				"cf-router": tests.MarshalRoutes([]tests.RouteInfo{
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
		if natsClient != nil {
			natsClient.Close()
		}
	})

	Describe("Desiring an app", func() {
		JustBeforeEach(func() {
			Expect(desireLRP(lrp)).To(Equal(http.StatusAccepted))
		})

		It("continuously registers its routes", func() {
			count := 0

			Eventually(func() (int, error) {
				msg := <-registerChan
				var actualMessage route.RegistryMessage
				Expect(json.Unmarshal(msg.Data, &actualMessage)).To(Succeed())
				if actualMessage.App != lrp.GUID {
					return count, nil
				}

				Expect(net.ParseIP(actualMessage.Host).IsUnspecified()).To(BeFalse())
				Expect(actualMessage.Port).To(BeNumerically("==", 8080))
				Expect(actualMessage.URIs).To(ConsistOf("app-hostname-1"))
				Expect(actualMessage.PrivateInstanceID).To(ContainSubstring(lrp.GUID))
				count++

				return count, nil
			}).Should(BeNumerically(">", 2))
		})

		When("the app fails to start", func() {
			BeforeEach(func() {
				lrp.Lifecycle.DockerLifecycle.Image = "eirini/does-not-exist"
			})

			It("does not register routes", func() {
				Consistently(func() bool {
					select {
					case msg := <-registerChan:
						var actualMessage route.RegistryMessage
						Expect(json.Unmarshal(msg.Data, &actualMessage)).To(Succeed())

						return actualMessage.App != lrp.GUID
					case <-time.After(100 * time.Millisecond):
						return true
					}
				}).Should(BeTrue())
			})
		})
	})

	Describe("Updating an app", func() {
		var (
			desiredRoutes []tests.RouteInfo
			instances     int
		)

		JustBeforeEach(func() {
			Expect(desireLRP(lrp)).To(Equal(http.StatusAccepted))
			Eventually(func() string {
				msg := receivedMessage(registerChan)

				return msg.App
			}).Should(Equal(lrp.GUID))

			resp, err := updateLRP(cf.UpdateDesiredLRPRequest{
				GUID:    lrp.GUID,
				Version: lrp.Version,
				Update: cf.DesiredLRPUpdate{
					Instances: instances,
					Routes: map[string]json.RawMessage{
						"cf-router": tests.MarshalRoutes(desiredRoutes),
					},
				},
			})
			Expect(err).NotTo(HaveOccurred())
			Expect(resp.StatusCode).To(Equal(http.StatusOK))
		})

		When("a new route is added to the app", func() {
			BeforeEach(func() {
				instances = lrp.NumInstances
				desiredRoutes = []tests.RouteInfo{
					{Hostname: "app-hostname-1", Port: 8080},
					{Hostname: "app-hostname-2", Port: 8080},
				}
			})

			It("registers the new route", func() {
				Eventually(func() route.RegistryMessage {
					return receivedMessage(registerChan)
				}).Should(MatchFields(IgnoreExtras, Fields{
					"App":  Equal(lrp.GUID),
					"URIs": ConsistOf("app-hostname-1", "app-hostname-2"),
				}))
			})
		})

		When("a route is removed from the app", func() {
			BeforeEach(func() {
				instances = lrp.NumInstances
				desiredRoutes = []tests.RouteInfo{}
			})

			It("unregisters the route", func() {
				Eventually(func() route.RegistryMessage {
					return receivedMessage(unregisterChan)
				}).Should(MatchFields(IgnoreExtras, Fields{
					"App":  Equal(lrp.GUID),
					"URIs": ConsistOf("app-hostname-1"),
				}))
			})
		})

		When("an app is scaled up", func() {
			BeforeEach(func() {
				instances = lrp.NumInstances + 1
				desiredRoutes = []tests.RouteInfo{
					{Hostname: "app-hostname-1", Port: 8080},
				}
			})

			It("registers the route for new instance", func() {
				Eventually(func() route.RegistryMessage {
					return receivedMessage(registerChan)
				}).Should(MatchFields(IgnoreExtras, Fields{
					"App":               Equal(lrp.GUID),
					"URIs":              ConsistOf("app-hostname-1"),
					"PrivateInstanceID": SatisfyAll(ContainSubstring(lrp.GUID), MatchRegexp("-1$")),
				}))
			})
		})

		When("an app is scaled down", func() {
			BeforeEach(func() {
				instances = 0
				desiredRoutes = []tests.RouteInfo{
					{Hostname: "app-hostname-1", Port: 8080},
				}
			})

			It("registers the route for new instance", func() {
				Eventually(func() route.RegistryMessage {
					return receivedMessage(unregisterChan)
				}).Should(MatchFields(IgnoreExtras, Fields{
					"App":               Equal(lrp.GUID),
					"URIs":              ConsistOf("app-hostname-1"),
					"PrivateInstanceID": SatisfyAll(ContainSubstring(lrp.GUID), MatchRegexp("-0$")),
				}))
			})
		})
	})

	Describe("Stopping an app", func() {
		JustBeforeEach(func() {
			Expect(desireLRP(lrp)).To(Equal(http.StatusAccepted))
			Eventually(func() string {
				msg := receivedMessage(registerChan)

				return msg.App
			}).Should(Equal(lrp.GUID))

			resp, err := stopLRP(lrp.GUID, lrp.Version)
			Expect(err).NotTo(HaveOccurred())
			Expect(resp.StatusCode).To(Equal(http.StatusOK))
		})

		It("unregisters the app route", func() {
			Eventually(func() route.RegistryMessage {
				return receivedMessage(unregisterChan)
			}).Should(MatchFields(IgnoreExtras, Fields{
				"App":  Equal(lrp.GUID),
				"URIs": ConsistOf("app-hostname-1"),
			}))
		})
	})

	Describe("Stopping an app instance", func() {
		JustBeforeEach(func() {
			Expect(desireLRP(lrp)).To(Equal(http.StatusAccepted))
			Eventually(func() string {
				msg := receivedMessage(registerChan)

				return msg.App
			}).Should(Equal(lrp.GUID))

			resp, err := stopLRPInstance(lrp.GUID, lrp.Version, 0)
			Expect(err).NotTo(HaveOccurred())
			Expect(resp.StatusCode).To(Equal(http.StatusOK))
		})

		It("unregisters the app route", func() {
			Eventually(func() route.RegistryMessage {
				return receivedMessage(unregisterChan)
			}).Should(MatchFields(IgnoreExtras, Fields{
				"App":               Equal(lrp.GUID),
				"URIs":              ConsistOf("app-hostname-1"),
				"PrivateInstanceID": SatisfyAll(ContainSubstring(lrp.GUID), MatchRegexp("-0$")),
			}))
			Eventually(func() route.RegistryMessage {
				return receivedMessage(registerChan)
			}).Should(MatchFields(IgnoreExtras, Fields{
				"App":               Equal(lrp.GUID),
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
		Host:           fmt.Sprintf("nats-client.%s.svc.cluster.local", tests.GetEiriniSystemNamespace()),
		Port:           4222,
		NoLog:          true,
		NoSigs:         true,
		MaxControlLine: 2048,
		Username:       "nats",
		Password:       fixture.GetNATSPassword(),
	}
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
