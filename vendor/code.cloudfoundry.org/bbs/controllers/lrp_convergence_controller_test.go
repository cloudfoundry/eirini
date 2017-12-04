package controllers_test

import (
	"errors"

	"code.cloudfoundry.org/auctioneer"
	"code.cloudfoundry.org/auctioneer/auctioneerfakes"
	"code.cloudfoundry.org/bbs/controllers"
	"code.cloudfoundry.org/bbs/controllers/fakes"
	"code.cloudfoundry.org/bbs/db/dbfakes"
	"code.cloudfoundry.org/bbs/events/eventfakes"
	"code.cloudfoundry.org/bbs/models"
	"code.cloudfoundry.org/bbs/models/test/model_helpers"
	"code.cloudfoundry.org/bbs/serviceclient/serviceclientfakes"
	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/lager/lagertest"
	"code.cloudfoundry.org/rep/repfakes"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
)

var _ = Describe("LRP Convergence Controllers", func() {
	var (
		err                  error
		logger               *lagertest.TestLogger
		fakeLRPDB            *dbfakes.FakeLRPDB
		actualHub            *eventfakes.FakeHub
		retirer              *fakes.FakeRetirer
		fakeAuctioneerClient *auctioneerfakes.FakeClient

		keysToAuction        []*auctioneer.LRPStartRequest
		keysToRetire         []*models.ActualLRPKey
		keysWithMissingCells []*models.ActualLRPKeyWithSchedulingInfo

		retiringActualLRP1 *models.ActualLRP
		retiringActualLRP2 *models.ActualLRP

		desiredLRP1, desiredLRP2 models.DesiredLRPSchedulingInfo
		unclaimingActualLRP1     *models.ActualLRP
		unclaimingActualLRP2     *models.ActualLRP

		cellID  string
		cellSet models.CellSet

		controller *controllers.LRPConvergenceController
	)

	BeforeEach(func() {
		fakeLRPDB = new(dbfakes.FakeLRPDB)
		fakeAuctioneerClient = new(auctioneerfakes.FakeClient)
		logger = lagertest.NewTestLogger("test")

		request1 := auctioneer.NewLRPStartRequestFromModel(model_helpers.NewValidDesiredLRP("to-auction-1"), 1, 2)
		request2 := auctioneer.NewLRPStartRequestFromModel(model_helpers.NewValidDesiredLRP("to-auction-2"), 0, 4)

		retiringActualLRP1 = model_helpers.NewValidActualLRP("to-retire-1", 0)
		retiringActualLRP2 = model_helpers.NewValidActualLRP("to-retire-2", 1)
		keysToRetire = []*models.ActualLRPKey{&retiringActualLRP1.ActualLRPKey, &retiringActualLRP2.ActualLRPKey}

		desiredLRP1 = model_helpers.NewValidDesiredLRP("to-unclaim-1").DesiredLRPSchedulingInfo()
		unclaimingActualLRP1 = model_helpers.NewValidActualLRP("to-unclaim-1", 0)
		desiredLRP2 = model_helpers.NewValidDesiredLRP("to-unclaim-2").DesiredLRPSchedulingInfo()
		unclaimingActualLRP2 = model_helpers.NewValidActualLRP("to-unclaim-2", 1)
		keysWithMissingCells = []*models.ActualLRPKeyWithSchedulingInfo{
			{Key: &unclaimingActualLRP1.ActualLRPKey, SchedulingInfo: &desiredLRP1},
			{Key: &unclaimingActualLRP2.ActualLRPKey, SchedulingInfo: &desiredLRP2},
		}

		keysToAuction = []*auctioneer.LRPStartRequest{&request1, &request2}

		cellID = "cell-id"
		instanceKey := models.NewActualLRPInstanceKey("instance-guid", cellID)

		retiringActualLRP1.CellId = cellID
		retiringActualLRP1.ActualLRPInstanceKey = instanceKey
		retiringActualLRP1.State = models.ActualLRPStateClaimed
		group1 := &models.ActualLRPGroup{Instance: retiringActualLRP1}

		retiringActualLRP2.CellId = cellID
		retiringActualLRP2.ActualLRPInstanceKey = instanceKey
		retiringActualLRP2.State = models.ActualLRPStateClaimed
		group2 := &models.ActualLRPGroup{Instance: retiringActualLRP2}

		fakeLRPDB.ActualLRPGroupByProcessGuidAndIndexStub = func(_ lager.Logger, processGuid string, _ int32) (*models.ActualLRPGroup, error) {
			if processGuid == retiringActualLRP1.ProcessGuid {
				return group1, nil
			}
			if processGuid == retiringActualLRP2.ProcessGuid {
				return group2, nil
			}

			return nil, models.ErrResourceNotFound
		}

		fakeLRPDB.UnclaimActualLRPStub = func(_ lager.Logger, key *models.ActualLRPKey) (*models.ActualLRPGroup, *models.ActualLRPGroup, error) {
			if key.ProcessGuid == unclaimingActualLRP1.ProcessGuid {
				return &models.ActualLRPGroup{Instance: unclaimingActualLRP1},
					&models.ActualLRPGroup{Instance: unclaimingActualLRP1}, nil
			}
			if key.ProcessGuid == unclaimingActualLRP2.ProcessGuid {
				return &models.ActualLRPGroup{Instance: unclaimingActualLRP2},
					&models.ActualLRPGroup{Instance: unclaimingActualLRP2}, nil
			}
			return nil, nil, models.ErrResourceNotFound
		}

		fakeLRPDB.ConvergeLRPsReturns(keysToAuction, keysWithMissingCells, keysToRetire, nil)

		logger.RegisterSink(lager.NewWriterSink(GinkgoWriter, lager.DEBUG))

		fakeServiceClient = new(serviceclientfakes.FakeServiceClient)
		fakeRepClientFactory = new(repfakes.FakeClientFactory)
		fakeRepClient = new(repfakes.FakeClient)
		fakeRepClientFactory.CreateClientReturns(fakeRepClient, nil)
		fakeServiceClient.CellByIdReturns(nil, errors.New("hi"))

		cellPresence := models.NewCellPresence("cell-id", "1.1.1.1", "", "z1", models.CellCapacity{}, nil, nil, nil, nil)
		cellSet = models.CellSet{"cell-id": &cellPresence}
		fakeServiceClient.CellsReturns(cellSet, nil)

		actualHub = &eventfakes.FakeHub{}
		retirer = &fakes.FakeRetirer{}
		controller = controllers.NewLRPConvergenceController(logger, fakeLRPDB, actualHub, fakeAuctioneerClient, fakeServiceClient, retirer, 2)
	})

	JustBeforeEach(func() {
		err = controller.ConvergeLRPs(logger)
	})

	It("calls ConvergeLRPs", func() {
		Expect(err).NotTo(HaveOccurred())
		Expect(fakeLRPDB.ConvergeLRPsCallCount()).To(Equal(1))
		_, actualCellSet := fakeLRPDB.ConvergeLRPsArgsForCall(0)
		Expect(actualCellSet).To(BeEquivalentTo(cellSet))
	})

	Context("when fetching the cells fails", func() {
		BeforeEach(func() {
			fakeServiceClient.CellsReturns(nil, errors.New("kaboom"))
		})

		It("does not return an error", func() {
			Expect(err).NotTo(HaveOccurred())
		})

		It("does not call ConvergeLRPs", func() {
			Expect(fakeLRPDB.ConvergeLRPsCallCount()).To(Equal(0))
		})

		It("logs the error", func() {
			Eventually(logger).Should(gbytes.Say("failed-listing-cells"))
		})
	})

	Context("when fetching the cells returns ErrResourceNotFound", func() {
		BeforeEach(func() {
			fakeServiceClient.CellsReturns(nil, models.ErrResourceNotFound)
		})

		It("calls ConvergeLRPs with an empty CellSet", func() {
			Expect(err).NotTo(HaveOccurred())
			Expect(fakeLRPDB.ConvergeLRPsCallCount()).To(Equal(1))
			_, actualCellSet := fakeLRPDB.ConvergeLRPsArgsForCall(0)
			Expect(actualCellSet).To(BeEquivalentTo(models.CellSet{}))
		})
	})

	It("auctions off the returned keys", func() {
		Expect(fakeAuctioneerClient.RequestLRPAuctionsCallCount()).To(Equal(1))

		unclaimedStartRequest1 := auctioneer.NewLRPStartRequestFromSchedulingInfo(&desiredLRP1, 0)
		unclaimedStartRequest2 := auctioneer.NewLRPStartRequestFromSchedulingInfo(&desiredLRP2, 1)

		expectedStartRequests := append(keysToAuction, &unclaimedStartRequest1)
		expectedStartRequests = append(expectedStartRequests, &unclaimedStartRequest2)

		_, startAuctions := fakeAuctioneerClient.RequestLRPAuctionsArgsForCall(0)
		Expect(startAuctions).To(HaveLen(4))
		Expect(startAuctions).To(ConsistOf(expectedStartRequests))
	})

	Context("when no lrps to auction", func() {
		BeforeEach(func() {
			fakeLRPDB.ConvergeLRPsReturns(nil, nil, nil, nil)
		})

		It("doesn't start the auctions", func() {
			Consistently(fakeAuctioneerClient.RequestLRPAuctionsCallCount).Should(Equal(0))
		})
	})

	It("unclaims and auctions the actual lrps with missing cells", func() {
		Eventually(fakeLRPDB.UnclaimActualLRPCallCount).Should(Equal(2))

		unclaimedKeys := []*models.ActualLRPKey{}
		for i := 0; i < fakeLRPDB.UnclaimActualLRPCallCount(); i++ {
			_, key := fakeLRPDB.UnclaimActualLRPArgsForCall(i)
			unclaimedKeys = append(unclaimedKeys, key)
		}
		Expect(unclaimedKeys).To(ContainElement(&unclaimingActualLRP1.ActualLRPKey))
		Expect(unclaimedKeys).To(ContainElement(&unclaimingActualLRP2.ActualLRPKey))

		Eventually(actualHub.EmitCallCount).Should(Equal(2))
		changeEvents := []*models.ActualLRPChangedEvent{}
		for i := 0; i < actualHub.EmitCallCount(); i++ {
			event := actualHub.EmitArgsForCall(i)
			if changeEvent, ok := event.(*models.ActualLRPChangedEvent); ok {
				changeEvents = append(changeEvents, changeEvent)
			}
		}
		group1 := &models.ActualLRPGroup{Instance: unclaimingActualLRP1}
		group2 := &models.ActualLRPGroup{Instance: unclaimingActualLRP2}
		Expect(changeEvents).To(ContainElement(models.NewActualLRPChangedEvent(group1, group1)))
		Expect(changeEvents).To(ContainElement(models.NewActualLRPChangedEvent(group2, group2)))
	})

	It("logs the reason for starting actual lrps with missing cells", func() {
		Eventually(logger).Should(gbytes.Say("creating-start-request.*reason\":\"missing-cell"))
	})

	Context("when the DB returns an unrecoverable error", func() {
		BeforeEach(func() {
			fakeLRPDB.UnclaimActualLRPReturns(nil, nil, models.NewUnrecoverableError(nil))
		})

		It("logs the error", func() {
			Eventually(logger).Should(gbytes.Say("unrecoverable-error"))
		})

		It("returns the error", func() {
			Expect(err).To(HaveOccurred())
			Expect(err).Should(Equal(models.NewUnrecoverableError(nil)))
		})
	})

	Context("when unclaiming the actual lrp fails", func() {
		BeforeEach(func() {
			fakeLRPDB.UnclaimActualLRPReturns(nil, nil, errors.New("terrrible"))
		})

		It("auctions off the returned keys", func() {
			Expect(fakeAuctioneerClient.RequestLRPAuctionsCallCount()).To(Equal(1))

			_, startAuctions := fakeAuctioneerClient.RequestLRPAuctionsArgsForCall(0)
			Expect(startAuctions).To(HaveLen(2))
			Expect(startAuctions).To(ConsistOf(keysToAuction))
		})

		It("does not emit change events", func() {
			Eventually(fakeLRPDB.UnclaimActualLRPCallCount).Should(Equal(2))
			Consistently(actualHub.EmitCallCount).Should(Equal(0))
		})
	})

	Describe("stopping extra LRPs", func() {
		var (
			cellPresence models.CellPresence
		)

		Context("when the cell", func() {
			Context("is present", func() {
				BeforeEach(func() {
					cellPresence = models.NewCellPresence(
						cellID,
						"cell1.addr",
						"",
						"the-zone",
						models.NewCellCapacity(128, 1024, 6),
						[]string{},
						[]string{},
						[]string{},
						[]string{},
					)

					fakeServiceClient.CellByIdReturns(&cellPresence, nil)
				})

				It("stops the LRPs", func() {
					Eventually(retirer.RetireActualLRPCallCount()).Should(Equal(2))

					stoppedKeys := make([]*models.ActualLRPKey, 2)

					for i := 0; i < 2; i++ {
						_, key := retirer.RetireActualLRPArgsForCall(i)
						stoppedKeys[i] = key
					}

					Expect(stoppedKeys).To(ContainElement(&retiringActualLRP1.ActualLRPKey))
					Expect(stoppedKeys).To(ContainElement(&retiringActualLRP2.ActualLRPKey))
				})

				Context("when the retirer returns an error", func() {
					BeforeEach(func() {
						retirer.RetireActualLRPReturns(errors.New("BOOM!!!"))
					})

					It("should log the error", func() {
						Expect(logger.Buffer()).To(gbytes.Say("BOOM!!!"))
					})

					It("should return the error", func() {
						Expect(err).NotTo(HaveOccurred())
					})
				})
			})

			Context("is not present", func() {
				BeforeEach(func() {
					fakeServiceClient.CellByIdReturns(nil,
						&models.Error{
							Type:    models.Error_ResourceNotFound,
							Message: "cell not found",
						})
				})

				Context("removing the actualLRP succeeds", func() {
					It("removes the LRPs", func() {
						Eventually(retirer.RetireActualLRPCallCount()).Should(Equal(2))

						deletedKeys := make([]*models.ActualLRPKey, 2)

						for i := 0; i < 2; i++ {
							_, key := retirer.RetireActualLRPArgsForCall(i)
							deletedKeys[i] = key
						}

						Expect(deletedKeys).To(ContainElement(&retiringActualLRP1.ActualLRPKey))
						Expect(deletedKeys).To(ContainElement(&retiringActualLRP2.ActualLRPKey))
					})
				})
			})
		})
	})

	Context("when the db returns events", func() {
		var expectedRemovedEvent *models.ActualLRPRemovedEvent
		BeforeEach(func() {
			group1 := &models.ActualLRPGroup{Instance: model_helpers.NewValidActualLRP("evacuating-lrp", 0)}
			expectedRemovedEvent = models.NewActualLRPRemovedEvent(group1)
			events := []models.Event{expectedRemovedEvent}
			fakeLRPDB.ConvergeLRPsReturns([]*auctioneer.LRPStartRequest{}, []*models.ActualLRPKeyWithSchedulingInfo{}, []*models.ActualLRPKey{}, events)
		})

		It("emits those events", func() {

			Eventually(actualHub.EmitCallCount).Should(Equal(1))
			event := actualHub.EmitArgsForCall(0)
			removedEvent, ok := event.(*models.ActualLRPRemovedEvent)
			Expect(ok).To(BeTrue())
			Expect(removedEvent).To(Equal(expectedRemovedEvent))
		})
	})
})
