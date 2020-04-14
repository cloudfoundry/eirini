package eats_test

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"net"
	"net/http"
	"os"

	"code.cloudfoundry.org/bbs/models"
	"code.cloudfoundry.org/eirini"
	"code.cloudfoundry.org/eirini/models/cf"
	"code.cloudfoundry.org/eirini/route"
	"github.com/nats-io/nats-server/v2/server"
	natstest "github.com/nats-io/nats-server/v2/test"
	"github.com/nats-io/nats.go"
	. "github.com/onsi/ginkgo"
	ginkgoconfig "github.com/onsi/ginkgo/config"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"
)

type routeInfo struct {
	Hostname string `json:"hostname"`
	Port     int    `json:"port"`
}

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
		natsServer = natstest.RunServer(natsConfig)
		natsClient = subscribeToNats(natsConfig, registerChan, unregisterChan)

		eiriniRouteConfig := eirini.RouteEmitterConfig{
			NatsPassword:        natsConfig.Password,
			NatsIP:              natsConfig.Host,
			NatsPort:            natsConfig.Port,
			EmitPeriodInSeconds: 1,
			KubeConfig: eirini.KubeConfig{
				ConfigPath: fixture.KubeConfigPath,
				Namespace:  fixture.Namespace,
			},
		}
		collectorSession, collectorConfig = runBinary("code.cloudfoundry.org/eirini/cmd/route-collector", eiriniRouteConfig)
		uriInformerSession, uriInformerConfig = runBinary("code.cloudfoundry.org/eirini/cmd/route-statefulset-informer", eiriniRouteConfig)
		instanceInformerSession, instanceInformerConfig = runBinary("code.cloudfoundry.org/eirini/cmd/route-pod-informer", eiriniRouteConfig)

		lrp = cf.DesireLRPRequest{
			GUID:         "the-app-guid",
			Version:      "the-version",
			NumInstances: 1,
			Routes: map[string]*json.RawMessage{
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
			resp, err := desireLRP(httpClient, opiURL, lrp)
			Expect(err).NotTo(HaveOccurred())
			Expect(resp.StatusCode).To(Equal(http.StatusAccepted))
		})

		It("continuously registers its routes", func() {
			var msg *nats.Msg

			for i := 0; i < 5; i++ {
				Eventually(registerChan, "15s").Should(Receive(&msg))
				var actualMessage route.RegistryMessage
				Expect(json.Unmarshal(msg.Data, &actualMessage)).To(Succeed())
				Expect(net.ParseIP(actualMessage.Host).IsUnspecified()).To(BeFalse())
				Expect(actualMessage.Port).To(BeNumerically("==", 8080))
				Expect(actualMessage.URIs).To(ConsistOf("app-hostname-1"))
				Expect(actualMessage.App).To(Equal("the-app-guid"))
				Expect(actualMessage.PrivateInstanceID).To(ContainSubstring("the-app-guid"))
			}
		})

		When("the app fails to start", func() {
			BeforeEach(func() {
				lrp.Lifecycle.DockerLifecycle.Image = "eirini/does-not-exist"
			})

			It("does not register routes", func() {
				Consistently(registerChan, "5s").ShouldNot(Receive())
			})
		})
	})

	Describe("Updating an app", func() {
		var (
			desiredRoutes []routeInfo
			emittedRoutes []string
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
			resp, err := desireLRP(httpClient, opiURL, lrp)
			Expect(err).NotTo(HaveOccurred())
			Expect(resp.StatusCode).To(Equal(http.StatusAccepted))

			resp, err = updateLRP(httpClient, opiURL, cf.UpdateDesiredLRPRequest{
				GUID:    lrp.GUID,
				Version: lrp.Version,
				UpdateDesiredLRPRequest: models.UpdateDesiredLRPRequest{
					Update: &models.DesiredLRPUpdate{
						OptionalInstances: &models.DesiredLRPUpdate_Instances{
							Instances: int32(lrp.NumInstances),
						},
						Routes: &models.Routes{
							"cf-router": marshalRoutes(desiredRoutes),
						},
					},
				},
			})
			Expect(err).NotTo(HaveOccurred())
			Expect(resp.StatusCode).To(Equal(http.StatusOK))
		})

		When("a new route is added to the app", func() {
			BeforeEach(func() {
				desiredRoutes = []routeInfo{
					{Hostname: "app-hostname-1", Port: 8080},
					{Hostname: "app-hostname-2", Port: 8080},
				}
			})

			It("registers the new route", func() {
				Eventually(appRoutes).Should(ConsistOf("app-hostname-1", "app-hostname-2"))
			})
		})
	})
})

func getNatsServerConfig() *server.Options {
	return &server.Options{
		Host:           "127.0.0.1",
		Port:           51000 + rand.Intn(1000) + ginkgoconfig.GinkgoConfig.ParallelNode,
		NoLog:          true,
		NoSigs:         true,
		MaxControlLine: 2048,
		Username:       "nats",
		Password:       "s3cr3t",
	}
}

func marshalRoutes(routes []routeInfo) *json.RawMessage {
	bytes, err := json.Marshal(routes)
	Expect(err).NotTo(HaveOccurred())

	rawMessage := &json.RawMessage{}
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
