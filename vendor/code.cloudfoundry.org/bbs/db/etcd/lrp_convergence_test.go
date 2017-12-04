package etcd_test

import (
	"fmt"
	"time"

	"code.cloudfoundry.org/auctioneer"
	"code.cloudfoundry.org/bbs/db/etcd"
	"code.cloudfoundry.org/bbs/models"
	"code.cloudfoundry.org/bbs/models/test/model_helpers"
	"code.cloudfoundry.org/clock/fakeclock"
	"code.cloudfoundry.org/lager/lagertest"

	etcddb "code.cloudfoundry.org/bbs/db/etcd"

	etcderror "github.com/coreos/etcd/error"
	etcdclient "github.com/coreos/go-etcd/etcd"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
)

var _ = Describe("LRPConvergence", func() {
	var (
		testData *testDataForConvergenceGatherer
	)

	Describe("Convergence Fetching and Pruning", func() {
		BeforeEach(func() {
			testData = createTestData(3, 1, 1, 3, 1, 1, 3, 1, 1, 1, 1, 1)
		})

		Describe("general metrics", func() {
			It("emits a metric for domains", func() {
				_, gatherError := etcdDB.GatherAndPruneLRPs(logger, testData.cells)
				Expect(gatherError).NotTo(HaveOccurred())

				Expect(fakeMetronClient.SendMetricCallCount()).To(Equal(7))
				name, value := fakeMetronClient.SendMetricArgsForCall(6)
				Expect(name).To(Equal("Domain.test-domain"))
				Expect(value).To(Equal(1))
			})

			It("emits metrics for lrps", func() {
				_, gatherError := etcdDB.GatherAndPruneLRPs(logger, testData.cells)
				Expect(gatherError).NotTo(HaveOccurred())

				name, value := fakeMetronClient.SendMetricArgsForCall(0)
				Expect(name).To(Equal("LRPsUnclaimed"))
				Expect(value).To(Equal(0))

				name, value = fakeMetronClient.SendMetricArgsForCall(1)
				Expect(name).To(Equal("LRPsClaimed"))
				Expect(value).To(Equal(0))

				name, value = fakeMetronClient.SendMetricArgsForCall(2)
				Expect(name).To(Equal("LRPsRunning"))
				Expect(value).To(Equal(15))

				name, value = fakeMetronClient.SendMetricArgsForCall(3)
				Expect(name).To(Equal("CrashedActualLRPs"))
				Expect(value).To(Equal(0))

				name, value = fakeMetronClient.SendMetricArgsForCall(4)
				Expect(name).To(Equal("CrashingDesiredLRPs"))
				Expect(value).To(Equal(0))

				name, value = fakeMetronClient.SendMetricArgsForCall(5)
				Expect(name).To(Equal("LRPsDesired"))
				Expect(value).To(Equal(6))
			})
		})

		Context("Desired LRPs", func() {
			It("provides the correct desired LRPs", func() {
				data, gatherError := etcdDB.GatherAndPruneLRPs(logger, testData.cells)
				Expect(gatherError).NotTo(HaveOccurred())

				expectedLength := len(testData.validDesiredGuidsWithSomeValidActuals) +
					len(testData.validDesiredGuidsWithNoActuals) +
					len(testData.validDesiredGuidsWithOnlyInvalidActuals)
				Expect(data.DesiredLRPs).To(HaveLen(expectedLength))

				for _, desiredGuid := range testData.validDesiredGuidsWithSomeValidActuals {
					Expect(data.DesiredLRPs).To(HaveKey(desiredGuid))
					desiredLRP, err := etcdDB.DesiredLRPByProcessGuid(logger, desiredGuid)
					Expect(err).NotTo(HaveOccurred())
					Expect(data.DesiredLRPs[desiredGuid]).To(Equal(desiredLRP))
				}
				for _, desiredGuid := range testData.validDesiredGuidsWithNoActuals {
					Expect(data.DesiredLRPs).To(HaveKey(desiredGuid))
					desiredLRP, err := etcdDB.DesiredLRPByProcessGuid(logger, desiredGuid)
					Expect(err).NotTo(HaveOccurred())
					Expect(data.DesiredLRPs[desiredGuid]).To(Equal(desiredLRP))
				}
				for _, desiredGuid := range testData.validDesiredGuidsWithOnlyInvalidActuals {
					Expect(data.DesiredLRPs).To(HaveKey(desiredGuid))
					desiredLRP, err := etcdDB.DesiredLRPByProcessGuid(logger, desiredGuid)
					Expect(err).NotTo(HaveOccurred())
					Expect(data.DesiredLRPs[desiredGuid]).To(Equal(desiredLRP))
				}
			})

			It("emits a metric for the number of malformed SchedulingInfos", func() {
				_, gatherError := etcdDB.GatherAndPruneLRPs(logger, testData.cells)
				Expect(gatherError).NotTo(HaveOccurred())

				expectedMetric := len(testData.invalidDesiredGuidsWithSomeValidActuals) +
					len(testData.invalidDesiredGuidsWithNoActuals) +
					len(testData.invalidDesiredGuidsWithOnlyInvalidActuals) +
					len(testData.unknownDesiredGuidsWithSomeValidActuals) +
					len(testData.unknownDesiredGuidsWithNoActuals) +
					len(testData.unknownDesiredGuidsWithOnlyInvalidActuals)

				Expect(fakeMetronClient.IncrementCounterWithDeltaCallCount()).To(Equal(3))
				name, value := fakeMetronClient.IncrementCounterWithDeltaArgsForCall(1)

				Expect(name).To(Equal("ConvergenceLRPPreProcessingMalformedSchedulingInfos"))
				Expect(value).To(BeNumerically("==", expectedMetric))
			})

			It("emits a metric for the number of malformed RunInfos", func() {
				_, gatherError := etcdDB.GatherAndPruneLRPs(logger, testData.cells)
				Expect(gatherError).NotTo(HaveOccurred())

				expectedMetric := len(testData.invalidDesiredGuidsWithSomeValidActuals) +
					len(testData.invalidDesiredGuidsWithNoActuals) +
					len(testData.invalidDesiredGuidsWithOnlyInvalidActuals) +
					len(testData.unknownDesiredGuidsWithSomeValidActuals) +
					len(testData.unknownDesiredGuidsWithNoActuals) +
					len(testData.unknownDesiredGuidsWithOnlyInvalidActuals)

				Expect(fakeMetronClient.IncrementCounterWithDeltaCallCount()).To(Equal(3))
				name, value := fakeMetronClient.IncrementCounterWithDeltaArgsForCall(2)

				Expect(name).To(Equal("ConvergenceLRPPreProcessingMalformedRunInfos"))
				Expect(value).To(BeNumerically("==", expectedMetric))
			})

			It("emits a metric for the number of orphaned RunInfos", func() {
				_, gatherError := etcdDB.GatherAndPruneLRPs(logger, testData.cells)
				Expect(gatherError).NotTo(HaveOccurred())

				expectedMetric := len(testData.oldOrphanedRunInfoGuids)
				Expect(fakeMetronClient.IncrementCounterCallCount()).To(Equal(expectedMetric))
				name := fakeMetronClient.IncrementCounterArgsForCall(0)

				Expect(name).To(Equal("ConvergenceLRPPreProcessingOrphanedRunInfos"))
			})

			Context("when there is a RunInfo without a matching SchedulingInfo", func() {
				JustBeforeEach(func() {
					_, gatherError := etcdDB.GatherAndPruneLRPs(logger, testData.cells)
					Expect(gatherError).NotTo(HaveOccurred())
				})

				It("deletes old orphaned RunInfos", func() {
					for _, guid := range testData.oldOrphanedRunInfoGuids {
						_, err := storeClient.Get(etcd.DesiredLRPRunInfoSchemaPath(guid), false, true)
						Expect(err).To(HaveOccurred())
						etcdErr, ok := err.(*etcdclient.EtcdError)
						Expect(ok).To(BeTrue())
						Expect(etcdErr.ErrorCode).To(Equal(etcderror.EcodeKeyNotFound))
					}
				})

				It("keeps recent orphaned RunInfos", func() {
					for _, guid := range testData.recentOrphanedRunInfoGuids {
						_, err := storeClient.Get(etcd.DesiredLRPRunInfoSchemaPath(guid), false, true)
						Expect(err).NotTo(HaveOccurred())
					}
				})
			})

			Context("when there is a SchedulingInfo without a matching RunInfo", func() {
				JustBeforeEach(func() {
					_, gatherError := etcdDB.GatherAndPruneLRPs(logger, testData.cells)
					Expect(gatherError).NotTo(HaveOccurred())
				})

				It("deletes orphaned schedulingInfos", func() {
					for _, guid := range testData.orphanedSchedulingInfoGuids {
						_, err := storeClient.Get(etcd.DesiredLRPSchedulingInfoSchemaPath(guid), false, true)
						Expect(err).To(HaveOccurred())
						etcdErr, ok := err.(*etcdclient.EtcdError)
						Expect(ok).To(BeTrue())
						Expect(etcdErr.ErrorCode).To(Equal(etcderror.EcodeKeyNotFound))
					}

					// This assertion is not great, however it checks that we do not attempt
					// to delete the desired lrp scheduling infos more than once
					Expect(logger).NotTo(gbytes.Say("failed-to-delete"))
				})
			})

			Context("when the desired LRP root in ETCD has an invalid child node", func() {
				BeforeEach(func() {
					etcdHelper.CreateInvalidDesiredLRPComponent()
				})

				It("returns an error", func() {
					_, gatherError := etcdDB.GatherAndPruneLRPs(logger, testData.cells)
					Expect(gatherError).To(MatchError(HavePrefix("unrecognized node under desired LRPs root node: ")))
				})
			})
		})

		Context("Actual LRPs", func() {
			It("emits a metric for the number of pruned ActualLRPs", func() {
				_, gatherError := etcdDB.GatherAndPruneLRPs(logger, testData.cells)
				Expect(gatherError).NotTo(HaveOccurred())

				expectedMetric := len(testData.instanceKeysToPrune) +
					len(testData.evacuatingKeysToPrune)

				Expect(fakeMetronClient.IncrementCounterWithDeltaCallCount()).To(Equal(3))
				name, value := fakeMetronClient.IncrementCounterWithDeltaArgsForCall(0)
				Expect(name).To(Equal("ConvergenceLRPPreProcessingActualLRPsDeleted"))
				Expect(value).To(BeNumerically("==", expectedMetric))
			})

			It("provides the correct actualLRPs", func() {
				data, gatherError := etcdDB.GatherAndPruneLRPs(logger, testData.cells)
				Expect(gatherError).NotTo(HaveOccurred())

				for actualData := range testData.instanceKeysToKeep {
					actualByIndex, ok := data.ActualLRPs[actualData.processGuid]
					Expect(ok).To(BeTrue(), fmt.Sprintf("expected actualIndex for process '%s' to be present", actualData.processGuid))

					_, ok = actualByIndex[actualData.index]
					Expect(ok).To(BeTrue(), fmt.Sprintf("expected actual for process '%s' and index %d to be present", actualData.processGuid, actualData.index))
				}

				for guid, actuals := range data.ActualLRPs {
					for index, _ := range actuals {
						_, ok := testData.instanceKeysToKeep[processGuidAndIndex{guid, index}]
						Expect(ok).To(BeTrue(), fmt.Sprintf("did not expect actual for process '%s' and index %d to be present", guid, index))
					}
				}
			})

			containIndices := func(groups []*models.ActualLRPGroup, indices ...int32) {
				for _, actualLRPGroup := range groups {
					Expect(indices).To(ContainElement(actualLRPGroup.Instance.ActualLRPKey.Index))
				}
			}

			// We need to use the ETCD Store Client to check for non existance, because
			// the ETCDDB will not return Invalid Records when fetching multiple records.
			It("prunes only the invalid ActualLRPs from the datastore", func() {
				_, gatherError := etcdDB.GatherAndPruneLRPs(logger, testData.cells)
				Expect(gatherError).NotTo(HaveOccurred())

				for _, guid := range testData.validDesiredGuidsWithOnlyInvalidActuals {
					_, err := storeClient.Get(etcd.ActualLRPProcessDir(guid), false, true)
					etcdErr, ok := err.(*etcdclient.EtcdError)
					Expect(ok).To(BeTrue())
					Expect(etcdErr.ErrorCode).To(Equal(etcderror.EcodeKeyNotFound))
				}

				for i, guid := range testData.validDesiredGuidsWithSomeValidActuals {
					switch i % 3 {
					case 0:
						groups, err := etcdDB.ActualLRPGroupsByProcessGuid(logger, guid)
						Expect(err).NotTo(HaveOccurred())
						Expect(groups).To(HaveLen(2))
						containIndices(groups, randomIndex1, randomIndex2)
					case 1:
						response, err := storeClient.Get(etcd.ActualLRPProcessDir(guid), false, true)
						Expect(err).NotTo(HaveOccurred())
						Expect(response.Node.Nodes).To(HaveLen(1))

						groups, err := etcdDB.ActualLRPGroupsByProcessGuid(logger, guid)
						Expect(err).NotTo(HaveOccurred())
						Expect(groups).To(HaveLen(1))
						containIndices(groups, randomIndex1)
					case 2:
						group1, err := etcdDB.ActualLRPGroupByProcessGuidAndIndex(logger, guid, randomIndex1)
						Expect(err).NotTo(HaveOccurred())
						Expect(group1.Instance).To(BeNil())
						Expect(group1.Evacuating).NotTo(BeNil())

						group2, err := etcdDB.ActualLRPGroupByProcessGuidAndIndex(logger, guid, randomIndex2)
						Expect(err).NotTo(HaveOccurred())
						Expect(group2.Instance).NotTo(BeNil())
						Expect(group2.Evacuating).To(BeNil())
					}
				}

				for _, guid := range testData.invalidDesiredGuidsWithOnlyInvalidActuals {
					_, err := storeClient.Get(etcd.ActualLRPProcessDir(guid), false, true)
					etcdErr, ok := err.(*etcdclient.EtcdError)
					Expect(ok).To(BeTrue())
					Expect(etcdErr.ErrorCode).To(Equal(etcderror.EcodeKeyNotFound))
				}

				for i, guid := range testData.invalidDesiredGuidsWithSomeValidActuals {
					switch i % 3 {
					case 0:
						groups, err := etcdDB.ActualLRPGroupsByProcessGuid(logger, guid)
						Expect(err).NotTo(HaveOccurred())
						Expect(groups).To(HaveLen(2))
						containIndices(groups, randomIndex1, randomIndex2)
					case 1:
						response, err := storeClient.Get(etcd.ActualLRPProcessDir(guid), false, true)
						Expect(err).NotTo(HaveOccurred())
						Expect(response.Node.Nodes).To(HaveLen(1))

						groups, err := etcdDB.ActualLRPGroupsByProcessGuid(logger, guid)
						Expect(err).NotTo(HaveOccurred())
						Expect(groups).To(HaveLen(1))
						containIndices(groups, randomIndex1)
					case 2:
						group1, err := etcdDB.ActualLRPGroupByProcessGuidAndIndex(logger, guid, randomIndex1)
						Expect(err).NotTo(HaveOccurred())
						Expect(group1.Instance).To(BeNil())
						Expect(group1.Evacuating).NotTo(BeNil())

						group2, err := etcdDB.ActualLRPGroupByProcessGuidAndIndex(logger, guid, randomIndex2)
						Expect(err).NotTo(HaveOccurred())
						Expect(group2.Instance).NotTo(BeNil())
						Expect(group2.Evacuating).To(BeNil())
					}
				}

				for _, guid := range testData.unknownDesiredGuidsWithOnlyInvalidActuals {
					_, err := storeClient.Get(etcd.ActualLRPProcessDir(guid), false, true)
					etcdErr, ok := err.(*etcdclient.EtcdError)
					Expect(ok).To(BeTrue())
					Expect(etcdErr.ErrorCode).To(Equal(etcderror.EcodeKeyNotFound))
				}

				for i, guid := range testData.unknownDesiredGuidsWithSomeValidActuals {
					switch i % 3 {
					case 0:
						groups, err := etcdDB.ActualLRPGroupsByProcessGuid(logger, guid)
						Expect(err).NotTo(HaveOccurred())
						Expect(groups).To(HaveLen(2))
						containIndices(groups, randomIndex1, randomIndex2)
					case 1:
						response, err := storeClient.Get(etcd.ActualLRPProcessDir(guid), false, true)
						Expect(err).NotTo(HaveOccurred())
						Expect(response.Node.Nodes).To(HaveLen(1))

						groups, err := etcdDB.ActualLRPGroupsByProcessGuid(logger, guid)
						Expect(err).NotTo(HaveOccurred())
						Expect(groups).To(HaveLen(1))
						containIndices(groups, randomIndex1)
					case 2:
						group1, err := etcdDB.ActualLRPGroupByProcessGuidAndIndex(logger, guid, randomIndex1)
						Expect(err).NotTo(HaveOccurred())
						Expect(group1.Instance).To(BeNil())
						Expect(group1.Evacuating).NotTo(BeNil())

						group2, err := etcdDB.ActualLRPGroupByProcessGuidAndIndex(logger, guid, randomIndex2)
						Expect(err).NotTo(HaveOccurred())
						Expect(group2.Instance).NotTo(BeNil())
						Expect(group2.Evacuating).To(BeNil())
					}
				}
			})
		})

		Context("Domains", func() {
			It("gets all the domains", func() {
				data, gatherError := etcdDB.GatherAndPruneLRPs(logger, testData.cells)
				Expect(gatherError).NotTo(HaveOccurred())

				Expect(data.Domains).To(HaveLen(len(testData.domains)))
				for _, domain := range testData.domains {
					Expect(data.Domains).To(HaveKey(domain))
				}
			})
		})

		Context("Cells", func() {
			It("gets all the cells", func() {
				data, gatherError := etcdDB.GatherAndPruneLRPs(logger, testData.cells)
				Expect(gatherError).NotTo(HaveOccurred())

				Expect(data.Cells).To(HaveLen(len(testData.cells)))
				testData.cells.Each(func(cell *models.CellPresence) {
					Expect(data.Cells).To(ContainElement(cell))
				})
			})
		})

		It("provides all processGuids in the system", func() {
			data, gatherError := etcdDB.GatherAndPruneLRPs(logger, testData.cells)
			Expect(gatherError).NotTo(HaveOccurred())

			expectedGuids := map[string]struct{}{}
			for _, desiredGuid := range testData.validDesiredGuidsWithSomeValidActuals {
				expectedGuids[desiredGuid] = struct{}{}
			}
			for _, desiredGuid := range testData.validDesiredGuidsWithNoActuals {
				expectedGuids[desiredGuid] = struct{}{}
			}
			for _, desiredGuid := range testData.validDesiredGuidsWithOnlyInvalidActuals {
				expectedGuids[desiredGuid] = struct{}{}
			}
			for _, desiredGuid := range testData.invalidDesiredGuidsWithSomeValidActuals {
				expectedGuids[desiredGuid] = struct{}{}
			}
			for _, desiredGuid := range testData.unknownDesiredGuidsWithSomeValidActuals {
				expectedGuids[desiredGuid] = struct{}{}
			}
			for _, desiredGuid := range testData.orphanedSchedulingInfoGuids {
				expectedGuids[desiredGuid] = struct{}{}
			}

			Expect(data.AllProcessGuids).To(Equal(expectedGuids))
		})

		Context("when root nodes are missing", func() {
			BeforeEach(func() {
				etcdRunner.Reset()
			})

			It("returns empty convergence input", func() {
				data, gatherError := etcdDB.GatherAndPruneLRPs(logger, testData.cells)

				Expect(gatherError).NotTo(HaveOccurred())
				Expect(data.AllProcessGuids).To(BeEmpty())
				Expect(data.DesiredLRPs).To(BeEmpty())
				Expect(data.ActualLRPs).To(BeEmpty())
				Expect(data.Domains).To(BeEmpty())
			})
		})
	})

	Describe("Convergence Calculation", func() {
		var cellA = &models.CellPresence{
			CellId:     "cell-a",
			RepAddress: "some-rep-address",
			Zone:       "some-zone",
		}

		var cellB = &models.CellPresence{
			CellId:     "cell-b",
			RepAddress: "some-rep-address",
			Zone:       "some-zone",
		}

		var lrpA = &models.DesiredLRP{
			ProcessGuid: "process-guid-a",
			Instances:   2,
			Domain:      domainA,
		}

		var lrpB = &models.DesiredLRP{
			ProcessGuid: "process-guid-b",
			Instances:   2,
			Domain:      domainB,
		}

		var (
			logger    *lagertest.TestLogger
			fakeClock *fakeclock.FakeClock
			input     *models.ConvergenceInput

			changes *models.ConvergenceChanges
		)

		BeforeEach(func() {
			logger = lagertest.NewTestLogger("test")
			fakeClock = fakeclock.NewFakeClock(time.Unix(0, 1138))
			input = nil
		})

		JustBeforeEach(func() {
			v, ok := etcdDB.(*etcddb.ETCDDB)
			Expect(ok).To(BeTrue())
			changes = etcd.CalculateConvergence(logger, v, fakeClock, models.NewDefaultRestartCalculator(), input)
		})

		Context("actual LRPs with a desired LRP", func() {
			Context("with missing cells", func() {
				BeforeEach(func() {
					input = &models.ConvergenceInput{
						AllProcessGuids: map[string]struct{}{lrpA.ProcessGuid: struct{}{}},
						DesiredLRPs:     desiredLRPs(lrpA),
						ActualLRPs: actualLRPs(
							newRunningActualLRP(lrpA, cellA.CellId, 0),
							newRunningActualLRP(lrpA, cellA.CellId, 1),
						),
						Domains: models.DomainSet{},
						Cells:   models.CellSet{},
					}
				})

				It("reports them", func() {
					output := &models.ConvergenceChanges{
						ActualLRPsWithMissingCells: []*models.ActualLRP{
							newRunningActualLRP(lrpA, cellA.CellId, 0),
							newRunningActualLRP(lrpA, cellA.CellId, 1),
						},
					}

					changesEqual(changes, output)
				})
			})

			Context("with missing desired indices", func() {
				BeforeEach(func() {
					input = &models.ConvergenceInput{
						AllProcessGuids: map[string]struct{}{lrpA.ProcessGuid: struct{}{}},
						DesiredLRPs:     desiredLRPs(lrpA),
						ActualLRPs:      actualLRPs(),
						Domains:         models.NewDomainSet([]string{domainA}),
						Cells:           cellSet(cellA),
					}
				})

				It("reports them", func() {
					output := &models.ConvergenceChanges{
						ActualLRPKeysForMissingIndices: []*models.ActualLRPKey{
							actualLRPKey(lrpA, 0),
							actualLRPKey(lrpA, 1),
						},
					}

					changesEqual(changes, output)
				})

				It("emits missing LRP metrics", func() {
					name, value := fakeMetronClient.SendMetricArgsForCall(0)
					Expect(name).To(Equal("LRPsMissing"))
					Expect(value).To(Equal(2))
				})
			})

			Context("with indices we don't desire", func() {
				BeforeEach(func() {
					input = &models.ConvergenceInput{
						AllProcessGuids: map[string]struct{}{lrpA.ProcessGuid: struct{}{}},
						DesiredLRPs:     desiredLRPs(lrpA),
						ActualLRPs: actualLRPs(
							newRunningActualLRP(lrpA, cellA.CellId, 0),
							newRunningActualLRP(lrpA, cellA.CellId, 1),
							newRunningActualLRP(lrpA, cellA.CellId, 2),
						),
						Domains: models.NewDomainSet([]string{domainA}),
						Cells:   cellSet(cellA),
					}
				})

				It("reports them", func() {
					output := &models.ConvergenceChanges{
						ActualLRPsForExtraIndices: []*models.ActualLRP{
							newRunningActualLRP(lrpA, cellA.CellId, 2),
						},
					}

					changesEqual(changes, output)
				})

				It("emits extra LRP metrics", func() {
					Expect(fakeMetronClient.SendMetricCallCount()).To(Equal(2))
					name, value := fakeMetronClient.SendMetricArgsForCall(1)
					Expect(name).To(Equal("LRPsExtra"))
					Expect(value).To(Equal(1))
				})
			})

			Context("crashed actual LRPS ready to be restarted", func() {
				BeforeEach(func() {
					input = &models.ConvergenceInput{
						AllProcessGuids: map[string]struct{}{lrpA.ProcessGuid: struct{}{}},
						DesiredLRPs:     desiredLRPs(lrpA),
						ActualLRPs: actualLRPs(
							newStartableCrashedActualLRP(lrpA, 0),
							newUnstartableCrashedActualLRP(lrpA, 1),
						),
						Domains: models.NewDomainSet([]string{domainA}),
						Cells:   cellSet(cellA),
					}
				})

				It("reports them", func() {
					output := &models.ConvergenceChanges{
						RestartableCrashedActualLRPs: []*models.ActualLRP{
							newStartableCrashedActualLRP(lrpA, 0),
						},
					}

					changesEqual(changes, output)
				})
			})

			Context("with stale unclaimed actual LRPs", func() {
				BeforeEach(func() {
					input = &models.ConvergenceInput{
						AllProcessGuids: map[string]struct{}{lrpA.ProcessGuid: struct{}{}},
						DesiredLRPs:     desiredLRPs(lrpA),
						ActualLRPs: actualLRPs(
							newRunningActualLRP(lrpA, cellA.CellId, 0),
							newStaleUnclaimedActualLRP(lrpA, 1),
						),
						Domains: models.NewDomainSet([]string{domainA}),
						Cells:   cellSet(cellA),
					}
				})

				It("reports them", func() {
					output := &models.ConvergenceChanges{
						StaleUnclaimedActualLRPs: []*models.ActualLRP{
							newStaleUnclaimedActualLRP(lrpA, 1),
						},
					}

					changesEqual(changes, output)
				})
			})

			Context("an unfresh domain", func() {
				BeforeEach(func() {
					input = &models.ConvergenceInput{
						AllProcessGuids: map[string]struct{}{
							lrpA.ProcessGuid: struct{}{},
							lrpB.ProcessGuid: struct{}{},
						},
						DesiredLRPs: desiredLRPs(lrpA, lrpB),
						ActualLRPs:  actualLRPs(newRunningActualLRP(lrpA, cellA.CellId, 7)),
						Domains:     models.NewDomainSet([]string{domainB}),
						Cells:       cellSet(cellA, cellB),
					}
				})

				It("performs all checks except stopping extra indices", func() {
					output := &models.ConvergenceChanges{
						ActualLRPKeysForMissingIndices: []*models.ActualLRPKey{
							actualLRPKey(lrpA, 0),
							actualLRPKey(lrpA, 1),
							actualLRPKey(lrpB, 0),
							actualLRPKey(lrpB, 1),
						},
					}

					changesEqual(changes, output)
				})
			})
		})

		Context("actual LRPs with no desired LRP", func() {
			BeforeEach(func() {
				input = &models.ConvergenceInput{
					AllProcessGuids: map[string]struct{}{lrpA.ProcessGuid: struct{}{}},
					ActualLRPs: actualLRPs(
						newRunningActualLRP(lrpA, cellA.CellId, 0),
						newRunningActualLRP(lrpA, cellA.CellId, 1),
					),
					Domains: models.NewDomainSet([]string{domainA}),
					Cells:   cellSet(cellA),
				}
			})

			It("reports them", func() {
				output := &models.ConvergenceChanges{
					ActualLRPsForExtraIndices: []*models.ActualLRP{
						newRunningActualLRP(lrpA, cellA.CellId, 0),
						newRunningActualLRP(lrpA, cellA.CellId, 1),
					},
				}

				changesEqual(changes, output)
			})

			It("emits extra LRP metrics", func() {
				Expect(fakeMetronClient.SendMetricCallCount()).To(Equal(2))

				name, value := fakeMetronClient.SendMetricArgsForCall(1)
				Expect(name).To(Equal("LRPsExtra"))
				Expect(value).To(Equal(2))
			})

			Context("with missing cells", func() {
				BeforeEach(func() {
					input.Cells = cellSet()
				})

				It("reports them as extra indices", func() {
					output := &models.ConvergenceChanges{
						ActualLRPsForExtraIndices: []*models.ActualLRP{
							newRunningActualLRP(lrpA, cellA.CellId, 0),
							newRunningActualLRP(lrpA, cellA.CellId, 1),
						},
					}

					changesEqual(changes, output)
				})
			})

			Context("an unfresh domain", func() {
				BeforeEach(func() {
					input.Domains = models.DomainSet{}
				})

				It("does nothing", func() {
					changesEqual(changes, &models.ConvergenceChanges{})
				})
			})
		})

		Context("stable state", func() {
			BeforeEach(func() {
				input = &models.ConvergenceInput{
					AllProcessGuids: map[string]struct{}{lrpA.ProcessGuid: struct{}{}},
					DesiredLRPs:     desiredLRPs(lrpA),
					ActualLRPs: actualLRPs(
						newStableRunningActualLRP(lrpA, cellA.CellId, 0),
						newStableRunningActualLRP(lrpA, cellA.CellId, 1),
					),
					Domains: models.NewDomainSet([]string{domainA}),
					Cells:   cellSet(cellA),
				}
			})

			It("reports nothing", func() {
				changesEqual(changes, &models.ConvergenceChanges{})
			})

			It("emits no missing or extra lrps", func() {
				Expect(fakeMetronClient.SendMetricCallCount()).To(Equal(2))
				name, value := fakeMetronClient.SendMetricArgsForCall(0)
				Expect(name).To(Equal("LRPsMissing"))
				Expect(value).To(Equal(0))

				name, value = fakeMetronClient.SendMetricArgsForCall(1)
				Expect(name).To(Equal("LRPsExtra"))
				Expect(value).To(Equal(0))
			})
		})
	})

	Describe("convergence counters", func() {
		It("bumps the convergence counter", func() {
			Expect(fakeMetronClient.IncrementCounterCallCount()).To(Equal(0))
			etcdDB.ConvergeLRPs(logger, models.CellSet{})
			name := fakeMetronClient.IncrementCounterArgsForCall(0)
			Expect(name).To(Equal("ConvergenceLRPRuns"))
			Expect(fakeMetronClient.IncrementCounterCallCount()).To(Equal(1))
			etcdDB.ConvergeLRPs(logger, models.CellSet{})
			name = fakeMetronClient.IncrementCounterArgsForCall(1)
			Expect(name).To(Equal("ConvergenceLRPRuns"))
			Expect(fakeMetronClient.IncrementCounterCallCount()).To(Equal(2))
		})

		It("reports the duration that it took to converge", func() {
			etcdDB.ConvergeLRPs(logger, models.CellSet{})
			Expect(fakeMetronClient.SendDurationCallCount()).To(Equal(1))
			name, value := fakeMetronClient.SendDurationArgsForCall(0)
			Expect(name).To(Equal("ConvergenceLRPDuration"))
			Expect(value).NotTo(BeZero())
		})
	})

	Describe("converging missing actual LRPs", func() {
		var (
			desiredLRP          *models.DesiredLRP
			processGuid, cellId string

			lrpStartRequests []*auctioneer.LRPStartRequest
			cells            models.CellSet
		)

		BeforeEach(func() {
			processGuid = "some-proccess-guid"
			cellId = "cell-id"

			desiredLRP = model_helpers.NewValidDesiredLRP(processGuid)
			desiredLRP.Instances = 2
			etcdHelper.SetRawDesiredLRP(desiredLRP)

			cellPresence := models.NewCellPresence(
				cellId,
				"cell.example.com",
				"",
				"the-zone",
				models.CellCapacity{MemoryMb: 128, DiskMb: 1024, Containers: 3},
				[]string{},
				[]string{},
				[]string{},
				[]string{},
			)
			cells = models.CellSet{cellId: &cellPresence}
		})

		JustBeforeEach(func() {
			lrpStartRequests, _, _, _ = etcdDB.ConvergeLRPs(logger, cells)
		})

		Context("when there are no actuals for desired LRP", func() {
			It("returns start auction requests for the correct indices", func() {
				Expect(lrpStartRequests).To(HaveLen(1))

				startAuction := lrpStartRequests[0]
				Expect(startAuction.ProcessGuid).To(Equal(desiredLRP.ProcessGuid))
				Expect(startAuction.Indices).To(ConsistOf([]int{0, 1}))
			})
		})

		Context("when there are fewer actuals for desired LRP", func() {
			BeforeEach(func() {
				actualLRP := &models.ActualLRP{
					ActualLRPKey:         models.NewActualLRPKey(desiredLRP.ProcessGuid, 0, desiredLRP.Domain),
					ActualLRPInstanceKey: models.NewActualLRPInstanceKey("some-instance-guid", cellId),
					ActualLRPNetInfo:     models.NewActualLRPNetInfo("1.2.3.4", "2.2.2.2", &models.PortMapping{ContainerPort: 1234, HostPort: 5678}),
					State:                models.ActualLRPStateRunning,
					Since:                clock.Now().Add(-time.Minute).UnixNano(),
				}
				etcdHelper.SetRawActualLRP(actualLRP)
			})

			It("returns start auction requests for the missing index", func() {
				Expect(lrpStartRequests).To(HaveLen(1))

				startAuction := lrpStartRequests[0]
				Expect(startAuction.ProcessGuid).To(Equal(desiredLRP.ProcessGuid))
				Expect(startAuction.Indices).To(ConsistOf([]int{1}))
			})
		})

		Context("when instances are crashing", func() {
			const missingIndex = 0

			BeforeEach(func() {
				now := clock.Now().UnixNano()
				twentyMinutesAgo := clock.Now().Add(-20 * time.Minute).UnixNano()

				crashedRecently := &models.ActualLRP{
					ActualLRPKey: models.NewActualLRPKey(desiredLRP.ProcessGuid, 0, desiredLRP.Domain),
					CrashCount:   5,
					State:        models.ActualLRPStateCrashed,
					Since:        now,
				}

				crashedLongAgo := &models.ActualLRP{
					ActualLRPKey: models.NewActualLRPKey(desiredLRP.ProcessGuid, 1, desiredLRP.Domain),
					CrashCount:   5,
					State:        models.ActualLRPStateCrashed,
					Since:        twentyMinutesAgo,
				}

				etcdHelper.SetRawActualLRP(crashedRecently)
				etcdHelper.SetRawActualLRP(crashedLongAgo)
			})

			It("emits a start auction request for the crashed index", func() {
				Expect(lrpStartRequests).To(HaveLen(1))

				startAuction := lrpStartRequests[0]
				Expect(startAuction.ProcessGuid).To(Equal(desiredLRP.ProcessGuid))
				Expect(startAuction.Indices).To(ConsistOf([]int{1}))
			})

			It("unclaims the crashed actual lrp", func() {
				actualLRPGroup, err := etcdDB.ActualLRPGroupByProcessGuidAndIndex(logger, desiredLRP.ProcessGuid, 1)
				Expect(err).NotTo(HaveOccurred())
				Expect(actualLRPGroup.Instance.State).To(Equal(models.ActualLRPStateUnclaimed))
			})
		})
	})

	Context("when the desired LRP has malformed JSON", func() {
		const processGuid = "bogus-desired"

		BeforeEach(func() {
			etcdHelper.CreateMalformedDesiredLRP(processGuid)
			etcdDB.ConvergeLRPs(logger, models.CellSet{})
		})

		It("logs", func() {
			Expect(logger.TestSink).To(gbytes.Say("failed-to-deserialize-scheduling-info"))
		})
	})

	Context("when the desired LRP has invalid data", func() {
		BeforeEach(func() {
			desiredLRP := model_helpers.NewValidDesiredLRP("process-guid")
			desiredLRP.Instances = -1
			desiredLRP.CpuWeight = 666

			etcdHelper.SetRawDesiredLRP(desiredLRP)
			clock.Increment(10000 * time.Second)
			etcdDB.ConvergeLRPs(logger, models.CellSet{})
		})

		It("deletes the invalid scheduling info and run info", func() {
			_, err := storeClient.Get(etcd.DesiredLRPRunInfoSchemaPath("process-guid"), false, false)
			Expect(err).To(HaveOccurred())

			_, err = storeClient.Get(etcd.DesiredLRPSchedulingInfoSchemaPath("process-guid"), false, false)
			Expect(err).To(HaveOccurred())
		})
	})

	Context("when the acutal LRP has invalid data", func() {
		BeforeEach(func() {
			desiredLRP := model_helpers.NewValidDesiredLRP("process-guid")
			etcdHelper.SetRawDesiredLRP(desiredLRP)

			actualLRP := model_helpers.NewValidActualLRP("process-guid", 9)
			actualLRP.Since = 0
			etcdHelper.SetRawActualLRP(actualLRP)

			etcdDB.ConvergeLRPs(logger, models.CellSet{})
		})

		It("deletes the invalid scheduling info and run info", func() {
			_, err := storeClient.Get(etcd.ActualLRPSchemaPath("process-guid", 9), false, false)
			Expect(err).To(HaveOccurred())
		})
	})

	Describe("pruning LRPs by cell", func() {
		var (
			cellPresence models.CellPresence
			processGuid  string
			desiredLRP   *models.DesiredLRP
			freshDomain  = "some-fresh-domain"

			keysWithMissingCells []*models.ActualLRPKeyWithSchedulingInfo
			lrpKey0, lrpKey1     models.ActualLRPKey

			cells models.CellSet
		)

		BeforeEach(func() {
			cells = models.CellSet{}
			processGuid = "process-guid-for-pruning"

			desiredLRP = model_helpers.NewValidDesiredLRP(processGuid)
			desiredLRP.Instances = 2
			desiredLRP.Domain = freshDomain
			etcdHelper.SetRawDesiredLRP(desiredLRP)

			cellPresence := models.NewCellPresence(
				"cell-id",
				"cell.example.com",
				"",
				"the-zone",
				models.CellCapacity{MemoryMb: 128, DiskMb: 1024, Containers: 3},
				[]string{},
				[]string{},
				[]string{},
				[]string{},
			)

			lrpKey0 = models.NewActualLRPKey(processGuid, 0, freshDomain)
			etcdHelper.SetRawActualLRP(models.NewUnclaimedActualLRP(lrpKey0, 1))

			lrpKey1 = models.NewActualLRPKey(processGuid, 1, freshDomain)
			etcdHelper.SetRawActualLRP(models.NewUnclaimedActualLRP(lrpKey1, 1))

			instanceKey := models.NewActualLRPInstanceKey("instance-guid", cellPresence.CellId)
			_, _, err := etcdDB.ClaimActualLRP(logger, processGuid, 0, &instanceKey)
			Expect(err).NotTo(HaveOccurred())
		})

		JustBeforeEach(func() {
			_, keysWithMissingCells, _, _ = etcdDB.ConvergeLRPs(logger, cells)
		})

		Context("when the cell is present", func() {
			BeforeEach(func() {
				cells["cell-id"] = &cellPresence
			})

			It("should not prune any LRPs", func() {
				groups, err := etcdDB.ActualLRPGroups(logger, models.ActualLRPFilter{})
				Expect(err).NotTo(HaveOccurred())
				Expect(groups).To(HaveLen(2))
			})
		})

		Context("when the cell goes away", func() {
			It("returns the lrp keys to be unclaimed and their start requests", func() {
				Expect(logger.TestSink).To(gbytes.Say("missing-cell"))

				expectedSched := desiredLRP.DesiredLRPSchedulingInfo()
				Expect(keysWithMissingCells).To(ConsistOf(&models.ActualLRPKeyWithSchedulingInfo{
					Key:            &lrpKey0,
					SchedulingInfo: &expectedSched,
				}))
			})

			It("should prune LRP directories for apps that are no longer running", func() {
				actual, err := storeClient.Get(etcd.ActualLRPSchemaRoot, false, true)
				Expect(err).NotTo(HaveOccurred())

				Expect(actual).NotTo(BeNil())
				Expect(actual.Node).NotTo(BeNil())

				Expect(actual.Node.Nodes).To(HaveLen(1))
				Expect(actual.Node.Nodes[0].Key).To(Equal(etcd.ActualLRPProcessDir(processGuid)))
			})
		})
	})

	Describe("converging extra actual LRPs", func() {
		var processGuid string
		var index int32
		var domain string

		BeforeEach(func() {
			domain = "funky-fresh"
			processGuid = "process-guid"
			index = 0

			etcdHelper.SetRawDomain(domain)
		})

		Context("when the actual LRP has no corresponding desired LRP", func() {
			JustBeforeEach(func() {
				actualUnclaimedLRP := &models.ActualLRP{
					ActualLRPKey: models.NewActualLRPKey(processGuid, index, domain),
					State:        models.ActualLRPStateUnclaimed,
					Since:        clock.Now().UnixNano(),
				}

				etcdHelper.SetRawActualLRP(actualUnclaimedLRP)
			})

			Context("when the actual LRP is UNCLAIMED", func() {
				It("returns the lrp to be retired", func() {
					_, _, keysToRetire, _ := etcdDB.ConvergeLRPs(logger, models.CellSet{})
					Expect(keysToRetire).To(ConsistOf(&models.ActualLRPKey{
						ProcessGuid: processGuid,
						Index:       index,
						Domain:      domain,
					}))
				})

				It("logs", func() {
					etcdDB.ConvergeLRPs(logger, models.CellSet{})
					Expect(logger.TestSink).To(gbytes.Say("no-longer-desired"))
				})

				Context("when the LRP domain is not fresh", func() {
					BeforeEach(func() {
						domain = "expired-domain"
					})

					It("returns no lrp to be retired", func() {
						_, _, keysToRetire, _ := etcdDB.ConvergeLRPs(logger, models.CellSet{})
						Expect(keysToRetire).To(BeEmpty())
					})
				})
			})

			Context("when the actual LRP is CLAIMED", func() {
				var (
					cellPresence models.CellPresence
					cells        models.CellSet
				)

				BeforeEach(func() {
					cells = models.CellSet{}
					cellPresence = models.NewCellPresence(
						"cell-id",
						"cell.example.com",
						"",
						"the-zone",
						models.NewCellCapacity(128, 1024, 3),
						[]string{},
						[]string{},
						[]string{},
						[]string{},
					)
				})

				JustBeforeEach(func() {
					actualLRPGroup, err := etcdDB.ActualLRPGroupByProcessGuidAndIndex(logger, processGuid, index)
					Expect(err).NotTo(HaveOccurred())

					instanceKey := models.NewActualLRPInstanceKey("instance-guid", cellPresence.CellId)
					_, _, err = etcdDB.ClaimActualLRP(
						logger,
						actualLRPGroup.Instance.ActualLRPKey.ProcessGuid,
						actualLRPGroup.Instance.ActualLRPKey.Index,
						&instanceKey,
					)
					Expect(err).NotTo(HaveOccurred())
				})

				Context("when the cell exists", func() {
					BeforeEach(func() {
						cells["cell-id"] = &cellPresence
					})

					It("returns the lrp to be retired", func() {
						_, _, keysToRetire, _ := etcdDB.ConvergeLRPs(logger, cells)
						Expect(keysToRetire).To(ConsistOf(&models.ActualLRPKey{
							ProcessGuid: processGuid,
							Index:       index,
							Domain:      domain,
						}))
					})

					It("logs", func() {
						etcdDB.ConvergeLRPs(logger, cells)
						Expect(logger.TestSink).To(gbytes.Say("no-longer-desired"))
					})

					Context("when the LRP domain is not fresh", func() {
						BeforeEach(func() {
							domain = "expired-domain"
						})

						It("returns no lrps to be retired", func() {
							_, _, keysToRetire, _ := etcdDB.ConvergeLRPs(logger, cells)
							Expect(keysToRetire).To(BeEmpty())
						})
					})
				})

				Context("when the cell is missing", func() {
					It("returns the lrp to be retired", func() {
						_, _, keysToRetire, _ := etcdDB.ConvergeLRPs(logger, cells)
						Expect(keysToRetire).To(ConsistOf(&models.ActualLRPKey{
							ProcessGuid: processGuid,
							Index:       index,
							Domain:      domain,
						}))
					})

					Context("when the LRP domain is not fresh", func() {
						BeforeEach(func() {
							domain = "expired-domain"
						})

						It("returns no lrp to be retired", func() {
							_, _, keysToRetire, _ := etcdDB.ConvergeLRPs(logger, cells)
							Expect(keysToRetire).To(BeEmpty())
						})
					})
				})
			})

			Context("when the actual LRP is RUNNING", func() {
				var (
					cellPresence models.CellPresence
					cells        models.CellSet
				)

				JustBeforeEach(func() {
					cellPresence = models.NewCellPresence(
						"cell-id",
						"cell.example.com",
						"",
						"the-zone",
						models.NewCellCapacity(128, 1024, 3),
						[]string{},
						[]string{},
						[]string{},
						[]string{},
					)
					cells = models.CellSet{}
					cells["cell-id"] = &cellPresence

					actualLRPGroup, err := etcdDB.ActualLRPGroupByProcessGuidAndIndex(logger, processGuid, index)
					Expect(err).NotTo(HaveOccurred())

					instanceKey := models.NewActualLRPInstanceKey("instance-guid", cellPresence.CellId)
					netInfo := models.NewActualLRPNetInfo("host", "container", &models.PortMapping{HostPort: 1234, ContainerPort: 5678})
					_, _, err = etcdDB.ClaimActualLRP(
						logger,
						actualLRPGroup.Instance.ProcessGuid,
						actualLRPGroup.Instance.Index,
						&instanceKey,
					)
					Expect(err).NotTo(HaveOccurred())

					_, _, err = etcdDB.StartActualLRP(
						logger,
						&actualLRPGroup.Instance.ActualLRPKey,
						&instanceKey,
						&netInfo,
					)
					Expect(err).NotTo(HaveOccurred())
				})

				It("returns the correct lrps to retire", func() {
					_, _, keysToRetire, _ := etcdDB.ConvergeLRPs(logger, cells)
					Expect(keysToRetire).To(ConsistOf(&models.ActualLRPKey{
						ProcessGuid: processGuid,
						Index:       index,
						Domain:      domain,
					}))
				})

				Context("when the LRP domain is not fresh", func() {
					BeforeEach(func() {
						domain = "expired-domain"
					})

					It("returns no lrps to retire", func() {
						_, _, keysToRetire, _ := etcdDB.ConvergeLRPs(logger, cells)
						Expect(keysToRetire).To(BeEmpty())
					})
				})
			})
		})

		Context("when the actual LRP index is too large for its corresponding desired LRP", func() {
			var (
				desiredLRP   *models.DesiredLRP
				numInstances int32
			)

			BeforeEach(func() {
				processGuid = "process-guid"
				numInstances = 2

				domain = "always-fresh-never-frozen"
				etcdHelper.SetRawDomain(domain)
			})

			JustBeforeEach(func() {
				desiredLRP = model_helpers.NewValidDesiredLRP(processGuid)
				desiredLRP.Instances = numInstances
				desiredLRP.Domain = domain

				etcdHelper.SetRawDesiredLRP(desiredLRP)
			})

			Context("when the actual LRP is UNCLAIMED", func() {
				JustBeforeEach(func() {
					index = numInstances

					higherIndexActualLRP := &models.ActualLRP{
						ActualLRPKey: models.NewActualLRPKey(desiredLRP.ProcessGuid, index, desiredLRP.Domain),
						State:        models.ActualLRPStateUnclaimed,
						Since:        clock.Now().UnixNano(),
					}

					etcdHelper.SetRawActualLRP(higherIndexActualLRP)
				})

				It("returns the lrp to be retired", func() {
					_, _, keysToRetire, _ := etcdDB.ConvergeLRPs(logger, models.CellSet{})
					Expect(keysToRetire).To(ConsistOf(&models.ActualLRPKey{
						ProcessGuid: processGuid,
						Index:       index,
						Domain:      domain,
					}))
				})

				Context("when the LRP domain is not fresh", func() {
					BeforeEach(func() {
						domain = "expired-domain"
					})

					It("returns no lrp to be retired", func() {
						_, _, keysToRetire, _ := etcdDB.ConvergeLRPs(logger, models.CellSet{})
						Expect(keysToRetire).To(BeEmpty())
					})
				})
			})

			Context("when the actual LRP is CLAIMED", func() {
				var (
					cellPresence models.CellPresence
					cells        models.CellSet
				)

				JustBeforeEach(func() {
					cellPresence = models.NewCellPresence(
						"cell-id",
						"cell.example.com",
						"",
						"the-zone",
						models.NewCellCapacity(128, 1024, 100),
						[]string{},
						[]string{},
						[]string{},
						[]string{},
					)
					cells = models.CellSet{}
					cells["cell-id"] = &cellPresence

					index = numInstances

					higherIndexActualLRP := &models.ActualLRP{
						ActualLRPKey:         models.NewActualLRPKey(desiredLRP.ProcessGuid, index, desiredLRP.Domain),
						ActualLRPInstanceKey: models.NewActualLRPInstanceKey("instance-guid", "cell-id"),
						State:                models.ActualLRPStateClaimed,
						Since:                clock.Now().UnixNano(),
					}

					etcdHelper.SetRawActualLRP(higherIndexActualLRP)

					actualLRPGroup, err := etcdDB.ActualLRPGroupByProcessGuidAndIndex(logger, processGuid, index)
					Expect(err).NotTo(HaveOccurred())

					instanceKey := models.NewActualLRPInstanceKey("instance-guid", cellPresence.CellId)
					_, _, err = etcdDB.ClaimActualLRP(
						logger,
						actualLRPGroup.Instance.ActualLRPKey.ProcessGuid,
						actualLRPGroup.Instance.ActualLRPKey.Index,
						&instanceKey,
					)
					Expect(err).NotTo(HaveOccurred())
				})

				It("returns the lrp to be retired", func() {
					_, _, keysToRetire, _ := etcdDB.ConvergeLRPs(logger, cells)
					Expect(keysToRetire).To(ConsistOf(&models.ActualLRPKey{
						ProcessGuid: processGuid,
						Index:       index,
						Domain:      domain,
					}))
				})

				Context("when the LRP domain is not fresh", func() {
					BeforeEach(func() {
						domain = "expired-domain"
					})

					It("returns no lrp to be retired", func() {
						_, _, keysToRetire, _ := etcdDB.ConvergeLRPs(logger, cells)
						Expect(keysToRetire).To(BeEmpty())
					})
				})
			})

			Context("when the actual LRP is RUNNING", func() {
				var (
					cellPresence models.CellPresence
					cells        models.CellSet
				)

				JustBeforeEach(func() {
					cellPresence = models.NewCellPresence(
						"cell-id",
						"cell.example.com",
						"",
						"the-zone",
						models.NewCellCapacity(124, 1024, 6),
						[]string{},
						[]string{},
						[]string{},
						[]string{},
					)
					cells = models.CellSet{}
					cells["cell-id"] = &cellPresence

					index = numInstances

					higherIndexActualLRP := &models.ActualLRP{
						ActualLRPKey:         models.NewActualLRPKey(desiredLRP.ProcessGuid, index, desiredLRP.Domain),
						ActualLRPInstanceKey: models.NewActualLRPInstanceKey("instance-guid", "cell-id"),
						ActualLRPNetInfo:     models.NewActualLRPNetInfo("127.0.0.1", "2.2.2.2", &models.PortMapping{ContainerPort: 8080, HostPort: 80}),
						State:                models.ActualLRPStateRunning,
						Since:                clock.Now().UnixNano(),
					}

					etcdHelper.SetRawActualLRP(higherIndexActualLRP)

					actualLRPGroup, err := etcdDB.ActualLRPGroupByProcessGuidAndIndex(logger, processGuid, index)
					Expect(err).NotTo(HaveOccurred())

					instanceKey := models.NewActualLRPInstanceKey("instance-guid", cellPresence.CellId)
					netInfo := models.NewActualLRPNetInfo("host", "2.2.2.2", &models.PortMapping{HostPort: 1234, ContainerPort: 5678})
					_, _, err = etcdDB.ClaimActualLRP(
						logger,
						actualLRPGroup.Instance.ActualLRPKey.ProcessGuid,
						actualLRPGroup.Instance.ActualLRPKey.Index,
						&instanceKey,
					)
					Expect(err).NotTo(HaveOccurred())

					_, _, err = etcdDB.StartActualLRP(
						logger,
						&actualLRPGroup.Instance.ActualLRPKey,
						&instanceKey,
						&netInfo,
					)
					Expect(err).NotTo(HaveOccurred())
				})

				It("sends a stop request to the corresponding cell", func() {
					_, _, keysToRetire, _ := etcdDB.ConvergeLRPs(logger, cells)
					Expect(keysToRetire).To(ConsistOf(&models.ActualLRPKey{
						ProcessGuid: processGuid,
						Index:       index,
						Domain:      domain,
					}))
				})

				Context("when the LRP domain is not fresh", func() {
					BeforeEach(func() {
						domain = "expired-domain"
					})

					It("does not stop the actual LRP", func() {
						_, _, keysToRetire, _ := etcdDB.ConvergeLRPs(logger, cells)
						Expect(keysToRetire).To(HaveLen(0))
					})
				})
			})
		})
	})

	Describe("converging actual LRPs that are UNCLAIMED for too long", func() {
		var (
			desiredLRP          *models.DesiredLRP
			processGuid, domain string
		)

		BeforeEach(func() {
			processGuid = "processedGuid"
			domain = "the-freshmaker"

			desiredLRP = model_helpers.NewValidDesiredLRP(processGuid)
			desiredLRP.Domain = domain

			err := etcdDB.DesireLRP(logger, desiredLRP)
			Expect(err).NotTo(HaveOccurred())

			clock.Increment(models.StaleUnclaimedActualLRPDuration + 1*time.Second)
		})

		It("logs", func() {
			etcdDB.ConvergeLRPs(logger, models.CellSet{})
			Expect(logger.TestSink).To(gbytes.Say("adding-start-auction"))
		})

		It("re-returns start auction requests", func() {
			startRequests, _, _, _ := etcdDB.ConvergeLRPs(logger, models.CellSet{})
			Expect(startRequests).To(HaveLen(1))

			startAuction := startRequests[0]
			Expect(startAuction.ProcessGuid).To(Equal(desiredLRP.ProcessGuid))
			Expect(startAuction.Indices).To(ConsistOf([]int{0}))
		})
	})
})

