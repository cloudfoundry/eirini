package controllers_test

import (
	"errors"

	"code.cloudfoundry.org/auctioneer"
	"code.cloudfoundry.org/auctioneer/auctioneerfakes"
	"code.cloudfoundry.org/bbs/controllers"
	"code.cloudfoundry.org/bbs/db/dbfakes"
	"code.cloudfoundry.org/bbs/events/eventfakes"
	"code.cloudfoundry.org/bbs/models"
	"code.cloudfoundry.org/bbs/models/test/model_helpers"
	"code.cloudfoundry.org/bbs/serviceclient/serviceclientfakes"
	"code.cloudfoundry.org/lager/lagertest"
	"code.cloudfoundry.org/rep/repfakes"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
)

var _ = Describe("ActualLRP Lifecycle Controller", func() {
	var (
		logger               *lagertest.TestLogger
		fakeActualLRPDB      *dbfakes.FakeActualLRPDB
		fakeDesiredLRPDB     *dbfakes.FakeDesiredLRPDB
		fakeEvacuationDB     *dbfakes.FakeEvacuationDB
		fakeSuspectDB        *dbfakes.FakeSuspectDB
		fakeAuctioneerClient *auctioneerfakes.FakeClient
		actualHub            *eventfakes.FakeHub
		actualLRPInstanceHub *eventfakes.FakeHub

		controller *controllers.ActualLRPLifecycleController
		err        error

		actualLRPKey models.ActualLRPKey

		actualLRP      *models.ActualLRP
		actualLRPState string
		presence       models.ActualLRP_Presence

		afterActualLRP            *models.ActualLRP
		afterActualLRPState       string
		afterPresence             models.ActualLRP_Presence
		afterActualLRPCrashCount  int32
		afterActualLRPCrashReason string

		beforeInstanceKey models.ActualLRPInstanceKey
		afterInstanceKey  models.ActualLRPInstanceKey
		processGuid       string
		index             int32
	)

	BeforeEach(func() {
		fakeActualLRPDB = new(dbfakes.FakeActualLRPDB)
		fakeSuspectDB = new(dbfakes.FakeSuspectDB)
		fakeDesiredLRPDB = new(dbfakes.FakeDesiredLRPDB)
		fakeEvacuationDB = new(dbfakes.FakeEvacuationDB)
		fakeAuctioneerClient = new(auctioneerfakes.FakeClient)
		logger = lagertest.NewTestLogger("test")

		fakeServiceClient = new(serviceclientfakes.FakeServiceClient)
		fakeRepClientFactory = new(repfakes.FakeClientFactory)
		fakeRepClient = new(repfakes.FakeClient)
		fakeRepClientFactory.CreateClientReturns(fakeRepClient, nil)

		actualHub = &eventfakes.FakeHub{}
		actualLRPInstanceHub = &eventfakes.FakeHub{}
		controller = controllers.NewActualLRPLifecycleController(
			fakeActualLRPDB,
			fakeSuspectDB,
			fakeEvacuationDB,
			fakeDesiredLRPDB,
			fakeAuctioneerClient,
			fakeServiceClient,
			fakeRepClientFactory,
			actualHub,
			actualLRPInstanceHub,
		)

		beforeInstanceKey = models.NewActualLRPInstanceKey(
			"instance-guid-0",
			"cell-id-0",
		)

		afterInstanceKey = models.NewActualLRPInstanceKey(
			"instance-guid-0",
			"cell-id-0",
		)

		processGuid = "process-guid"
		index = 1

		actualLRPKey = models.NewActualLRPKey(
			processGuid,
			index,
			"domain-0",
		)

		presence = models.ActualLRP_Ordinary
		afterPresence = models.ActualLRP_Ordinary
	})

	JustBeforeEach(func() {
		actualLRP = &models.ActualLRP{
			ActualLRPKey:         actualLRPKey,
			ActualLRPInstanceKey: beforeInstanceKey,
			State:                actualLRPState,
			Since:                1138,
			Presence:             presence,
		}

		afterActualLRP = &models.ActualLRP{
			ActualLRPKey:         actualLRPKey,
			ActualLRPInstanceKey: afterInstanceKey,
			State:                afterActualLRPState,
			Since:                1140,
			Presence:             afterPresence,
			CrashCount:           afterActualLRPCrashCount,
			CrashReason:          afterActualLRPCrashReason,
		}
	})

	Describe("ClaimActualLRP", func() {
		BeforeEach(func() {
			actualLRPState = models.ActualLRPStateUnclaimed
			afterActualLRPState = models.ActualLRPStateClaimed
		})

		JustBeforeEach(func() {
			fakeActualLRPDB.ActualLRPsReturns([]*models.ActualLRP{actualLRP}, nil)
			fakeActualLRPDB.ClaimActualLRPReturns(actualLRP, afterActualLRP, nil)
		})

		It("calls the DB successfully", func() {
			err = controller.ClaimActualLRP(logger, processGuid, index, &afterInstanceKey)
			Expect(err).NotTo(HaveOccurred())
			Expect(fakeActualLRPDB.ClaimActualLRPCallCount()).To(Equal(1))
		})

		It("emits a LRP group change to the hub", func() {
			err = controller.ClaimActualLRP(logger, processGuid, index, &afterInstanceKey)
			Eventually(actualHub.EmitCallCount).Should(Equal(1))
			event := actualHub.EmitArgsForCall(0)
			changedEvent, ok := event.(*models.ActualLRPChangedEvent)
			Expect(ok).To(BeTrue())
			Expect(changedEvent.Before).To(Equal(actualLRP.ToActualLRPGroup()))
			Expect(changedEvent.After).To(Equal(afterActualLRP.ToActualLRPGroup()))
		})

		It("emits a LRP instance change to the hub", func() {
			err = controller.ClaimActualLRP(logger, processGuid, index, &afterInstanceKey)
			Eventually(actualLRPInstanceHub.EmitCallCount).Should(Equal(1))
			event := actualLRPInstanceHub.EmitArgsForCall(0)
			Expect(event).To(Equal(models.NewActualLRPInstanceChangedEvent(actualLRP, afterActualLRP)))
		})

		Context("when the actual lrp did not actually change", func() {
			JustBeforeEach(func() {
				fakeActualLRPDB.ActualLRPsReturns([]*models.ActualLRP{afterActualLRP}, nil)
				fakeActualLRPDB.ClaimActualLRPReturns(
					afterActualLRP,
					afterActualLRP,
					nil,
				)
			})

			It("does not emit a change event to the hub", func() {
				err = controller.ClaimActualLRP(logger, processGuid, index, &afterInstanceKey)
				Consistently(actualHub.EmitCallCount).Should(BeZero())
			})
		})

		Context("when there is a running Suspect LRP", func() {
			JustBeforeEach(func() {
				suspect := &models.ActualLRP{
					State:        models.ActualLRPStateRunning,
					Presence:     models.ActualLRP_Suspect,
					ActualLRPKey: actualLRPKey,
					ActualLRPInstanceKey: models.ActualLRPInstanceKey{
						InstanceGuid: "suspect-ig",
						CellId:       "suspect-cell-id",
					},
				}
				fakeActualLRPDB.ActualLRPsReturns([]*models.ActualLRP{suspect}, nil)
			})

			It("does not emit ActualLRPChangedEvent", func() {
				err = controller.ClaimActualLRP(logger, processGuid, index, &afterInstanceKey)
				Expect(err).NotTo(HaveOccurred())
				Consistently(actualHub.EmitCallCount).Should(BeZero())
			})
		})

		Context("when there is a claimed Suspect LRP", func() {
			var suspectLRP *models.ActualLRP
			JustBeforeEach(func() {
				suspectLRP = &models.ActualLRP{
					State:        models.ActualLRPStateClaimed,
					Presence:     models.ActualLRP_Suspect,
					ActualLRPKey: actualLRPKey,
					ActualLRPInstanceKey: models.ActualLRPInstanceKey{
						InstanceGuid: "suspect-ig",
						CellId:       "suspect-cell-id",
					},
				}
				fakeActualLRPDB.ActualLRPsReturns([]*models.ActualLRP{suspectLRP}, nil)
			})
		})
	})

	Describe("StartActualLRP", func() {
		var (
			netInfo models.ActualLRPNetInfo
			err     error
		)

		BeforeEach(func() {
			netInfo = models.NewActualLRPNetInfo("1.1.1.1", "2.2.2.2", models.NewPortMapping(10, 20))

			actualLRPState = models.ActualLRPStateUnclaimed
			afterActualLRPState = models.ActualLRPStateRunning
		})

		Context("when there is a Suspect LRP running", func() {
			var (
				suspect *models.ActualLRP
			)

			BeforeEach(func() {
				suspect = &models.ActualLRP{
					Presence: models.ActualLRP_Suspect,
					State:    models.ActualLRPStateRunning,
					ActualLRPInstanceKey: models.ActualLRPInstanceKey{
						InstanceGuid: "suspect-instance-guid",
						CellId:       "cell-id-1",
					},
					ActualLRPKey: models.ActualLRPKey{
						ProcessGuid: processGuid,
						Index:       index,
						Domain:      "domain-0",
					},
				}

				fakeSuspectDB.RemoveSuspectActualLRPReturns(suspect, nil)
			})

			JustBeforeEach(func() {
				fakeActualLRPDB.ActualLRPsReturns([]*models.ActualLRP{actualLRP, suspect}, nil)
				fakeActualLRPDB.StartActualLRPReturns(actualLRP, afterActualLRP, nil)
			})

			It("removes the suspect lrp", func() {
				err = controller.StartActualLRP(logger, &actualLRPKey, &afterInstanceKey, &netInfo)
				Eventually(fakeSuspectDB.RemoveSuspectActualLRPCallCount).Should(Equal(1))
				_, lrpKey := fakeSuspectDB.RemoveSuspectActualLRPArgsForCall(0)
				Expect(lrpKey).To(Equal(&models.ActualLRPKey{
					ProcessGuid: processGuid,
					Index:       index,
					Domain:      "domain-0",
				}))
			})

			It("emits ActualLRPCreatedEvent and an ActualLRPRemovedEvent", func() {
				err = controller.StartActualLRP(logger, &actualLRPKey, &afterInstanceKey, &netInfo)
				Eventually(actualHub.EmitCallCount).Should(Equal(2))

				Expect(actualHub.EmitArgsForCall(0)).To(Equal(models.NewActualLRPCreatedEvent(
					afterActualLRP.ToActualLRPGroup(),
				)))
				Expect(actualHub.EmitArgsForCall(1)).To(Equal(models.NewActualLRPRemovedEvent(
					suspect.ToActualLRPGroup(),
				)))
			})

			It("emits ActualLRPInstanceChangedEvent and an ActualLRPInstanceRemovedEvent", func() {
				err = controller.StartActualLRP(logger, &actualLRPKey, &afterInstanceKey, &netInfo)
				Eventually(actualLRPInstanceHub.EmitCallCount).Should(Equal(2))

				event := actualLRPInstanceHub.EmitArgsForCall(0)
				Expect(event).To(Equal(models.NewActualLRPInstanceChangedEvent(actualLRP, afterActualLRP)))

				event = actualLRPInstanceHub.EmitArgsForCall(1)
				Expect(event).To(Equal(models.NewActualLRPInstanceRemovedEvent(suspect)))
			})

			Context("when RemoveSuspectActualLRP returns an error", func() {
				BeforeEach(func() {
					fakeSuspectDB.RemoveSuspectActualLRPReturns(nil, errors.New("boooom!"))
				})

				It("logs the error", func() {
					err = controller.StartActualLRP(logger, &actualLRPKey, &afterInstanceKey, &netInfo)
					Expect(logger.Buffer()).Should(gbytes.Say("boooom!"))
				})

				It("emits ActualLRPCreatedEvent and an ActualLRPRemovedEvent", func() {
					err = controller.StartActualLRP(logger, &actualLRPKey, &afterInstanceKey, &netInfo)
					Eventually(actualHub.EmitCallCount).Should(Equal(2))

					Expect(actualHub.EmitArgsForCall(0)).To(Equal(models.NewActualLRPCreatedEvent(
						afterActualLRP.ToActualLRPGroup(),
					)))
					Expect(actualHub.EmitArgsForCall(1)).To(Equal(models.NewActualLRPRemovedEvent(
						suspect.ToActualLRPGroup(),
					)))
				})
			})
		})

		Context("when the LRP being started is Suspect", func() {
			JustBeforeEach(func() {
				fakeActualLRPDB.ActualLRPsReturns([]*models.ActualLRP{actualLRP}, nil)
			})

			Context("when there is a Running Ordinary LRP", func() {
				BeforeEach(func() {
					// the db layer resolution logic will return the Ordinary LRP
					presence = models.ActualLRP_Ordinary
				})

				JustBeforeEach(func() {
					fakeActualLRPDB.StartActualLRPReturns(nil, nil, models.ErrActualLRPCannotBeStarted)
				})

				It("returns ErrActualLRPCannotBeStarted", func() {
					err = controller.StartActualLRP(logger, &actualLRPKey, &afterInstanceKey, &netInfo)
					Expect(err).To(MatchError(models.ErrActualLRPCannotBeStarted))
				})
			})

			Context("and the Ordinary LRP is not running", func() {
				BeforeEach(func() {
					// the db layer resolution logic will return the Suspect LRP
					presence = models.ActualLRP_Suspect
				})

				JustBeforeEach(func() {
					fakeActualLRPDB.ActualLRPsReturns([]*models.ActualLRP{actualLRP}, nil)
				})

				It("don't do anything", func() {
					err = controller.StartActualLRP(logger, &actualLRPKey, &afterInstanceKey, &netInfo)
					Expect(fakeActualLRPDB.StartActualLRPCallCount()).To(BeZero())
				})
			})
		})

		Context("when starting the actual lrp in the DB succeeds", func() {
			JustBeforeEach(func() {
				fakeActualLRPDB.StartActualLRPReturns(actualLRP, afterActualLRP, nil)
				fakeActualLRPDB.ActualLRPsReturns([]*models.ActualLRP{actualLRP}, nil)
			})

			It("calls DB successfully", func() {
				err = controller.StartActualLRP(logger, &actualLRPKey, &afterInstanceKey, &netInfo)
				Expect(err).NotTo(HaveOccurred())
				Expect(fakeActualLRPDB.StartActualLRPCallCount()).To(Equal(1))
				Expect(fakeActualLRPDB.ActualLRPsCallCount()).To(Equal(1))
			})

			Context("when a non-ResourceNotFound error occurs while fetching the lrp", func() {
				JustBeforeEach(func() {
					fakeActualLRPDB.ActualLRPsReturns(nil, errors.New("BOOM!!!"))
				})

				It("should return the error", func() {
					err = controller.StartActualLRP(logger, &actualLRPKey, &afterInstanceKey, &netInfo)
					Expect(err).To(MatchError("BOOM!!!"))
				})
			})

			Context("when a ResourceNotFound error occurs while fetching the lrp", func() {
				JustBeforeEach(func() {
					fakeActualLRPDB.ActualLRPsReturns(nil, models.ErrResourceNotFound)
				})

				It("should continue to start the LRP", func() {
					err = controller.StartActualLRP(logger, &actualLRPKey, &afterInstanceKey, &netInfo)
					Expect(err).NotTo(HaveOccurred())
					Expect(fakeActualLRPDB.StartActualLRPCallCount()).To(Equal(1))
				})
			})

			Context("when the lrp is evacuating", func() {
				var evacuating *models.ActualLRP

				BeforeEach(func() {
					evacuating = model_helpers.NewValidEvacuatingActualLRP(processGuid, index)
					evacuating.ActualLRPKey = actualLRPKey
					evacuating.State = models.ActualLRPStateRunning
				})

				JustBeforeEach(func() {
					fakeActualLRPDB.ActualLRPsReturns([]*models.ActualLRP{actualLRP, evacuating}, nil)
				})

				It("removes the evacuating lrp", func() {
					err = controller.StartActualLRP(logger, &actualLRPKey, &afterInstanceKey, &netInfo)
					Expect(fakeEvacuationDB.RemoveEvacuatingActualLRPCallCount()).To(Equal(1))
					_, lrpKey, lrpInstanceKey := fakeEvacuationDB.RemoveEvacuatingActualLRPArgsForCall(0)
					Expect(*lrpKey).To(Equal(evacuating.ActualLRPKey))
					Expect(*lrpInstanceKey).To(Equal(evacuating.ActualLRPInstanceKey))
				})

				It("should emit an ActualLRPChanged event and an ActualLRPRemoved event", func() {
					err = controller.StartActualLRP(logger, &actualLRPKey, &afterInstanceKey, &netInfo)

					Eventually(actualHub.EmitCallCount).Should(Equal(2))

					Expect(actualHub.EmitArgsForCall(0)).To(Equal(models.NewActualLRPChangedEvent(
						actualLRP.ToActualLRPGroup(),
						afterActualLRP.ToActualLRPGroup(),
					)))

					Expect(actualHub.EmitArgsForCall(1)).To(Equal(models.NewActualLRPRemovedEvent(
						evacuating.ToActualLRPGroup(),
					)))
				})

				It("should emit LRP instance changed event and LRP instance removed event", func() {
					err = controller.StartActualLRP(logger, &actualLRPKey, &afterInstanceKey, &netInfo)

					Eventually(actualLRPInstanceHub.EmitCallCount).Should(Equal(2))

					Expect(actualLRPInstanceHub.EmitArgsForCall(0)).To(Equal(models.NewActualLRPInstanceChangedEvent(actualLRP, afterActualLRP)))
					Expect(actualLRPInstanceHub.EmitArgsForCall(1)).To(Equal(models.NewActualLRPInstanceRemovedEvent(evacuating)))
				})
			})

			Context("when the actual lrp was created", func() {
				JustBeforeEach(func() {
					fakeActualLRPDB.ActualLRPsReturns(nil, nil)
					fakeActualLRPDB.StartActualLRPReturns(nil, afterActualLRP, nil)
				})

				It("emits a created event to the hub", func() {
					err = controller.StartActualLRP(logger, &actualLRPKey, &afterInstanceKey, &netInfo)

					Eventually(actualHub.EmitCallCount).Should(Equal(1))
					event := actualHub.EmitArgsForCall(0)
					createdEvent, ok := event.(*models.ActualLRPCreatedEvent)
					Expect(ok).To(BeTrue())
					Expect(createdEvent.ActualLrpGroup).To(Equal(afterActualLRP.ToActualLRPGroup()))
				})

				It("emits LRP instance create event", func() {
					err = controller.StartActualLRP(logger, &actualLRPKey, &afterInstanceKey, &netInfo)

					Eventually(actualLRPInstanceHub.EmitCallCount).Should(Equal(1))
					event := actualLRPInstanceHub.EmitArgsForCall(0)
					var e1 *models.ActualLRPInstanceCreatedEvent
					Expect(event).To(BeAssignableToTypeOf(e1))
					createdEvent := event.(*models.ActualLRPInstanceCreatedEvent)
					Expect(createdEvent.ActualLrp).To(Equal(afterActualLRP))
				})
			})

			Context("when the actual lrp was updated", func() {
				It("emits a change event to the hub", func() {
					err = controller.StartActualLRP(logger, &actualLRPKey, &afterInstanceKey, &netInfo)
					Eventually(actualHub.EmitCallCount).Should(Equal(1))
					event := actualHub.EmitArgsForCall(0)
					changedEvent, ok := event.(*models.ActualLRPChangedEvent)
					Expect(ok).To(BeTrue())
					Expect(changedEvent.Before).To(Equal(actualLRP.ToActualLRPGroup()))
					Expect(changedEvent.After).To(Equal(afterActualLRP.ToActualLRPGroup()))
				})
			})

			Context("when there is not change in the actual lrp state", func() {
				JustBeforeEach(func() {
					*actualLRP = *afterActualLRP
				})

				It("does not emit a change event to the hub", func() {
					err = controller.StartActualLRP(logger, &actualLRPKey, &afterInstanceKey, &netInfo)

					Consistently(actualHub.EmitCallCount).Should(Equal(0))
				})
			})
		})

		Context("when starting the actual lrp fails", func() {
			JustBeforeEach(func() {
				fakeActualLRPDB.ActualLRPsReturns([]*models.ActualLRP{actualLRP}, nil)
				fakeActualLRPDB.StartActualLRPReturns(nil, nil, models.ErrUnknownError)
			})

			It("responds with an error", func() {
				err = controller.StartActualLRP(logger, &actualLRPKey, &afterInstanceKey, &netInfo)
				Expect(err).To(HaveOccurred())
				Expect(err).To(Equal(models.ErrUnknownError))
			})

			It("does not emit a change event to the hub", func() {
				err = controller.StartActualLRP(logger, &actualLRPKey, &afterInstanceKey, &netInfo)
				Consistently(actualHub.EmitCallCount).Should(Equal(0))
			})
		})
	})

	Describe("CrashActualLRP", func() {
		var (
			errorMessage  string
			lrps          []*models.ActualLRP
			desiredLRP    *models.DesiredLRP
			shouldRestart bool
		)

		BeforeEach(func() {
			errorMessage = "something went wrong"

			actualLRPState = models.ActualLRPStateClaimed
			afterActualLRPState = models.ActualLRPStateUnclaimed
			afterInstanceKey = models.ActualLRPInstanceKey{}
			afterActualLRPCrashCount = 1
			afterActualLRPCrashReason = errorMessage
			shouldRestart = true
		})

		JustBeforeEach(func() {
			fakeActualLRPDB.CrashActualLRPReturns(
				actualLRP,
				afterActualLRP,
				shouldRestart,
				nil,
			)

			lrps = []*models.ActualLRP{actualLRP}
			fakeActualLRPDB.ActualLRPsReturns(lrps, nil)

			desiredLRP = &models.DesiredLRP{
				ProcessGuid: "process-guid",
				Domain:      "some-domain",
				RootFs:      "some-stack",
				MemoryMb:    128,
				DiskMb:      512,
				MaxPids:     100,
			}

			fakeDesiredLRPDB.DesiredLRPByProcessGuidReturns(desiredLRP, nil)
		})

		It("responds with no error", func() {
			err = controller.CrashActualLRP(logger, &actualLRPKey, &beforeInstanceKey, errorMessage)
			Expect(err).NotTo(HaveOccurred())
		})

		It("crashes the actual lrp by process guid and index", func() {
			err = controller.CrashActualLRP(logger, &actualLRPKey, &beforeInstanceKey, errorMessage)
			Expect(fakeActualLRPDB.CrashActualLRPCallCount()).To(Equal(1))
			_, actualKey, actualInstanceKey, actualErrorMessage := fakeActualLRPDB.CrashActualLRPArgsForCall(0)
			Expect(*actualKey).To(Equal(actualLRPKey))
			Expect(*actualInstanceKey).To(Equal(beforeInstanceKey))
			Expect(actualErrorMessage).To(Equal(errorMessage))
		})

		It("emits both crashed and change events to the hub", func() {
			err = controller.CrashActualLRP(logger, &actualLRPKey, &beforeInstanceKey, errorMessage)
			Expect(err).NotTo(HaveOccurred())
			Eventually(actualHub.EmitCallCount).Should(Equal(2))
			Consistently(actualHub.EmitCallCount).Should(Equal(2))

			events := []models.Event{
				actualHub.EmitArgsForCall(0),
				actualHub.EmitArgsForCall(1),
			}

			Expect(events).To(ConsistOf(&models.ActualLRPCrashedEvent{
				ActualLRPKey:         actualLRP.ActualLRPKey,
				ActualLRPInstanceKey: actualLRP.ActualLRPInstanceKey,
				Since:                afterActualLRP.Since,
				CrashCount:           1,
				CrashReason:          errorMessage,
			},
				&models.ActualLRPChangedEvent{
					Before: actualLRP.ToActualLRPGroup(),
					After:  afterActualLRP.ToActualLRPGroup(),
				}))
		})

		Describe("restarting the instance", func() {
			Context("when the actual LRP should be restarted", func() {
				It("request an auction", func() {
					err = controller.CrashActualLRP(logger, &actualLRPKey, &beforeInstanceKey, errorMessage)
					Expect(err).NotTo(HaveOccurred())

					Expect(fakeDesiredLRPDB.DesiredLRPByProcessGuidCallCount()).To(Equal(1))
					_, processGuid := fakeDesiredLRPDB.DesiredLRPByProcessGuidArgsForCall(0)
					Expect(processGuid).To(Equal("process-guid"))

					Expect(fakeAuctioneerClient.RequestLRPAuctionsCallCount()).To(Equal(1))
					_, startRequests := fakeAuctioneerClient.RequestLRPAuctionsArgsForCall(0)
					Expect(startRequests).To(HaveLen(1))
					schedulingInfo := desiredLRP.DesiredLRPSchedulingInfo()
					expectedStartRequest := auctioneer.NewLRPStartRequestFromSchedulingInfo(&schedulingInfo, 1)
					Expect(startRequests[0]).To(BeEquivalentTo(&expectedStartRequest))
				})

				It("emits crashed, created & removed events to the actual instance hub", func() {
					err = controller.CrashActualLRP(logger, &actualLRPKey, &beforeInstanceKey, errorMessage)
					Expect(err).NotTo(HaveOccurred())
					Eventually(actualLRPInstanceHub.EmitCallCount).Should(Equal(3))
					Consistently(actualLRPInstanceHub.EmitCallCount).Should(Equal(3))

					Expect(actualLRPInstanceHub.EmitArgsForCall(0)).To(Equal(&models.ActualLRPCrashedEvent{
						ActualLRPKey:         actualLRP.ActualLRPKey,
						ActualLRPInstanceKey: actualLRP.ActualLRPInstanceKey,
						Since:                afterActualLRP.Since,
						CrashCount:           1,
						CrashReason:          errorMessage,
					}))

					Expect(actualLRPInstanceHub.EmitArgsForCall(1)).To(Equal(&models.ActualLRPInstanceCreatedEvent{
						ActualLrp: afterActualLRP,
					}))

					Expect(actualLRPInstanceHub.EmitArgsForCall(2)).To(Equal(&models.ActualLRPInstanceRemovedEvent{
						ActualLrp: actualLRP,
					}))
				})
			})

			Context("when the actual lrp should not be restarted (e.g., crashed)", func() {
				BeforeEach(func() {
					afterActualLRPState = models.ActualLRPStateCrashed
					shouldRestart = false
				})

				It("does not request an auction", func() {
					err = controller.CrashActualLRP(logger, &actualLRPKey, &beforeInstanceKey, errorMessage)
					Expect(err).NotTo(HaveOccurred())
					Expect(fakeAuctioneerClient.RequestLRPAuctionsCallCount()).To(Equal(0))
				})

				It("emits crashed and changed event to the actual instance hub", func() {
					err = controller.CrashActualLRP(logger, &actualLRPKey, &beforeInstanceKey, errorMessage)
					Expect(err).NotTo(HaveOccurred())
					Eventually(actualLRPInstanceHub.EmitCallCount).Should(Equal(2))
					Consistently(actualLRPInstanceHub.EmitCallCount).Should(Equal(2))

					Expect(actualLRPInstanceHub.EmitArgsForCall(0)).To(Equal(&models.ActualLRPCrashedEvent{
						ActualLRPKey:         actualLRP.ActualLRPKey,
						ActualLRPInstanceKey: actualLRP.ActualLRPInstanceKey,
						Since:                afterActualLRP.Since,
						CrashCount:           1,
						CrashReason:          errorMessage,
					}))

					Expect(actualLRPInstanceHub.EmitArgsForCall(1)).To(Equal(models.NewActualLRPInstanceChangedEvent(actualLRP, afterActualLRP)))
				})
			})

			Context("when fetching the desired lrp fails", func() {
				JustBeforeEach(func() {
					fakeDesiredLRPDB.DesiredLRPByProcessGuidReturns(nil, errors.New("error occured"))
				})

				It("fails and does not request an auction", func() {
					err = controller.CrashActualLRP(logger, &actualLRPKey, &beforeInstanceKey, errorMessage)
					Expect(err).To(MatchError("error occured"))
					Expect(fakeAuctioneerClient.RequestLRPAuctionsCallCount()).To(Equal(0))
				})
			})

			Context("when requesting the auction fails", func() {
				BeforeEach(func() {
					fakeAuctioneerClient.RequestLRPAuctionsReturns(errors.New("some else bid higher"))
				})

				It("should not return an error", func() {
					err = controller.CrashActualLRP(logger, &actualLRPKey, &beforeInstanceKey, errorMessage)
					Expect(err).NotTo(HaveOccurred())
				})
			})
		})

		Context("when the crashed instance is a replacement for a Suspect ActualLRP", func() {
			var (
				suspectInstanceState string
				suspectLRP           *models.ActualLRP
			)

			BeforeEach(func() {
				suspectInstanceState = models.ActualLRPStateRunning
			})

			JustBeforeEach(func() {
				suspectInstanceKey := models.NewActualLRPInstanceKey("instance-guid-1", "cell-id-1")
				suspectLRP = &models.ActualLRP{
					ActualLRPKey:         actualLRPKey,
					ActualLRPInstanceKey: suspectInstanceKey,
					State:                suspectInstanceState,
					Presence:             models.ActualLRP_Suspect,
				}

				lrps = []*models.ActualLRP{actualLRP, suspectLRP}
				fakeActualLRPDB.ActualLRPsReturns(lrps, nil)
			})

			It("should not emit any events because the ActualLRPGroup remains unchanged", func() {
				err = controller.CrashActualLRP(logger, &actualLRPKey, &beforeInstanceKey, errorMessage)
				Expect(err).NotTo(HaveOccurred())

				Consistently(actualHub.EmitCallCount).Should(BeZero())
			})

			It("should emit ActualLRPInstanceCrashedEvent, ActualLRPInstnceRemovedEvent & ActualLRPInstanceCreatedEvent", func() {
				err = controller.CrashActualLRP(logger, &actualLRPKey, &beforeInstanceKey, errorMessage)
				Expect(err).NotTo(HaveOccurred())

				Eventually(actualLRPInstanceHub.EmitCallCount).Should(Equal(3))
				Consistently(actualLRPInstanceHub.EmitCallCount).Should(Equal(3))

				Expect(actualLRPInstanceHub.EmitArgsForCall(0)).To(Equal(&models.ActualLRPCrashedEvent{
					ActualLRPKey:         actualLRP.ActualLRPKey,
					ActualLRPInstanceKey: actualLRP.ActualLRPInstanceKey,
					Since:                afterActualLRP.Since,
					CrashCount:           1,
					CrashReason:          errorMessage,
				}))

				Expect(actualLRPInstanceHub.EmitArgsForCall(1)).To(Equal(&models.ActualLRPInstanceCreatedEvent{
					ActualLrp: afterActualLRP,
				}))

				Expect(actualLRPInstanceHub.EmitArgsForCall(2)).To(Equal(&models.ActualLRPInstanceRemovedEvent{
					ActualLrp: actualLRP,
				}))
			})
		})

		Context("when the LRP being crashed is a Suspect LRP", func() {
			BeforeEach(func() {
				presence = models.ActualLRP_Suspect
			})

			JustBeforeEach(func() {
				fakeSuspectDB.RemoveSuspectActualLRPReturns(actualLRP, nil)
			})

			It("removes the Suspect LRP", func() {
				err = controller.CrashActualLRP(logger, &actualLRPKey, &beforeInstanceKey, errorMessage)
				Expect(fakeSuspectDB.RemoveSuspectActualLRPCallCount()).To(Equal(1))

				_, lrpKey := fakeSuspectDB.RemoveSuspectActualLRPArgsForCall(0)
				Expect(lrpKey.ProcessGuid).To(Equal(processGuid))
				Expect(lrpKey.Index).To(BeEquivalentTo(index))
			})

			It("emits an ActualLRPRemovedEvent", func() {
				err = controller.CrashActualLRP(logger, &actualLRPKey, &beforeInstanceKey, errorMessage)
				Eventually(actualHub.EmitCallCount).Should(Equal(1))
				Consistently(actualHub.EmitCallCount).Should(Equal(1))

				event := actualHub.EmitArgsForCall(0)
				removedEvent, ok := event.(*models.ActualLRPRemovedEvent)
				Expect(ok).To(BeTrue())
				Expect(removedEvent.ActualLrpGroup).To(Equal(actualLRP.ToActualLRPGroup()))
			})

			It("emits an ActualLRPInstanceRemovedEvent", func() {
				err = controller.CrashActualLRP(logger, &actualLRPKey, &beforeInstanceKey, errorMessage)
				Eventually(actualLRPInstanceHub.EmitCallCount).Should(Equal(1))
				Consistently(actualLRPInstanceHub.EmitCallCount).Should(Equal(1))

				event := actualLRPInstanceHub.EmitArgsForCall(0)
				removedEvent, ok := event.(*models.ActualLRPInstanceRemovedEvent)
				Expect(ok).To(BeTrue())
				Expect(removedEvent.ActualLrp).To(Equal(actualLRP))
			})

			Context("when RemoveSuspectActualLRP returns an error", func() {
				JustBeforeEach(func() {
					fakeSuspectDB.RemoveSuspectActualLRPReturns(nil, errors.New("boooom!"))
				})

				It("returns the error to the caller", func() {
					err = controller.CrashActualLRP(logger, &actualLRPKey, &beforeInstanceKey, errorMessage)
					Expect(err).To(MatchError("boooom!"))
				})

				It("does not emit any events", func() {
					err = controller.CrashActualLRP(logger, &actualLRPKey, &beforeInstanceKey, errorMessage)
					Consistently(actualHub.EmitCallCount).Should(BeZero())
				})

				It("does not emit any events", func() {
					err = controller.CrashActualLRP(logger, &actualLRPKey, &beforeInstanceKey, errorMessage)
					Consistently(actualLRPInstanceHub.EmitCallCount).Should(BeZero())
				})
			})

			Context("and a replacement instance has already been created, but not claimed", func() {
				var replacementLRP *models.ActualLRP

				BeforeEach(func() {
					replacementLRP = &models.ActualLRP{
						ActualLRPKey: actualLRPKey,
						State:        models.ActualLRPStateUnclaimed,
						Presence:     models.ActualLRP_Ordinary,
					}
				})

				JustBeforeEach(func() {
					lrps = []*models.ActualLRP{actualLRP, replacementLRP}
					fakeActualLRPDB.ActualLRPsReturns(lrps, nil)
				})

				It("emits an ActualLRPChangedEvent because the replacement instance is unclaimed and is indistinguishable from changing the suspect LRP", func() {
					err = controller.CrashActualLRP(logger, &actualLRPKey, &beforeInstanceKey, errorMessage)
					Eventually(actualHub.EmitCallCount).Should(Equal(1))

					event := actualHub.EmitArgsForCall(0)
					var createdEvent *models.ActualLRPChangedEvent
					Expect(event).To(BeAssignableToTypeOf(createdEvent))
					createdEvent = event.(*models.ActualLRPChangedEvent)
					Expect(createdEvent.Before).To(Equal(actualLRP.ToActualLRPGroup()))
					Expect(createdEvent.After).To(Equal(replacementLRP.ToActualLRPGroup()))
				})

				It("emits an ActualLRPInstanceRemovedEvent", func() {
					err = controller.CrashActualLRP(logger, &actualLRPKey, &beforeInstanceKey, errorMessage)
					Eventually(actualLRPInstanceHub.EmitCallCount).Should(Equal(1))
					Consistently(actualLRPInstanceHub.EmitCallCount).Should(Equal(1))

					event := actualLRPInstanceHub.EmitArgsForCall(0)
					var removedEvent *models.ActualLRPInstanceRemovedEvent
					Expect(event).To(BeAssignableToTypeOf(removedEvent))
					removedEvent = event.(*models.ActualLRPInstanceRemovedEvent)
					Expect(removedEvent.ActualLrp).To(Equal(actualLRP))
				})
			})

			Context("and a replacement instance has already been claimed", func() {
				var replacementLRP *models.ActualLRP
				BeforeEach(func() {
					ordinaryInstanceKey := models.NewActualLRPInstanceKey("instance-guid-1", "cell-id-1")
					replacementLRP = &models.ActualLRP{
						ActualLRPKey:         actualLRPKey,
						ActualLRPInstanceKey: ordinaryInstanceKey,
						State:                models.ActualLRPStateClaimed,
						Presence:             models.ActualLRP_Ordinary,
					}
				})

				JustBeforeEach(func() {
					lrps = []*models.ActualLRP{actualLRP, replacementLRP}
					fakeActualLRPDB.ActualLRPsReturns(lrps, nil)
				})

				It("emits an ActualLRPCreatedEvent and an ActualLRPRemovedEvent because the replacement instance has taken over the ActualLRPGroup", func() {
					err = controller.CrashActualLRP(logger, &actualLRPKey, &beforeInstanceKey, errorMessage)
					Eventually(actualHub.EmitCallCount).Should(Equal(2))

					event := actualHub.EmitArgsForCall(0)
					var createdEvent *models.ActualLRPCreatedEvent
					Expect(event).To(BeAssignableToTypeOf(createdEvent))
					createdEvent = event.(*models.ActualLRPCreatedEvent)
					Expect(createdEvent.ActualLrpGroup).To(Equal(replacementLRP.ToActualLRPGroup()))

					event = actualHub.EmitArgsForCall(1)
					var removedEvent *models.ActualLRPRemovedEvent
					Expect(event).To(BeAssignableToTypeOf(removedEvent))
					removedEvent = event.(*models.ActualLRPRemovedEvent)
					Expect(removedEvent.ActualLrpGroup).To(Equal(actualLRP.ToActualLRPGroup()))
				})

				It("emits an ActualLRPInstanceRemovedEvent", func() {
					err = controller.CrashActualLRP(logger, &actualLRPKey, &beforeInstanceKey, errorMessage)
					Eventually(actualLRPInstanceHub.EmitCallCount).Should(Equal(1))
					Consistently(actualLRPInstanceHub.EmitCallCount).Should(Equal(1))

					event := actualLRPInstanceHub.EmitArgsForCall(0)
					var removedEvent *models.ActualLRPInstanceRemovedEvent
					Expect(event).To(BeAssignableToTypeOf(removedEvent))
					removedEvent = event.(*models.ActualLRPInstanceRemovedEvent)
					Expect(removedEvent.ActualLrp).To(Equal(actualLRP))
				})
			})
		})

		Context("when crashing the actual lrp fails", func() {
			JustBeforeEach(func() {
				fakeActualLRPDB.CrashActualLRPReturns(nil, nil, false, models.ErrUnknownError)
			})

			It("responds with an error", func() {
				err = controller.CrashActualLRP(logger, &actualLRPKey, &beforeInstanceKey, errorMessage)
				Expect(err).To(MatchError(models.ErrUnknownError))
			})

			It("does not emit a change event to the hub", func() {
				err = controller.CrashActualLRP(logger, &actualLRPKey, &beforeInstanceKey, errorMessage)
				Consistently(actualHub.EmitCallCount).Should(Equal(0))
			})

			It("does not emit an instance change event to the hub", func() {
				err = controller.CrashActualLRP(logger, &actualLRPKey, &beforeInstanceKey, errorMessage)
				Consistently(actualLRPInstanceHub.EmitCallCount).Should(Equal(0))
			})
		})
	})

	Describe("FailActualLRP", func() {
		var (
			errorMessage string
		)

		BeforeEach(func() {
			errorMessage = "something went wrong"
			actualLRPState = models.ActualLRPStateUnclaimed
			afterActualLRPState = models.ActualLRPStateUnclaimed
			afterActualLRPCrashCount = 0
			afterActualLRPCrashReason = ""
		})

		JustBeforeEach(func() {
			afterActualLRP.PlacementError = "cannot place the LRP"
			fakeActualLRPDB.FailActualLRPReturns(actualLRP, afterActualLRP, nil)
			fakeActualLRPDB.ActualLRPsReturns([]*models.ActualLRP{actualLRP}, nil)
		})

		Context("when failing the actual lrp in the DB succeeds", func() {
			JustBeforeEach(func() {
				fakeActualLRPDB.FailActualLRPReturns(actualLRP, afterActualLRP, nil)
			})

			It("fails the actual lrp by process guid and index", func() {
				err = controller.FailActualLRP(logger, &actualLRPKey, errorMessage)
				Expect(fakeActualLRPDB.FailActualLRPCallCount()).To(Equal(1))
				_, actualKey, actualErrorMessage := fakeActualLRPDB.FailActualLRPArgsForCall(0)
				Expect(*actualKey).To(Equal(actualLRPKey))
				Expect(actualErrorMessage).To(Equal(errorMessage))

				Expect(err).NotTo(HaveOccurred())
			})

			It("emits a change event to the hub", func() {
				err = controller.FailActualLRP(logger, &actualLRPKey, errorMessage)
				Eventually(actualHub.EmitCallCount).Should(Equal(1))
				event := actualHub.EmitArgsForCall(0)
				Expect(event).To(Equal(models.NewActualLRPChangedEvent(
					actualLRP.ToActualLRPGroup(),
					afterActualLRP.ToActualLRPGroup(),
				)))
			})

			It("emits an instance change event to the hub", func() {
				err = controller.FailActualLRP(logger, &actualLRPKey, errorMessage)
				Eventually(actualLRPInstanceHub.EmitCallCount).Should(Equal(1))
				Expect(actualLRPInstanceHub.EmitArgsForCall(0)).To(Equal(
					models.NewActualLRPInstanceChangedEvent(actualLRP, afterActualLRP),
				))
			})
		})

		Context("when there is a Suspect LRP running", func() {
			JustBeforeEach(func() {
				suspectLRP := model_helpers.NewValidActualLRP(actualLRP.ProcessGuid, actualLRP.Index)
				suspectLRP.Domain = actualLRP.Domain
				suspectLRP.Presence = models.ActualLRP_Suspect

				fakeActualLRPDB.ActualLRPsReturns([]*models.ActualLRP{suspectLRP, actualLRP}, nil)
			})

			It("does not emit a ActualLRPChangedEvent", func() {
				err = controller.FailActualLRP(logger, &actualLRPKey, errorMessage)
				Consistently(actualHub.EmitCallCount).Should(BeZero())
			})

			It("emits an ActualLRPInstanceChangedEvent", func() {
				err = controller.FailActualLRP(logger, &actualLRPKey, errorMessage)
				Expect(err).NotTo(HaveOccurred())
				Eventually(actualLRPInstanceHub.EmitCallCount).Should(Equal(1))
				Expect(actualLRPInstanceHub.EmitArgsForCall(0)).To(Equal(
					models.NewActualLRPInstanceChangedEvent(actualLRP, afterActualLRP),
				))
			})

			Context("when there is only a suspect instance", func() {
				JustBeforeEach(func() {
					fakeActualLRPDB.FailActualLRPReturns(nil, nil, models.ErrResourceNotFound)
				})

				It("does not error", func() {
					err = controller.FailActualLRP(logger, &actualLRPKey, errorMessage)
					Expect(err).To(BeNil())
				})

				It("does not emit a ActualLRPChangedEvent", func() {
					err = controller.FailActualLRP(logger, &actualLRPKey, errorMessage)
					Consistently(actualHub.EmitCallCount).Should(BeZero())
				})

				It("does not emit an ActualLRPInstanceChangedEvent", func() {
					err = controller.FailActualLRP(logger, &actualLRPKey, errorMessage)
					Consistently(actualHub.EmitCallCount).Should(BeZero())
				})
			})
		})

		Context("when failing the actual lrp fails with a non-ResourceNotFound error", func() {
			JustBeforeEach(func() {
				fakeActualLRPDB.FailActualLRPReturns(nil, nil, models.ErrUnknownError)
			})

			It("responds with an error", func() {
				err = controller.FailActualLRP(logger, &actualLRPKey, errorMessage)
				Expect(err).To(MatchError(models.ErrUnknownError))
			})

			It("does not emit a change event to the hub", func() {
				err = controller.FailActualLRP(logger, &actualLRPKey, errorMessage)
				Consistently(actualHub.EmitCallCount).Should(Equal(0))
			})

			It("does not emit a ActualLRPInstanceChangedEvent", func() {
				err = controller.FailActualLRP(logger, &actualLRPKey, errorMessage)
				Consistently(actualLRPInstanceHub.EmitCallCount).Should(BeZero())
			})
		})

		Context("when there is no LRP", func() {
			JustBeforeEach(func() {
				fakeActualLRPDB.FailActualLRPReturns(nil, nil, models.ErrResourceNotFound)
				fakeActualLRPDB.ActualLRPsReturns(nil, models.ErrResourceNotFound)
			})

			It("responds with a ResourceNotFound error", func() {
				err = controller.FailActualLRP(logger, &actualLRPKey, errorMessage)
				Expect(err).To(MatchError(models.ErrResourceNotFound))
			})

			It("does not emit a change event to the hub", func() {
				err = controller.FailActualLRP(logger, &actualLRPKey, errorMessage)
				Consistently(actualHub.EmitCallCount).Should(Equal(0))
			})

			It("does not emit a ActualLRPInstanceChangedEvent", func() {
				err = controller.FailActualLRP(logger, &actualLRPKey, errorMessage)
				Consistently(actualLRPInstanceHub.EmitCallCount).Should(BeZero())
			})
		})
	})

	Describe("RemoveActualLRP", func() {
		JustBeforeEach(func() {
			fakeActualLRPDB.ActualLRPsReturns([]*models.ActualLRP{actualLRP}, nil)
		})

		Context("when removing the actual lrp in the DB succeeds", func() {
			JustBeforeEach(func() {
				fakeActualLRPDB.RemoveActualLRPReturns(nil)
			})

			It("removes the actual lrp by process guid and index", func() {
				controller.RemoveActualLRP(logger, processGuid, index, &afterInstanceKey)
				Expect(fakeActualLRPDB.RemoveActualLRPCallCount()).To(Equal(1))

				_, actualProcessGuid, idx, actualInstanceKey := fakeActualLRPDB.RemoveActualLRPArgsForCall(0)
				Expect(actualProcessGuid).To(Equal(processGuid))
				Expect(idx).To(BeEquivalentTo(index))
				Expect(actualInstanceKey).To(Equal(&afterInstanceKey))
			})

			It("response with no error", func() {
				err = controller.RemoveActualLRP(logger, processGuid, index, &afterInstanceKey)
				Expect(err).NotTo(HaveOccurred())
			})

			It("emits a removed event to the hub", func() {
				controller.RemoveActualLRP(logger, processGuid, index, &afterInstanceKey)
				Eventually(actualHub.EmitCallCount).Should(Equal(1))
				event := actualHub.EmitArgsForCall(0)
				removedEvent, ok := event.(*models.ActualLRPRemovedEvent)
				Expect(ok).To(BeTrue())
				Expect(removedEvent.ActualLrpGroup).To(Equal(actualLRP.ToActualLRPGroup()))
			})

			It("emits an instance removed event to the hub", func() {
				controller.RemoveActualLRP(logger, processGuid, index, &afterInstanceKey)
				Eventually(actualLRPInstanceHub.EmitCallCount).Should(Equal(1))
				event := actualLRPInstanceHub.EmitArgsForCall(0)
				removedEvent, ok := event.(*models.ActualLRPInstanceRemovedEvent)
				Expect(ok).To(BeTrue())
				Expect(removedEvent.ActualLrp).To(Equal(actualLRP))
			})
		})

		Context("when the DB does not return any LRPs", func() {
			JustBeforeEach(func() {
				fakeActualLRPDB.ActualLRPsReturns([]*models.ActualLRP{}, nil)
			})

			It("returns an ErrResourceNotFound", func() {
				err = controller.RemoveActualLRP(logger, processGuid, index, &afterInstanceKey)
				Expect(err).To(MatchError(models.ErrResourceNotFound))
			})

			It("does not emit a removed event to the hub", func() {
				controller.RemoveActualLRP(logger, processGuid, index, &afterInstanceKey)
				Consistently(actualHub.EmitCallCount).Should(Equal(0))
			})

			It("does not emit an instance removed event to the hub", func() {
				controller.RemoveActualLRP(logger, processGuid, index, &afterInstanceKey)
				Consistently(actualLRPInstanceHub.EmitCallCount).Should(Equal(0))
			})
		})

		Context("when there is no ordinary LRP with a matching process guid and index", func() {
			JustBeforeEach(func() {
				actualLRP.Presence = models.ActualLRP_Evacuating
				fakeActualLRPDB.ActualLRPsReturns([]*models.ActualLRP{actualLRP}, nil)
			})

			It("returns an ErrResourceNotFound", func() {
				err = controller.RemoveActualLRP(logger, processGuid, index, &afterInstanceKey)
				Expect(err).To(MatchError(models.ErrResourceNotFound))
			})

			It("does not emit a removed event to the hub", func() {
				controller.RemoveActualLRP(logger, processGuid, index, &afterInstanceKey)
				Consistently(actualHub.EmitCallCount).Should(Equal(0))
			})

			It("does not emit an instance removed event to the hub", func() {
				controller.RemoveActualLRP(logger, processGuid, index, &afterInstanceKey)
				Consistently(actualLRPInstanceHub.EmitCallCount).Should(Equal(0))
			})
		})

		Context("when the DB returns an error", func() {
			Context("when doing the actual LRP lookup", func() {
				JustBeforeEach(func() {
					fakeActualLRPDB.ActualLRPsReturns(nil, models.ErrUnknownError)
				})

				It("returns the error", func() {
					err = controller.RemoveActualLRP(logger, processGuid, index, &afterInstanceKey)
					Expect(err).To(MatchError(models.ErrUnknownError))
				})

				It("does not emit a removed event to the hub", func() {
					controller.RemoveActualLRP(logger, processGuid, index, &afterInstanceKey)
					Consistently(actualHub.EmitCallCount).Should(Equal(0))
				})

				It("does not emit an instance removed event to the hub", func() {
					controller.RemoveActualLRP(logger, processGuid, index, &afterInstanceKey)
					Consistently(actualLRPInstanceHub.EmitCallCount).Should(Equal(0))
				})
			})

			Context("when doing the actual LRP removal", func() {
				JustBeforeEach(func() {
					fakeActualLRPDB.RemoveActualLRPReturns(models.ErrUnknownError)
				})

				It("returns the error", func() {
					err = controller.RemoveActualLRP(logger, processGuid, index, &afterInstanceKey)
					Expect(err).To(MatchError(models.ErrUnknownError))
				})

				It("does not emit a removed event to the hub", func() {
					controller.RemoveActualLRP(logger, processGuid, index, &afterInstanceKey)
					Consistently(actualHub.EmitCallCount).Should(Equal(0))
				})

				It("does not emit an instance removed event to the hub", func() {
					controller.RemoveActualLRP(logger, processGuid, index, &afterInstanceKey)
					Consistently(actualLRPInstanceHub.EmitCallCount).Should(Equal(0))
				})
			})
		})
	})

	Describe("RetireActualLRP", func() {
		Context("when finding the actualLRP fails", func() {
			JustBeforeEach(func() {
				fakeActualLRPDB.ActualLRPsReturns(nil, models.ErrUnknownError)
			})

			It("returns an error and does not retry", func() {
				err = controller.RetireActualLRP(logger, &actualLRPKey)
				Expect(err).To(MatchError(models.ErrUnknownError))
				Expect(fakeActualLRPDB.ActualLRPsCallCount()).To(Equal(1))
			})

			It("does not emit a removed event to the hub", func() {
				err = controller.RetireActualLRP(logger, &actualLRPKey)
				Consistently(actualHub.EmitCallCount).Should(Equal(0))
			})

			It("does not emit an instance removed event to the hub", func() {
				controller.RetireActualLRP(logger, &actualLRPKey)
				Consistently(actualLRPInstanceHub.EmitCallCount).Should(Equal(0))
			})
		})

		Context("when there is no matching actual lrp", func() {
			JustBeforeEach(func() {
				fakeActualLRPDB.ActualLRPsReturns([]*models.ActualLRP{}, nil)
			})

			It("returns an error and does not retry", func() {
				err = controller.RetireActualLRP(logger, &actualLRPKey)
				Expect(err).To(Equal(models.ErrResourceNotFound))
				Expect(fakeActualLRPDB.ActualLRPsCallCount()).To(Equal(1))
			})

			It("does not emit a removed event to the hub", func() {
				err = controller.RetireActualLRP(logger, &actualLRPKey)
				Consistently(actualHub.EmitCallCount).Should(Equal(0))
			})

			It("does not emit an instance removed event to the hub", func() {
				controller.RetireActualLRP(logger, &actualLRPKey)
				Consistently(actualLRPInstanceHub.EmitCallCount).Should(Equal(0))
			})
		})

		Context("with an Unclaimed LRP", func() {
			BeforeEach(func() {
				actualLRPState = models.ActualLRPStateUnclaimed
			})

			JustBeforeEach(func() {
				fakeActualLRPDB.ActualLRPsReturns([]*models.ActualLRP{actualLRP}, nil)
			})

			It("removes the LRP", func() {
				err = controller.RetireActualLRP(logger, &actualLRPKey)
				Expect(err).NotTo(HaveOccurred())
				Expect(fakeActualLRPDB.RemoveActualLRPCallCount()).To(Equal(1))

				_, deletedLRPGuid, deletedLRPIndex, deletedLRPInstanceKey := fakeActualLRPDB.RemoveActualLRPArgsForCall(0)
				Expect(deletedLRPGuid).To(Equal(processGuid))
				Expect(deletedLRPIndex).To(Equal(index))
				Expect(deletedLRPInstanceKey).To(Equal(&actualLRP.ActualLRPInstanceKey))
			})

			It("emits a removed event to the hub", func() {
				err = controller.RetireActualLRP(logger, &actualLRPKey)
				Eventually(actualHub.EmitCallCount).Should(Equal(1))
				event := actualHub.EmitArgsForCall(0)
				removedEvent, ok := event.(*models.ActualLRPRemovedEvent)
				Expect(ok).To(BeTrue())
				Expect(removedEvent.ActualLrpGroup).To(Equal(actualLRP.ToActualLRPGroup()))
			})

			It("emits an instance removed event to the hub", func() {
				err = controller.RetireActualLRP(logger, &actualLRPKey)
				Eventually(actualLRPInstanceHub.EmitCallCount).Should(Equal(1))
				event := actualLRPInstanceHub.EmitArgsForCall(0)
				removedEvent, ok := event.(*models.ActualLRPInstanceRemovedEvent)
				Expect(ok).To(BeTrue())
				Expect(removedEvent.ActualLrp).To(Equal(actualLRP))
			})

			Context("when removing the actual lrp fails", func() {
				JustBeforeEach(func() {
					fakeActualLRPDB.RemoveActualLRPReturns(errors.New("boom!"))
				})

				It("retries removing up to RetireActualLRPRetryAttempts times", func() {
					err = controller.RetireActualLRP(logger, &actualLRPKey)
					Expect(err).To(MatchError("boom!"))
					Expect(fakeActualLRPDB.RemoveActualLRPCallCount()).To(Equal(5))
				})

				It("does not emit a change event to the hub", func() {
					err = controller.RetireActualLRP(logger, &actualLRPKey)
					Consistently(actualHub.EmitCallCount).Should(Equal(0))
				})

				It("does not emit an instance removed event to the hub", func() {
					controller.RetireActualLRP(logger, &actualLRPKey)
					Consistently(actualLRPInstanceHub.EmitCallCount).Should(Equal(0))
				})
			})
		})

		Context("when the LRP is crashed", func() {
			BeforeEach(func() {
				actualLRPState = models.ActualLRPStateCrashed
			})

			JustBeforeEach(func() {
				fakeActualLRPDB.ActualLRPsReturns([]*models.ActualLRP{actualLRP}, nil)
			})

			It("removes the LRP", func() {
				err = controller.RetireActualLRP(logger, &actualLRPKey)
				Expect(err).NotTo(HaveOccurred())
				Expect(fakeActualLRPDB.RemoveActualLRPCallCount()).To(Equal(1))

				_, deletedLRPGuid, deletedLRPIndex, deletedLRPInstanceKey := fakeActualLRPDB.RemoveActualLRPArgsForCall(0)
				Expect(deletedLRPGuid).To(Equal(processGuid))
				Expect(deletedLRPIndex).To(Equal(index))
				Expect(deletedLRPInstanceKey).To(Equal(&actualLRP.ActualLRPInstanceKey))
			})

			It("emits a removed event to the hub", func() {
				err = controller.RetireActualLRP(logger, &actualLRPKey)
				Eventually(actualHub.EmitCallCount).Should(Equal(1))
				event := actualHub.EmitArgsForCall(0)
				removedEvent, ok := event.(*models.ActualLRPRemovedEvent)
				Expect(ok).To(BeTrue())
				Expect(removedEvent.ActualLrpGroup).To(Equal(actualLRP.ToActualLRPGroup()))
			})

			It("emits an instance removed event to the hub", func() {
				err = controller.RetireActualLRP(logger, &actualLRPKey)
				Eventually(actualLRPInstanceHub.EmitCallCount).Should(Equal(1))
				event := actualLRPInstanceHub.EmitArgsForCall(0)
				removedEvent, ok := event.(*models.ActualLRPInstanceRemovedEvent)
				Expect(ok).To(BeTrue())
				Expect(removedEvent.ActualLrp).To(Equal(actualLRP))
			})

			Context("when removing the actual lrp fails", func() {
				BeforeEach(func() {
					fakeActualLRPDB.RemoveActualLRPReturns(errors.New("boom!"))
				})

				It("retries removing up to RetireActualLRPRetryAttempts times", func() {
					err = controller.RetireActualLRP(logger, &actualLRPKey)
					Expect(err).To(MatchError("boom!"))
					Expect(fakeActualLRPDB.RemoveActualLRPCallCount()).To(Equal(5))
				})

				It("does not emit a change event to the hub", func() {
					err = controller.RetireActualLRP(logger, &actualLRPKey)
					Consistently(actualHub.EmitCallCount).Should(Equal(0))
				})

				It("does not emit an instance removed event to the hub", func() {
					controller.RetireActualLRP(logger, &actualLRPKey)
					Consistently(actualLRPInstanceHub.EmitCallCount).Should(Equal(0))
				})
			})
		})

		Context("when the LRP is Claimed or Running", func() {
			var (
				cellPresence models.CellPresence
			)

			BeforeEach(func() {
				actualLRPState = models.ActualLRPStateClaimed
			})

			JustBeforeEach(func() {
				fakeActualLRPDB.ActualLRPsReturns([]*models.ActualLRP{actualLRP}, nil)
			})

			Context("when the cell", func() {
				Context("is present", func() {
					BeforeEach(func() {
						cellPresence = models.NewCellPresence(
							"cell-id-0",
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
						err = controller.RetireActualLRP(logger, &actualLRPKey)
						Expect(fakeRepClientFactory.CreateClientCallCount()).To(Equal(1))
						Expect(fakeRepClientFactory.CreateClientArgsForCall(0)).To(Equal(cellPresence.RepAddress))

						Expect(fakeServiceClient.CellByIdCallCount()).To(Equal(1))
						_, fetchedCellID := fakeServiceClient.CellByIdArgsForCall(0)
						Expect(fetchedCellID).To(Equal("cell-id-0"))

						Expect(fakeRepClient.StopLRPInstanceCallCount()).Should(Equal(1))
						_, stoppedKey, stoppedInstanceKey := fakeRepClient.StopLRPInstanceArgsForCall(0)
						Expect(stoppedKey).To(Equal(actualLRPKey))
						Expect(stoppedInstanceKey).To(Equal(afterInstanceKey))
					})

					Context("when the rep announces a rep url", func() {
						BeforeEach(func() {
							cellPresence = models.NewCellPresence(
								"cell-id-0",
								"cell1.addr",
								"http://cell1.addr",
								"the-zone",
								models.NewCellCapacity(128, 1024, 6),
								[]string{},
								[]string{},
								[]string{},
								[]string{},
							)

							fakeServiceClient.CellByIdReturns(&cellPresence, nil)
						})

						It("passes the url when creating a rep client", func() {
							err = controller.RetireActualLRP(logger, &actualLRPKey)
							Expect(fakeRepClientFactory.CreateClientCallCount()).To(Equal(1))
							repAddr, repURL := fakeRepClientFactory.CreateClientArgsForCall(0)
							Expect(repAddr).To(Equal(cellPresence.RepAddress))
							Expect(repURL).To(Equal(cellPresence.RepUrl))
						})
					})

					Context("when creating a rep client fails", func() {
						BeforeEach(func() {
							err := errors.New("BOOM!!!")
							fakeRepClientFactory.CreateClientReturns(nil, err)
						})

						It("should return the error", func() {
							err = controller.RetireActualLRP(logger, &actualLRPKey)
							Expect(err).To(MatchError("BOOM!!!"))
						})
					})

					Context("Stopping the LRP fails", func() {
						JustBeforeEach(func() {
							fakeRepClient.StopLRPInstanceReturns(errors.New("Failed to stop app"))
						})

						It("retries to stop the app", func() {
							err = controller.RetireActualLRP(logger, &actualLRPKey)
							Expect(err).To(MatchError("Failed to stop app"))
							Expect(fakeRepClient.StopLRPInstanceCallCount()).Should(Equal(5))
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
							err = controller.RetireActualLRP(logger, &actualLRPKey)
							Expect(err).NotTo(HaveOccurred())
							Expect(fakeActualLRPDB.RemoveActualLRPCallCount()).To(Equal(1))

							_, deletedLRPGuid, deletedLRPIndex, deletedLRPInstanceKey := fakeActualLRPDB.RemoveActualLRPArgsForCall(0)
							Expect(deletedLRPGuid).To(Equal(processGuid))
							Expect(deletedLRPIndex).To(Equal(index))
							Expect(deletedLRPInstanceKey).To(Equal(&afterInstanceKey))
						})

						It("emits a removed event to the hub", func() {
							err = controller.RetireActualLRP(logger, &actualLRPKey)
							Eventually(actualHub.EmitCallCount).Should(Equal(1))
							event := actualHub.EmitArgsForCall(0)
							removedEvent, ok := event.(*models.ActualLRPRemovedEvent)
							Expect(ok).To(BeTrue())
							Expect(removedEvent.ActualLrpGroup).To(Equal(actualLRP.ToActualLRPGroup()))
						})

						It("emits an instance removed event to the hub", func() {
							err = controller.RetireActualLRP(logger, &actualLRPKey)
							Eventually(actualLRPInstanceHub.EmitCallCount).Should(Equal(1))
							event := actualLRPInstanceHub.EmitArgsForCall(0)
							removedEvent, ok := event.(*models.ActualLRPInstanceRemovedEvent)
							Expect(ok).To(BeTrue())
							Expect(removedEvent.ActualLrp).To(Equal(actualLRP))
						})
					})

					Context("removing the actualLRP fails", func() {
						JustBeforeEach(func() {
							fakeActualLRPDB.RemoveActualLRPReturns(errors.New("failed to delete actual LRP"))
						})

						It("returns an error and does not retry", func() {
							err = controller.RetireActualLRP(logger, &actualLRPKey)
							Expect(err).To(MatchError("failed to delete actual LRP"))
							Expect(fakeActualLRPDB.RemoveActualLRPCallCount()).To(Equal(1))
						})

						It("does not emit a change event to the hub", func() {
							err = controller.RetireActualLRP(logger, &actualLRPKey)
							Consistently(actualHub.EmitCallCount).Should(Equal(0))
						})

						It("does not emit an instance removed event to the hub", func() {
							controller.RetireActualLRP(logger, &actualLRPKey)
							Consistently(actualLRPInstanceHub.EmitCallCount).Should(Equal(0))
						})
					})
				})

				Context("is present, but returns an error on lookup", func() {
					BeforeEach(func() {
						fakeServiceClient.CellByIdReturns(nil, errors.New("cell error"))
					})

					It("returns an error and retries", func() {
						err = controller.RetireActualLRP(logger, &actualLRPKey)
						Expect(err).To(MatchError("cell error"))
						Expect(fakeActualLRPDB.RemoveActualLRPCallCount()).To(Equal(0))
						Expect(fakeServiceClient.CellByIdCallCount()).To(Equal(1))
					})

					It("does not emit a change event to the hub", func() {
						err = controller.RetireActualLRP(logger, &actualLRPKey)
						Consistently(actualHub.EmitCallCount).Should(Equal(0))
					})

					It("does not emit an instance removed event to the hub", func() {
						controller.RetireActualLRP(logger, &actualLRPKey)
						Consistently(actualLRPInstanceHub.EmitCallCount).Should(Equal(0))
					})
				})
			})
		})
	})
})
