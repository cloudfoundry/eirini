package handlers_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"net/url"

	"code.cloudfoundry.org/bbs/fake_bbs"
	"code.cloudfoundry.org/bbs/models"
	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/lager/lagertest"
	"code.cloudfoundry.org/nsync/bulk/fakes"
	"code.cloudfoundry.org/nsync/handlers"
	"code.cloudfoundry.org/nsync/recipebuilder"
	"code.cloudfoundry.org/runtimeschema/cc_messages"
	"github.com/cloudfoundry-incubator/routing-info/cfroutes"
	"github.com/cloudfoundry/dropsonde/metric_sender/fake"
	"github.com/cloudfoundry/dropsonde/metrics"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
)

var _ = Describe("DesireAppHandler", func() {
	var (
		logger           *lagertest.TestLogger
		fakeBBS          *fake_bbs.FakeClient
		buildpackBuilder *fakes.FakeRecipeBuilder
		dockerBuilder    *fakes.FakeRecipeBuilder
		desireAppRequest cc_messages.DesireAppRequestFromCC
		metricSender     *fake.FakeMetricSender

		request          *http.Request
		responseRecorder *httptest.ResponseRecorder
	)

	BeforeEach(func() {
		var err error

		logger = lagertest.NewTestLogger("test")
		fakeBBS = new(fake_bbs.FakeClient)
		buildpackBuilder = new(fakes.FakeRecipeBuilder)
		dockerBuilder = new(fakes.FakeRecipeBuilder)

		routingInfo, err := cc_messages.CCHTTPRoutes{
			{Hostname: "route1"},
			{Hostname: "route2"},
		}.CCRouteInfo()
		Expect(err).NotTo(HaveOccurred())

		desireAppRequest = cc_messages.DesireAppRequestFromCC{
			ProcessGuid:  "some-guid",
			DropletUri:   "http://the-droplet.uri.com",
			Stack:        "some-stack",
			StartCommand: "the-start-command",
			Environment: []*models.EnvironmentVariable{
				{Name: "foo", Value: "bar"},
				{Name: "VCAP_APPLICATION", Value: "{\"application_name\":\"my-app\"}"},
			},
			MemoryMB:        128,
			DiskMB:          512,
			FileDescriptors: 32,
			NumInstances:    2,
			RoutingInfo:     routingInfo,
			LogGuid:         "some-log-guid",
			ETag:            "last-modified-etag",
		}

		metricSender = fake.NewFakeMetricSender()
		metrics.Initialize(metricSender, nil)

		responseRecorder = httptest.NewRecorder()

		request, err = http.NewRequest("POST", "", nil)
		Expect(err).NotTo(HaveOccurred())
		request.Form = url.Values{
			":process_guid": []string{"some-guid"},
		}
	})

	JustBeforeEach(func() {
		if request.Body == nil {
			jsonBytes, err := json.Marshal(&desireAppRequest)
			Expect(err).NotTo(HaveOccurred())
			reader := bytes.NewReader(jsonBytes)

			request.Body = ioutil.NopCloser(reader)
		}

		handler := handlers.NewDesireAppHandler(logger, fakeBBS, map[string]recipebuilder.RecipeBuilder{
			"buildpack": buildpackBuilder,
			"docker":    dockerBuilder,
		})
		handler.DesireApp(responseRecorder, request)
	})

	Context("when the desired LRP does not exist", func() {
		var newlyDesiredLRP *models.DesiredLRP

		BeforeEach(func() {
			newlyDesiredLRP = &models.DesiredLRP{
				ProcessGuid: "new-process-guid",
				Instances:   1,
				RootFs:      models.PreloadedRootFS("stack-2"),
				Action: models.WrapAction(&models.RunAction{
					User: "me",
					Path: "ls",
				}),
				Annotation: "last-modified-etag",
			}

			fakeBBS.DesiredLRPByProcessGuidReturns(&models.DesiredLRP{}, models.ErrResourceNotFound)
			buildpackBuilder.BuildReturns(newlyDesiredLRP, nil)
		})

		It("logs the incoming and outgoing request", func() {
			Eventually(logger.TestSink.Buffer).Should(gbytes.Say("request-from-cc"))
			Eventually(logger.TestSink.Buffer).Should(gbytes.Say("creating-desired-lrp"))
		})

		It("creates the desired LRP", func() {
			Expect(fakeBBS.DesireLRPCallCount()).To(Equal(1))

			Expect(fakeBBS.DesiredLRPByProcessGuidCallCount()).To(Equal(1))
			_, desiredLRP := fakeBBS.DesireLRPArgsForCall(0)
			Expect(desiredLRP).To(Equal(newlyDesiredLRP))

			Expect(buildpackBuilder.BuildArgsForCall(0)).To(Equal(&desireAppRequest))
		})

		It("responds with 202 Accepted", func() {
			Expect(responseRecorder.Code).To(Equal(http.StatusAccepted))
		})

		It("increments the desired LRPs counter", func() {
			Expect(metricSender.GetCounter("LRPsDesired")).To(Equal(uint64(1)))
		})

		Context("when the bbs fails", func() {
			BeforeEach(func() {
				fakeBBS.DesireLRPReturns(errors.New("oh no"))
			})

			It("responds with a ServiceUnavailabe error", func() {
				Expect(responseRecorder.Code).To(Equal(http.StatusServiceUnavailable))
			})
		})

		Context("when the bbs fails with a Conflict error", func() {
			BeforeEach(func() {
				fakeBBS.DesireLRPStub = func(_ lager.Logger, _ *models.DesiredLRP) error {
					fakeBBS.DesiredLRPByProcessGuidReturns(&models.DesiredLRP{
						ProcessGuid: "some-guid",
					}, nil)
					return models.ErrResourceExists
				}
			})

			It("retries", func() {
				Expect(fakeBBS.DesireLRPCallCount()).To(Equal(1))
				Expect(fakeBBS.UpdateDesiredLRPCallCount()).To(Equal(1))
			})

			It("suceeds if the second try is sucessful", func() {
				Expect(responseRecorder.Code).To(Equal(http.StatusAccepted))
			})

			Context("when updating the desired LRP fails with a conflict error", func() {
				BeforeEach(func() {
					fakeBBS.UpdateDesiredLRPReturns(models.ErrResourceConflict)
				})

				It("fails with a 409 Conflict if the second try is unsuccessful", func() {
					Expect(responseRecorder.Code).To(Equal(http.StatusConflict))
				})
			})
		})

		Context("when building the recipe fails to build", func() {
			BeforeEach(func() {
				buildpackBuilder.BuildReturns(nil, recipebuilder.ErrDropletSourceMissing)
			})

			It("logs an error", func() {
				Eventually(logger.TestSink.Buffer).Should(gbytes.Say("failed-to-build-recipe"))
				Eventually(logger.TestSink.Buffer).Should(gbytes.Say(recipebuilder.ErrDropletSourceMissing.Message))
			})

			It("does not desire the LRP", func() {
				Consistently(fakeBBS.RemoveDesiredLRPCallCount).Should(Equal(0))
			})

			It("responds with 400 Bad Request", func() {
				Expect(responseRecorder.Code).To(Equal(http.StatusBadRequest))
			})
		})

		Context("when the LRP has docker image", func() {
			var newlyDesiredDockerLRP *models.DesiredLRP

			BeforeEach(func() {
				desireAppRequest.DropletUri = ""
				desireAppRequest.DockerImageUrl = "docker:///user/repo#tag"

				newlyDesiredDockerLRP = &models.DesiredLRP{
					ProcessGuid: "new-process-guid",
					Instances:   1,
					RootFs:      "docker:///user/repo#tag",
					Action: models.WrapAction(&models.RunAction{
						User: "me",
						Path: "ls",
					}),
					Annotation: "last-modified-etag",
				}

				dockerBuilder.BuildReturns(newlyDesiredDockerLRP, nil)
			})

			It("creates the desired LRP", func() {
				Expect(fakeBBS.DesireLRPCallCount()).To(Equal(1))

				Expect(fakeBBS.DesiredLRPByProcessGuidCallCount()).To(Equal(1))
				_, desiredLRP := fakeBBS.DesireLRPArgsForCall(0)
				Expect(desiredLRP).To(Equal(newlyDesiredDockerLRP))

				Expect(dockerBuilder.BuildArgsForCall(0)).To(Equal(&desireAppRequest))
			})

			It("responds with 202 Accepted", func() {
				Expect(responseRecorder.Code).To(Equal(http.StatusAccepted))
			})

			It("increments the desired LRPs counter", func() {
				Expect(metricSender.GetCounter("LRPsDesired")).To(Equal(uint64(1)))
			})
		})
	})

	Context("when desired LRP already exists", func() {
		var opaqueRoutingMessage json.RawMessage

		BeforeEach(func() {
			buildpackBuilder.ExtractExposedPortsStub = func(ccRequest *cc_messages.DesireAppRequestFromCC) ([]uint32, error) {
				return []uint32{8080}, nil
			}

			cfRoute := cfroutes.CFRoute{
				Hostnames: []string{"route1"},
				Port:      8080,
			}
			cfRoutePayload, err := json.Marshal(cfRoute)
			Expect(err).NotTo(HaveOccurred())

			cfRouteMessage := json.RawMessage(cfRoutePayload)
			opaqueRoutingMessage = json.RawMessage([]byte(`{"some": "value"}`))

			fakeBBS.DesiredLRPByProcessGuidReturns(&models.DesiredLRP{
				ProcessGuid: "some-guid",
				Routes: &models.Routes{
					cfroutes.CF_ROUTER:        &cfRouteMessage,
					"some-other-routing-data": &opaqueRoutingMessage,
				},
			}, nil)
		})

		It("logs the incoming and outgoing request", func() {
			Eventually(logger.TestSink.Buffer).Should(gbytes.Say("request-from-cc"))
			Eventually(logger.TestSink.Buffer).Should(gbytes.Say("updating-desired-lrp"))
		})

		It("checks to see if LRP already exists", func() {
			Eventually(fakeBBS.DesiredLRPByProcessGuidCallCount).Should(Equal(1))
		})

		opaqueRoutingDataCheck := func(expectedRoutes cfroutes.CFRoutes) {
			Eventually(fakeBBS.UpdateDesiredLRPCallCount).Should(Equal(1))

			_, processGuid, updateRequest := fakeBBS.UpdateDesiredLRPArgsForCall(0)
			Expect(processGuid).To(Equal("some-guid"))
			Expect(*updateRequest.Instances).To(BeEquivalentTo(2))
			Expect(*updateRequest.Annotation).To(Equal("last-modified-etag"))

			cfJson := (*updateRequest.Routes)[cfroutes.CF_ROUTER]
			otherJson := (*updateRequest.Routes)["some-other-routing-data"]

			var cfRoutes cfroutes.CFRoutes
			err := json.Unmarshal(*cfJson, &cfRoutes)
			Expect(err).NotTo(HaveOccurred())

			Expect(cfRoutes).To(ConsistOf(expectedRoutes))
			Expect(cfRoutes).To(HaveLen(len(expectedRoutes)))
			Expect(otherJson).To(Equal(&opaqueRoutingMessage))
		}

		It("updates the LRP without stepping on opaque routing data", func() {
			expected := cfroutes.CFRoutes{
				{Hostnames: []string{"route1", "route2"}, Port: 8080},
			}
			opaqueRoutingDataCheck(expected)
		})

		It("responds with 202 Accepted", func() {
			Expect(responseRecorder.Code).To(Equal(http.StatusAccepted))
		})

		It("uses buildpack builder", func() {
			Expect(dockerBuilder.ExtractExposedPortsCallCount()).To(Equal(0))
			Expect(buildpackBuilder.ExtractExposedPortsCallCount()).To(Equal(1))

			Expect(buildpackBuilder.ExtractExposedPortsArgsForCall(0)).To(Equal(&desireAppRequest))
		})

		Context("when multiple routes with same route service are sent", func() {
			var routesToEmit cfroutes.CFRoutes
			BeforeEach(func() {
				routingInfo, err := cc_messages.CCHTTPRoutes{
					{Hostname: "route1", RouteServiceUrl: "https://rs.example.com"},
					{Hostname: "route2", RouteServiceUrl: "https://rs.example.com"},
					{Hostname: "route3"},
				}.CCRouteInfo()
				Expect(err).NotTo(HaveOccurred())

				desireAppRequest.RoutingInfo = routingInfo
			})

			It("aggregates the http routes with the same route service url", func() {
				routesToEmit = cfroutes.CFRoutes{
					{Hostnames: []string{"route1", "route2"}, Port: 8080, RouteServiceUrl: "https://rs.example.com"},
					{Hostnames: []string{"route3"}, Port: 8080},
				}
				opaqueRoutingDataCheck(routesToEmit)
			})
		})

		Context("when multiple routes with different route service are sent", func() {
			var routesToEmit cfroutes.CFRoutes
			BeforeEach(func() {
				routingInfo, err := cc_messages.CCHTTPRoutes{
					{Hostname: "route1", RouteServiceUrl: "https://rs.example.com"},
					{Hostname: "route2", RouteServiceUrl: "https://rs.example.com"},
					{Hostname: "route3"},
					{Hostname: "route4", RouteServiceUrl: "https://rs.other.com"},
					{Hostname: "route5", RouteServiceUrl: "https://rs.other.com"},
					{Hostname: "route6"},
					{Hostname: "route7", RouteServiceUrl: "https://rs.another.com"},
				}.CCRouteInfo()
				Expect(err).NotTo(HaveOccurred())

				desireAppRequest.RoutingInfo = routingInfo
			})

			It("aggregates the http routes with the same route service url", func() {
				routesToEmit = cfroutes.CFRoutes{
					{Hostnames: []string{"route1", "route2"}, Port: 8080, RouteServiceUrl: "https://rs.example.com"},
					{Hostnames: []string{"route3", "route6"}, Port: 8080},
					{Hostnames: []string{"route4", "route5"}, Port: 8080, RouteServiceUrl: "https://rs.other.com"},
					{Hostnames: []string{"route7"}, Port: 8080, RouteServiceUrl: "https://rs.another.com"},
				}
				opaqueRoutingDataCheck(routesToEmit)
			})
		})

		Context("when the bbs fails", func() {
			BeforeEach(func() {
				fakeBBS.UpdateDesiredLRPReturns(errors.New("oh no"))
			})

			It("responds with a ServiceUnavailabe error", func() {
				Expect(responseRecorder.Code).To(Equal(http.StatusServiceUnavailable))
			})
		})

		Context("when the bbs fails with a conflict", func() {
			BeforeEach(func() {
				fakeBBS.UpdateDesiredLRPReturns(models.ErrResourceConflict)
			})

			It("responds with a Conflict error", func() {
				Expect(responseRecorder.Code).To(Equal(http.StatusConflict))
			})
		})

		Context("when the LRP has docker image", func() {
			var (
				existingDesiredDockerLRP *models.DesiredLRP
				expectedPort             uint32
				expectedMetadata         string
			)

			BeforeEach(func() {
				desireAppRequest.DropletUri = ""
				desireAppRequest.DockerImageUrl = "docker:///user/repo#tag"

				expectedMetadata = fmt.Sprintf(`{"ports": {"port": %d, "protocol":"http"}}`, expectedPort)
				desireAppRequest.ExecutionMetadata = expectedMetadata

				dockerBuilder.ExtractExposedPortsStub = func(ccRequest *cc_messages.DesireAppRequestFromCC) ([]uint32, error) {
					return []uint32{expectedPort}, nil
				}

				existingDesiredDockerLRP = &models.DesiredLRP{
					ProcessGuid: "new-process-guid",
					Instances:   1,
					RootFs:      "docker:///user/repo#tag",
					Action: models.WrapAction(&models.RunAction{
						User: "me",
						Path: "ls",
					}),
					Annotation: "last-modified-etag",
				}

				dockerBuilder.BuildReturns(existingDesiredDockerLRP, nil)
			})

			It("checks to see if LRP already exists", func() {
				Eventually(fakeBBS.DesiredLRPByProcessGuidCallCount).Should(Equal(1))
			})

			It("updates the LRP without stepping on opaque routing data", func() {
				expected := cfroutes.CFRoutes{
					{Hostnames: []string{"route1", "route2"}, Port: expectedPort},
				}
				opaqueRoutingDataCheck(expected)
			})

			It("responds with 202 Accepted", func() {
				Expect(responseRecorder.Code).To(Equal(http.StatusAccepted))
			})

			It("uses docker builder", func() {
				Expect(buildpackBuilder.ExtractExposedPortsCallCount()).To(Equal(0))
				Expect(dockerBuilder.ExtractExposedPortsCallCount()).To(Equal(1))

				Expect(dockerBuilder.ExtractExposedPortsArgsForCall(0)).To(Equal(&desireAppRequest))
			})
		})
	})

	Context("when an invalid desire app message is received", func() {
		BeforeEach(func() {
			reader := bytes.NewBufferString("not valid json")
			request.Body = ioutil.NopCloser(reader)
		})

		It("does not call the bbs", func() {
			Expect(fakeBBS.RetireActualLRPCallCount()).To(BeZero())
		})

		It("responds with 400 Bad Request", func() {
			Expect(responseRecorder.Code).To(Equal(http.StatusBadRequest))
		})

		It("logs an error", func() {
			Eventually(logger.TestSink.Buffer).Should(gbytes.Say("parse-desired-app-request-failed"))
		})

		It("does not touch the LRP", func() {
			Expect(fakeBBS.DesireLRPCallCount()).To(Equal(0))
			Expect(fakeBBS.UpdateDesiredLRPCallCount()).To(Equal(0))
			Expect(fakeBBS.RemoveDesiredLRPCallCount()).To(Equal(0))
		})
	})

	Context("when the process guids do not match", func() {
		BeforeEach(func() {
			request.Form.Set(":process_guid", "another-guid")
		})

		It("does not call the bbs", func() {
			Expect(fakeBBS.RetireActualLRPCallCount()).To(BeZero())
		})

		It("responds with 400 Bad Request", func() {
			Expect(responseRecorder.Code).To(Equal(http.StatusBadRequest))
		})

		It("logs an error", func() {
			Eventually(logger.TestSink.Buffer).Should(gbytes.Say("desire-app.process-guid-mismatch"))
		})

		It("does not touch the LRP", func() {
			Expect(fakeBBS.DesireLRPCallCount()).To(Equal(0))
			Expect(fakeBBS.UpdateDesiredLRPCallCount()).To(Equal(0))
			Expect(fakeBBS.RemoveDesiredLRPCallCount()).To(Equal(0))
		})
	})
})
