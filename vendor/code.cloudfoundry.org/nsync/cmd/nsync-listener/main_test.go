package main_test

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"code.cloudfoundry.org/bbs/models"
	"code.cloudfoundry.org/nsync"
	"code.cloudfoundry.org/nsync/config"
	"github.com/gogo/protobuf/proto"
	"github.com/hashicorp/consul/api"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"
	"github.com/onsi/gomega/ghttp"
	"github.com/onsi/gomega/types"
	"github.com/tedsuo/ifrit"
	"github.com/tedsuo/ifrit/ginkgomon"
	"github.com/tedsuo/rata"
)

var _ = Describe("Nsync Listener", func() {
	const exitDuration = 3 * time.Second

	var (
		nsyncPort int

		requestGenerator *rata.RequestGenerator
		httpClient       *http.Client
		response         *http.Response
		err              error

		process ifrit.Process
	)

	requestDesireWithInstances := func(nInstances int) (*http.Response, error) {
		req, err := requestGenerator.CreateRequest(nsync.DesireAppRoute, rata.Params{"process_guid": "the-guid"}, strings.NewReader(`{
        "process_guid": "the-guid",
        "droplet_uri": "http://the-droplet.uri.com",
        "start_command": "the-start-command",
        "execution_metadata": "execution-metadata-1",
        "memory_mb": 128,
        "disk_mb": 512,
        "file_descriptors": 32,
        "num_instances": `+strconv.Itoa(nInstances)+`,
        "stack": "some-stack",
        "log_guid": "the-log-guid",
        "health_check_timeout_in_seconds": 123456,
        "ports": [8080,5222],
        "etag": "2.1",
        "routing_info": {
			"http_routes": [
			{"hostname": "route-1"}
			],
			"tcp_routes": [
			{"router_group_guid": "guid-1", "external_port":5222, "container_port":60000}
			]
		}
			}`))
		Expect(err).NotTo(HaveOccurred())
		req.Header.Set("Content-Type", "application/json")

		return httpClient.Do(req)
	}

	BeforeEach(func() {
		nsyncPort = 8888 + GinkgoParallelNode()
		nsyncURL := fmt.Sprintf("http://127.0.0.1:%d", nsyncPort)

		requestGenerator = rata.NewRequestGenerator(nsyncURL, nsync.Routes)
		httpClient = http.DefaultClient

		runner := newNSyncRunner(fmt.Sprintf("127.0.0.1:%d", nsyncPort))
		process = ginkgomon.Invoke(runner)
	})

	AfterEach(func() {
		ginkgomon.Interrupt(process, exitDuration)
	})

	Describe("Desire an app", func() {
		var (
			desiredLRPGuid string
		)
		BeforeEach(func() {
			fakeBBS.RouteToHandler("POST", "/v1/desired_lrps/get_by_process_guid.r2",
				ghttp.RespondWith(200, ``),
			)

			fakeBBS.RouteToHandler("POST", "/v1/desired_lrp/desire.r2",
				ghttp.CombineHandlers(
					ghttp.VerifyContentType("application/x-protobuf"),
					func(w http.ResponseWriter, req *http.Request) {
						body, err := ioutil.ReadAll(req.Body)
						Expect(err).ShouldNot(HaveOccurred())
						defer req.Body.Close()

						protoMessage := &models.DesireLRPRequest{}
						err = proto.Unmarshal(body, protoMessage)
						Expect(err).ToNot(HaveOccurred(), "Failed to unmarshal protobuf")

						desiredLRPGuid = protoMessage.DesiredLrp.ProcessGuid
					},
				),
			)

			response, err = requestDesireWithInstances(3)
			Expect(err).NotTo(HaveOccurred())
		})

		It("sends the app desire to the BBS", func() {
			Expect(response.StatusCode).To(Equal(http.StatusAccepted))
			Eventually(func() string { return desiredLRPGuid }, 10*time.Second).Should(Equal("the-guid"))
		})
	})

	Describe("Stop an app", func() {
		stopApp := func(guid string) (*http.Response, error) {
			req, err := requestGenerator.CreateRequest(nsync.StopAppRoute, rata.Params{"process_guid": guid}, nil)
			Expect(err).NotTo(HaveOccurred())

			return httpClient.Do(req)
		}

		It("forwards the stop request to the BBS", func() {
			deletedTheLRP := false

			fakeBBS.RouteToHandler("POST", "/v1/desired_lrp/remove",
				ghttp.CombineHandlers(
					ghttp.VerifyContentType("application/x-protobuf"),
					func(w http.ResponseWriter, req *http.Request) {
						body, err := ioutil.ReadAll(req.Body)
						Expect(err).ShouldNot(HaveOccurred())
						defer req.Body.Close()

						protoMessage := &models.RemoveDesiredLRPRequest{}
						err = proto.Unmarshal(body, protoMessage)
						Expect(err).ToNot(HaveOccurred(), "Failed to unmarshal protobuf")

						if protoMessage.ProcessGuid == "the-guid" {
							deletedTheLRP = true
						}
					},
				),
			)

			stopResponse, err := stopApp("the-guid")
			Expect(err).NotTo(HaveOccurred())

			Expect(stopResponse.StatusCode).To(Equal(http.StatusAccepted))
			Expect(deletedTheLRP).To(BeTrue())
		})
	})

	Describe("Kill an app instance", func() {
		killIndex := func(guid string, index int) (*http.Response, error) {
			req, err := requestGenerator.CreateRequest(nsync.KillIndexRoute, rata.Params{"process_guid": "the-guid", "index": strconv.Itoa(index)}, nil)
			Expect(err).NotTo(HaveOccurred())

			return httpClient.Do(req)
		}

		It("forwards an index kill to the BBS", func() {
			actualLRPKey := models.ActualLRPKey{ProcessGuid: "the-guid", Index: 7}

			fakeBBS.RouteToHandler("POST", "/v1/actual_lrp_groups/get_by_process_guid_and_index",
				ghttp.RespondWithProto(200, &models.ActualLRPGroupResponse{
					ActualLrpGroup: &models.ActualLRPGroup{
						Instance: &models.ActualLRP{ActualLRPKey: actualLRPKey},
					},
				}),
			)

			fakeBBS.RouteToHandler("POST", "/v1/actual_lrps/retire",
				ghttp.CombineHandlers(
					ghttp.VerifyContentType("application/x-protobuf"),
					ghttp.VerifyProtoRepresenting(&models.RetireActualLRPRequest{
						ActualLrpKey: &actualLRPKey,
					}),
				),
			)

			resp, err := killIndex("the-guid", 7)
			Expect(err).NotTo(HaveOccurred())
			Expect(resp.StatusCode).To(Equal(http.StatusAccepted))

			Expect(fakeBBS).To(HaveReceivedRequest("/v1/actual_lrps/retire"))
		})
	})

	Describe("Desire a task", func() {
		It("forwards the desire request to the BBS", func() {
			desiredTheTask := false

			fakeBBS.RouteToHandler("POST", "/v1/tasks/desire.r2",
				ghttp.CombineHandlers(
					ghttp.VerifyContentType("application/x-protobuf"),
					func(w http.ResponseWriter, req *http.Request) {
						body, err := ioutil.ReadAll(req.Body)
						Expect(err).ShouldNot(HaveOccurred())
						defer req.Body.Close()

						protoMessage := &models.DesireTaskRequest{}
						err = proto.Unmarshal(body, protoMessage)
						Expect(err).ToNot(HaveOccurred(), "Failed to unmarshal protobuf")

						if protoMessage.TaskGuid == "the-guid" {
							desiredTheTask = true
						}
					},
				),
			)

			req, err := requestGenerator.CreateRequest(nsync.TasksRoute, rata.Params{}, strings.NewReader(`{
				"task_guid": "the-guid",
				"droplet_uri": "http://the-droplet.uri.com",
				"command": "the-start-command",
				"memory_mb": 128,
				"disk_mb": 512,
				"rootfs": "some-stack",
				"log_guid": "the-log-guid",
				"completion_callback": "http://google.com",
				"lifecycle": "buildpack",
				"log_source": "APP/TASK/my-task"
			}`))

			Expect(err).NotTo(HaveOccurred())
			req.Header.Set("Content-Type", "application/json")

			response, err = httpClient.Do(req)

			Expect(err).NotTo(HaveOccurred())
			Expect(response.StatusCode).To(Equal(http.StatusAccepted))

			Expect(desiredTheTask).To(BeTrue())
		})
	})
})

