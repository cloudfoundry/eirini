package calculator_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"code.cloudfoundry.org/bbs/events/calculator"
	"code.cloudfoundry.org/bbs/events/eventfakes"
	"code.cloudfoundry.org/bbs/models"
	"code.cloudfoundry.org/bbs/models/test/model_helpers"
	"code.cloudfoundry.org/bbs/test_helpers"
)

var _ = Describe("ActualLrpEventCalculator", func() {
	var eventCalculator calculator.ActualLRPEventCalculator
	var actualHub, actualInstanceHub *eventfakes.FakeHub

	BeforeEach(func() {
		actualHub = &eventfakes.FakeHub{}
		actualInstanceHub = &eventfakes.FakeHub{}
		eventCalculator = calculator.ActualLRPEventCalculator{
			ActualLRPGroupHub:    actualHub,
			ActualLRPInstanceHub: actualInstanceHub,
		}
	})

	Describe("EmitEvents", func() {
		var beforeSet, afterSet []*models.ActualLRP

		Context("when the before and after LRP sets are identical", func() {
			BeforeEach(func() {
				actualLRP1 := model_helpers.NewValidActualLRP("some-guid-1", 0)
				actualLRP2 := model_helpers.NewValidEvacuatingActualLRP("some-guid-1", 0)

				beforeSet = []*models.ActualLRP{actualLRP1, actualLRP2}
				afterSet = beforeSet
			})

			It("emits no events", func() {
				eventCalculator.EmitEvents(beforeSet, afterSet)
				Expect(actualHub.EmitCallCount()).To(Equal(0))
				Expect(actualInstanceHub.EmitCallCount()).To(Equal(0))
			})
		})

		Context("when an LRP is being created (i.e. the 'after' set has a new LRP in it)", func() {
			var newLRP *models.ActualLRP

			BeforeEach(func() {
				newLRP = model_helpers.NewValidActualLRP("some-guid-1", 0)

				beforeSet = []*models.ActualLRP{}
				afterSet = []*models.ActualLRP{newLRP}
			})

			It("emits a ActualLRPCreatedEvent", func() {
				eventCalculator.EmitEvents(beforeSet, afterSet)

				Expect(actualHub.EmitCallCount()).To(Equal(1))
				lrpGroupEvent := actualHub.EmitArgsForCall(0)
				Expect(lrpGroupEvent).To(Equal(&models.ActualLRPCreatedEvent{ActualLrpGroup: newLRP.ToActualLRPGroup()}))
			})

			It("emits a ActualLRPInstanceCreatedEvent", func() {
				eventCalculator.EmitEvents(beforeSet, afterSet)

				Expect(actualInstanceHub.EmitCallCount()).To(Equal(1))
				lrpInstanceEvent := actualInstanceHub.EmitArgsForCall(0)
				Expect(lrpInstanceEvent).To(Equal(&models.ActualLRPInstanceCreatedEvent{ActualLrp: newLRP}))
			})
		})

		Context("when an LRP is being deleted (i.e., the 'after' set has a nil value)", func() {
			var deletedLRP *models.ActualLRP

			BeforeEach(func() {
				deletedLRP = model_helpers.NewValidActualLRP("some-guid-1", 0)

				beforeSet = []*models.ActualLRP{deletedLRP}
				afterSet = []*models.ActualLRP{nil}
			})

			It("emits an ActualLRPRemovedEvent", func() {
				eventCalculator.EmitEvents(beforeSet, afterSet)

				Expect(actualHub.EmitCallCount()).To(Equal(1))
				lrpGroupEvent := actualHub.EmitArgsForCall(0)
				Expect(lrpGroupEvent).To(Equal(&models.ActualLRPRemovedEvent{ActualLrpGroup: deletedLRP.ToActualLRPGroup()}))
			})

			It("emits an ActualLRPInstanceRemovedEvent", func() {
				eventCalculator.EmitEvents(beforeSet, afterSet)

				Expect(actualInstanceHub.EmitCallCount()).To(Equal(1))
				lrpInstanceEvent := actualInstanceHub.EmitArgsForCall(0)
				Expect(lrpInstanceEvent).To(Equal(&models.ActualLRPInstanceRemovedEvent{ActualLrp: deletedLRP}))
			})
		})

		Context("when an ActualLRP instance changes state", func() {
			var originalLRP, updatedLRP *models.ActualLRP

			BeforeEach(func() {
				originalLRP = model_helpers.NewValidActualLRP("some-guid", 0)
				updatedLRP = model_helpers.NewValidActualLRP("some-guid", 0)

				beforeSet = []*models.ActualLRP{originalLRP}
				afterSet = []*models.ActualLRP{updatedLRP}
			})

			Context("to UNCLAIMED", func() {
				BeforeEach(func() {
					updatedLRP.State = models.ActualLRPStateUnclaimed
				})

				Context("from UNCLAIMED", func() {
					BeforeEach(func() {
						originalLRP.State = models.ActualLRPStateUnclaimed
					})

					Context("with an additional placement error", func() {
						BeforeEach(func() {
							updatedLRP.PlacementError = "Some new placemenet error"
						})

						It("emits an ActualLRPChangedEvent", func() {
							eventCalculator.EmitEvents(beforeSet, afterSet)
							Expect(actualInstanceHub.EmitCallCount()).To(Equal(1))

							changedEvent := actualInstanceHub.EmitArgsForCall(0)

							Expect(changedEvent).To(Equal(models.NewActualLRPInstanceChangedEvent(originalLRP, updatedLRP)))
						})

						It("emits no ActualLRPGroup events", func() {
							eventCalculator.EmitEvents(beforeSet, afterSet)
							Expect(actualHub.EmitCallCount()).To(Equal(1))

							changedEvent := actualHub.EmitArgsForCall(0)
							Expect(changedEvent).To(Equal(models.NewActualLRPChangedEvent(originalLRP.ToActualLRPGroup(), updatedLRP.ToActualLRPGroup())))
						})
					})
				})

				Context("from CLAIMED", func() {
					BeforeEach(func() {
						originalLRP.State = models.ActualLRPStateClaimed
					})

					It("emits a ActualLRPChangedEvent", func() {
						eventCalculator.EmitEvents(beforeSet, afterSet)
						Expect(actualHub.EmitCallCount()).To(Equal(1))

						changedEvent := actualHub.EmitArgsForCall(0)
						Expect(changedEvent).To(Equal(models.NewActualLRPChangedEvent(originalLRP.ToActualLRPGroup(), updatedLRP.ToActualLRPGroup())))
					})

					It("emits an ActualLRPInstanceCreatedEvent and a ActualLRPInstanceRemovedEvent", func() {
						eventCalculator.EmitEvents(beforeSet, afterSet)
						Expect(actualInstanceHub.EmitCallCount()).To(Equal(2))

						createdEvent := actualInstanceHub.EmitArgsForCall(0)
						removedEvent := actualInstanceHub.EmitArgsForCall(1)

						Expect(createdEvent).To(Equal(models.NewActualLRPInstanceCreatedEvent(updatedLRP)))
						Expect(removedEvent).To(Equal(models.NewActualLRPInstanceRemovedEvent(originalLRP)))
					})
				})

				Context("from RUNNING", func() {
					BeforeEach(func() {
						originalLRP.State = models.ActualLRPStateRunning
					})

					It("emits a ActualLRPChangedEvent", func() {
						eventCalculator.EmitEvents(beforeSet, afterSet)
						Expect(actualHub.EmitCallCount()).To(Equal(1))

						changedEvent := actualHub.EmitArgsForCall(0)
						Expect(changedEvent).To(Equal(models.NewActualLRPChangedEvent(originalLRP.ToActualLRPGroup(), updatedLRP.ToActualLRPGroup())))
					})

					It("emits an ActualLRPInstanceCreatedEvent and a ActualLRPInstanceRemovedEvent", func() {
						eventCalculator.EmitEvents(beforeSet, afterSet)
						Expect(actualInstanceHub.EmitCallCount()).To(Equal(2))

						createdEvent := actualInstanceHub.EmitArgsForCall(0)
						removedEvent := actualInstanceHub.EmitArgsForCall(1)

						Expect(createdEvent).To(Equal(models.NewActualLRPInstanceCreatedEvent(updatedLRP)))
						Expect(removedEvent).To(Equal(models.NewActualLRPInstanceRemovedEvent(originalLRP)))
					})
				})

				Context("from CRASHED", func() {
					BeforeEach(func() {
						originalLRP.State = models.ActualLRPStateCrashed
					})

					It("emits a ActualLRPChangedEvent", func() {
						eventCalculator.EmitEvents(beforeSet, afterSet)
						Expect(actualHub.EmitCallCount()).To(Equal(1))

						changedEvent := actualHub.EmitArgsForCall(0)
						Expect(changedEvent).To(Equal(models.NewActualLRPChangedEvent(originalLRP.ToActualLRPGroup(), updatedLRP.ToActualLRPGroup())))
					})

					It("emits an ActualLRPInstanceCreatedEvent and a ActualLRPInstanceRemovedEvent", func() {
						eventCalculator.EmitEvents(beforeSet, afterSet)
						Expect(actualInstanceHub.EmitCallCount()).To(Equal(2))

						createdEvent := actualInstanceHub.EmitArgsForCall(0)
						removedEvent := actualInstanceHub.EmitArgsForCall(1)

						Expect(createdEvent).To(Equal(models.NewActualLRPInstanceCreatedEvent(updatedLRP)))
						Expect(removedEvent).To(Equal(models.NewActualLRPInstanceRemovedEvent(originalLRP)))
					})
				})

				Context("with an incremented crash count", func() {
					BeforeEach(func() {
						updatedLRP.CrashCount += 1
					})

					It("emits an additional ActualLRPCrashedEvent to each hub", func() {
						eventCalculator.EmitEvents(beforeSet, afterSet)
						Expect(actualHub.EmitCallCount()).To(Equal(2))
						Expect(actualInstanceHub.EmitCallCount()).To(Equal(3))

						groupCrashedEvent := actualHub.EmitArgsForCall(0)
						instanceCrashedEvent := actualInstanceHub.EmitArgsForCall(0)

						Expect(groupCrashedEvent).To(Equal(models.NewActualLRPCrashedEvent(originalLRP, updatedLRP)))
						Expect(instanceCrashedEvent).To(Equal(models.NewActualLRPCrashedEvent(originalLRP, updatedLRP)))
					})
				})
			})

			Context("to CLAIMED", func() {
				BeforeEach(func() {
					updatedLRP.State = models.ActualLRPStateClaimed
				})

				Context("from UNCLAIMED", func() {
					BeforeEach(func() {
						originalLRP.State = models.ActualLRPStateUnclaimed
						originalLRP.ActualLRPInstanceKey = models.ActualLRPInstanceKey{}
					})

					It("emits an ActualLRPChanged event", func() {
						eventCalculator.EmitEvents(beforeSet, afterSet)

						Expect(actualHub.EmitCallCount()).To(Equal(1))
						changedEvent := actualHub.EmitArgsForCall(0)
						Expect(changedEvent).To(Equal(models.NewActualLRPChangedEvent(originalLRP.ToActualLRPGroup(), updatedLRP.ToActualLRPGroup())))
					})

					It("emits an ActualLRPInstanceChangedEvent", func() {
						eventCalculator.EmitEvents(beforeSet, afterSet)

						Expect(actualInstanceHub.EmitCallCount()).To(Equal(1))
						changedEvent := actualInstanceHub.EmitArgsForCall(0)
						Expect(changedEvent).To(Equal(models.NewActualLRPInstanceChangedEvent(originalLRP, updatedLRP)))
					})
				})

				Context("from RUNNING", func() {
					BeforeEach(func() {
						originalLRP.State = models.ActualLRPStateRunning
					})

					It("emits an ActualLRPChanged event", func() {
						eventCalculator.EmitEvents(beforeSet, afterSet)

						Expect(actualHub.EmitCallCount()).To(Equal(1))
						changedEvent := actualHub.EmitArgsForCall(0)
						Expect(changedEvent).To(Equal(models.NewActualLRPChangedEvent(
							originalLRP.ToActualLRPGroup(),
							updatedLRP.ToActualLRPGroup(),
						)))
					})

					It("emits an ActualLRPInstanceChangedEvent", func() {
						eventCalculator.EmitEvents(beforeSet, afterSet)

						Expect(actualInstanceHub.EmitCallCount()).To(Equal(1))
						changedEvent := actualInstanceHub.EmitArgsForCall(0)
						Expect(changedEvent).To(Equal(models.NewActualLRPInstanceChangedEvent(
							originalLRP,
							updatedLRP,
						)))
					})
				})
			})

			Context("to RUNNING", func() {
				BeforeEach(func() {
					updatedLRP.State = models.ActualLRPStateRunning
				})

				Context("from UNCLAIMED", func() {
					BeforeEach(func() {
						originalLRP.State = models.ActualLRPStateUnclaimed
					})

					It("emits an ActualLRPChangedEvent", func() {
						eventCalculator.EmitEvents(beforeSet, afterSet)
						Expect(actualHub.EmitCallCount()).To(Equal(1))
						changedEvent := actualHub.EmitArgsForCall(0)
						Expect(changedEvent).To(Equal(models.NewActualLRPChangedEvent(originalLRP.ToActualLRPGroup(), updatedLRP.ToActualLRPGroup())))
					})

					It("emits an ActualLRPInstanceChangedEvent", func() {
						eventCalculator.EmitEvents(beforeSet, afterSet)
						Expect(actualInstanceHub.EmitCallCount()).To(Equal(1))
						changedEvent := actualInstanceHub.EmitArgsForCall(0)
						Expect(changedEvent).To(Equal(models.NewActualLRPInstanceChangedEvent(originalLRP, updatedLRP)))
					})
				})

				Context("from CLAIMED", func() {
					BeforeEach(func() {
						originalLRP.State = models.ActualLRPStateClaimed
					})

					It("emits an ActualLRPChangedEvent", func() {
						eventCalculator.EmitEvents(beforeSet, afterSet)
						Expect(actualHub.EmitCallCount()).To(Equal(1))
						changedEvent := actualHub.EmitArgsForCall(0)
						Expect(changedEvent).To(Equal(models.NewActualLRPChangedEvent(originalLRP.ToActualLRPGroup(), updatedLRP.ToActualLRPGroup())))
					})

					It("emits an ActualLRPInstanceChangedEvent", func() {
						eventCalculator.EmitEvents(beforeSet, afterSet)
						Expect(actualInstanceHub.EmitCallCount()).To(Equal(1))
						changedEvent := actualInstanceHub.EmitArgsForCall(0)
						Expect(changedEvent).To(Equal(models.NewActualLRPInstanceChangedEvent(originalLRP, updatedLRP)))
					})
				})
			})

			Context("to CRASHED", func() {
				BeforeEach(func() {
					updatedLRP.State = models.ActualLRPStateCrashed
				})

				Context("from CLAIMED", func() {
					BeforeEach(func() {
						originalLRP.State = models.ActualLRPStateClaimed
					})

					It("emits Crashed and Changed events for the ActualLRPGroup", func() {
						eventCalculator.EmitEvents(beforeSet, afterSet)
						Expect(actualHub.EmitCallCount()).To(Equal(2))

						crashedEvent := actualHub.EmitArgsForCall(0)
						changedEvent := actualHub.EmitArgsForCall(1)

						Expect(crashedEvent).To(Equal(models.NewActualLRPCrashedEvent(originalLRP, updatedLRP)))
						Expect(changedEvent).To(Equal(models.NewActualLRPChangedEvent(originalLRP.ToActualLRPGroup(), updatedLRP.ToActualLRPGroup())))
					})

					It("emits Crashed and Changed events for the ActualLRP instances", func() {
						eventCalculator.EmitEvents(beforeSet, afterSet)
						Expect(actualInstanceHub.EmitCallCount()).To(Equal(2))

						crashedEvent := actualInstanceHub.EmitArgsForCall(0)
						changedEvent := actualInstanceHub.EmitArgsForCall(1)

						Expect(crashedEvent).To(Equal(models.NewActualLRPCrashedEvent(originalLRP, updatedLRP)))
						Expect(changedEvent).To(Equal(models.NewActualLRPInstanceChangedEvent(originalLRP, updatedLRP)))
					})
				})

				Context("from RUNNING", func() {
					BeforeEach(func() {
						originalLRP.State = models.ActualLRPStateRunning
					})

					It("emits Crashed and Changed events for the ActualLRPGroup", func() {
						eventCalculator.EmitEvents(beforeSet, afterSet)
						Expect(actualHub.EmitCallCount()).To(Equal(2))

						crashedEvent := actualHub.EmitArgsForCall(0)
						changedEvent := actualHub.EmitArgsForCall(1)

						Expect(crashedEvent).To(Equal(models.NewActualLRPCrashedEvent(originalLRP, updatedLRP)))
						Expect(changedEvent).To(Equal(models.NewActualLRPChangedEvent(originalLRP.ToActualLRPGroup(), updatedLRP.ToActualLRPGroup())))
					})

					It("emits Crashed and Changed events for the ActualLRP instances", func() {
						eventCalculator.EmitEvents(beforeSet, afterSet)
						Expect(actualInstanceHub.EmitCallCount()).To(Equal(2))

						crashedEvent := actualInstanceHub.EmitArgsForCall(0)
						changedEvent := actualInstanceHub.EmitArgsForCall(1)

						Expect(crashedEvent).To(Equal(models.NewActualLRPCrashedEvent(originalLRP, updatedLRP)))
						Expect(changedEvent).To(Equal(models.NewActualLRPInstanceChangedEvent(originalLRP, updatedLRP)))
					})
				})
			})
		})

		Context("when an ordinary LRP is replacing a suspect LRP", func() {
			var suspectLRP, replacementLRP *models.ActualLRP

			BeforeEach(func() {
				suspectLRP = model_helpers.NewValidActualLRP("some-guid-1", 0)
				suspectLRP.Presence = models.ActualLRP_Suspect

				replacementLRP = model_helpers.NewValidActualLRP("some-guid-1", 0)
				replacementLRP.ActualLRPInstanceKey = models.NewActualLRPInstanceKey(
					"replacement",
					"replacement-cell",
				)
			})

			JustBeforeEach(func() {
				beforeSet = []*models.ActualLRP{suspectLRP}
				afterSet = []*models.ActualLRP{suspectLRP, replacementLRP}
			})

			It("emits an ActualLrpInstanceCreatedEvent", func() {
				eventCalculator.EmitEvents(beforeSet, afterSet)
				Expect(actualInstanceHub.EmitCallCount()).To(Equal(1))

				lrpInstanceEvent := actualInstanceHub.EmitArgsForCall(0)
				Expect(lrpInstanceEvent).To(Equal(models.NewActualLRPInstanceCreatedEvent(replacementLRP)))
			})

			Context("and the ordinary LRP is in non-RUNNING state", func() {
				BeforeEach(func() {
					replacementLRP.State = models.ActualLRPStateClaimed
				})

				It("emits no ActualLRPGroup events", func() {
					eventCalculator.EmitEvents(beforeSet, afterSet)

					Expect(actualHub.EmitCallCount()).To(Equal(0))
				})
			})

			Context("and the ordinary LRP is in RUNNING state", func() {
				BeforeEach(func() {
					replacementLRP.State = models.ActualLRPStateRunning
				})

				It("emits ActualLRPGroupCreatedEvent for the replacement LRP,", func() {
					eventCalculator.EmitEvents(beforeSet, afterSet)

					Expect(actualHub.EmitCallCount()).To(Equal(2))

					lrpGroupCreatedEvent := actualHub.EmitArgsForCall(0)
					Expect(lrpGroupCreatedEvent).To(Equal(models.NewActualLRPCreatedEvent(
						replacementLRP.ToActualLRPGroup(),
					)))
				})

				It("emits a ActualLRPGroupRemovedEvent for the suspect LRP", func() {
					eventCalculator.EmitEvents(beforeSet, afterSet)

					Expect(actualHub.EmitCallCount()).To(Equal(2))

					lrpGroupRemovedEvent := actualHub.EmitArgsForCall(1)
					Expect(lrpGroupRemovedEvent).To(Equal(models.NewActualLRPRemovedEvent(
						suspectLRP.ToActualLRPGroup(),
					)))
				})
			})
		})

		Context("evacuation dance", func() {
			var originalLRP, unclaimedLRP, evacuatingLRP *models.ActualLRP

			BeforeEach(func() {
				unclaimedLRP = model_helpers.NewValidActualLRP("some-guid-1", 0)
				unclaimedLRP.ActualLRPInstanceKey = models.ActualLRPInstanceKey{}
				unclaimedLRP.State = models.ActualLRPStateUnclaimed

				originalLRP = model_helpers.NewValidActualLRP("some-guid-1", 0)
				originalLRP.State = models.ActualLRPStateRunning

				evacuatingLRP = model_helpers.NewValidActualLRP("some-guid-1", 0)
				evacuatingLRP.Presence = models.ActualLRP_Evacuating
				originalLRP.State = models.ActualLRPStateRunning
			})

			JustBeforeEach(func() {
				beforeSet = []*models.ActualLRP{originalLRP}
				afterSet = []*models.ActualLRP{evacuatingLRP, unclaimedLRP}
			})

			It("emits ActualLRPCreatedEvent for the evacuating instance", func() {
				eventCalculator.EmitEvents(beforeSet, afterSet)

				// this also emit a changed event but that is covered by the `to
				// UNCLAIMED` context above
				Expect(actualHub.EmitCallCount()).To(Equal(2))

				events := []models.Event{
					actualHub.EmitArgsForCall(0),
					actualHub.EmitArgsForCall(1),
				}
				Expect(events).To(ContainElement(models.NewActualLRPCreatedEvent(
					evacuatingLRP.ToActualLRPGroup(),
				)))
			})

			It("emits ActualLrpInstanceChangedEvent", func() {
				eventCalculator.EmitEvents(beforeSet, afterSet)

				// this also emit a changed event but that is covered by the `to
				// UNCLAIMED` context above
				Expect(actualInstanceHub.EmitCallCount()).To(Equal(2))

				events := []models.Event{
					actualInstanceHub.EmitArgsForCall(0),
					actualInstanceHub.EmitArgsForCall(1),
				}

				Expect(events).To(ContainElement(test_helpers.DeepEqual(models.NewActualLRPInstanceChangedEvent(
					originalLRP,
					evacuatingLRP,
				))))
			})
		})
	})

	Describe("RecordChange", func() {
		var beforeActualLRP, afterActualLRP *models.ActualLRP
		var actualLRPList []*models.ActualLRP

		BeforeEach(func() {
			beforeActualLRP = model_helpers.NewValidActualLRP("some-guid", 0)
			afterActualLRP = model_helpers.NewValidActualLRP("some-guid", 0)

			actualLRPList = []*models.ActualLRP{
				beforeActualLRP,
				model_helpers.NewValidEvacuatingActualLRP("some-guid", 0),
			}

		})

		It("returns the original LRP list, with the 'after' LRP replaced by the 'before' LRP", func() {
			updatedActualLRPSet := eventCalculator.RecordChange(beforeActualLRP, afterActualLRP, actualLRPList)
			Expect(updatedActualLRPSet).To(Equal([]*models.ActualLRP{
				afterActualLRP,
				actualLRPList[1],
			}))
		})

		Context("when an ActualLRP is being created (i.e. the 'before' ActualLRP is nil)", func() {
			BeforeEach(func() {
				beforeActualLRP = nil

				actualLRPList = []*models.ActualLRP{
					model_helpers.NewValidEvacuatingActualLRP("some-guid", 0),
				}
			})

			It("appends the new ActualLRP (i.e. the 'after' ActualLRP) to the set of ActualLRPs", func() {
				updatedActualLRPSet := eventCalculator.RecordChange(beforeActualLRP, afterActualLRP, actualLRPList)
				Expect(updatedActualLRPSet).To(Equal([]*models.ActualLRP{
					actualLRPList[0],
					afterActualLRP,
				}))
			})
		})

		Context("when an ActualLRP is being removed (i.e. the 'after' ActualLRP is nil)", func() {
			BeforeEach(func() {
				afterActualLRP = nil
			})

			It("replaced the original ActualLRP with a nil value to represent the removal", func() {
				updatedActualLRPSet := eventCalculator.RecordChange(beforeActualLRP, afterActualLRP, actualLRPList)
				Expect(updatedActualLRPSet).To(Equal([]*models.ActualLRP{
					nil,
					actualLRPList[1],
				}))
			})
		})

		Context("when a previous call to RecordChanges removed an ActualLRP (i.e. an lrp in the lrp list is nil)", func() {
			BeforeEach(func() {
				actualLRPList = []*models.ActualLRP{
					nil,
					model_helpers.NewValidEvacuatingActualLRP("some-guid", 0),
				}
			})

			It("preserves the nil value in the list of ActualLRPs", func() {
				updatedActualLRPSet := eventCalculator.RecordChange(beforeActualLRP, afterActualLRP, actualLRPList)
				Expect(len(updatedActualLRPSet)).To(BeNumerically(">=", len(actualLRPList)))
				Expect(updatedActualLRPSet[0]).To(BeNil())
				Expect(updatedActualLRPSet[1]).To(Equal(actualLRPList[1]))
			})
		})

	})
})

