package main_test

import (
	"crypto/rand"
	"os"
	"time"

	"code.cloudfoundry.org/bbs"
	"code.cloudfoundry.org/bbs/cmd/bbs/testrunner"
	"code.cloudfoundry.org/bbs/db/sqldb"
	"code.cloudfoundry.org/bbs/db/sqldb/helpers"
	"code.cloudfoundry.org/bbs/db/sqldb/helpers/monitor"
	"code.cloudfoundry.org/bbs/encryption"
	"code.cloudfoundry.org/bbs/events"
	"code.cloudfoundry.org/bbs/guidprovider"
	"code.cloudfoundry.org/bbs/models"
	"code.cloudfoundry.org/bbs/models/test/model_helpers"
	"code.cloudfoundry.org/clock"
	"code.cloudfoundry.org/diego-logging-client/testhelpers"
	"code.cloudfoundry.org/durationjson"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/tedsuo/ifrit/ginkgomon"
)

var _ = Describe("Convergence API", func() {
	Describe("ConvergeLRPs", func() {
		var processGuid string

		BeforeEach(func() {
			// make the converger more aggressive by running every second
			bbsConfig.ConvergeRepeatInterval = durationjson.Duration(time.Second)
			bbsRunner = testrunner.New(bbsBinPath, bbsConfig)
			bbsProcess = ginkgomon.Invoke(bbsRunner)

			cellPresence := models.NewCellPresence(
				"some-cell",
				"cell.example.com",
				"http://cell.example.com",
				"the-zone",
				models.NewCellCapacity(128, 1024, 6),
				[]string{},
				[]string{},
				[]string{},
				[]string{},
			)
			consulHelper.RegisterCell(&cellPresence)
			processGuid = "some-process-guid"
			desiredLRP := model_helpers.NewValidDesiredLRP(processGuid)
			err := client.DesireLRP(logger, desiredLRP)
			Expect(err).NotTo(HaveOccurred())
		})

		Context("when an LRP's cell is dead", func() {
			var (
				lrpKey                *models.ActualLRPKey
				suspectLRPInstanceKey *models.ActualLRPInstanceKey
			)

			BeforeEach(func() {
				netInfo := models.NewActualLRPNetInfo("127.0.0.1", "10.10.10.10", models.NewPortMapping(8080, 80))

				lrpKey = &models.ActualLRPKey{
					ProcessGuid: processGuid,
					Index:       0,
					Domain:      "some-domain",
				}
				suspectLRPInstanceKey = &models.ActualLRPInstanceKey{
					InstanceGuid: "ig-1",
					CellId:       "missing-cell",
				}
				err := client.StartActualLRP(logger, lrpKey, suspectLRPInstanceKey, &netInfo)

				Expect(err).NotTo(HaveOccurred())
			})

			It("makes the LRP suspect", func() {
				Eventually(func() models.ActualLRP_Presence {
					group, err := client.ActualLRPGroupByProcessGuidAndIndex(logger, processGuid, 0)
					Expect(err).NotTo(HaveOccurred())
					return group.Instance.Presence
				}).Should(Equal(models.ActualLRP_Suspect))
			})

			Context("and the LRP is marked Suspect", func() {
				var (
					db     *sqldb.SQLDB
					events events.EventSource
				)

				BeforeEach(func() {
					var err error
					events, err = client.SubscribeToEvents(logger)
					Expect(err).NotTo(HaveOccurred())

					key, keys, err := bbsConfig.EncryptionConfig.Parse()
					Expect(err).NotTo(HaveOccurred())
					keyManager, err := encryption.NewKeyManager(key, keys)
					cryptor := encryption.NewCryptor(keyManager, rand.Reader)
					wrappedDB := helpers.NewMonitoredDB(sqlRunner.DB(), monitor.New())
					metronClient := &testhelpers.FakeIngressClient{}
					db = sqldb.NewSQLDB(
						wrappedDB,
						1,
						1,
						cryptor,
						guidprovider.DefaultGuidProvider,
						clock.NewClock(),
						sqlRunner.DriverName(),
						metronClient,
					)

					Eventually(func() models.ActualLRP_Presence {
						group, err := client.ActualLRPGroupByProcessGuidAndIndex(logger, processGuid, 0)
						Expect(err).NotTo(HaveOccurred())
						return group.Instance.Presence
					}).Should(Equal(models.ActualLRP_Suspect))
				})

				Context("when the cell is back", func() {
					BeforeEach(func() {
						cellPresence := models.NewCellPresence(
							"missing-cell",
							"cell.example.com",
							"http://cell.example.com",
							"the-zone",
							models.NewCellCapacity(128, 1024, 6),
							[]string{},
							[]string{},
							[]string{},
							[]string{},
						)
						consulHelper.RegisterCell(&cellPresence)
					})

					It("it transitions back to Ordinary", func() {
						Eventually(func() models.ActualLRP_Presence {
							group, err := client.ActualLRPGroupByProcessGuidAndIndex(logger, processGuid, 0)
							Expect(err).NotTo(HaveOccurred())
							return group.Instance.Presence
						}).Should(Equal(models.ActualLRP_Ordinary))
					})
				})

				Context("when there is a new Ordinary LRP in Running state", func() {
					BeforeEach(func() {
						Eventually(func() bool {
							index := int32(0)
							lrps, err := client.ActualLRPs(logger, models.ActualLRPFilter{Index: &index, ProcessGuid: processGuid})
							Expect(err).NotTo(HaveOccurred())
							for _, lrp := range lrps {
								if lrp.State == models.ActualLRPStateUnclaimed {
									return true
								}
							}
							return false
						}).Should(BeTrue())

						netInfo := models.NewActualLRPNetInfo("127.0.0.1", "10.10.10.10", models.NewPortMapping(8080, 80))
						_, _, err := db.StartActualLRP(logger, &models.ActualLRPKey{
							ProcessGuid: processGuid,
							Index:       0,
							Domain:      "some-domain",
						}, &models.ActualLRPInstanceKey{
							InstanceGuid: "ig-2",
							CellId:       "some-cell",
						}, &netInfo)
						Expect(err).NotTo(HaveOccurred())
					})

					It("removes the suspect LRP", func() {
						var lrp *models.ActualLRP
						Eventually(func() string {
							group, err := client.ActualLRPGroupByProcessGuidAndIndex(logger, processGuid, 0)
							Expect(err).NotTo(HaveOccurred())
							lrp = group.Instance
							return lrp.InstanceGuid
						}).Should(Equal("ig-2"))
						Expect(lrp.Presence).To(Equal(models.ActualLRP_Ordinary))
					})

					It("emits a LRPRemoved event", func() {
						eventCh := streamEvents(events)
						var removedEvent *models.ActualLRPRemovedEvent
						Eventually(eventCh).Should(Receive(&removedEvent))

						Expect(removedEvent.ActualLrpGroup.Evacuating).To(BeNil())
						Expect(removedEvent.ActualLrpGroup.Instance.InstanceGuid).To(Equal("ig-1"))
						Expect(removedEvent.ActualLrpGroup.Instance.Presence).To(Equal(models.ActualLRP_Suspect))
					})
				})

				Context("when the new Ordinary LRP cells goes missing", func() {
					BeforeEach(func() {
						Eventually(func() bool {
							index := int32(0)
							lrps, err := client.ActualLRPs(logger, models.ActualLRPFilter{Index: &index, ProcessGuid: "some-process-guid"})
							Expect(err).NotTo(HaveOccurred())
							for _, lrp := range lrps {
								if lrp.State == models.ActualLRPStateUnclaimed {
									return true
								}
							}
							return false
						}).Should(BeTrue())

						var err error
						events, err = client.SubscribeToEvents(logger)
						Expect(err).NotTo(HaveOccurred())

						err = client.ClaimActualLRP(logger, lrpKey, &models.ActualLRPInstanceKey{
							InstanceGuid: "ig-2",
							CellId:       "another-missing-cell",
						})
						Expect(err).NotTo(HaveOccurred())
					})

					It("keeps the suspect LRP untouched", func() {
						Consistently(func() models.ActualLRP_Presence {
							group, err := client.ActualLRPGroupByProcessGuidAndIndex(logger, processGuid, 0)
							Expect(err).NotTo(HaveOccurred())
							return group.Instance.Presence
						}).Should(Equal(models.ActualLRP_Suspect))
					})

					It("unclaims the replacement", func() {
						Eventually(func() string {
							index := int32(0)
							lrps, err := client.ActualLRPs(logger, models.ActualLRPFilter{ProcessGuid: processGuid, Index: &index})
							Expect(err).NotTo(HaveOccurred())
							for _, lrp := range lrps {
								if lrp.Presence == models.ActualLRP_Ordinary {
									return lrp.State
								}
							}
							return ""
						}).Should(Equal(models.ActualLRPStateUnclaimed))
					})

					It("does not emit any events", func() {
						eventCh := streamEvents(events)

						Consistently(eventCh).ShouldNot(Receive())
					})
				})

				Context("when the Auctioneer calls FailActualLRP", func() {
					BeforeEach(func() {
						err := client.FailActualLRP(logger, &models.ActualLRPKey{
							ProcessGuid: "some-process-guid",
							Index:       0,
							Domain:      "some-domain",
						}, "boom!")
						Expect(err).NotTo(HaveOccurred())
					})

					It("keeps the suspect LRP untouched", func() {
						Consistently(func() models.ActualLRP_Presence {
							group, err := client.ActualLRPGroupByProcessGuidAndIndex(logger, processGuid, 0)
							Expect(err).NotTo(HaveOccurred())
							return group.Instance.Presence
						}).Should(Equal(models.ActualLRP_Suspect))
					})
				})

				// All tests in this context need to use a non aggressive converger to
				// ensure they are testing state transitions as a result of the RPC
				// calls (.e.g StartActualLRP) instead of testing converger behavior.
				// We also have to initially make the convergence aggressive in the
				// outer Context in order to ensure the LRP transition from Ordinary to
				// Suspect within 1 second instead of waiting for the default 30
				// second.
				Context("with a less aggressive converger", func() {
					BeforeEach(func() {
						bbsProcess.Signal(os.Interrupt)
						Eventually(bbsProcess.Wait()).Should(Receive())
						bbsConfig.ConvergeRepeatInterval = durationjson.Duration(time.Hour)
						bbsRunner = testrunner.New(bbsBinPath, bbsConfig)
						bbsProcess = ginkgomon.Invoke(bbsRunner)

						// recreate the event stream
						var err error
						events, err = client.SubscribeToEvents(logger)
						Expect(err).NotTo(HaveOccurred())
					})

					Context("when the auctioneer fails to place the replacement instance", func() {
						BeforeEach(func() {
							err := client.FailActualLRP(logger, &models.ActualLRPKey{
								ProcessGuid: "some-process-guid",
								Index:       0,
								Domain:      "some-domain",
							}, "boooom!")
							Expect(err).NotTo(HaveOccurred())
						})

						It("does not emit a ActualLRPChangedEvent", func() {
							eventCh := streamEvents(events)
							Consistently(eventCh, 5*time.Second).ShouldNot(Receive())
						})
					})

					Context("when the replacement LRP is claimed", func() {
						BeforeEach(func() {
							err := client.ClaimActualLRP(logger, &models.ActualLRPKey{
								ProcessGuid: "some-process-guid",
								Index:       0,
								Domain:      "some-domain",
							}, &models.ActualLRPInstanceKey{
								InstanceGuid: "ig-2",
								CellId:       "some-cell",
							})
							Expect(err).NotTo(HaveOccurred())
						})

						It("does not emit any events", func() {
							eventCh := streamEvents(events)
							Consistently(eventCh, 5*time.Second).ShouldNot(Receive())
						})
					})

					Context("when the replacement LRP is started by calling StartActualLRP", func() {
						BeforeEach(func() {
							netInfo := models.NewActualLRPNetInfo("127.0.0.1", "10.10.10.10", models.NewPortMapping(8080, 80))
							err := client.StartActualLRP(logger, &models.ActualLRPKey{
								ProcessGuid: "some-process-guid",
								Index:       0,
								Domain:      "some-domain",
							}, &models.ActualLRPInstanceKey{
								InstanceGuid: "ig-2",
								CellId:       "some-cell",
							}, &netInfo)
							Expect(err).NotTo(HaveOccurred())
						})

						It("replaces the Running LRP instance with the ordinary one", func() {
							group, err := client.ActualLRPGroupByProcessGuidAndIndex(logger, processGuid, 0)
							Expect(err).NotTo(HaveOccurred())
							Expect(group.Instance.Presence).To(Equal(models.ActualLRP_Ordinary))
						})

						It("emits a LRPCreated event", func() {
							eventCh := streamEvents(events)

							var e *models.ActualLRPCreatedEvent

							Eventually(eventCh, 2*time.Second).Should(Receive(&e))
							Expect(e.ActualLrpGroup.Instance.InstanceGuid).To(Equal("ig-2"))
							Expect(e.ActualLrpGroup.Instance.Presence).To(Equal(models.ActualLRP_Ordinary))
						})

						It("emits a LRPRemoved event", func() {
							eventCh := streamEvents(events)

							var e *models.ActualLRPRemovedEvent

							Eventually(eventCh, 2*time.Second).Should(Receive(&e))
							Expect(e.ActualLrpGroup.Instance.InstanceGuid).To(Equal("ig-1"))
							Expect(e.ActualLrpGroup.Instance.Presence).To(Equal(models.ActualLRP_Suspect))
						})

						It("returns ErrActualLRPCannotBeStarted when the Suspect LRP is started", func() {
							netInfo := models.NewActualLRPNetInfo("127.0.0.1", "10.10.10.10", models.NewPortMapping(8080, 80))
							err := client.StartActualLRP(logger, &models.ActualLRPKey{
								ProcessGuid: "some-process-guid",
								Index:       0,
								Domain:      "some-domain",
							}, &models.ActualLRPInstanceKey{
								InstanceGuid: "ig-1",
								CellId:       "missing-cell",
							}, &netInfo)
							Expect(err).To(MatchError(models.ErrActualLRPCannotBeStarted))
						})
					})

					Context("when the suspect LRP is started by calling StartActualLRP", func() {
						BeforeEach(func() {
							netInfo := models.NewActualLRPNetInfo("127.0.0.1", "10.10.10.10", models.NewPortMapping(8080, 80))
							err := client.StartActualLRP(logger, &models.ActualLRPKey{
								ProcessGuid: "some-process-guid",
								Index:       0,
								Domain:      "some-domain",
							}, &models.ActualLRPInstanceKey{
								InstanceGuid: "ig-1",
								CellId:       "missing-cell",
							}, &netInfo)
							Expect(err).NotTo(HaveOccurred())
						})

						It("does not change the ActualLRPGroups returned from the API", func() {
							group, err := client.ActualLRPGroupByProcessGuidAndIndex(logger, processGuid, 0)
							Expect(err).NotTo(HaveOccurred())
							Expect(group.Instance.Presence).To(Equal(models.ActualLRP_Suspect))
						})

						It("does not emit any events", func() {
							eventCh := streamEvents(events)
							Consistently(eventCh, 2*time.Second).ShouldNot(Receive())
						})
					})

					Context("when the replacement is in the CLAIMED state", func() {
						var (
							replacementLRPInstanceKey *models.ActualLRPInstanceKey
						)

						BeforeEach(func() {
							replacementLRPInstanceKey = &models.ActualLRPInstanceKey{
								InstanceGuid: "ig-2",
								CellId:       "some-cell",
							}
							err := client.ClaimActualLRP(logger, lrpKey, replacementLRPInstanceKey)
							Expect(err).NotTo(HaveOccurred())
						})

						Context("when the suspect LRP crashes", func() {
							BeforeEach(func() {
								err := client.CrashActualLRP(logger, lrpKey, suspectLRPInstanceKey, "boooom!")
								Expect(err).NotTo(HaveOccurred())
							})

							It("emits an ActualLRPRemovedEvent event", func() {
								eventCh := streamEvents(events)

								var e *models.ActualLRPRemovedEvent

								Eventually(eventCh, 2*time.Second).Should(Receive(&e))
								Expect(e.ActualLrpGroup.Instance.InstanceGuid).To(Equal("ig-1"))
								Expect(e.ActualLrpGroup.Instance.Presence).To(Equal(models.ActualLRP_Suspect))
							})

							It("removes the Suspect instance from the database", func() {
								group, err := client.ActualLRPGroupByProcessGuidAndIndex(logger, processGuid, 0)
								Expect(err).NotTo(HaveOccurred())
								Expect(group.Instance.Presence).To(Equal(models.ActualLRP_Ordinary))
							})
						})

						Context("when the replacement crashes", func() {
							BeforeEach(func() {
								err := client.CrashActualLRP(logger, lrpKey, replacementLRPInstanceKey, "boooom!")
								Expect(err).NotTo(HaveOccurred())
							})

							It("is unclaimed", func() {
								lrps, err := client.ActualLRPs(logger, models.ActualLRPFilter{
									ProcessGuid: lrpKey.ProcessGuid,
									Index:       &lrpKey.Index,
								})
								Expect(err).NotTo(HaveOccurred())
								foundUnclaimed := false
								for _, lrp := range lrps {
									if lrp.Presence == models.ActualLRP_Ordinary {
										Expect(lrp.State).To(Equal(models.ActualLRPStateUnclaimed))
										foundUnclaimed = true
									}
								}
								Expect(foundUnclaimed).To(BeTrue())
							})
						})

						Context("when the suspect LRP is evacuated", func() {
							BeforeEach(func() {
								netInfo := models.NewActualLRPNetInfo("127.0.0.1", "10.10.10.10", models.NewPortMapping(8080, 80))
								_, err := client.EvacuateRunningActualLRP(logger, lrpKey, suspectLRPInstanceKey, &netInfo)
								Expect(err).NotTo(HaveOccurred())
							})

							It("creates an evacuating LRP", func() {
								group, err := client.ActualLRPGroupByProcessGuidAndIndex(logger, processGuid, 0)
								Expect(err).NotTo(HaveOccurred())
								Expect(group.Evacuating.Presence).To(Equal(models.ActualLRP_Evacuating))
								Expect(group.Evacuating.ActualLRPInstanceKey).To(Equal(*suspectLRPInstanceKey))
							})

							It("removes the suspect LRP", func() {
								group, err := client.ActualLRPGroupByProcessGuidAndIndex(logger, processGuid, 0)
								Expect(err).NotTo(HaveOccurred())
								Expect(group.Instance.Presence).NotTo(Equal(models.ActualLRP_Suspect))
							})

							It("emits two LRPCreated events and a LRPRemoved event", func() {
								eventCh := streamEvents(events)

								var ce *models.ActualLRPCreatedEvent

								Eventually(eventCh, 2*time.Second).Should(Receive(&ce))
								Expect(ce.ActualLrpGroup.Evacuating.InstanceGuid).To(Equal("ig-1"))
								Expect(ce.ActualLrpGroup.Evacuating.Presence).To(Equal(models.ActualLRP_Evacuating))

								var ce2 *models.ActualLRPCreatedEvent

								Eventually(eventCh, 2*time.Second).Should(Receive(&ce2))
								Expect(ce2.ActualLrpGroup.Instance.InstanceGuid).To(Equal("ig-2"))
								Expect(ce2.ActualLrpGroup.Instance.Presence).To(Equal(models.ActualLRP_Ordinary))

								var re *models.ActualLRPRemovedEvent

								Eventually(eventCh, 2*time.Second).Should(Receive(&re))
								Expect(re.ActualLrpGroup.Instance.InstanceGuid).To(Equal("ig-1"))
								Expect(re.ActualLrpGroup.Instance.Presence).To(Equal(models.ActualLRP_Suspect))
							})
						})

						Context("when the replacement is evacuated", func() {
							BeforeEach(func() {
								_, err := client.EvacuateClaimedActualLRP(logger, lrpKey, replacementLRPInstanceKey)
								Expect(err).NotTo(HaveOccurred())
							})

							It("is unclaimed", func() {
								lrps, err := client.ActualLRPs(logger, models.ActualLRPFilter{
									ProcessGuid: lrpKey.ProcessGuid,
									Index:       &lrpKey.Index,
								})
								Expect(err).NotTo(HaveOccurred())
								foundUnclaimed := false
								for _, lrp := range lrps {
									if lrp.Presence == models.ActualLRP_Ordinary {
										Expect(lrp.State).To(Equal(models.ActualLRPStateUnclaimed))
										foundUnclaimed = true
									}
								}
								Expect(foundUnclaimed).To(BeTrue())
							})

							It("does not remove the suspect LRP", func() {
								eventCh := streamEvents(events)

								group, err := client.ActualLRPGroupByProcessGuidAndIndex(logger, processGuid, 0)
								Expect(err).NotTo(HaveOccurred())

								Expect(group.Instance.Presence).To(Equal(models.ActualLRP_Suspect))
								Consistently(eventCh).ShouldNot(Receive(BeAssignableToTypeOf(&models.ActualLRPRemovedEvent{})))
							})
						})

						Context("when the suspect LRP is evacuating after crashing", func() {
							BeforeEach(func() {
								_, err := client.EvacuateCrashedActualLRP(logger, lrpKey, suspectLRPInstanceKey, "boom!")
								Expect(err).NotTo(HaveOccurred())
							})

							It("removes the suspect LRP", func() {
								group, err := client.ActualLRPGroupByProcessGuidAndIndex(logger, processGuid, 0)
								Expect(err).NotTo(HaveOccurred())
								Expect(group.Instance.Presence).NotTo(Equal(models.ActualLRP_Suspect))
							})

							It("emits a LRPRemoved event", func() {
								eventCh := streamEvents(events)

								var re *models.ActualLRPRemovedEvent

								Eventually(eventCh, 2*time.Second).Should(Receive(&re))
								Expect(re.ActualLrpGroup.Instance.InstanceGuid).To(Equal("ig-1"))
								Expect(re.ActualLrpGroup.Instance.Presence).To(Equal(models.ActualLRP_Suspect))
							})
						})

						Context("when the suspect LRP is evacuating after stopping", func() {
							BeforeEach(func() {
								_, err := client.EvacuateStoppedActualLRP(logger, lrpKey, suspectLRPInstanceKey)
								Expect(err).NotTo(HaveOccurred())
							})

							It("removes the suspect LRP", func() {
								group, err := client.ActualLRPGroupByProcessGuidAndIndex(logger, processGuid, 0)
								Expect(err).NotTo(HaveOccurred())
								Expect(group.Instance.Presence).NotTo(Equal(models.ActualLRP_Suspect))
							})

							It("emits a LRPRemoved event", func() {
								eventCh := streamEvents(events)

								var re *models.ActualLRPRemovedEvent

								Eventually(eventCh, 2*time.Second).Should(Receive(&re))
								Expect(re.ActualLrpGroup.Instance.InstanceGuid).To(Equal("ig-1"))
								Expect(re.ActualLrpGroup.Instance.Presence).To(Equal(models.ActualLRP_Suspect))
							})
						})
					})
				})
			})
		})

		Context("when the lrp goes missing", func() {
			BeforeEach(func() {
				err := client.RemoveActualLRP(logger, &models.ActualLRPKey{
					ProcessGuid: processGuid,
					Index:       0,
					Domain:      "some-domain",
				}, nil)
				Expect(err).NotTo(HaveOccurred())
			})

			It("converges the lrps", func() {
				Eventually(func() []*models.ActualLRPGroup {
					groups, err := client.ActualLRPGroupsByProcessGuid(logger, processGuid)
					Expect(err).NotTo(HaveOccurred())
					return groups
				}).Should(HaveLen(1))
			})
		})

		Context("when a task is desired but its cell is dead", func() {
			BeforeEach(func() {
				task := model_helpers.NewValidTask("task-guid")

				err := client.DesireTask(logger, task.TaskGuid, task.Domain, task.TaskDefinition)
				Expect(err).NotTo(HaveOccurred())

				_, err = client.StartTask(logger, task.TaskGuid, "dead-cell")
				Expect(err).NotTo(HaveOccurred())
			})

			It("marks the task as completed and failed", func() {
				Eventually(func() []*models.Task {
					return getTasksByState(client, models.Task_Completed)
				}).Should(HaveLen(1))

				Expect(getTasksByState(client, models.Task_Completed)[0].Failed).To(BeTrue())
			})
		})
	})
})

func getTasksByState(client bbs.InternalClient, state models.Task_State) []*models.Task {
	tasks, err := client.Tasks(logger)
	Expect(err).NotTo(HaveOccurred())

	filteredTasks := make([]*models.Task, 0)
	for _, task := range tasks {
		if task.State == state {
			filteredTasks = append(filteredTasks, task)
		}
	}
	return filteredTasks
}