var _ = Describe("Nsync Listener Initialization", func() {
	const exitDuration = 3 * time.Second

	var (
		nsyncPort int

		runner  *ginkgomon.Runner
		process ifrit.Process
	)

	BeforeEach(func() {
		nsyncPort = 8888 + GinkgoParallelNode()
		nsyncAddress := fmt.Sprintf("127.0.0.1:%d", nsyncPort)

		runner = newNSyncRunner(nsyncAddress)
	})

	JustBeforeEach(func() {
		process = ifrit.Invoke(runner)
	})

	AfterEach(func() {
		ginkgomon.Interrupt(process, exitDuration)
	})

	Describe("Flag validation", func() {
		Context("when the listenAddress does not match host:port pattern", func() {
			BeforeEach(func() {
				runner = newNSyncRunner("portless")
			})

			It("exits with an error", func() {
				Eventually(runner).Should(gexec.Exit(2))
				Expect(runner.Buffer()).Should(gbytes.Say("missing port"))
			})
		})

		Context("when the listenAddress port is not a number or recognized service", func() {
			BeforeEach(func() {
				runner = newNSyncRunner("127.0.0.1:onehundred")
			})

			It("exits with an error", func() {
				Eventually(runner).Should(gexec.Exit(2))
				Expect(runner.Buffer()).Should(gbytes.Say("nsync-listener.failed-invalid-listen-port"))
			})
		})
	})

	Describe("Initialization", func() {
		It("registers itself with consul", func() {
			services, err := consulClient.Agent().Services()
			Expect(err).ToNot(HaveOccurred())

			Expect(services).To(HaveKeyWithValue("nsync",
				&api.AgentService{
					Service: "nsync",
					ID:      "nsync",
					Port:    nsyncPort,
				}))
		})

		It("registers a TTL healthcheck", func() {
			checks, err := consulClient.Agent().Checks()
			Expect(err).ToNot(HaveOccurred())

			Expect(checks).To(HaveKeyWithValue("service:nsync",
				&api.AgentCheck{
					Node:        "0",
					CheckID:     "service:nsync",
					Name:        "Service 'nsync' check",
					Status:      "passing",
					ServiceID:   "nsync",
					ServiceName: "nsync",
				}))
		})
	})
})

