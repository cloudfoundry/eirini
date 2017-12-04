package handlers_test

import (
	"errors"
	"net/http"
	"net/http/httptest"

	"code.cloudfoundry.org/auctioneer"
	"code.cloudfoundry.org/auctioneer/auctioneerfakes"
	"code.cloudfoundry.org/bbs/db/dbfakes"
	"code.cloudfoundry.org/bbs/events/eventfakes"
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
		Expect(err).NotTo(HaveOccurred())
		exitCh = make(chan struct{}, 1)
		handler = handlers.NewDesiredLRPHandler(
			5,
			fakeDesiredLRPDB,
			fakeActualLRPDB,
			desiredHub,
			actualHub,
			fakeAuctioneerClient,
			fakeRepClientFactory,
			fakeServiceClient,
			exitCh,
		)
	})

	Describe("DesiredLRPs", func() {
		var requestBody interface{}

		BeforeEach(func() {
			requestBody = &models.DesiredLRPsRequest{}
			desiredLRP1 = models.DesiredLRP{}
			desiredLRP2 = models.DesiredLRP{}
		})

		JustBeforeEach(func() {
			request := newTestRequest(requestBody)
			handler.DesiredLRPs(logger, responseRecorder, request)
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

			It("returns a list of desired lrp groups", func() {
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

		Context("when the DB returns no desired lrp groups", func() {
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
					var actualLRPGroups []*models.ActualLRPGroup

					BeforeEach(func() {
						actualLRPGroups = []*models.ActualLRPGroup{}
						for i := 4; i >= 0; i-- {
							actualLRPGroups = append(actualLRPGroups, &models.ActualLRPGroup{
								Instance: model_helpers.NewValidActualLRP("some-guid", int32(i)),
							})
						}

						fakeActualLRPDB.ActualLRPGroupsByProcessGuidReturns(actualLRPGroups, nil)
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
						Expect(key).To(Equal(actualLRPGroups[0].Instance.ActualLRPKey))
						Expect(instanceKey).To(Equal(actualLRPGroups[0].Instance.ActualLRPInstanceKey))
						_, key, instanceKey = fakeRepClient.StopLRPInstanceArgsForCall(1)
						Expect(key).To(Equal(actualLRPGroups[1].Instance.ActualLRPKey))
						Expect(instanceKey).To(Equal(actualLRPGroups[1].Instance.ActualLRPInstanceKey))
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
					var runningActualLRPGroup *models.ActualLRPGroup

					BeforeEach(func() {
						beforeDesiredLRP.Instances = 1
						fakeDesiredLRPDB.UpdateDesiredLRPReturns(beforeDesiredLRP, nil)
						runningActualLRPGroup = &models.ActualLRPGroup{
							Instance: model_helpers.NewValidActualLRP("some-guid", 0),
						}
						actualLRPGroups := []*models.ActualLRPGroup{
							runningActualLRPGroup,
						}
						fakeActualLRPDB.ActualLRPGroupsByProcessGuidReturns(actualLRPGroups, nil)
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
						fakeActualLRPDB.ActualLRPGroupsByProcessGuidReturns(nil, errors.New("you lose."))
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
					runningActualLRPGroup, evacuatingAndRunningActualLRPGroup, evacuatingActualLRPGroup *models.ActualLRPGroup
					unclaimedActualLRPGroup, crashedActualLRPGroup                                      *models.ActualLRPGroup
				)

				BeforeEach(func() {
					runningActualLRPGroup = &models.ActualLRPGroup{
						Instance: model_helpers.NewValidActualLRP("some-guid", 0),
					}

					evacuatingAndRunningActualLRPGroup = &models.ActualLRPGroup{
						Instance:   model_helpers.NewValidActualLRP("some-guid", 1),
						Evacuating: model_helpers.NewValidActualLRP("some-guid", 1),
					}
					evacuatingActualLRPGroup = &models.ActualLRPGroup{
						Evacuating: model_helpers.NewValidActualLRP("some-guid", 2),
					}

					unclaimedActualLRPGroup = &models.ActualLRPGroup{
						Instance: model_helpers.NewValidActualLRP("some-guid", 3),
					}
					unclaimedActualLRPGroup.Instance.State = models.ActualLRPStateUnclaimed

					crashedActualLRPGroup = &models.ActualLRPGroup{
						Instance: model_helpers.NewValidActualLRP("some-guid", 4),
					}
					crashedActualLRPGroup.Instance.State = models.ActualLRPStateCrashed

					actualLRPGroups := []*models.ActualLRPGroup{
						runningActualLRPGroup,
						evacuatingAndRunningActualLRPGroup,
						evacuatingActualLRPGroup,
						unclaimedActualLRPGroup,
						crashedActualLRPGroup,
					}

					fakeActualLRPDB.ActualLRPGroupsByProcessGuidReturns(actualLRPGroups, nil)
				})

				It("stops all of the corresponding running actual lrps", func() {
					Expect(fakeActualLRPDB.ActualLRPGroupsByProcessGuidCallCount()).To(Equal(1))

					_, processGuid := fakeActualLRPDB.ActualLRPGroupsByProcessGuidArgsForCall(0)
					Expect(processGuid).To(Equal("some-guid"))

					Expect(fakeRepClientFactory.CreateClientCallCount()).To(Equal(2))
					Expect(fakeRepClientFactory.CreateClientArgsForCall(0)).To(Equal("some-address"))
					Expect(fakeRepClientFactory.CreateClientArgsForCall(1)).To(Equal("some-address"))

					Expect(fakeRepClient.StopLRPInstanceCallCount()).To(Equal(2))
					_, key, instanceKey := fakeRepClient.StopLRPInstanceArgsForCall(0)
					Expect(key).To(Equal(runningActualLRPGroup.Instance.ActualLRPKey))
					Expect(instanceKey).To(Equal(runningActualLRPGroup.Instance.ActualLRPInstanceKey))
					_, key, instanceKey = fakeRepClient.StopLRPInstanceArgsForCall(1)
					Expect(key).To(Equal(evacuatingAndRunningActualLRPGroup.Instance.ActualLRPKey))
					Expect(instanceKey).To(Equal(evacuatingAndRunningActualLRPGroup.Instance.ActualLRPInstanceKey))
				})

				It("removes all of the corresponding unclaimed and crashed actual lrps", func() {
					Expect(fakeActualLRPDB.ActualLRPGroupsByProcessGuidCallCount()).To(Equal(1))

					_, processGuid := fakeActualLRPDB.ActualLRPGroupsByProcessGuidArgsForCall(0)
					Expect(processGuid).To(Equal("some-guid"))
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

				Context("when fetching the actual lrps fails", func() {
					BeforeEach(func() {
						fakeActualLRPDB.ActualLRPGroupsByProcessGuidReturns(nil, errors.New("new error dawg"))
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
