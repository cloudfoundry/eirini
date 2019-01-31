package handlers_test

import (
	"errors"
	"net/http"
	"net/http/httptest"

	"code.cloudfoundry.org/auctioneer"
	"code.cloudfoundry.org/auctioneer/auctioneerfakes"
	"code.cloudfoundry.org/bbs/db/dbfakes"
	"code.cloudfoundry.org/bbs/events/eventfakes"
	"code.cloudfoundry.org/bbs/format"
	"code.cloudfoundry.org/bbs/handlers"
	"code.cloudfoundry.org/bbs/models"
	"code.cloudfoundry.org/bbs/models/test/model_helpers"
	. "code.cloudfoundry.org/bbs/test_helpers"
	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/lager/lagertest"
	"code.cloudfoundry.org/rep"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
)

var _ = Describe("DesiredLRP Handlers", func() {
	var (
		logger               *lagertest.TestLogger
		fakeDesiredLRPDB     *dbfakes.FakeDesiredLRPDB
		fakeActualLRPDB      *dbfakes.FakeActualLRPDB
		fakeAuctioneerClient *auctioneerfakes.FakeClient
		desiredHub           *eventfakes.FakeHub
		actualHub            *eventfakes.FakeHub
		actualLRPInstanceHub *eventfakes.FakeHub

		responseRecorder *httptest.ResponseRecorder
		handler          *handlers.DesiredLRPHandler
		exitCh           chan struct{}

		desiredLRP1 models.DesiredLRP
		desiredLRP2 models.DesiredLRP
	)

	BeforeEach(func() {
		var err error
		fakeDesiredLRPDB = new(dbfakes.FakeDesiredLRPDB)
		fakeActualLRPDB = new(dbfakes.FakeActualLRPDB)
		fakeAuctioneerClient = new(auctioneerfakes.FakeClient)
		logger = lagertest.NewTestLogger("test")
		logger.RegisterSink(lager.NewWriterSink(GinkgoWriter, lager.DEBUG))
		responseRecorder = httptest.NewRecorder()
		desiredHub = new(eventfakes.FakeHub)
		actualHub = new(eventfakes.FakeHub)
		actualLRPInstanceHub = new(eventfakes.FakeHub)
		Expect(err).NotTo(HaveOccurred())
		exitCh = make(chan struct{}, 1)
		handler = handlers.NewDesiredLRPHandler(
			5,
			fakeDesiredLRPDB,
			fakeActualLRPDB,
			desiredHub,
			actualHub,
			actualLRPInstanceHub,
			fakeAuctioneerClient,
			fakeRepClientFactory,
			fakeServiceClient,
			exitCh,
		)
	})

	Describe("DesiredLRPs_r2", func() {
		var requestBody interface{}

		BeforeEach(func() {
			requestBody = &models.DesiredLRPsRequest{}
			desiredLRP1 = models.DesiredLRP{}
			desiredLRP2 = models.DesiredLRP{}
		})

		JustBeforeEach(func() {
			request := newTestRequest(requestBody)
			handler.DesiredLRPs_r2(logger, responseRecorder, request)
		})

		Context("when reading desired lrps from DB succeeds", func() {
			var desiredLRPs []*models.DesiredLRP

			BeforeEach(func() {
				desiredLRPs = []*models.DesiredLRP{&desiredLRP1, &desiredLRP2}
				fakeDesiredLRPDB.DesiredLRPsReturns(desiredLRPs, nil)
			})

			It("returns a list of desired lrps", func() {
				Expect(responseRecorder.Code).To(Equal(http.StatusOK))
				response := models.DesiredLRPsResponse{}
				err := response.Unmarshal(responseRecorder.Body.Bytes())
				Expect(err).NotTo(HaveOccurred())

				Expect(response.Error).To(BeNil())

				for _, lrp := range response.DesiredLrps {
					Expect(lrp.CachedDependencies).To(BeNil())
				}

				Expect(response.DesiredLrps).To(Equal(desiredLRPs))
			})

			Context("when the desired lrps contain image layers", func() {
				var downgradedDesiredLRPs []*models.DesiredLRP

				BeforeEach(func() {
					desiredLRPsWithImageLayers := []*models.DesiredLRP{
						&models.DesiredLRP{ImageLayers: []*models.ImageLayer{{LayerType: models.LayerTypeExclusive}, {LayerType: models.LayerTypeShared}}},
						&models.DesiredLRP{ImageLayers: []*models.ImageLayer{{LayerType: models.LayerTypeExclusive}, {LayerType: models.LayerTypeShared}}},
					}
					fakeDesiredLRPDB.DesiredLRPsReturns(desiredLRPsWithImageLayers, nil)

					for _, d := range desiredLRPsWithImageLayers {
						desiredLRP := d.Copy()
						downgradedDesiredLRPs = append(downgradedDesiredLRPs, desiredLRP.VersionDownTo(format.V2))
					}
				})

				It("returns a list of desired lrps downgraded to convert image layers to cached dependencies and download actions", func() {
					Expect(responseRecorder.Code).To(Equal(http.StatusOK))
					response := models.DesiredLRPsResponse{}
					err := response.Unmarshal(responseRecorder.Body.Bytes())
					Expect(err).NotTo(HaveOccurred())

					Expect(response.Error).To(BeNil())
					Expect(response.DesiredLrps).To(ConsistOf(downgradedDesiredLRPs[0], downgradedDesiredLRPs[1]))
				})
			})

			Context("when the desired lrps contain metric tags source id", func() {
				var updatedDesiredLRPs []*models.DesiredLRP

				BeforeEach(func() {
					desiredLRPsWithMetricTags := []*models.DesiredLRP{
						&models.DesiredLRP{MetricTags: map[string]*models.MetricTagValue{"source_id": &models.MetricTagValue{Static: "some-guid"}}},
						&models.DesiredLRP{MetricsGuid: "some-metrics-guid"},
					}
					fakeDesiredLRPDB.DesiredLRPsReturns(desiredLRPsWithMetricTags, nil)

					for _, d := range desiredLRPsWithMetricTags {
						desiredLRP := d.Copy()
						updatedDesiredLRPs = append(updatedDesiredLRPs, desiredLRP.PopulateMetricsGuid())
					}
				})

				It("returns desired lrps with populated metrics_guid", func() {
					Expect(responseRecorder.Code).To(Equal(http.StatusOK))
					response := models.DesiredLRPsResponse{}
					err := response.Unmarshal(responseRecorder.Body.Bytes())
					Expect(err).NotTo(HaveOccurred())

					Expect(response.Error).To(BeNil())
					Expect(response.DesiredLrps).To(ConsistOf(updatedDesiredLRPs[0], updatedDesiredLRPs[1]))
				})
			})

			Context("and no filter is provided", func() {
				It("call the DB with no filters to retrieve the desired lrps", func() {
					Expect(fakeDesiredLRPDB.DesiredLRPsCallCount()).To(Equal(1))
					_, filter := fakeDesiredLRPDB.DesiredLRPsArgsForCall(0)
					Expect(filter).To(Equal(models.DesiredLRPFilter{}))
				})
			})

			Context("and filtering by domain", func() {
				BeforeEach(func() {
					requestBody = &models.DesiredLRPsRequest{Domain: "domain-1"}
				})

				It("call the DB with the domain filter to retrieve the desired lrps", func() {
					Expect(fakeDesiredLRPDB.DesiredLRPsCallCount()).To(Equal(1))
					_, filter := fakeDesiredLRPDB.DesiredLRPsArgsForCall(0)
					Expect(filter.Domain).To(Equal("domain-1"))
				})
			})

			Context("and filtering by process guids", func() {
				BeforeEach(func() {
					requestBody = &models.DesiredLRPsRequest{ProcessGuids: []string{"g1", "g2"}}
				})

				It("call the DB with the process guid filter to retrieve the desired lrps", func() {
					Expect(fakeDesiredLRPDB.DesiredLRPsCallCount()).To(Equal(1))
					_, filter := fakeDesiredLRPDB.DesiredLRPsArgsForCall(0)
					Expect(filter.ProcessGuids).To(Equal([]string{"g1", "g2"}))
				})
			})
		})

		Context("when the DB returns no desired lrps", func() {
			BeforeEach(func() {
				fakeDesiredLRPDB.DesiredLRPsReturns([]*models.DesiredLRP{}, nil)
			})

			It("returns an empty list", func() {
				Expect(responseRecorder.Code).To(Equal(http.StatusOK))
				response := models.DesiredLRPsResponse{}
				err := response.Unmarshal(responseRecorder.Body.Bytes())
				Expect(err).NotTo(HaveOccurred())

				Expect(response.Error).To(BeNil())
				Expect(response.DesiredLrps).To(BeEmpty())
			})
		})

		Context("when the DB returns an unrecoverable error", func() {
			BeforeEach(func() {
				fakeDesiredLRPDB.DesiredLRPsReturns([]*models.DesiredLRP{}, models.NewUnrecoverableError(nil))
			})

			It("logs and writes to the exit channel", func() {
				Eventually(logger).Should(gbytes.Say("unrecoverable-error"))
				Eventually(exitCh).Should(Receive())
			})
		})

		Context("when the DB errors out", func() {
			BeforeEach(func() {
				fakeDesiredLRPDB.DesiredLRPsReturns([]*models.DesiredLRP{}, models.ErrUnknownError)
			})

			It("provides relevant error information", func() {
				Expect(responseRecorder.Code).To(Equal(http.StatusOK))
				response := models.DesiredLRPsResponse{}
				err := response.Unmarshal(responseRecorder.Body.Bytes())
				Expect(err).NotTo(HaveOccurred())

				Expect(response.Error).To(Equal(models.ErrUnknownError))
			})
		})
	})

	Describe("DesiredLRPs", func() {
		var requestBody interface{}

		BeforeEach(func() {
			requestBody = &models.DesiredLRPsRequest{}
			desiredLRP1 = models.DesiredLRP{ImageLayers: []*models.ImageLayer{{LayerType: models.LayerTypeExclusive}, {LayerType: models.LayerTypeShared}}}
			desiredLRP2 = models.DesiredLRP{ImageLayers: []*models.ImageLayer{{LayerType: models.LayerTypeExclusive}, {LayerType: models.LayerTypeShared}}}
		})

		JustBeforeEach(func() {
			request := newTestRequest(requestBody)
			handler.DesiredLRPs(logger, responseRecorder, request)
		})

		Context("when reading desired lrps from DB succeeds", func() {

			BeforeEach(func() {
				fakeDesiredLRPDB.DesiredLRPsReturns([]*models.DesiredLRP{desiredLRP1.Copy(), desiredLRP2.Copy()}, nil)
			})

			It("returns a list of desired lrps", func() {
				Expect(responseRecorder.Code).To(Equal(http.StatusOK))
				response := models.DesiredLRPsResponse{}
				err := response.Unmarshal(responseRecorder.Body.Bytes())
				Expect(err).NotTo(HaveOccurred())

				Expect(response.Error).To(BeNil())

				Expect(response.DesiredLrps).To(DeepEqual([]*models.DesiredLRP{desiredLRP1.Copy(), desiredLRP2.Copy()}))
			})

			Context("when the desired lrps contain metric tags source id", func() {
				var updatedDesiredLRPs []*models.DesiredLRP

				BeforeEach(func() {
					desiredLRPsWithMetricTags := []*models.DesiredLRP{
						&models.DesiredLRP{MetricTags: map[string]*models.MetricTagValue{"source_id": &models.MetricTagValue{Static: "some-guid"}}},
						&models.DesiredLRP{MetricsGuid: "some-metrics-guid"},
					}
					fakeDesiredLRPDB.DesiredLRPsReturns(desiredLRPsWithMetricTags, nil)

					for _, d := range desiredLRPsWithMetricTags {
						desiredLRP := d.Copy()
						updatedDesiredLRPs = append(updatedDesiredLRPs, desiredLRP.PopulateMetricsGuid())
					}
				})

				It("returns desired lrps with populated metrics_guid", func() {
					Expect(responseRecorder.Code).To(Equal(http.StatusOK))
					response := models.DesiredLRPsResponse{}
					err := response.Unmarshal(responseRecorder.Body.Bytes())
					Expect(err).NotTo(HaveOccurred())

					Expect(response.Error).To(BeNil())
					Expect(response.DesiredLrps).To(ConsistOf(updatedDesiredLRPs[0], updatedDesiredLRPs[1]))
				})
			})

			Context("and no filter is provided", func() {
				It("call the DB with no filters to retrieve the desired lrps", func() {
					Expect(fakeDesiredLRPDB.DesiredLRPsCallCount()).To(Equal(1))
					_, filter := fakeDesiredLRPDB.DesiredLRPsArgsForCall(0)
					Expect(filter).To(Equal(models.DesiredLRPFilter{}))
				})
			})

			Context("and filtering by domain", func() {
				BeforeEach(func() {
					requestBody = &models.DesiredLRPsRequest{Domain: "domain-1"}
				})

				It("call the DB with the domain filter to retrieve the desired lrps", func() {
					Expect(fakeDesiredLRPDB.DesiredLRPsCallCount()).To(Equal(1))
					_, filter := fakeDesiredLRPDB.DesiredLRPsArgsForCall(0)
					Expect(filter.Domain).To(Equal("domain-1"))
				})
			})

			Context("and filtering by process guids", func() {
				BeforeEach(func() {
					requestBody = &models.DesiredLRPsRequest{ProcessGuids: []string{"g1", "g2"}}
				})

				It("call the DB with the process guid filter to retrieve the desired lrps", func() {
					Expect(fakeDesiredLRPDB.DesiredLRPsCallCount()).To(Equal(1))
					_, filter := fakeDesiredLRPDB.DesiredLRPsArgsForCall(0)
					Expect(filter.ProcessGuids).To(Equal([]string{"g1", "g2"}))
				})
			})
		})

		Context("when the DB returns no desired lrps", func() {
			BeforeEach(func() {
				fakeDesiredLRPDB.DesiredLRPsReturns([]*models.DesiredLRP{}, nil)
			})

			It("returns an empty list", func() {
				Expect(responseRecorder.Code).To(Equal(http.StatusOK))
				response := models.DesiredLRPsResponse{}
				err := response.Unmarshal(responseRecorder.Body.Bytes())
				Expect(err).NotTo(HaveOccurred())

				Expect(response.Error).To(BeNil())
				Expect(response.DesiredLrps).To(BeEmpty())
			})
		})

		Context("when the DB returns an unrecoverable error", func() {
			BeforeEach(func() {
				fakeDesiredLRPDB.DesiredLRPsReturns([]*models.DesiredLRP{}, models.NewUnrecoverableError(nil))
			})

			It("logs and writes to the exit channel", func() {
				Eventually(logger).Should(gbytes.Say("unrecoverable-error"))
				Eventually(exitCh).Should(Receive())
			})
		})

		Context("when the DB errors out", func() {
			BeforeEach(func() {
				fakeDesiredLRPDB.DesiredLRPsReturns([]*models.DesiredLRP{}, models.ErrUnknownError)
			})

			It("provides relevant error information", func() {
				Expect(responseRecorder.Code).To(Equal(http.StatusOK))
				response := models.DesiredLRPsResponse{}
				err := response.Unmarshal(responseRecorder.Body.Bytes())
				Expect(err).NotTo(HaveOccurred())

				Expect(response.Error).To(Equal(models.ErrUnknownError))
			})
		})
	})

	Describe("DesiredLRPByProcessGuid_r2", func() {
		var (
			processGuid = "process-guid"

			requestBody interface{}
		)

		BeforeEach(func() {
			requestBody = &models.DesiredLRPByProcessGuidRequest{
				ProcessGuid: processGuid,
			}
		})

		JustBeforeEach(func() {
			request := newTestRequest(requestBody)
			handler.DesiredLRPByProcessGuid_r2(logger, responseRecorder, request)
		})

		Context("when reading desired lrp from DB succeeds", func() {
			var desiredLRP *models.DesiredLRP

			BeforeEach(func() {
				desiredLRP = &models.DesiredLRP{ProcessGuid: processGuid}
				fakeDesiredLRPDB.DesiredLRPByProcessGuidReturns(desiredLRP, nil)
			})

			It("fetches desired lrp by process guid", func() {
				Expect(fakeDesiredLRPDB.DesiredLRPByProcessGuidCallCount()).To(Equal(1))
				_, actualProcessGuid := fakeDesiredLRPDB.DesiredLRPByProcessGuidArgsForCall(0)
				Expect(actualProcessGuid).To(Equal(processGuid))

				Expect(responseRecorder.Code).To(Equal(http.StatusOK))
				response := models.DesiredLRPResponse{}
				err := response.Unmarshal(responseRecorder.Body.Bytes())
				Expect(err).NotTo(HaveOccurred())

				Expect(response.Error).To(BeNil())
				Expect(response.DesiredLrp).To(Equal(desiredLRP))
			})
		})

		Context("when the desired lrps contain image layers", func() {
			var downgradedDesiredLRP *models.DesiredLRP

			BeforeEach(func() {
				desiredLRP := &models.DesiredLRP{
					ProcessGuid: processGuid,
					ImageLayers: []*models.ImageLayer{{LayerType: models.LayerTypeExclusive}, {LayerType: models.LayerTypeShared}},
				}
				fakeDesiredLRPDB.DesiredLRPByProcessGuidReturns(desiredLRP.Copy(), nil)

				downgradedDesiredLRP = desiredLRP.VersionDownTo(format.V2)
			})

			It("returns a list of desired lrps downgraded to convert image layers to cached dependencies and download actions", func() {
				Expect(responseRecorder.Code).To(Equal(http.StatusOK))
				response := models.DesiredLRPResponse{}
				err := response.Unmarshal(responseRecorder.Body.Bytes())
				Expect(err).NotTo(HaveOccurred())

				Expect(response.Error).To(BeNil())
				Expect(response.DesiredLrp.CachedDependencies).To(HaveLen(1))
				Expect(response.DesiredLrp.Setup.ParallelAction.Actions).To(HaveLen(1))
				Expect(response.DesiredLrp).To(Equal(downgradedDesiredLRP))
			})
		})

		Context("when the desired lrp contains metric tags source id", func() {
			var updatedDesiredLRP *models.DesiredLRP

			BeforeEach(func() {
				desiredLRPWithMetricTags := &models.DesiredLRP{
					ProcessGuid: processGuid,
					MetricTags:  map[string]*models.MetricTagValue{"source_id": &models.MetricTagValue{Static: "some-guid"}},
				}
				fakeDesiredLRPDB.DesiredLRPByProcessGuidReturns(desiredLRPWithMetricTags, nil)
				updatedDesiredLRP = desiredLRPWithMetricTags.Copy().PopulateMetricsGuid()
			})

			It("returns desired lrps with populated metrics_guid", func() {
				Expect(responseRecorder.Code).To(Equal(http.StatusOK))
				response := models.DesiredLRPResponse{}
				err := response.Unmarshal(responseRecorder.Body.Bytes())
				Expect(err).NotTo(HaveOccurred())

				Expect(response.Error).To(BeNil())
				Expect(response.DesiredLrp).To(Equal(updatedDesiredLRP))
			})
		})

		Context("when the DB returns no desired lrp", func() {
			BeforeEach(func() {
				fakeDesiredLRPDB.DesiredLRPByProcessGuidReturns(nil, models.ErrResourceNotFound)
			})

			It("returns a resource not found error", func() {
				Expect(responseRecorder.Code).To(Equal(http.StatusOK))
				response := models.DesiredLRPResponse{}
				err := response.Unmarshal(responseRecorder.Body.Bytes())
				Expect(err).NotTo(HaveOccurred())

				Expect(response.Error).To(Equal(models.ErrResourceNotFound))
			})
		})

		Context("when the DB returns an unrecoverable error", func() {
			BeforeEach(func() {
				fakeDesiredLRPDB.DesiredLRPByProcessGuidReturns(nil, models.NewUnrecoverableError(nil))
			})

			It("logs and writes to the exit channel", func() {
				Eventually(logger).Should(gbytes.Say("unrecoverable-error"))
				Eventually(exitCh).Should(Receive())
			})
		})

		Context("when the DB errors out", func() {
			BeforeEach(func() {
				fakeDesiredLRPDB.DesiredLRPByProcessGuidReturns(nil, models.ErrUnknownError)
			})

			It("provides relevant error information", func() {
				Expect(responseRecorder.Code).To(Equal(http.StatusOK))
				response := models.DesiredLRPResponse{}
				err := response.Unmarshal(responseRecorder.Body.Bytes())
				Expect(err).NotTo(HaveOccurred())

				Expect(response.Error).To(Equal(models.ErrUnknownError))
			})
		})
	})

	Describe("DesiredLRPByProcessGuid", func() {
		var (
			processGuid = "process-guid"

			requestBody interface{}
		)

		BeforeEach(func() {
			requestBody = &models.DesiredLRPByProcessGuidRequest{
				ProcessGuid: processGuid,
			}
		})

		JustBeforeEach(func() {
			request := newTestRequest(requestBody)
			handler.DesiredLRPByProcessGuid(logger, responseRecorder, request)
		})

		Context("when reading desired lrp from DB succeeds", func() {
			var desiredLRP models.DesiredLRP

			BeforeEach(func() {
				desiredLRP = models.DesiredLRP{
					ProcessGuid: processGuid,
					ImageLayers: []*models.ImageLayer{{LayerType: models.LayerTypeExclusive}, {LayerType: models.LayerTypeShared}},
				}
				fakeDesiredLRPDB.DesiredLRPByProcessGuidReturns(desiredLRP.Copy(), nil)
			})

			It("fetches desired lrp by process guid", func() {
				Expect(fakeDesiredLRPDB.DesiredLRPByProcessGuidCallCount()).To(Equal(1))
				_, actualProcessGuid := fakeDesiredLRPDB.DesiredLRPByProcessGuidArgsForCall(0)
				Expect(actualProcessGuid).To(Equal(processGuid))

				Expect(responseRecorder.Code).To(Equal(http.StatusOK))
				response := models.DesiredLRPResponse{}
				err := response.Unmarshal(responseRecorder.Body.Bytes())
				Expect(err).NotTo(HaveOccurred())

				Expect(response.Error).To(BeNil())
				Expect(response.DesiredLrp).To(DeepEqual(desiredLRP.Copy()))
			})
		})

		Context("when the desired lrp contains metric tags source id", func() {
			var updatedDesiredLRP *models.DesiredLRP

			BeforeEach(func() {
				desiredLRPWithMetricTags := &models.DesiredLRP{
					ProcessGuid: processGuid,
					MetricTags:  map[string]*models.MetricTagValue{"source_id": &models.MetricTagValue{Static: "some-guid"}},
				}
				fakeDesiredLRPDB.DesiredLRPByProcessGuidReturns(desiredLRPWithMetricTags, nil)
				updatedDesiredLRP = desiredLRPWithMetricTags.Copy().PopulateMetricsGuid()
			})

			It("returns desired lrps with populated metrics_guid", func() {
				Expect(responseRecorder.Code).To(Equal(http.StatusOK))
				response := models.DesiredLRPResponse{}
				err := response.Unmarshal(responseRecorder.Body.Bytes())
				Expect(err).NotTo(HaveOccurred())

				Expect(response.Error).To(BeNil())
				Expect(response.DesiredLrp).To(Equal(updatedDesiredLRP))
			})
		})

		Context("when the DB returns no desired lrp", func() {
			BeforeEach(func() {
				fakeDesiredLRPDB.DesiredLRPByProcessGuidReturns(nil, models.ErrResourceNotFound)
			})

			It("returns a resource not found error", func() {
				Expect(responseRecorder.Code).To(Equal(http.StatusOK))
				response := models.DesiredLRPResponse{}
				err := response.Unmarshal(responseRecorder.Body.Bytes())
				Expect(err).NotTo(HaveOccurred())

				Expect(response.Error).To(Equal(models.ErrResourceNotFound))
			})
		})

		Context("when the DB returns an unrecoverable error", func() {
			BeforeEach(func() {
				fakeDesiredLRPDB.DesiredLRPByProcessGuidReturns(nil, models.NewUnrecoverableError(nil))
			})

			It("logs and writes to the exit channel", func() {
				Eventually(logger).Should(gbytes.Say("unrecoverable-error"))
				Eventually(exitCh).Should(Receive())
			})
		})

		Context("when the DB errors out", func() {
			BeforeEach(func() {
				fakeDesiredLRPDB.DesiredLRPByProcessGuidReturns(nil, models.ErrUnknownError)
			})

			It("provides relevant error information", func() {
				Expect(responseRecorder.Code).To(Equal(http.StatusOK))
				response := models.DesiredLRPResponse{}
				err := response.Unmarshal(responseRecorder.Body.Bytes())
				Expect(err).NotTo(HaveOccurred())

				Expect(response.Error).To(Equal(models.ErrUnknownError))
			})
		})
	})

	Describe("DesiredLRPSchedulingInfos", func() {
		var (
			requestBody     interface{}
			schedulingInfo1 models.DesiredLRPSchedulingInfo
			schedulingInfo2 models.DesiredLRPSchedulingInfo
		)

		BeforeEach(func() {
			requestBody = &models.DesiredLRPsRequest{}
			schedulingInfo1 = models.DesiredLRPSchedulingInfo{}
			schedulingInfo2 = models.DesiredLRPSchedulingInfo{}
		})

		JustBeforeEach(func() {
			request := newTestRequest(requestBody)
			handler.DesiredLRPSchedulingInfos(logger, responseRecorder, request)
		})

		Context("when reading scheduling infos from DB succeeds", func() {
			var schedulingInfos []*models.DesiredLRPSchedulingInfo

			BeforeEach(func() {
				schedulingInfos = []*models.DesiredLRPSchedulingInfo{&schedulingInfo1, &schedulingInfo2}
				fakeDesiredLRPDB.DesiredLRPSchedulingInfosReturns(schedulingInfos, nil)
			})

			It("returns a list of desired lrps", func() {
				Expect(responseRecorder.Code).To(Equal(http.StatusOK))
				response := models.DesiredLRPSchedulingInfosResponse{}
				err := response.Unmarshal(responseRecorder.Body.Bytes())
				Expect(err).NotTo(HaveOccurred())

				Expect(response.Error).To(BeNil())
				Expect(response.DesiredLrpSchedulingInfos).To(Equal(schedulingInfos))
			})

			Context("and no filter is provided", func() {
				It("call the DB with no filters to retrieve the desired lrps", func() {
					Expect(fakeDesiredLRPDB.DesiredLRPSchedulingInfosCallCount()).To(Equal(1))
					_, filter := fakeDesiredLRPDB.DesiredLRPSchedulingInfosArgsForCall(0)
					Expect(filter).To(Equal(models.DesiredLRPFilter{}))
				})
			})

			Context("and filtering by domain", func() {
				BeforeEach(func() {
					requestBody = &models.DesiredLRPsRequest{Domain: "domain-1"}
				})

				It("call the DB with the domain filter to retrieve the desired lrps", func() {
					Expect(fakeDesiredLRPDB.DesiredLRPSchedulingInfosCallCount()).To(Equal(1))
					_, filter := fakeDesiredLRPDB.DesiredLRPSchedulingInfosArgsForCall(0)
					Expect(filter.Domain).To(Equal("domain-1"))
				})
			})

			Context("and filtering by process guids", func() {
				BeforeEach(func() {
					requestBody = &models.DesiredLRPsRequest{ProcessGuids: []string{"guid-1", "guid-2"}}
				})

				It("call the DB with the process guids filter to retrieve the desired lrps", func() {
					Expect(fakeDesiredLRPDB.DesiredLRPSchedulingInfosCallCount()).To(Equal(1))
					_, filter := fakeDesiredLRPDB.DesiredLRPSchedulingInfosArgsForCall(0)
					Expect(filter.ProcessGuids).To(Equal([]string{"guid-1", "guid-2"}))
				})
			})
		})

		Context("when the DB returns no desired lrps", func() {
			BeforeEach(func() {
				fakeDesiredLRPDB.DesiredLRPSchedulingInfosReturns([]*models.DesiredLRPSchedulingInfo{}, nil)
			})

			It("returns an empty list", func() {
				Expect(responseRecorder.Code).To(Equal(http.StatusOK))
				response := models.DesiredLRPSchedulingInfosResponse{}
				err := response.Unmarshal(responseRecorder.Body.Bytes())
				Expect(err).NotTo(HaveOccurred())

				Expect(response.Error).To(BeNil())
				Expect(response.DesiredLrpSchedulingInfos).To(BeEmpty())
			})
		})

		Context("when the DB returns an unrecoverable error", func() {
			BeforeEach(func() {
				fakeDesiredLRPDB.DesiredLRPSchedulingInfosReturns([]*models.DesiredLRPSchedulingInfo{}, models.NewUnrecoverableError(nil))
			})

			It("logs and writes to the exit channel", func() {
				Eventually(logger).Should(gbytes.Say("unrecoverable-error"))
				Eventually(exitCh).Should(Receive())
			})
		})

		Context("when the DB errors out", func() {
			BeforeEach(func() {
				fakeDesiredLRPDB.DesiredLRPSchedulingInfosReturns([]*models.DesiredLRPSchedulingInfo{}, models.ErrUnknownError)
			})

			It("provides relevant error information", func() {
				Expect(responseRecorder.Code).To(Equal(http.StatusOK))
				response := models.DesiredLRPSchedulingInfosResponse{}
				err := response.Unmarshal(responseRecorder.Body.Bytes())
				Expect(err).NotTo(HaveOccurred())

				Expect(response.Error).To(Equal(models.ErrUnknownError))
			})
		})
	})

	Describe("DesireDesiredLRP", func() {
		var (
			desiredLRP *models.DesiredLRP

			requestBody interface{}
		)

		BeforeEach(func() {
			desiredLRP = model_helpers.NewValidDesiredLRP("some-guid")
			desiredLRP.Instances = 5
			requestBody = &models.DesireLRPRequest{
				DesiredLrp: desiredLRP,
			}
		})

		JustBeforeEach(func() {
			request := newTestRequest(requestBody)
			handler.DesireDesiredLRP(logger, responseRecorder, request)
		})

		Context("when creating desired lrp in DB succeeds", func() {
			var createdActualLRPs []*models.ActualLRP

			BeforeEach(func() {
				createdActualLRPs = []*models.ActualLRP{}
				for i := 0; i < 5; i++ {
					createdActualLRPs = append(createdActualLRPs, model_helpers.NewValidActualLRP("some-guid", int32(i)))
				}
				fakeDesiredLRPDB.DesireLRPReturns(nil)
				fakeActualLRPDB.CreateUnclaimedActualLRPStub = func(_ lager.Logger, key *models.ActualLRPKey) (*models.ActualLRP, error) {
					if int(key.Index) > len(createdActualLRPs)-1 {
						return nil, errors.New("boom")
					}
					return createdActualLRPs[int(key.Index)], nil
				}
				fakeDesiredLRPDB.DesiredLRPByProcessGuidReturns(desiredLRP, nil)
			})

			It("creates desired lrp", func() {
				Expect(fakeDesiredLRPDB.DesireLRPCallCount()).To(Equal(1))
				_, actualDesiredLRP := fakeDesiredLRPDB.DesireLRPArgsForCall(0)
				Expect(actualDesiredLRP).To(Equal(desiredLRP))

				Expect(responseRecorder.Code).To(Equal(http.StatusOK))
				response := models.DesiredLRPLifecycleResponse{}
				err := response.Unmarshal(responseRecorder.Body.Bytes())
				Expect(err).NotTo(HaveOccurred())

				Expect(response.Error).To(BeNil())
			})

			It("emits a create event to the hub", func() {
				Eventually(desiredHub.EmitCallCount).Should(Equal(1))
				event := desiredHub.EmitArgsForCall(0)
				createEvent, ok := event.(*models.DesiredLRPCreatedEvent)
				Expect(ok).To(BeTrue())
				Expect(createEvent.DesiredLrp).To(Equal(desiredLRP))
			})

			It("creates and emits a ActualLRPCreatedEvent per index", func() {
				Expect(fakeActualLRPDB.CreateUnclaimedActualLRPCallCount()).To(Equal(5))
				Eventually(actualHub.EmitCallCount).Should(Equal(5))

				expectedLRPKeys := []*models.ActualLRPKey{}

				for i := 0; i < 5; i++ {
					expectedLRPKeys = append(expectedLRPKeys, &models.ActualLRPKey{
						ProcessGuid: desiredLRP.ProcessGuid,
						Domain:      desiredLRP.Domain,
						Index:       int32(i),
					})

				}

				for i := 0; i < 5; i++ {
					_, actualLRPKey := fakeActualLRPDB.CreateUnclaimedActualLRPArgsForCall(i)
					Expect(expectedLRPKeys).To(ContainElement(actualLRPKey))
					event := actualHub.EmitArgsForCall(i)
					createdEvent, ok := event.(*models.ActualLRPCreatedEvent)
					Expect(ok).To(BeTrue())
					Expect(createdActualLRPs).To(ContainElement(createdEvent.ActualLrpGroup.Instance))
				}

				Expect(responseRecorder.Code).To(Equal(http.StatusOK))
			})

			It("creates and emits a ActualLRPInstanceCreatedEvent per index", func() {
				Expect(fakeActualLRPDB.CreateUnclaimedActualLRPCallCount()).To(Equal(5))
				Eventually(actualLRPInstanceHub.EmitCallCount).Should(Equal(5))

				expectedLRPKeys := []*models.ActualLRPKey{}

				for i := 0; i < 5; i++ {
					expectedLRPKeys = append(expectedLRPKeys, &models.ActualLRPKey{
						ProcessGuid: desiredLRP.ProcessGuid,
						Domain:      desiredLRP.Domain,
						Index:       int32(i),
					})

				}

				for i := 0; i < 5; i++ {
					_, actualLRPKey := fakeActualLRPDB.CreateUnclaimedActualLRPArgsForCall(i)
					Expect(expectedLRPKeys).To(ContainElement(actualLRPKey))
					event := actualLRPInstanceHub.EmitArgsForCall(i)
					createdEvent, ok := event.(*models.ActualLRPInstanceCreatedEvent)
					Expect(ok).To(BeTrue())
					Expect(createdActualLRPs).To(ContainElement(createdEvent.ActualLrp))
				}

				Expect(responseRecorder.Code).To(Equal(http.StatusOK))
			})

			Context("when an auctioneer is present", func() {
				It("emits start auction requests", func() {
					Expect(fakeAuctioneerClient.RequestLRPAuctionsCallCount()).To(Equal(1))

					volumeDrivers := []string{}
					for _, volumeMount := range desiredLRP.VolumeMounts {
						volumeDrivers = append(volumeDrivers, volumeMount.Driver)
					}

					expectedStartRequest := auctioneer.LRPStartRequest{
						ProcessGuid: desiredLRP.ProcessGuid,
						Domain:      desiredLRP.Domain,
						Indices:     []int{0, 1, 2, 3, 4},
						Resource: rep.Resource{
							MemoryMB: desiredLRP.MemoryMb,
							DiskMB:   desiredLRP.DiskMb,
							MaxPids:  desiredLRP.MaxPids,
						},
						PlacementConstraint: rep.PlacementConstraint{
							RootFs:        desiredLRP.RootFs,
							VolumeDrivers: volumeDrivers,
							PlacementTags: desiredLRP.PlacementTags,
						},
					}

					Expect(fakeAuctioneerClient.RequestLRPAuctionsCallCount()).To(Equal(1))
					_, startAuctions := fakeAuctioneerClient.RequestLRPAuctionsArgsForCall(0)
					Expect(startAuctions).To(HaveLen(1))
					Expect(startAuctions[0].ProcessGuid).To(Equal(expectedStartRequest.ProcessGuid))
					Expect(startAuctions[0].Domain).To(Equal(expectedStartRequest.Domain))
					Expect(startAuctions[0].Indices).To(ConsistOf(expectedStartRequest.Indices))
					Expect(startAuctions[0].Resource).To(Equal(expectedStartRequest.Resource))
				})
			})
		})

		Context("when the DB returns an unrecoverable error", func() {
			BeforeEach(func() {
				fakeDesiredLRPDB.DesireLRPReturns(models.NewUnrecoverableError(nil))
			})

			It("logs and writes to the exit channel", func() {
				Eventually(logger).Should(gbytes.Say("unrecoverable-error"))
				Eventually(exitCh).Should(Receive())
			})
		})

		Context("when the DB errors out", func() {
			BeforeEach(func() {
				fakeDesiredLRPDB.DesireLRPReturns(models.ErrUnknownError)
			})

			It("provides relevant error information", func() {
				Expect(responseRecorder.Code).To(Equal(http.StatusOK))
				response := models.DesiredLRPLifecycleResponse{}
				err := response.Unmarshal(responseRecorder.Body.Bytes())
				Expect(err).NotTo(HaveOccurred())

				Expect(response.Error).To(Equal(models.ErrUnknownError))
			})

			It("does not try to create actual LRPs", func() {
				Expect(fakeActualLRPDB.CreateUnclaimedActualLRPCallCount()).To(Equal(0))
			})
		})
	})

	Describe("UpdateDesiredLRP", func() {
		var (
			processGuid      string
			update           *models.DesiredLRPUpdate
			beforeDesiredLRP *models.DesiredLRP
			afterDesiredLRP  *models.DesiredLRP

			requestBody interface{}
		)

		BeforeEach(func() {
			processGuid = "some-guid"
			someText := "some-text"
			beforeDesiredLRP = model_helpers.NewValidDesiredLRP(processGuid)
			beforeDesiredLRP.Instances = 4
			afterDesiredLRP = model_helpers.NewValidDesiredLRP(processGuid)
			afterDesiredLRP.Annotation = someText

			update = &models.DesiredLRPUpdate{
				Annotation: &someText,
			}
			requestBody = &models.UpdateDesiredLRPRequest{
				ProcessGuid: processGuid,
				Update:      update,
			}
		})

		JustBeforeEach(func() {
			request := newTestRequest(requestBody)
			handler.UpdateDesiredLRP(logger, responseRecorder, request)
		})

		Context("when updating desired lrp in DB succeeds", func() {
			BeforeEach(func() {
				fakeDesiredLRPDB.UpdateDesiredLRPReturns(beforeDesiredLRP, nil)
				fakeDesiredLRPDB.DesiredLRPByProcessGuidReturns(afterDesiredLRP, nil)
			})

			It("updates the desired lrp", func() {
				Expect(fakeDesiredLRPDB.UpdateDesiredLRPCallCount()).To(Equal(1))
				_, actualProcessGuid, actualUpdate := fakeDesiredLRPDB.UpdateDesiredLRPArgsForCall(0)
				Expect(actualProcessGuid).To(Equal(processGuid))
				Expect(actualUpdate).To(Equal(update))

				Expect(responseRecorder.Code).To(Equal(http.StatusOK))
				response := models.DesiredLRPLifecycleResponse{}
				err := response.Unmarshal(responseRecorder.Body.Bytes())
				Expect(err).NotTo(HaveOccurred())
				Expect(response.Error).To(BeNil())
			})

			It("emits a create event to the hub", func(done Done) {
				Eventually(desiredHub.EmitCallCount).Should(Equal(1))
				event := desiredHub.EmitArgsForCall(0)
				changeEvent, ok := event.(*models.DesiredLRPChangedEvent)
				Expect(ok).To(BeTrue())
				Expect(changeEvent.Before).To(Equal(beforeDesiredLRP))
				Expect(changeEvent.After).To(Equal(afterDesiredLRP))
				close(done)
			})

			Context("when the number of instances changes", func() {
				BeforeEach(func() {
					instances := int32(3)
					update.Instances = &instances

					desiredLRP := &models.DesiredLRP{
						ProcessGuid:   "some-guid",
						Domain:        "some-domain",
						RootFs:        "some-stack",
						PlacementTags: []string{"taggggg"},
						MemoryMb:      128,
						DiskMb:        512,
						Instances:     3,
					}

					fakeDesiredLRPDB.DesiredLRPByProcessGuidReturns(desiredLRP, nil)
					fakeServiceClient.CellByIdReturns(&models.CellPresence{
						RepAddress: "some-address",
						RepUrl:     "http://some-address",
					}, nil)
				})

				Context("when the number of instances decreased", func() {
					var actualLRPs []*models.ActualLRP

					BeforeEach(func() {
						actualLRPs = []*models.ActualLRP{}
						for i := 4; i >= 0; i-- {
							actualLRPs = append(actualLRPs, model_helpers.NewValidActualLRP("some-guid", int32(i)))
						}

						fakeActualLRPDB.ActualLRPsReturns(actualLRPs, nil)
					})

					It("stops extra actual lrps", func() {
						Expect(fakeDesiredLRPDB.DesiredLRPByProcessGuidCallCount()).To(Equal(1))
						_, processGuid := fakeDesiredLRPDB.DesiredLRPByProcessGuidArgsForCall(0)
						Expect(processGuid).To(Equal("some-guid"))

						Expect(fakeServiceClient.CellByIdCallCount()).To(Equal(2))
						Expect(fakeRepClientFactory.CreateClientCallCount()).To(Equal(2))
						repAddr, repURL := fakeRepClientFactory.CreateClientArgsForCall(0)
						Expect(repAddr).To(Equal("some-address"))
						Expect(repURL).To(Equal("http://some-address"))
						repAddr, repURL = fakeRepClientFactory.CreateClientArgsForCall(1)
						Expect(repAddr).To(Equal("some-address"))
						Expect(repURL).To(Equal("http://some-address"))

						Expect(fakeRepClient.StopLRPInstanceCallCount()).To(Equal(2))
						_, key, instanceKey := fakeRepClient.StopLRPInstanceArgsForCall(0)
						Expect(key).To(Equal(actualLRPs[0].ActualLRPKey))
						Expect(instanceKey).To(Equal(actualLRPs[0].ActualLRPInstanceKey))
						_, key, instanceKey = fakeRepClient.StopLRPInstanceArgsForCall(1)
						Expect(key).To(Equal(actualLRPs[1].ActualLRPKey))
						Expect(instanceKey).To(Equal(actualLRPs[1].ActualLRPInstanceKey))
					})

					Context("when the rep announces a url", func() {
						BeforeEach(func() {
							cellPresence := models.CellPresence{CellId: "cell-id", RepAddress: "some-address", RepUrl: "http://some-address"}
							fakeServiceClient.CellByIdReturns(&cellPresence, nil)
						})

						It("creates a rep client using the rep url", func() {
							repAddr, repURL := fakeRepClientFactory.CreateClientArgsForCall(0)
							Expect(repAddr).To(Equal("some-address"))
							Expect(repURL).To(Equal("http://some-address"))
						})

						Context("when creating a rep client fails", func() {
							BeforeEach(func() {
								err := errors.New("BOOM!!!")
								fakeRepClientFactory.CreateClientReturns(nil, err)
							})

							It("should log the error", func() {
								Expect(logger.Buffer()).To(gbytes.Say("BOOM!!!"))
							})

							It("should return the error", func() {
								response := models.DesiredLRPLifecycleResponse{}
								err := response.Unmarshal(responseRecorder.Body.Bytes())
								Expect(err).NotTo(HaveOccurred())

								Expect(response.Error).To(BeNil())
							})
						})
					})

					Context("when fetching cell presence fails", func() {
						BeforeEach(func() {
							fakeServiceClient.CellByIdStub = func(lager.Logger, string) (*models.CellPresence, error) {
								if fakeRepClient.StopLRPInstanceCallCount() == 1 {
									return nil, errors.New("ohhhhh nooooo, mr billlll")
								} else {
									return &models.CellPresence{RepAddress: "some-address"}, nil
								}
							}
						})

						It("continues stopping the rest of the lrps and logs", func() {
							Expect(fakeRepClient.StopLRPInstanceCallCount()).To(Equal(1))
							Expect(logger).To(gbytes.Say("failed-fetching-cell-presence"))
						})
					})

					Context("when stopping the lrp fails", func() {
						BeforeEach(func() {
							fakeRepClient.StopLRPInstanceStub = func(lager.Logger, models.ActualLRPKey, models.ActualLRPInstanceKey) error {
								if fakeRepClient.StopLRPInstanceCallCount() == 1 {
									return errors.New("ohhhhh nooooo, mr billlll")
								} else {
									return nil
								}
							}
						})

						It("continues stopping the rest of the lrps and logs", func() {
							Expect(fakeRepClient.StopLRPInstanceCallCount()).To(Equal(2))
							Expect(logger).To(gbytes.Say("failed-stopping-lrp-instance"))
						})
					})
				})

				Context("when the number of instances increases", func() {

					BeforeEach(func() {
						beforeDesiredLRP.Instances = 1
						fakeDesiredLRPDB.UpdateDesiredLRPReturns(beforeDesiredLRP, nil)
						actualLRP := model_helpers.NewValidActualLRP("some-guid", 0)
						fakeActualLRPDB.ActualLRPsReturns([]*models.ActualLRP{actualLRP}, nil)
					})

					It("creates missing actual lrps", func() {
						Expect(fakeDesiredLRPDB.DesiredLRPByProcessGuidCallCount()).To(Equal(1))
						_, processGuid := fakeDesiredLRPDB.DesiredLRPByProcessGuidArgsForCall(0)
						Expect(processGuid).To(Equal("some-guid"))

						keys := make([]*models.ActualLRPKey, 2)

						Expect(fakeActualLRPDB.CreateUnclaimedActualLRPCallCount()).To(Equal(2))
						_, keys[0] = fakeActualLRPDB.CreateUnclaimedActualLRPArgsForCall(0)
						_, keys[1] = fakeActualLRPDB.CreateUnclaimedActualLRPArgsForCall(1)

						Expect(keys).To(ContainElement(&models.ActualLRPKey{
							ProcessGuid: "some-guid",
							Index:       2,
							Domain:      "some-domain",
						}))

						Expect(keys).To(ContainElement(&models.ActualLRPKey{
							ProcessGuid: "some-guid",
							Index:       1,
							Domain:      "some-domain",
						}))

						Expect(fakeAuctioneerClient.RequestLRPAuctionsCallCount()).To(Equal(1))
						_, startRequests := fakeAuctioneerClient.RequestLRPAuctionsArgsForCall(0)
						Expect(startRequests).To(HaveLen(1))
						startReq := startRequests[0]
						Expect(startReq.ProcessGuid).To(Equal("some-guid"))
						Expect(startReq.Domain).To(Equal("some-domain"))
						Expect(startReq.Resource).To(Equal(rep.Resource{MemoryMB: 128, DiskMB: 512}))
						Expect(startReq.PlacementConstraint).To(Equal(rep.PlacementConstraint{
							RootFs:        "some-stack",
							VolumeDrivers: []string{},
							PlacementTags: []string{"taggggg"},
						}))
						Expect(startReq.Indices).To(ContainElement(2))
						Expect(startReq.Indices).To(ContainElement(1))
					})
				})

				Context("when fetching the desired lrp fails", func() {
					BeforeEach(func() {
						fakeDesiredLRPDB.DesiredLRPByProcessGuidReturns(nil, errors.New("you lose."))
					})

					It("does not update the actual lrps", func() {
						Expect(responseRecorder.Code).To(Equal(http.StatusOK))
						response := models.DesiredLRPLifecycleResponse{}
						err := response.Unmarshal(responseRecorder.Body.Bytes())
						Expect(err).NotTo(HaveOccurred())
						Expect(response.Error).To(BeNil())

						Expect(fakeActualLRPDB.UnclaimActualLRPCallCount()).To(Equal(0))
						Expect(fakeAuctioneerClient.RequestLRPAuctionsCallCount()).To(Equal(0))
					})
				})

				Context("when fetching the actual lrps groups fails", func() {
					BeforeEach(func() {
						fakeActualLRPDB.ActualLRPsReturns(nil, errors.New("you lose."))
					})

					It("does not update the actual lrps", func() {
						Expect(responseRecorder.Code).To(Equal(http.StatusOK))
						response := models.DesiredLRPLifecycleResponse{}
						err := response.Unmarshal(responseRecorder.Body.Bytes())
						Expect(err).NotTo(HaveOccurred())
						Expect(response.Error).To(BeNil())

						Expect(fakeActualLRPDB.UnclaimActualLRPCallCount()).To(Equal(0))
						Expect(fakeAuctioneerClient.RequestLRPAuctionsCallCount()).To(Equal(0))
					})
				})
			})
		})

		Context("when the DB returns an unrecoverable error", func() {
			BeforeEach(func() {
				fakeDesiredLRPDB.UpdateDesiredLRPReturns(nil, models.NewUnrecoverableError(nil))
			})

			It("logs and writes to the exit channel", func() {
				Eventually(logger).Should(gbytes.Say("unrecoverable-error"))
				Eventually(exitCh).Should(Receive())
			})
		})

		Context("when the DB errors out", func() {
			BeforeEach(func() {
				fakeDesiredLRPDB.UpdateDesiredLRPReturns(nil, models.ErrUnknownError)
			})

			It("provides relevant error information", func() {
				Expect(responseRecorder.Code).To(Equal(http.StatusOK))
				response := models.DesiredLRPLifecycleResponse{}
				err := response.Unmarshal(responseRecorder.Body.Bytes())
				Expect(err).NotTo(HaveOccurred())

				Expect(response.Error).To(Equal(models.ErrUnknownError))
			})
		})
	})

	Describe("RemoveDesiredLRP", func() {
		var (
			processGuid string

			requestBody interface{}
		)

		BeforeEach(func() {
			processGuid = "some-guid"
			requestBody = &models.RemoveDesiredLRPRequest{
				ProcessGuid: processGuid,
			}
			fakeServiceClient.CellByIdReturns(&models.CellPresence{RepAddress: "some-address"}, nil)
		})

		JustBeforeEach(func() {
			request := newTestRequest(requestBody)
			handler.RemoveDesiredLRP(logger, responseRecorder, request)
		})

		Context("when removing desired lrp in DB succeeds", func() {
			var desiredLRP *models.DesiredLRP

			BeforeEach(func() {
				desiredLRP = model_helpers.NewValidDesiredLRP("guid")
				fakeDesiredLRPDB.DesiredLRPByProcessGuidReturns(desiredLRP, nil)
				fakeDesiredLRPDB.RemoveDesiredLRPReturns(nil)
			})

			It("removes the desired lrp", func() {
				Expect(fakeDesiredLRPDB.RemoveDesiredLRPCallCount()).To(Equal(1))
				_, actualProcessGuid := fakeDesiredLRPDB.RemoveDesiredLRPArgsForCall(0)
				Expect(actualProcessGuid).To(Equal(processGuid))

				Expect(responseRecorder.Code).To(Equal(http.StatusOK))
				response := models.DesiredLRPLifecycleResponse{}
				err := response.Unmarshal(responseRecorder.Body.Bytes())
				Expect(err).NotTo(HaveOccurred())

				Expect(response.Error).To(BeNil())
			})

			It("emits a delete event to the hub", func(done Done) {
				Expect(fakeDesiredLRPDB.DesiredLRPByProcessGuidCallCount()).To(Equal(1))
				_, actualProcessGuid := fakeDesiredLRPDB.DesiredLRPByProcessGuidArgsForCall(0)
				Expect(actualProcessGuid).To(Equal(processGuid))

				Eventually(desiredHub.EmitCallCount).Should(Equal(1))
				event := desiredHub.EmitArgsForCall(0)
				removeEvent, ok := event.(*models.DesiredLRPRemovedEvent)
				Expect(ok).To(BeTrue())
				Expect(removeEvent.DesiredLrp).To(Equal(desiredLRP))
				close(done)
			})

			Context("when there are running instances on a present cell", func() {
				var (
					runningActualLRP0    *models.ActualLRP
					runningActualLRP1    *models.ActualLRP
					evacuatingActualLRP1 *models.ActualLRP
					evacuatingActualLRP2 *models.ActualLRP
					unclaimedActualLRP3  *models.ActualLRP
					crashedActualLRP4    *models.ActualLRP
				)

				BeforeEach(func() {
					runningActualLRP0 = model_helpers.NewValidActualLRP("some-guid", 0)

					evacuatingActualLRP1 = model_helpers.NewValidActualLRP("some-guid", 1)
					runningActualLRP1 = model_helpers.NewValidEvacuatingActualLRP("some-guid", 1)

					evacuatingActualLRP2 = model_helpers.NewValidEvacuatingActualLRP("some-guid", 2)

					unclaimedActualLRP3 = model_helpers.NewValidActualLRP("some-guid", 3)
					unclaimedActualLRP3.State = models.ActualLRPStateUnclaimed

					crashedActualLRP4 = model_helpers.NewValidActualLRP("some-guid", 4)
					crashedActualLRP4.State = models.ActualLRPStateCrashed

					actualLRPs := []*models.ActualLRP{
						runningActualLRP0,
						runningActualLRP1,
						evacuatingActualLRP1,
						evacuatingActualLRP2,
						unclaimedActualLRP3,
						crashedActualLRP4,
					}
					fakeActualLRPDB.ActualLRPsReturns(actualLRPs, nil)
					fakeActualLRPDB.RemoveActualLRPReturns(nil)
				})

				It("stops all of the corresponding running actual lrps", func() {
					Expect(fakeActualLRPDB.ActualLRPsCallCount()).To(Equal(1))

					_, filter := fakeActualLRPDB.ActualLRPsArgsForCall(0)
					Expect(filter.ProcessGuid).To(Equal("some-guid"))

					Expect(fakeRepClientFactory.CreateClientCallCount()).To(Equal(2))
					Expect(fakeRepClientFactory.CreateClientArgsForCall(0)).To(Equal("some-address"))
					Expect(fakeRepClientFactory.CreateClientArgsForCall(1)).To(Equal("some-address"))

					Expect(fakeRepClient.StopLRPInstanceCallCount()).To(Equal(2))
					_, key, instanceKey := fakeRepClient.StopLRPInstanceArgsForCall(0)
					Expect(key).To(Equal(runningActualLRP0.ActualLRPKey))
					Expect(instanceKey).To(Equal(runningActualLRP0.ActualLRPInstanceKey))
					_, key, instanceKey = fakeRepClient.StopLRPInstanceArgsForCall(1)
					Expect(key).To(Equal(evacuatingActualLRP1.ActualLRPKey))
					Expect(instanceKey).To(Equal(evacuatingActualLRP1.ActualLRPInstanceKey))
				})

				It("removes all of the corresponding unclaimed and crashed actual lrps", func() {
					Expect(fakeActualLRPDB.ActualLRPsCallCount()).To(Equal(1))

					// _, returnedActualLRPFilter := fakeActualLRPDB.ActualLRPsArgsForCall(0)
					// Expect(processGuidStr).To(Equal("some-guid"))
					Expect(fakeActualLRPDB.RemoveActualLRPCallCount()).To(Equal(2))

					_, processGuid, index, actualLRPInstanceKey := fakeActualLRPDB.RemoveActualLRPArgsForCall(0)
					Expect(index).To(BeEquivalentTo(3))
					Expect(processGuid).To(Equal("some-guid"))
					Expect(actualLRPInstanceKey).To(BeNil())

					_, processGuid, index, actualLRPInstanceKey = fakeActualLRPDB.RemoveActualLRPArgsForCall(1)
					Expect(index).To(BeEquivalentTo(4))
					Expect(processGuid).To(Equal("some-guid"))
					Expect(actualLRPInstanceKey).To(BeNil())
				})

				It("emits an ActualLRPRemovedEvent per unclaimed or crashed actual lrp", func() {
					Eventually(actualHub.EmitCallCount).Should(Equal(2))

					removedGroups := []*models.ActualLRPGroup{}

					event := actualHub.EmitArgsForCall(0)
					removedEvent, ok := event.(*models.ActualLRPRemovedEvent)
					Expect(ok).To(BeTrue())
					removedGroups = append(removedGroups, removedEvent.ActualLrpGroup)

					event = actualHub.EmitArgsForCall(1)
					removedEvent, ok = event.(*models.ActualLRPRemovedEvent)
					Expect(ok).To(BeTrue())
					removedGroups = append(removedGroups, removedEvent.ActualLrpGroup)

					Expect(removedGroups).To(ConsistOf(unclaimedActualLRP3.ToActualLRPGroup(), crashedActualLRP4.ToActualLRPGroup()))
				})

				It("emits an ActualLRPInstanceRemovedEvent per unclaimed or crashed actual lrp", func() {
					Eventually(actualLRPInstanceHub.EmitCallCount).Should(Equal(2))

					removedActualLrps := []*models.ActualLRP{}

					event := actualLRPInstanceHub.EmitArgsForCall(0)
					removedEvent, ok := event.(*models.ActualLRPInstanceRemovedEvent)
					Expect(ok).To(BeTrue())
					removedActualLrps = append(removedActualLrps, removedEvent.ActualLrp)

					event = actualLRPInstanceHub.EmitArgsForCall(1)
					removedEvent, ok = event.(*models.ActualLRPInstanceRemovedEvent)
					Expect(ok).To(BeTrue())
					removedActualLrps = append(removedActualLrps, removedEvent.ActualLrp)
					Expect(removedActualLrps).To(ConsistOf(unclaimedActualLRP3, crashedActualLRP4))
				})

				Context("when fetching the actual lrps fails", func() {
					BeforeEach(func() {
						fakeActualLRPDB.ActualLRPsReturns(nil, errors.New("new error dawg"))
					})

					It("logs the error but still succeeds", func() {
						Expect(fakeRepClientFactory.CreateClientCallCount()).To(Equal(0))
						Expect(responseRecorder.Code).To(Equal(http.StatusOK))
						response := models.DesiredLRPLifecycleResponse{}
						err := response.Unmarshal(responseRecorder.Body.Bytes())
						Expect(err).NotTo(HaveOccurred())

						Expect(response.Error).To(BeNil())
						Expect(logger).To(gbytes.Say("failed-fetching-actual-lrps"))
					})
				})
			})
		})

		Context("when the DB returns an unrecoverable error", func() {
			BeforeEach(func() {
				fakeDesiredLRPDB.RemoveDesiredLRPReturns(models.NewUnrecoverableError(nil))
			})

			It("logs and writes to the exit channel", func() {
				Eventually(logger).Should(gbytes.Say("unrecoverable-error"))
				Eventually(exitCh).Should(Receive())
			})
		})

		Context("when the DB errors out", func() {
			BeforeEach(func() {
				fakeDesiredLRPDB.RemoveDesiredLRPReturns(models.ErrUnknownError)
			})

			It("provides relevant error information", func() {
				Expect(responseRecorder.Code).To(Equal(http.StatusOK))
				response := models.DesiredLRPLifecycleResponse{}
				err := response.Unmarshal(responseRecorder.Body.Bytes())
				Expect(err).NotTo(HaveOccurred())

				Expect(response.Error).To(Equal(models.ErrUnknownError))
			})
		})
	})
})