const cellID = "some-cell-id"
const domain = "test-domain"

const domainA = "domain-a"
const domainB = "domain-b"

const staleUnclaimedDuration = 30 * time.Second

// ActualLRPs with indices that don't make sense for their corresponding DesiredLRPs
// are *not* pruned at this phase
const randomIndex1 = 9001
const randomIndex2 = 1337

type processGuidAndIndex struct {
	processGuid string
	index       int32
}

type testDataForConvergenceGatherer struct {
	instanceKeysToKeep    map[processGuidAndIndex]struct{}
	instanceKeysToPrune   map[processGuidAndIndex]struct{}
	evacuatingKeysToKeep  map[processGuidAndIndex]struct{}
	evacuatingKeysToPrune map[processGuidAndIndex]struct{}
	domains               []string
	cells                 models.CellSet

	validDesiredGuidsWithSomeValidActuals     []string
	validDesiredGuidsWithNoActuals            []string
	validDesiredGuidsWithOnlyInvalidActuals   []string
	invalidDesiredGuidsWithSomeValidActuals   []string
	invalidDesiredGuidsWithNoActuals          []string
	invalidDesiredGuidsWithOnlyInvalidActuals []string
	unknownDesiredGuidsWithSomeValidActuals   []string
	unknownDesiredGuidsWithNoActuals          []string
	unknownDesiredGuidsWithOnlyInvalidActuals []string
	oldOrphanedRunInfoGuids                   []string
	recentOrphanedRunInfoGuids                []string
	orphanedSchedulingInfoGuids               []string
}