var _ = Describe("EventScore", func() {
	var actualLRP *models.ActualLRP
	var event models.Event

	BeforeEach(func() {
		actualLRP = model_helpers.NewValidActualLRP("some-guid", 0)
	})

	itReturnsTheCorrectScore := func(score int) {
		It("returns the correct score", func() {
			Expect(calculator.EventScore(event)).To(Equal(score))
		})
	}

	Context("for ActualLRPCreatedEvent", func() {
		JustBeforeEach(func() {
			event = models.NewActualLRPCreatedEvent(actualLRP.ToActualLRPGroup())
		})

		itReturnsTheCorrectScore(1)

		Context("when the new ActualLRP is in a non-Running state", func() {
			BeforeEach(func() {
				actualLRP.State = models.ActualLRPStateUnclaimed
			})

			itReturnsTheCorrectScore(0)
		})
	})

	Context("for ActualLRPChangedEvent", func() {
		var beforeActualLRPGroup *models.ActualLRPGroup
		BeforeEach(func() {
			beforeActualLRPGroup = model_helpers.NewValidActualLRP("some-guid", 0).ToActualLRPGroup()
		})

		JustBeforeEach(func() {
			event = models.NewActualLRPChangedEvent(beforeActualLRPGroup, actualLRP.ToActualLRPGroup())
		})

		itReturnsTheCorrectScore(1)

		Context("when the updated ActualLRP is in a non-Running state", func() {
			BeforeEach(func() {
				actualLRP.State = models.ActualLRPStateUnclaimed
			})

			itReturnsTheCorrectScore(0)
		})
	})

	Context("for ActualLRPCrashedEvent", func() {
		var beforeActualLRP *models.ActualLRP
		BeforeEach(func() {
			beforeActualLRP = model_helpers.NewValidActualLRP("some-guid", 0)
			event = models.NewActualLRPCrashedEvent(beforeActualLRP, actualLRP)
		})

		itReturnsTheCorrectScore(2)
	})

	Context("for ActualLRPRemovedEvent", func() {
		JustBeforeEach(func() {
			event = models.NewActualLRPRemovedEvent(actualLRP.ToActualLRPGroup())
		})

		itReturnsTheCorrectScore(0)
	})

	Context("for ActualLRPInstanceCreatedEvent", func() {
		JustBeforeEach(func() {
			event = models.NewActualLRPInstanceCreatedEvent(actualLRP)
		})

		itReturnsTheCorrectScore(1)

		Context("when the new ActualLRP is in a non-Running state", func() {
			BeforeEach(func() {
				actualLRP.State = models.ActualLRPStateUnclaimed
			})

			itReturnsTheCorrectScore(0)
		})
	})

	Context("for ActualLRPInstanceChangedEvent", func() {
		var beforeActualLRP *models.ActualLRP
		BeforeEach(func() {
			beforeActualLRP = model_helpers.NewValidActualLRP("some-guid", 0)
		})

		JustBeforeEach(func() {
			event = models.NewActualLRPInstanceChangedEvent(beforeActualLRP, actualLRP)
		})

		itReturnsTheCorrectScore(1)

		Context("when the updated ActualLRP is in a non-Running state", func() {
			BeforeEach(func() {
				actualLRP.State = models.ActualLRPStateUnclaimed
			})

			itReturnsTheCorrectScore(0)
		})
	})

	Context("for ActualLRPInstanceRemovedEvent", func() {
		JustBeforeEach(func() {
			event = models.NewActualLRPInstanceRemovedEvent(actualLRP)
		})

		itReturnsTheCorrectScore(0)
	})
})