var newNSyncRunner = func(nsyncListenAddress string) *ginkgomon.Runner {
	configFile, err := ioutil.TempFile("", "listener_config")
	Expect(err).NotTo(HaveOccurred())

	listenerConfig := config.DefaultListenerConfig()
	listenerConfig.BBSAddress = fakeBBS.URL()
	listenerConfig.ListenAddress = nsyncListenAddress
	listenerConfig.Lifecycles = []string{
		"buildpack/some-stack:some-health-check.tar.gz",
		"docker:the/docker/lifecycle/path.tgz",
	}
	listenerConfig.FileServerURL = "http://file-server.com"
	listenerConfig.LagerConfig.LogLevel = "debug"
	listenerConfig.ConsulCluster = consulRunner.ConsulCluster()

	listenerJSON, err := json.Marshal(listenerConfig)
	Expect(err).NotTo(HaveOccurred())
	err = ioutil.WriteFile(configFile.Name(), listenerJSON, 0644)
	Expect(err).NotTo(HaveOccurred())

	return ginkgomon.New(ginkgomon.Config{
		Name:          "nsync",
		AnsiColorCode: "97m",
		StartCheck:    "nsync.listener.started",
		Command: exec.Command(
			listenerPath,
			"-configPath", configFile.Name(),
		),
	})
}

func HavePath(path string) types.GomegaMatcher {
	return WithTransform(func(r *http.Request) string {
		return r.URL.Path
	}, Equal(path))
}

type HaveReceivedRequestMatcher struct {
	expectedPath string
}

func (m HaveReceivedRequestMatcher) Match(actual interface{}) (bool, error) {
	server, ok := actual.(*ghttp.Server)

	if !ok {
		return false, fmt.Errorf("HaveReceivedRequest matcher expects a *ghttp.Server")
	}

	for _, r := range server.ReceivedRequests() {
		if r.URL.Path == m.expectedPath {
			return true, nil
		}
	}

	return false, nil
}

func (m HaveReceivedRequestMatcher) FailureMessage(interface{}) string {
	return fmt.Sprintf("Expected server to have received request \"%s\", but it did not.", m.expectedPath)
}

func (m HaveReceivedRequestMatcher) NegatedFailureMessage(interface{}) string {
	return fmt.Sprintf("Expected server to not have received request \"%s\", but it did.", m.expectedPath)
}

func HaveReceivedRequest(path string) types.GomegaMatcher {
	return HaveReceivedRequestMatcher{path}
}