func createTestData(
	numValidDesiredGuidsWithSomeValidActuals,
	numValidDesiredGuidsWithNoActuals,
	numValidDesiredGuidsWithOnlyInvalidActuals,
	numInvalidDesiredGuidsWithSomeValidActuals,
	numInvalidDesiredGuidsWithNoActuals,
	numInvalidDesiredGuidsWithOnlyInvalidActuals,
	numUnknownDesiredGuidsWithSomeValidActuals,
	numUnknownDesiredGuidsWithNoActuals,
	numUnknownDesiredGuidsWithOnlyInvalidActuals,
	numOldOrphanedRunInfos,
	numRecentOrphanedRunInfos,
	numOrphanedSchedulingInfos int,
) *testDataForConvergenceGatherer {
	testData := &testDataForConvergenceGatherer{
		instanceKeysToKeep:    map[processGuidAndIndex]struct{}{},
		instanceKeysToPrune:   map[processGuidAndIndex]struct{}{},
		evacuatingKeysToKeep:  map[processGuidAndIndex]struct{}{},
		evacuatingKeysToPrune: map[processGuidAndIndex]struct{}{},
		domains:               []string{},
		cells:                 models.CellSet{},

		validDesiredGuidsWithSomeValidActuals:     []string{},
		validDesiredGuidsWithNoActuals:            []string{},
		validDesiredGuidsWithOnlyInvalidActuals:   []string{},
		invalidDesiredGuidsWithSomeValidActuals:   []string{},
		invalidDesiredGuidsWithNoActuals:          []string{},
		invalidDesiredGuidsWithOnlyInvalidActuals: []string{},
		unknownDesiredGuidsWithSomeValidActuals:   []string{},
		unknownDesiredGuidsWithNoActuals:          []string{},
		unknownDesiredGuidsWithOnlyInvalidActuals: []string{},
		oldOrphanedRunInfoGuids:                   []string{},
		recentOrphanedRunInfoGuids:                []string{},
		orphanedSchedulingInfoGuids:               []string{},
	}

	for i := 0; i < numValidDesiredGuidsWithSomeValidActuals; i++ {
		guid := fmt.Sprintf("valid-desired-with-some-valid-actuals-%d", i)
		testData.validDesiredGuidsWithSomeValidActuals = append(
			testData.validDesiredGuidsWithSomeValidActuals,
			guid,
		)

		switch i % 3 {
		case 0:
			testData.instanceKeysToKeep[processGuidAndIndex{guid, randomIndex1}] = struct{}{}
			testData.instanceKeysToKeep[processGuidAndIndex{guid, randomIndex2}] = struct{}{}
		case 1:
			testData.instanceKeysToKeep[processGuidAndIndex{guid, randomIndex1}] = struct{}{}
			testData.instanceKeysToPrune[processGuidAndIndex{guid, randomIndex2}] = struct{}{}
		case 2:
			testData.evacuatingKeysToKeep[processGuidAndIndex{guid, randomIndex1}] = struct{}{}
			testData.instanceKeysToKeep[processGuidAndIndex{guid, randomIndex2}] = struct{}{}

			testData.instanceKeysToPrune[processGuidAndIndex{guid, randomIndex1}] = struct{}{}
			testData.evacuatingKeysToPrune[processGuidAndIndex{guid, randomIndex2}] = struct{}{}
		}
	}

	for i := 0; i < numValidDesiredGuidsWithNoActuals; i++ {
		guid := fmt.Sprintf("valid-desired-with-no-actuals-%d", i)
		testData.validDesiredGuidsWithNoActuals = append(
			testData.validDesiredGuidsWithNoActuals,
			guid,
		)
	}

	for i := 0; i < numValidDesiredGuidsWithOnlyInvalidActuals; i++ {
		guid := fmt.Sprintf("valid-desired-with-only-invalid-actuals-%d", i)
		testData.validDesiredGuidsWithOnlyInvalidActuals = append(
			testData.validDesiredGuidsWithOnlyInvalidActuals,
			guid,
		)

		testData.instanceKeysToPrune[processGuidAndIndex{guid, randomIndex1}] = struct{}{}
		testData.instanceKeysToPrune[processGuidAndIndex{guid, randomIndex2}] = struct{}{}
	}

	for i := 0; i < numInvalidDesiredGuidsWithSomeValidActuals; i++ {
		guid := fmt.Sprintf("invalid-desired-with-some-valid-actuals-%d", i)
		testData.invalidDesiredGuidsWithSomeValidActuals = append(
			testData.invalidDesiredGuidsWithSomeValidActuals,
			guid,
		)

		switch i % 3 {
		case 0:
			testData.instanceKeysToKeep[processGuidAndIndex{guid, randomIndex1}] = struct{}{}
			testData.instanceKeysToKeep[processGuidAndIndex{guid, randomIndex2}] = struct{}{}
		case 1:
			testData.instanceKeysToKeep[processGuidAndIndex{guid, randomIndex1}] = struct{}{}
			testData.instanceKeysToPrune[processGuidAndIndex{guid, randomIndex2}] = struct{}{}
		case 2:
			testData.evacuatingKeysToKeep[processGuidAndIndex{guid, randomIndex1}] = struct{}{}
			testData.instanceKeysToPrune[processGuidAndIndex{guid, randomIndex1}] = struct{}{}
			testData.evacuatingKeysToPrune[processGuidAndIndex{guid, randomIndex2}] = struct{}{}
			testData.instanceKeysToKeep[processGuidAndIndex{guid, randomIndex2}] = struct{}{}
		}
	}

	for i := 0; i < numInvalidDesiredGuidsWithNoActuals; i++ {
		guid := fmt.Sprintf("invalid-desired-with-no-actuals-%d", i)
		testData.invalidDesiredGuidsWithNoActuals = append(
			testData.invalidDesiredGuidsWithNoActuals,
			guid,
		)
	}

	for i := 0; i < numInvalidDesiredGuidsWithOnlyInvalidActuals; i++ {
		guid := fmt.Sprintf("invalid-desired-with-only-invalid-actuals-%d", i)
		testData.invalidDesiredGuidsWithOnlyInvalidActuals = append(
			testData.invalidDesiredGuidsWithOnlyInvalidActuals,
			guid,
		)

		testData.instanceKeysToPrune[processGuidAndIndex{guid, randomIndex1}] = struct{}{}
		testData.instanceKeysToPrune[processGuidAndIndex{guid, randomIndex2}] = struct{}{}
	}

	for i := 0; i < numUnknownDesiredGuidsWithSomeValidActuals; i++ {
		guid := fmt.Sprintf("unknown-desired-with-some-valid-actuals-%d", i)
		testData.unknownDesiredGuidsWithSomeValidActuals = append(
			testData.unknownDesiredGuidsWithSomeValidActuals,
			guid,
		)

		switch i % 3 {
		case 0:
			testData.instanceKeysToKeep[processGuidAndIndex{guid, randomIndex1}] = struct{}{}
			testData.instanceKeysToKeep[processGuidAndIndex{guid, randomIndex2}] = struct{}{}
		case 1:
			testData.instanceKeysToKeep[processGuidAndIndex{guid, randomIndex1}] = struct{}{}
			testData.instanceKeysToPrune[processGuidAndIndex{guid, randomIndex2}] = struct{}{}
		case 2:
			testData.evacuatingKeysToKeep[processGuidAndIndex{guid, randomIndex1}] = struct{}{}
			testData.instanceKeysToPrune[processGuidAndIndex{guid, randomIndex1}] = struct{}{}
			testData.evacuatingKeysToPrune[processGuidAndIndex{guid, randomIndex2}] = struct{}{}
			testData.instanceKeysToKeep[processGuidAndIndex{guid, randomIndex2}] = struct{}{}
		}
	}

	for i := 0; i < numUnknownDesiredGuidsWithNoActuals; i++ {
		guid := fmt.Sprintf("unknown-desired-with-no-actuals-%d", i)
		testData.unknownDesiredGuidsWithNoActuals = append(
			testData.unknownDesiredGuidsWithNoActuals,
			guid,
		)
	}

	for i := 0; i < numUnknownDesiredGuidsWithOnlyInvalidActuals; i++ {
		guid := fmt.Sprintf("unknown-desired-with-only-invalid-actuals-%d", i)
		testData.unknownDesiredGuidsWithOnlyInvalidActuals = append(
			testData.unknownDesiredGuidsWithOnlyInvalidActuals,
			guid,
		)

		testData.instanceKeysToPrune[processGuidAndIndex{guid, randomIndex1}] = struct{}{}
		testData.instanceKeysToPrune[processGuidAndIndex{guid, randomIndex2}] = struct{}{}
	}

	for i := 0; i < numOldOrphanedRunInfos; i++ {
		guid := fmt.Sprintf("old-orphaned-run-info-with-no-scheduling-info-%d", i)
		testData.oldOrphanedRunInfoGuids = append(
			testData.oldOrphanedRunInfoGuids,
			guid,
		)
	}

	for i := 0; i < numRecentOrphanedRunInfos; i++ {
		guid := fmt.Sprintf("recent-orphaned-run-info-with-no-scheduling-info-%d", i)
		testData.recentOrphanedRunInfoGuids = append(
			testData.recentOrphanedRunInfoGuids,
			guid,
		)
	}

	for i := 0; i < numOrphanedSchedulingInfos; i++ {
		guid := fmt.Sprintf("orphaned-scheduling-info-with-no-run-info-%d", i)
		testData.orphanedSchedulingInfoGuids = append(
			testData.orphanedSchedulingInfoGuids,
			guid,
		)
	}

	testData.domains = append(testData.domains, domain)

	testData.cells = models.CellSet{
		cellID:       newCellPresence(cellID),
		"other-cell": newCellPresence("other-cell"),
	}

	for actualData := range testData.instanceKeysToKeep {
		etcdHelper.CreateValidActualLRP(actualData.processGuid, actualData.index)
	}

	for actualData := range testData.instanceKeysToPrune {
		etcdHelper.CreateMalformedActualLRP(actualData.processGuid, actualData.index)
	}

	for actualData := range testData.evacuatingKeysToKeep {
		etcdHelper.CreateValidEvacuatingLRP(actualData.processGuid, actualData.index)
	}

	for actualData := range testData.evacuatingKeysToPrune {
		etcdHelper.CreateMalformedEvacuatingLRP(actualData.processGuid, actualData.index)
	}

	for _, guid := range testData.validDesiredGuidsWithSomeValidActuals {
		etcdHelper.CreateValidDesiredLRP(guid)
	}

	for _, guid := range testData.validDesiredGuidsWithNoActuals {
		etcdHelper.CreateValidDesiredLRP(guid)
	}

	for _, guid := range testData.validDesiredGuidsWithOnlyInvalidActuals {
		etcdHelper.CreateValidDesiredLRP(guid)
	}

	for _, guid := range testData.invalidDesiredGuidsWithSomeValidActuals {
		etcdHelper.CreateMalformedDesiredLRP(guid)
	}

	for _, guid := range testData.invalidDesiredGuidsWithNoActuals {
		etcdHelper.CreateMalformedDesiredLRP(guid)
	}

	for _, guid := range testData.invalidDesiredGuidsWithOnlyInvalidActuals {
		etcdHelper.CreateMalformedDesiredLRP(guid)
	}

	for _, guid := range testData.unknownDesiredGuidsWithSomeValidActuals {
		etcdHelper.CreateMalformedDesiredLRP(guid)
	}

	for _, guid := range testData.unknownDesiredGuidsWithNoActuals {
		etcdHelper.CreateMalformedDesiredLRP(guid)
	}

	for _, guid := range testData.unknownDesiredGuidsWithOnlyInvalidActuals {
		etcdHelper.CreateMalformedDesiredLRP(guid)
	}

	for _, guid := range testData.oldOrphanedRunInfoGuids {
		etcdHelper.CreateOrphanedRunInfo(guid, clock.Now().Add(-time.Hour))
	}

	for _, guid := range testData.recentOrphanedRunInfoGuids {
		etcdHelper.CreateOrphanedRunInfo(guid, clock.Now())
	}

	for _, guid := range testData.orphanedSchedulingInfoGuids {
		etcdHelper.CreateOrphanedSchedulingInfo(guid, clock.Now())
	}

	for _, domain := range testData.domains {
		etcdHelper.SetRawDomain(domain)
	}

	return testData
}

