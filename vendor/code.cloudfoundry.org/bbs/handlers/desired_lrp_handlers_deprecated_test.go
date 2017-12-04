package handlers_test

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"time"

	"code.cloudfoundry.org/auctioneer"
	"code.cloudfoundry.org/auctioneer/auctioneerfakes"
	"code.cloudfoundry.org/bbs/db/dbfakes"
	"code.cloudfoundry.org/bbs/events/eventfakes"
	"code.cloudfoundry.org/bbs/format"
	"code.cloudfoundry.org/bbs/handlers"
	"code.cloudfoundry.org/bbs/models"
	"code.cloudfoundry.org/bbs/models/test/model_helpers"
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
		responseRecorder     *httptest.ResponseRecorder
		handler              *handlers.DesiredLRPHandler
		exitCh               chan struct{}

		desiredLRP1 models.DesiredLRP
		desiredLRP2 models.DesiredLRP
	)

	BeforeEach(func() {
		fakeDesiredLRPDB = new(dbfakes.FakeDesiredLRPDB)
		fakeActualLRPDB = new(dbfakes.FakeActualLRPDB)
		fakeAuctioneerClient = new(auctioneerfakes.FakeClient)
		logger = lagertest.NewTestLogger("test")
		responseRecorder = httptest.NewRecorder()
		desiredHub = new(eventfakes.FakeHub)
		actualHub = new(eventfakes.FakeHub)
		exitCh = make(chan struct{}, 1)
		handler = handlers.NewDesiredLRPHandler(5, fakeDesiredLRPDB,
			fakeActualLRPDB,
			desiredHub,
			actualHub,
			fakeAuctioneerClient,
			nil, nil, exitCh)
	})

	Describe("DesiredLRPs_r0", func() {
		var requestBody interface{}

		BeforeEach(func() {
			requestBody = &models.DesiredLRPsRequest{}
			desiredLRP1 = models.DesiredLRP{}
			desiredLRP2 = models.DesiredLRP{}
		})

		JustBeforeEach(func() {
			request := newTestRequest(requestBody)
			handler.DesiredLRPs_r0(logger, responseRecorder, request)
		})

		Context("when reading desired lrps from DB succeeds", func() {
			var desiredLRPs []*models.DesiredLRP

			BeforeEach(func() {
				desiredLRPs = []*models.DesiredLRP{&desiredLRP1, &desiredLRP2}
				fakeDesiredLRPDB.DesiredLRPsReturns(desiredLRPs, nil)
			})

			It("returns a list of desired lrp groups", func() {
				Expect(responseRecorder.Code).To(Equal(http.StatusOK))
				response := models.DesiredLRPsResponse{}
				err := response.Unmarshal(responseRecorder.Body.Bytes())
				Expect(err).NotTo(HaveOccurred())

				Expect(response.Error).To(BeNil())
				Expect(response.DesiredLrps).To(Equal(desiredLRPs))
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

			Context("when the desired LRPs have cache dependencies", func() {
				BeforeEach(func() {
					desiredLRP1.Setup = &models.Action{
						UploadAction: &models.UploadAction{
							From: "web_location",
						},
					}

					desiredLRP1.CachedDependencies = []*models.CachedDependency{
						{Name: "name-1", From: "from-1", To: "to-1", CacheKey: "cache-key-1", LogSource: "log-source-1"},
					}

					desiredLRP2.CachedDependencies = []*models.CachedDependency{
						{Name: "name-2", From: "from-2", To: "to-2", CacheKey: "cache-key-2", LogSource: "log-source-2"},
						{Name: "name-3", From: "from-3", To: "to-3", CacheKey: "cache-key-3", LogSource: "log-source-3"},
					}
				})

				It("returns the cache dependency along with any setup actions", func() {
					Expect(responseRecorder.Code).To(Equal(http.StatusOK))
					response := models.DesiredLRPsResponse{}
					err := response.Unmarshal(responseRecorder.Body.Bytes())
					Expect(err).NotTo(HaveOccurred())

					Expect(response.Error).To(BeNil())
					Expect(response.DesiredLrps).To(HaveLen(2))
					Expect(response.DesiredLrps[0]).To(Equal(desiredLRP1.VersionDownTo(format.V0)))
					Expect(response.DesiredLrps[1]).To(Equal(desiredLRP2.VersionDownTo(format.V0)))
				})
			})
		})

		Context("when the DB returns no desired lrp groups", func() {
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

	Describe("DesiredLRPs_r1", func() {
		var requestBody interface{}

		BeforeEach(func() {
			requestBody = &models.DesiredLRPsRequest{}
			desiredLRP1 = models.DesiredLRP{}
			desiredLRP2 = models.DesiredLRP{}
		})

		JustBeforeEach(func() {
			request := newTestRequest(requestBody)
			handler.DesiredLRPs_r1(logger, responseRecorder, request)
		})

		Context("when reading desired lrps from DB succeeds", func() {
			var desiredLRPs []*models.DesiredLRP

			BeforeEach(func() {
				desiredLRPs = []*models.DesiredLRP{&desiredLRP1, &desiredLRP2}
				fakeDesiredLRPDB.DesiredLRPsReturns(desiredLRPs, nil)
			})

			It("returns a list of desired lrp groups", func() {
				Expect(responseRecorder.Code).To(Equal(http.StatusOK))
				response := models.DesiredLRPsResponse{}
				err := response.Unmarshal(responseRecorder.Body.Bytes())
				Expect(err).NotTo(HaveOccurred())

				Expect(response.Error).To(BeNil())
				Expect(response.DesiredLrps).To(Equal(desiredLRPs))
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

			Context("when the desired LRPs have cache dependencies", func() {
				BeforeEach(func() {
					desiredLRP1.Setup = &models.Action{
						UploadAction: &models.UploadAction{
							From: "web_location",
						},
					}

					desiredLRP1.CachedDependencies = []*models.CachedDependency{
						{Name: "name-1", From: "from-1", To: "to-1", CacheKey: "cache-key-1", LogSource: "log-source-1"},
					}

					desiredLRP2.CachedDependencies = []*models.CachedDependency{
						{Name: "name-2", From: "from-2", To: "to-2", CacheKey: "cache-key-2", LogSource: "log-source-2"},
						{Name: "name-3", From: "from-3", To: "to-3", CacheKey: "cache-key-3", LogSource: "log-source-3"},
					}
				})

				It("returns the cache dependency along with any setup actions", func() {
					Expect(responseRecorder.Code).To(Equal(http.StatusOK))
					response := models.DesiredLRPsResponse{}
					err := response.Unmarshal(responseRecorder.Body.Bytes())
					Expect(err).NotTo(HaveOccurred())

					Expect(response.Error).To(BeNil())
					Expect(response.DesiredLrps).To(HaveLen(2))
					Expect(response.DesiredLrps[0]).To(Equal(desiredLRP1.VersionDownTo(format.V1)))
					Expect(response.DesiredLrps[1]).To(Equal(desiredLRP2.VersionDownTo(format.V1)))
				})
			})

			Context("and the desired LRPs have actions with timeout not timeout_ms", func() {
				BeforeEach(func() {
					desiredLRP1.Setup = &models.Action{
						TimeoutAction: &models.TimeoutAction{
							Action: models.WrapAction(&models.UploadAction{
								From: "web_location",
							}),
							TimeoutMs: 10000,
						},
					}
					desiredLRP1.Action = &models.Action{
						TimeoutAction: &models.TimeoutAction{
							Action: models.WrapAction(&models.UploadAction{
								From: "web_location",
							}),
							TimeoutMs: 20000,
						},
					}
					desiredLRP1.Monitor = &models.Action{
						TimeoutAction: &models.TimeoutAction{
							Action: models.WrapAction(&models.UploadAction{
								From: "web_location",
							}),
							TimeoutMs: 30000,
						},
					}
				})

				It("translates the timeoutMs to timeout", func() {
					Expect(responseRecorder.Code).To(Equal(http.StatusOK))
					response := models.DesiredLRPsResponse{}
					err := response.Unmarshal(responseRecorder.Body.Bytes())
					Expect(err).NotTo(HaveOccurred())

					Expect(response.Error).To(BeNil())
					Expect(response.DesiredLrps).To(HaveLen(2))
					Expect(response.DesiredLrps[0]).To(Equal(desiredLRP1.VersionDownTo(format.V0)))
					Expect(response.DesiredLrps[1]).To(Equal(desiredLRP2.VersionDownTo(format.V0)))
				})
			})

			Context("and the desired LRPs have startTimeout not start_timeout_ms", func() {
				BeforeEach(func() {
					desiredLRP1.StartTimeoutMs = 10000
					desiredLRP2.StartTimeoutMs = 20000
				})

				It("translates StringTimeoutMs to StartTimeout", func() {
					Expect(responseRecorder.Code).To(Equal(http.StatusOK))
					response := models.DesiredLRPsResponse{}
					err := response.Unmarshal(responseRecorder.Body.Bytes())
					Expect(err).NotTo(HaveOccurred())

					Expect(response.Error).To(BeNil())
					Expect(response.DesiredLrps).To(HaveLen(2))
					Expect(response.DesiredLrps[0]).To(Equal(desiredLRP1.VersionDownTo(format.V0)))
					Expect(response.DesiredLrps[1]).To(Equal(desiredLRP2.VersionDownTo(format.V0)))
				})
			})
		})

		Context("when the DB returns no desired lrp groups", func() {
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

	Describe("DesiredLRPByProcessGuid_r0", func() {
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
			handler.DesiredLRPByProcessGuid_r0(logger, responseRecorder, request)
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

			Context("when the desired LRP has cache dependencies", func() {
				BeforeEach(func() {
					desiredLRP.Setup = &models.Action{
						UploadAction: &models.UploadAction{
							From: "web_location",
						},
					}
					desiredLRP.CachedDependencies = []*models.CachedDependency{
						{Name: "name", From: "from", To: "to", CacheKey: "cache-key", LogSource: "log-source"},
					}
				})

				It("returns the cache dependency along with any setup actions", func() {
					Expect(responseRecorder.Code).To(Equal(http.StatusOK))
					response := models.DesiredLRPResponse{}
					err := response.Unmarshal(responseRecorder.Body.Bytes())
					Expect(err).NotTo(HaveOccurred())

					Expect(response.Error).To(BeNil())
					Expect(response.DesiredLrp).To(Equal(desiredLRP.VersionDownTo(format.V0)))
				})
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

	Describe("DesiredLRPByProcessGuid_r1", func() {
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
			handler.DesiredLRPByProcessGuid_r1(logger, responseRecorder, request)
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

			Context("when the desired LRP has cache dependencies", func() {
				BeforeEach(func() {
					desiredLRP.Setup = &models.Action{
						UploadAction: &models.UploadAction{
							From: "web_location",
						},
					}
					desiredLRP.CachedDependencies = []*models.CachedDependency{
						{Name: "name", From: "from", To: "to", CacheKey: "cache-key", LogSource: "log-source"},
					}
				})

				It("returns the cache dependency along with any setup actions", func() {
					Expect(responseRecorder.Code).To(Equal(http.StatusOK))
					response := models.DesiredLRPResponse{}
					err := response.Unmarshal(responseRecorder.Body.Bytes())
					Expect(err).NotTo(HaveOccurred())

					Expect(response.Error).To(BeNil())
					Expect(response.DesiredLrp).To(Equal(desiredLRP.VersionDownTo(format.V1)))
				})
			})

			Context("and the desire LRPs has actions with timeout not timeout_ms", func() {
				BeforeEach(func() {
					desiredLRP.Setup = &models.Action{
						TimeoutAction: &models.TimeoutAction{
							Action: models.WrapAction(&models.UploadAction{
								From: "web_location",
							}),
							TimeoutMs: 10000,
						},
					}
					desiredLRP.Action = &models.Action{
						TimeoutAction: &models.TimeoutAction{
							Action: models.WrapAction(&models.UploadAction{
								From: "web_location",
							}),
							TimeoutMs: 20000,
						},
					}
					desiredLRP.Monitor = &models.Action{
						TimeoutAction: &models.TimeoutAction{
							Action: models.WrapAction(&models.UploadAction{
								From: "web_location",
							}),
							TimeoutMs: 30000,
						},
					}
				})

				It("translates the timeoutMs to timeout", func() {
					Expect(responseRecorder.Code).To(Equal(http.StatusOK))
					response := models.DesiredLRPResponse{}
					err := response.Unmarshal(responseRecorder.Body.Bytes())
					Expect(err).NotTo(HaveOccurred())

					Expect(response.Error).To(BeNil())
					Expect(response.DesiredLrp).To(Equal(desiredLRP.VersionDownTo(format.V1)))
				})
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

	Describe("DesireDesiredLRP_r0", func() {
		var (
			desiredLRP         *models.DesiredLRP
			expectedDesiredLRP *models.DesiredLRP

			requestBody interface{}
		)

		BeforeEach(func() {
			desiredLRP = model_helpers.NewValidDesiredLRP("some-guid")
			desiredLRP.Instances = 5
			desiredLRP.StartTimeoutMs = 0
			desiredLRP.DeprecatedStartTimeoutS = 15
			desiredLRP.Setup = &models.Action{
				TimeoutAction: &models.TimeoutAction{
					Action: models.WrapAction(&models.UploadAction{
						From: "web_location",
						To:   "potato",
						User: "face",
					}),
					DeprecatedTimeoutNs: int64(time.Second),
				},
			}
			desiredLRP.Action = &models.Action{
				TimeoutAction: &models.TimeoutAction{
					Action: models.WrapAction(&models.UploadAction{
						From: "web_location",
						To:   "potato",
						User: "face",
					}),
					DeprecatedTimeoutNs: int64(time.Second),
				},
			}
			desiredLRP.Monitor = &models.Action{
				TimeoutAction: &models.TimeoutAction{
					Action: models.WrapAction(&models.UploadAction{
						From: "web_location",
						To:   "potato",
						User: "face",
					}),
					DeprecatedTimeoutNs: int64(time.Second),
				},
			}

			expectedDesiredLRP = model_helpers.NewValidDesiredLRP("some-guid")
			expectedDesiredLRP.Instances = 5
			expectedDesiredLRP.StartTimeoutMs = 15000
			expectedDesiredLRP.DeprecatedStartTimeoutS = 15
			expectedDesiredLRP.Setup = &models.Action{
				TimeoutAction: &models.TimeoutAction{
					Action: models.WrapAction(&models.UploadAction{
						From: "web_location",
						To:   "potato",
						User: "face",
					}),
					DeprecatedTimeoutNs: int64(time.Second),
					TimeoutMs:           1000,
				},
			}
			expectedDesiredLRP.Action = &models.Action{
				TimeoutAction: &models.TimeoutAction{
					Action: models.WrapAction(&models.UploadAction{
						From: "web_location",
						To:   "potato",
						User: "face",
					}),
					DeprecatedTimeoutNs: int64(time.Second),
					TimeoutMs:           1000,
				},
			}
			expectedDesiredLRP.Monitor = &models.Action{
				TimeoutAction: &models.TimeoutAction{
					Action: models.WrapAction(&models.UploadAction{
						From: "web_location",
						To:   "potato",
						User: "face",
					}),
					DeprecatedTimeoutNs: int64(time.Second),
					TimeoutMs:           1000,
				},
			}
			requestBody = &models.DesireLRPRequest{
				DesiredLrp: desiredLRP,
			}
		})

		JustBeforeEach(func() {
			request := newTestRequest(requestBody)
			handler.DesireDesiredLRP_r0(logger, responseRecorder, request)
		})

		Context("when creating desired lrp in DB succeeds", func() {
			var createdActualLRPGroups []*models.ActualLRPGroup

			BeforeEach(func() {
				createdActualLRPGroups = []*models.ActualLRPGroup{}
				for i := 0; i < 5; i++ {
					createdActualLRPGroups = append(createdActualLRPGroups, &models.ActualLRPGroup{Instance: model_helpers.NewValidActualLRP("some-guid", int32(i))})
				}
				fakeDesiredLRPDB.DesireLRPReturns(nil)
				fakeActualLRPDB.CreateUnclaimedActualLRPStub = func(_ lager.Logger, key *models.ActualLRPKey) (*models.ActualLRPGroup, error) {
					if int(key.Index) > len(createdActualLRPGroups)-1 {
						return nil, errors.New("boom")
					}
					return createdActualLRPGroups[int(key.Index)], nil
				}
				fakeDesiredLRPDB.DesiredLRPByProcessGuidReturns(expectedDesiredLRP, nil)
			})

			It("creates desired lrp", func() {
				Expect(fakeDesiredLRPDB.DesireLRPCallCount()).To(Equal(1))
				_, actualDesiredLRP := fakeDesiredLRPDB.DesireLRPArgsForCall(0)
				Expect(actualDesiredLRP).To(Equal(expectedDesiredLRP))

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
				Expect(createEvent.DesiredLrp).To(Equal(expectedDesiredLRP))
			})

			It("creates and emits an event for one ActualLRP per index", func() {
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
					Expect(createdActualLRPGroups).To(ContainElement(createdEvent.ActualLrpGroup))
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

	Describe("DesireDesiredLRP_r1", func() {
		var (
			desiredLRP         *models.DesiredLRP
			expectedDesiredLRP *models.DesiredLRP

			requestBody interface{}
		)

		BeforeEach(func() {
			desiredLRP = model_helpers.NewValidDesiredLRP("some-guid")

			config, err := json.Marshal(map[string]string{"foo": "bar"})
			Expect(err).NotTo(HaveOccurred())

			desiredLRP.VolumeMounts = []*models.VolumeMount{{
				Driver:             "my-driver",
				ContainerDir:       "/mnt/mypath",
				DeprecatedMode:     models.DeprecatedBindMountMode_RO,
				DeprecatedConfig:   config,
				DeprecatedVolumeId: "my-volume",
			}}

			expectedDesiredLRP = model_helpers.NewValidDesiredLRP("some-guid")

			requestBody = &models.DesireLRPRequest{
				DesiredLrp: desiredLRP,
			}
		})

		JustBeforeEach(func() {
			request := newTestRequest(requestBody)
			handler.DesireDesiredLRP_r1(logger, responseRecorder, request)
		})

		Context("when creating desired lrp in DB succeeds", func() {
			var createdActualLRPGroups []*models.ActualLRPGroup

			BeforeEach(func() {
				createdActualLRPGroups = []*models.ActualLRPGroup{}
				for i := 0; i < 5; i++ {
					createdActualLRPGroups = append(createdActualLRPGroups, &models.ActualLRPGroup{Instance: model_helpers.NewValidActualLRP("some-guid", int32(i))})
				}
				fakeDesiredLRPDB.DesireLRPReturns(nil)
				fakeActualLRPDB.CreateUnclaimedActualLRPStub = func(_ lager.Logger, key *models.ActualLRPKey) (*models.ActualLRPGroup, error) {
					if int(key.Index) > len(createdActualLRPGroups)-1 {
						return nil, errors.New("boom")
					}
					return createdActualLRPGroups[int(key.Index)], nil
				}
				fakeDesiredLRPDB.DesiredLRPByProcessGuidReturns(expectedDesiredLRP, nil)
			})

			It("creates desired lrp", func() {
				Expect(fakeDesiredLRPDB.DesireLRPCallCount()).To(Equal(1))
				_, actualDesiredLRP := fakeDesiredLRPDB.DesireLRPArgsForCall(0)
				Expect(actualDesiredLRP.VolumeMounts).To(Equal(expectedDesiredLRP.VolumeMounts))
				Expect(actualDesiredLRP).To(Equal(expectedDesiredLRP))

				Expect(responseRecorder.Code).To(Equal(http.StatusOK))
				response := models.DesiredLRPLifecycleResponse{}
				err := response.Unmarshal(responseRecorder.Body.Bytes())
				Expect(err).NotTo(HaveOccurred())

				Expect(response.Error).To(BeNil())
			})
		})
	})
})