func desiredLRPs(lrps ...*models.DesiredLRP) map[string]*models.DesiredLRP {
	set := map[string]*models.DesiredLRP{}

	for _, lrp := range lrps {
		set[lrp.ProcessGuid] = lrp
	}
	return set
}

func actualLRPs(lrps ...*models.ActualLRP) map[string]map[int32]*models.ActualLRP {
	set := map[string]map[int32]*models.ActualLRP{}

	for _, lrp := range lrps {
		byIndex, found := set[lrp.ProcessGuid]
		if !found {
			byIndex = map[int32]*models.ActualLRP{}
			set[lrp.ProcessGuid] = byIndex
		}

		byIndex[lrp.Index] = lrp
	}

	return set
}

func actualLRPKey(lrp *models.DesiredLRP, index int32) *models.ActualLRPKey {
	lrpKey := models.NewActualLRPKey(lrp.ProcessGuid, index, lrp.Domain)
	return &lrpKey
}

func crashedActualReadyForRestart(lrp *models.DesiredLRP, index int32) *models.ActualLRP {
	return &models.ActualLRP{
		ActualLRPKey: *actualLRPKey(lrp, index),
		CrashCount:   1,
		State:        models.ActualLRPStateCrashed,
		Since:        1138,
	}
}

func crashedActualNeverRestart(lrp *models.DesiredLRP, index int32) *models.ActualLRP {
	return &models.ActualLRP{
		ActualLRPKey: *actualLRPKey(lrp, index),
		CrashCount:   201,
		State:        models.ActualLRPStateCrashed,
		Since:        1138,
	}
}

func newNotStaleUnclaimedActualLRP(lrp *models.DesiredLRP, index int32) *models.ActualLRP {
	return &models.ActualLRP{
		ActualLRPKey: *actualLRPKey(lrp, index),
		State:        models.ActualLRPStateUnclaimed,
		Since:        1138,
	}
}

func newStaleUnclaimedActualLRP(lrp *models.DesiredLRP, index int32) *models.ActualLRP {
	return &models.ActualLRP{
		ActualLRPKey: *actualLRPKey(lrp, index),
		State:        models.ActualLRPStateUnclaimed,
		Since:        1138 - (staleUnclaimedDuration + time.Second).Nanoseconds(),
	}
}

func newStableRunningActualLRP(lrp *models.DesiredLRP, cellID string, index int32) *models.ActualLRP {
	return &models.ActualLRP{
		ActualLRPKey:         *actualLRPKey(lrp, index),
		ActualLRPInstanceKey: models.NewActualLRPInstanceKey("instance-guid", cellID),
		ActualLRPNetInfo:     models.NewActualLRPNetInfo("1.2.3.4", "2.2.2.2", &models.PortMapping{}),
		State:                models.ActualLRPStateRunning,
		Since:                1138 - (30 * time.Minute).Nanoseconds(),
	}
}

func newRunningActualLRP(d *models.DesiredLRP, cellID string, index int32) *models.ActualLRP {
	return &models.ActualLRP{
		ActualLRPKey:         models.NewActualLRPKey(d.ProcessGuid, index, d.Domain),
		ActualLRPInstanceKey: models.NewActualLRPInstanceKey("instance-guid", cellID),
		ActualLRPNetInfo:     models.NewActualLRPNetInfo("1.2.3.4", "2.2.2.2", &models.PortMapping{}),
		State:                models.ActualLRPStateRunning,
		Since:                1138,
	}
}

func newStartableCrashedActualLRP(d *models.DesiredLRP, index int32) *models.ActualLRP {
	return &models.ActualLRP{
		ActualLRPKey: models.NewActualLRPKey(d.ProcessGuid, index, d.Domain),
		CrashCount:   1,
		State:        models.ActualLRPStateCrashed,
		Since:        1138,
	}
}

func newUnstartableCrashedActualLRP(d *models.DesiredLRP, index int32) *models.ActualLRP {
	return &models.ActualLRP{
		ActualLRPKey: models.NewActualLRPKey(d.ProcessGuid, index, d.Domain),
		CrashCount:   201,
		State:        models.ActualLRPStateCrashed,
		Since:        1138,
	}
}

func newCellPresence(cellID string) *models.CellPresence {
	cellPresence := models.NewCellPresence(
		cellID,
		"1.2.3.4",
		"",
		"az-1",
		models.NewCellCapacity(128, 1024, 3),
		[]string{},
		nil,
		nil,
		nil,
	)
	return &cellPresence
}

func changesEqual(actual *models.ConvergenceChanges, expected *models.ConvergenceChanges) {
	Expect(actual).NotTo(BeNil())
	Expect(actual.ActualLRPsWithMissingCells).To(ConsistOf(expected.ActualLRPsWithMissingCells))
	Expect(actual.ActualLRPsForExtraIndices).To(ConsistOf(expected.ActualLRPsForExtraIndices))
	Expect(actual.ActualLRPKeysForMissingIndices).To(ConsistOf(expected.ActualLRPKeysForMissingIndices))
	Expect(actual.RestartableCrashedActualLRPs).To(ConsistOf(expected.RestartableCrashedActualLRPs))
	Expect(actual.StaleUnclaimedActualLRPs).To(ConsistOf(expected.StaleUnclaimedActualLRPs))
}

func cellSet(cells ...*models.CellPresence) models.CellSet {
	set := models.CellSet{}

	for _, cell := range cells {
		set.Add(cell)
	}

	return set
}
