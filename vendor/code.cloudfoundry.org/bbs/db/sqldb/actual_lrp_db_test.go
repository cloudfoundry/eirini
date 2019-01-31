package sqldb_test

import (
	"errors"
	"strings"
	"time"

	"code.cloudfoundry.org/bbs/models"
	"code.cloudfoundry.org/bbs/models/test/model_helpers"
	"code.cloudfoundry.org/bbs/test_helpers"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("ActualLRPDB", func() {
	BeforeEach(func() {
		fakeGUIDProvider.NextGUIDReturns("my-awesome-guid", nil)
	})

	Describe("ChangeActualLRPPresence", func() {
		var (
			key         *models.ActualLRPKey
			instanceKey *models.ActualLRPInstanceKey
			netInfo     *models.ActualLRPNetInfo
		)

		BeforeEach(func() {
			lrpKey := models.NewActualLRPKey("some-guid", 0, "some-domain")
			key = &lrpKey
			lrpInstanceKey := models.NewActualLRPInstanceKey("ig-1", "cell-id")
			instanceKey = &lrpInstanceKey
			netInfo = &models.ActualLRPNetInfo{
				Address:         "0.0.0.0",
				Ports:           []*models.PortMapping{},
				InstanceAddress: "1.1.1.1",
			}
		})

		Context("when the lrp exists", func() {
			BeforeEach(func() {
				_, err := sqlDB.CreateUnclaimedActualLRP(logger, key)
				Expect(err).NotTo(HaveOccurred())
				_, _, err = sqlDB.StartActualLRP(logger, key, instanceKey, netInfo)
				Expect(err).NotTo(HaveOccurred())
			})

			It("changes its presence", func() {
				before, after, err := sqlDB.ChangeActualLRPPresence(logger, key, models.ActualLRP_Ordinary, models.ActualLRP_Suspect)
				Expect(err).NotTo(HaveOccurred())

				Expect(before.Presence).To(Equal(models.ActualLRP_Ordinary))
				Expect(after.Presence).To(Equal(models.ActualLRP_Suspect))
			})

			Context("when an LRP with the desired presence already exist", func() {
				BeforeEach(func() {
					_, _, err := sqlDB.ChangeActualLRPPresence(logger, key, models.ActualLRP_Ordinary, models.ActualLRP_Suspect)
					Expect(err).NotTo(HaveOccurred())
					_, err = sqlDB.CreateUnclaimedActualLRP(logger, key)
					Expect(err).NotTo(HaveOccurred())
					_, _, err = sqlDB.StartActualLRP(logger, key, instanceKey, netInfo)
					Expect(err).NotTo(HaveOccurred())
				})

				It("returns a ResourceExists error", func() {
					_, _, err := sqlDB.ChangeActualLRPPresence(logger, key, models.ActualLRP_Ordinary, models.ActualLRP_Suspect)
					Expect(err).To(MatchError(models.ErrResourceExists))
				})
			})
		})

		Context("when the key doesn't exist", func() {
			Context("because it does not exist", func() {
				It("returns a ResourceNotFound error", func() {
					_, _, err := sqlDB.ChangeActualLRPPresence(logger, key, models.ActualLRP_Ordinary, models.ActualLRP_Suspect)
					Expect(err).To(MatchError(models.ErrResourceNotFound))
				})
			})

			Context("because it has the wrong presence", func() {
				BeforeEach(func() {
					_, err := sqlDB.EvacuateActualLRP(logger, key, instanceKey, netInfo)
					Expect(err).NotTo(HaveOccurred())
				})

				It("returns a ResourceNotFound error", func() {
					_, _, err := sqlDB.ChangeActualLRPPresence(logger, key, models.ActualLRP_Ordinary, models.ActualLRP_Suspect)
					Expect(err).To(MatchError(models.ErrResourceNotFound))
				})
			})
		})
	})

	Describe("CreateUnclaimedActualLRP", func() {
		var key *models.ActualLRPKey

		BeforeEach(func() {
			key = &models.ActualLRPKey{
				ProcessGuid: "the-guid",
				Index:       0,
				Domain:      "the-domain",
			}
		})

		It("persists the actual lrp into the database", func() {
			actualLRP, err := sqlDB.CreateUnclaimedActualLRP(logger, key)
			Expect(err).NotTo(HaveOccurred())

			expectedActualLRP := models.NewUnclaimedActualLRP(*key, fakeClock.Now().UnixNano())
			expectedActualLRP.ModificationTag.Epoch = "my-awesome-guid"
			expectedActualLRP.ModificationTag.Index = 0

			actualLRPs, err := sqlDB.ActualLRPs(logger, models.ActualLRPFilter{ProcessGuid: key.ProcessGuid, Index: &key.Index})
			Expect(err).NotTo(HaveOccurred())
			Expect(actualLRPs).NotTo(BeNil())
			Expect(actualLRPs).To(ConsistOf(expectedActualLRP))
			Expect(actualLRP).To(Equal(expectedActualLRP))
		})

		Context("when generating a guid fails", func() {
			BeforeEach(func() {
				fakeGUIDProvider.NextGUIDReturns("", errors.New("no guid for you"))
			})

			It("returns the error", func() {
				_, err := sqlDB.CreateUnclaimedActualLRP(logger, key)
				Expect(err).To(HaveOccurred())
				Expect(err).To(Equal(models.ErrGUIDGeneration))
			})
		})

		Context("when the actual lrp already exists", func() {
			BeforeEach(func() {
				_, err := sqlDB.CreateUnclaimedActualLRP(logger, key)
				Expect(err).NotTo(HaveOccurred())
			})

			It("returns a ResourceExists error", func() {
				_, err := sqlDB.CreateUnclaimedActualLRP(logger, key)
				Expect(err).To(HaveOccurred())
				Expect(err).To(Equal(models.ErrResourceExists))
			})
		})
	})

	Describe("ActualLRPs", func() {
		var allActualLRPs []*models.ActualLRP

		BeforeEach(func() {
			allActualLRPs = []*models.ActualLRP{}
			fakeGUIDProvider.NextGUIDReturns("mod-tag-guid", nil)

			actualLRPKey1 := &models.ActualLRPKey{
				ProcessGuid: "guid1",
				Index:       1,
				Domain:      "domain1",
			}
			instanceKey1 := &models.ActualLRPInstanceKey{
				InstanceGuid: "i-guid1",
				CellId:       "cell1",
			}

			_, err := sqlDB.CreateUnclaimedActualLRP(logger, actualLRPKey1)
			Expect(err).NotTo(HaveOccurred())

			fakeClock.Increment(time.Hour)

			_, _, err = sqlDB.ClaimActualLRP(logger, actualLRPKey1.ProcessGuid, actualLRPKey1.Index, instanceKey1)
			Expect(err).NotTo(HaveOccurred())
			allActualLRPs = append(allActualLRPs, &models.ActualLRP{
				ActualLRPKey:         *actualLRPKey1,
				ActualLRPInstanceKey: *instanceKey1,
				State:                models.ActualLRPStateClaimed,
				Since:                fakeClock.Now().UnixNano(),
				ModificationTag: models.ModificationTag{
					Epoch: "mod-tag-guid",
					Index: 1,
				},
			})

			actualLRPKey2 := &models.ActualLRPKey{
				ProcessGuid: "guid-2",
				Index:       1,
				Domain:      "domain2",
			}
			instanceKey2 := &models.ActualLRPInstanceKey{
				InstanceGuid: "i-guid2",
				CellId:       "cell1",
			}

			_, err = sqlDB.CreateUnclaimedActualLRP(logger, actualLRPKey2)
			Expect(err).NotTo(HaveOccurred())
			fakeClock.Increment(time.Hour)
			_, _, err = sqlDB.ClaimActualLRP(logger, actualLRPKey2.ProcessGuid, actualLRPKey2.Index, instanceKey2)
			Expect(err).NotTo(HaveOccurred())
			allActualLRPs = append(allActualLRPs, &models.ActualLRP{
				ActualLRPKey:         *actualLRPKey2,
				ActualLRPInstanceKey: *instanceKey2,
				State:                models.ActualLRPStateClaimed,
				Since:                fakeClock.Now().UnixNano(),
				ModificationTag: models.ModificationTag{
					Epoch: "mod-tag-guid",
					Index: 1,
				},
			})

			actualLRPKey3 := &models.ActualLRPKey{
				ProcessGuid: "guid3",
				Index:       1,
				Domain:      "domain1",
			}
			instanceKey3 := &models.ActualLRPInstanceKey{
				InstanceGuid: "i-guid3",
				CellId:       "cell2",
			}
			_, err = sqlDB.CreateUnclaimedActualLRP(logger, actualLRPKey3)
			Expect(err).NotTo(HaveOccurred())
			fakeClock.Increment(time.Hour)
			_, _, err = sqlDB.ClaimActualLRP(logger, actualLRPKey3.ProcessGuid, actualLRPKey3.Index, instanceKey3)
			Expect(err).NotTo(HaveOccurred())
			allActualLRPs = append(allActualLRPs, &models.ActualLRP{
				ActualLRPKey:         *actualLRPKey3,
				ActualLRPInstanceKey: *instanceKey3,
				State:                models.ActualLRPStateClaimed,
				Since:                fakeClock.Now().UnixNano(),
				ModificationTag: models.ModificationTag{
					Epoch: "mod-tag-guid",
					Index: 1,
				},
			})

			actualLRPKey4 := &models.ActualLRPKey{
				ProcessGuid: "guid4",
				Index:       1,
				Domain:      "domain2",
			}
			_, err = sqlDB.CreateUnclaimedActualLRP(logger, actualLRPKey4)
			Expect(err).NotTo(HaveOccurred())
			allActualLRPs = append(allActualLRPs, &models.ActualLRP{
				ActualLRPKey: *actualLRPKey4,
				State:        models.ActualLRPStateUnclaimed,
				Since:        fakeClock.Now().UnixNano(),
				ModificationTag: models.ModificationTag{
					Epoch: "mod-tag-guid",
					Index: 0,
				},
			})

			actualLRPKey5 := &models.ActualLRPKey{
				ProcessGuid: "guid5",
				Index:       1,
				Domain:      "domain2",
			}
			instanceKey5 := &models.ActualLRPInstanceKey{
				InstanceGuid: "i-guid5",
				CellId:       "cell2",
			}
			_, err = sqlDB.CreateUnclaimedActualLRP(logger, actualLRPKey5)
			Expect(err).NotTo(HaveOccurred())
			fakeClock.Increment(time.Hour)
			_, _, err = sqlDB.ClaimActualLRP(logger, actualLRPKey5.ProcessGuid, actualLRPKey5.Index, instanceKey5)
			Expect(err).NotTo(HaveOccurred())
			queryStr := "UPDATE actual_lrps SET presence = ? WHERE process_guid = ? AND instance_index = ? AND presence = ?"
			if test_helpers.UsePostgres() {
				queryStr = test_helpers.ReplaceQuestionMarks(queryStr)
			}
			_, err = db.Exec(queryStr, models.ActualLRP_Evacuating, actualLRPKey5.ProcessGuid, actualLRPKey5.Index, models.ActualLRP_Ordinary)
			Expect(err).NotTo(HaveOccurred())
			allActualLRPs = append(allActualLRPs, &models.ActualLRP{
				ActualLRPKey:         *actualLRPKey5,
				ActualLRPInstanceKey: *instanceKey5,
				State:                models.ActualLRPStateClaimed,
				Since:                fakeClock.Now().UnixNano(),
				ModificationTag: models.ModificationTag{
					Epoch: "mod-tag-guid",
					Index: 1,
				},
				Presence: models.ActualLRP_Evacuating,
			})

			actualLRPKey6 := &models.ActualLRPKey{
				ProcessGuid: "guid6",
				Index:       2,
				Domain:      "domain1",
			}
			instanceKey6 := &models.ActualLRPInstanceKey{
				InstanceGuid: "i-guid6",
				CellId:       "cell2",
			}
			fakeClock.Increment(time.Hour)
			_, err = sqlDB.CreateUnclaimedActualLRP(logger, actualLRPKey6)
			Expect(err).NotTo(HaveOccurred())
			_, _, err = sqlDB.ClaimActualLRP(logger, actualLRPKey6.ProcessGuid, actualLRPKey6.Index, instanceKey6)
			Expect(err).NotTo(HaveOccurred())
			netInfo := models.ActualLRPNetInfo{
				Address:         "0.0.0.0",
				InstanceAddress: "1.1.1.1",
			}
			_, _, err = sqlDB.StartActualLRP(logger, actualLRPKey6, instanceKey6, &netInfo)
			Expect(err).NotTo(HaveOccurred())
			queryStr = "UPDATE actual_lrps SET presence = ? WHERE process_guid = ? AND instance_index = ? AND presence = ?"
			if test_helpers.UsePostgres() {
				queryStr = test_helpers.ReplaceQuestionMarks(queryStr)
			}
			_, err = db.Exec(queryStr, models.ActualLRP_Suspect, actualLRPKey6.ProcessGuid, actualLRPKey6.Index, models.ActualLRP_Ordinary)
			Expect(err).NotTo(HaveOccurred())

			_, err = sqlDB.CreateUnclaimedActualLRP(logger, actualLRPKey6)
			Expect(err).NotTo(HaveOccurred())
			_, _, err = sqlDB.ClaimActualLRP(logger, actualLRPKey6.ProcessGuid, actualLRPKey6.Index, instanceKey6)
			Expect(err).NotTo(HaveOccurred())

			Expect(err).NotTo(HaveOccurred())
			allActualLRPs = append(allActualLRPs, &models.ActualLRP{
				ActualLRPKey:         *actualLRPKey6,
				ActualLRPInstanceKey: *instanceKey6,
				ActualLRPNetInfo:     netInfo,
				State:                models.ActualLRPStateRunning,
				Since:                fakeClock.Now().UnixNano(),
				ModificationTag: models.ModificationTag{
					Epoch: "mod-tag-guid",
					Index: 2,
				},
				Presence: models.ActualLRP_Suspect,
			})
			allActualLRPs = append(allActualLRPs, &models.ActualLRP{
				ActualLRPKey:         *actualLRPKey6,
				ActualLRPInstanceKey: *instanceKey6,
				State:                models.ActualLRPStateClaimed,
				Since:                fakeClock.Now().UnixNano(),
				ModificationTag: models.ModificationTag{
					Epoch: "mod-tag-guid",
					Index: 1,
				},
			})
		})

		It("returns all the actual lrps", func() {
			actualLRPs, err := sqlDB.ActualLRPs(logger, models.ActualLRPFilter{})
			Expect(err).NotTo(HaveOccurred())
			Expect(actualLRPs).To(ConsistOf(allActualLRPs))
		})

		It("prunes all actual lrps containing invalid data", func() {
			actualLRPWithInvalidData := model_helpers.NewValidActualLRP("invalid", 0)
			_, _, err := sqlDB.StartActualLRP(logger, &actualLRPWithInvalidData.ActualLRPKey, &actualLRPWithInvalidData.ActualLRPInstanceKey, &actualLRPWithInvalidData.ActualLRPNetInfo)
			Expect(err).NotTo(HaveOccurred())
			queryStr := `UPDATE actual_lrps SET net_info = 'garbage' WHERE process_guid = 'invalid'`
			if test_helpers.UsePostgres() {
				queryStr = test_helpers.ReplaceQuestionMarks(queryStr)
			}
			_, err = db.Exec(queryStr)
			Expect(err).NotTo(HaveOccurred())

			actualLRPs, err := sqlDB.ActualLRPs(logger, models.ActualLRPFilter{})
			Expect(err).NotTo(HaveOccurred())

			Expect(actualLRPs).NotTo(ContainElement(actualLRPWithInvalidData))
		})

		Context("when filtering on domains", func() {
			It("returns the actual lrps in the domain", func() {
				filter := models.ActualLRPFilter{
					Domain: "domain2",
				}
				actualLRPs, err := sqlDB.ActualLRPs(logger, filter)
				Expect(err).NotTo(HaveOccurred())
				Expect(actualLRPs).To(ConsistOf(allActualLRPs[1], allActualLRPs[3], allActualLRPs[4]))
			})
		})

		Context("when filtering on cell", func() {
			It("returns the actual lrps claimed by the cell", func() {
				filter := models.ActualLRPFilter{
					CellID: "cell1",
				}
				actualLRPs, err := sqlDB.ActualLRPs(logger, filter)
				Expect(err).NotTo(HaveOccurred())
				Expect(actualLRPs).To(ConsistOf(allActualLRPs[0], allActualLRPs[1]))
			})
		})

		Context("when filtering on process GUID", func() {
			It("returns the actual lrps with the matching process GUID", func() {
				filter := models.ActualLRPFilter{
					ProcessGuid: "guid6",
				}
				actualLRPs, err := sqlDB.ActualLRPs(logger, filter)
				Expect(err).NotTo(HaveOccurred())
				Expect(actualLRPs).To(ConsistOf(allActualLRPs[5], allActualLRPs[6]))
			})
		})

		Context("when filtering on instance index", func() {
			It("returns the actual lrps with the matching index", func() {
				index := int32(1)
				filter := models.ActualLRPFilter{
					Index: &index,
				}
				actualLRPs, err := sqlDB.ActualLRPs(logger, filter)
				Expect(err).NotTo(HaveOccurred())
				Expect(actualLRPs).To(ConsistOf(
					allActualLRPs[0],
					allActualLRPs[1],
					allActualLRPs[2],
					allActualLRPs[3],
					allActualLRPs[4],
				))
			})
		})

		Context("when filtering on multiple fields", func() {
			It("returns the actual lrps that match all the filters", func() {
				index := int32(1)
				filter := models.ActualLRPFilter{
					Domain:      "domain1",
					CellID:      "cell2",
					ProcessGuid: "guid3",
					Index:       &index,
				}
				actualLRPs, err := sqlDB.ActualLRPs(logger, filter)
				Expect(err).NotTo(HaveOccurred())
				Expect(actualLRPs).To(ConsistOf(allActualLRPs[2]))
			})
		})

		Context("when the filter does not match any ActualLRPs", func() {
			var filter models.ActualLRPFilter

			BeforeEach(func() {
				filter = models.ActualLRPFilter{
					Domain:      "domain1-that-doesnt-exist",
					CellID:      "cell2",
					ProcessGuid: "guid3",
				}
			})

			Context("without a specific filter defined", func() {
				It("returns an empty list", func() {
					actualLRPs, err := sqlDB.ActualLRPs(logger, filter)
					Expect(err).NotTo(HaveOccurred())
					Expect(actualLRPs).To(BeEmpty())
				})
			})

			Context("if the index is set", func() {
				BeforeEach(func() {
					index := int32(1)
					filter.Index = &index
				})

				It("returns an empty array", func() {
					actualLRPs, err := sqlDB.ActualLRPs(logger, filter)
					Expect(err).NotTo(HaveOccurred())
					Expect(actualLRPs).To(BeEmpty())
				})
			})
		})
	})

	Describe("ClaimActualLRP", func() {
		var instanceKey *models.ActualLRPInstanceKey

		BeforeEach(func() {
			instanceKey = &models.ActualLRPInstanceKey{
				InstanceGuid: "the-instance-guid",
				CellId:       "the-cell-id",
			}
		})

		Context("when the actual lrp exists", func() {
			var expectedActualLRP *models.ActualLRP
			var lrpCreationTime time.Time

			BeforeEach(func() {
				expectedActualLRP = &models.ActualLRP{
					ActualLRPKey: models.ActualLRPKey{
						ProcessGuid: "the-guid",
						Index:       1,
						Domain:      "the-domain",
					},
					ModificationTag: models.ModificationTag{
						Epoch: "my-awesome-guid",
						Index: 0,
					},
				}
				_, err := sqlDB.CreateUnclaimedActualLRP(logger, &expectedActualLRP.ActualLRPKey)
				Expect(err).NotTo(HaveOccurred())
				lrpCreationTime = fakeClock.Now()
				fakeClock.Increment(time.Hour)
			})

			Context("and the actual lrp is UNCLAIMED", func() {
				It("claims the actual lrp", func() {
					_, _, err := sqlDB.ClaimActualLRP(logger, expectedActualLRP.ProcessGuid, expectedActualLRP.Index, instanceKey)
					Expect(err).NotTo(HaveOccurred())

					expectedActualLRP.State = models.ActualLRPStateClaimed
					expectedActualLRP.ActualLRPInstanceKey = *instanceKey
					expectedActualLRP.ModificationTag.Increment()
					expectedActualLRP.Since = fakeClock.Now().UnixNano()

					actualLRPs, err := sqlDB.ActualLRPs(logger, models.ActualLRPFilter{ProcessGuid: expectedActualLRP.ProcessGuid, Index: &expectedActualLRP.Index})
					Expect(err).NotTo(HaveOccurred())
					Expect(actualLRPs).To(ConsistOf(expectedActualLRP))
				})

				It("returns the existing actual lrp", func() {
					beforeActualLRP, afterActualLRP, err := sqlDB.ClaimActualLRP(logger, expectedActualLRP.ProcessGuid, expectedActualLRP.Index, instanceKey)
					Expect(err).NotTo(HaveOccurred())

					expectedActualLRP.State = models.ActualLRPStateUnclaimed
					expectedActualLRP.Since = lrpCreationTime.UnixNano()
					Expect(beforeActualLRP).To(Equal(expectedActualLRP))

					actualLRPs, err := sqlDB.ActualLRPs(logger, models.ActualLRPFilter{ProcessGuid: expectedActualLRP.ProcessGuid, Index: &expectedActualLRP.Index})
					Expect(err).NotTo(HaveOccurred())
					Expect(actualLRPs).To(ConsistOf(afterActualLRP))
				})

				Context("and there is a placement error", func() {
					BeforeEach(func() {
						queryStr := `
						UPDATE actual_lrps SET placement_error = ?
						WHERE process_guid = ? AND instance_index = ?`
						if test_helpers.UsePostgres() {
							queryStr = test_helpers.ReplaceQuestionMarks(queryStr)
						}
						_, err := db.Exec(queryStr,
							"i am placement errror, how are you?",
							expectedActualLRP.ProcessGuid,
							expectedActualLRP.Index,
						)
						Expect(err).NotTo(HaveOccurred())
					})

					It("clears the placement error", func() {
						_, _, err := sqlDB.ClaimActualLRP(logger, expectedActualLRP.ProcessGuid, expectedActualLRP.Index, instanceKey)
						Expect(err).NotTo(HaveOccurred())

						expectedActualLRP.State = models.ActualLRPStateClaimed
						expectedActualLRP.ActualLRPInstanceKey = *instanceKey
						expectedActualLRP.ModificationTag.Increment()
						expectedActualLRP.Since = fakeClock.Now().UnixNano()

						actualLRPs, err := sqlDB.ActualLRPs(logger, models.ActualLRPFilter{ProcessGuid: expectedActualLRP.ProcessGuid, Index: &expectedActualLRP.Index})
						Expect(err).NotTo(HaveOccurred())
						Expect(actualLRPs).To(ConsistOf(expectedActualLRP))
					})
				})
			})

			Context("and the actual lrp is CLAIMED", func() {
				Context("when the actual lrp is already claimed with the same instance key", func() {
					BeforeEach(func() {
						_, _, err := sqlDB.ClaimActualLRP(logger, expectedActualLRP.ProcessGuid, expectedActualLRP.Index, instanceKey)
						Expect(err).NotTo(HaveOccurred())
						expectedActualLRP.ModificationTag.Increment()
					})

					It("does not update the actual lrp", func() {
						expectedActualLRP.State = models.ActualLRPStateClaimed
						expectedActualLRP.ActualLRPInstanceKey = *instanceKey
						expectedActualLRP.Since = fakeClock.Now().UnixNano()

						fakeClock.Increment(time.Hour)

						beforeActualLRP, afterActualLRP, err := sqlDB.ClaimActualLRP(logger, expectedActualLRP.ProcessGuid, expectedActualLRP.Index, instanceKey)
						Expect(err).NotTo(HaveOccurred())

						Expect(beforeActualLRP).To(Equal(afterActualLRP))
						Expect(afterActualLRP).To(Equal(expectedActualLRP))

						actualLRPs, err := sqlDB.ActualLRPs(logger, models.ActualLRPFilter{ProcessGuid: expectedActualLRP.ProcessGuid, Index: &expectedActualLRP.Index})
						Expect(err).NotTo(HaveOccurred())
						Expect(actualLRPs).To(ConsistOf(expectedActualLRP))
					})
				})

				Context("when the actual lrp is claimed by another cell", func() {
					BeforeEach(func() {
						_, _, err := sqlDB.ClaimActualLRP(logger, expectedActualLRP.ProcessGuid, expectedActualLRP.Index, instanceKey)
						Expect(err).NotTo(HaveOccurred())

						actualLRPs, err := sqlDB.ActualLRPs(logger, models.ActualLRPFilter{ProcessGuid: expectedActualLRP.ProcessGuid, Index: &expectedActualLRP.Index})
						Expect(actualLRPs).To(HaveLen(1))
						expectedActualLRP = actualLRPs[0]
					})

					It("returns an error", func() {
						instanceKey = &models.ActualLRPInstanceKey{
							InstanceGuid: "different-instance",
							CellId:       "different-cell",
						}

						_, _, err := sqlDB.ClaimActualLRP(logger, expectedActualLRP.ProcessGuid, expectedActualLRP.Index, instanceKey)
						Expect(err).To(HaveOccurred())

						actualLRPs, err := sqlDB.ActualLRPs(logger, models.ActualLRPFilter{ProcessGuid: expectedActualLRP.ProcessGuid, Index: &expectedActualLRP.Index})
						Expect(err).NotTo(HaveOccurred())
						Expect(actualLRPs).To(ConsistOf(expectedActualLRP))
					})
				})
			})

			Context("and the actual lrp is RUNNING", func() {
				BeforeEach(func() {
					netInfo := models.ActualLRPNetInfo{
						Address:         "0.0.0.0",
						Ports:           []*models.PortMapping{},
						InstanceAddress: "1.1.1.1",
					}

					expectedActualLRP.ModificationTag.Increment()

					_, _, err := sqlDB.StartActualLRP(logger, &models.ActualLRPKey{
						ProcessGuid: expectedActualLRP.ProcessGuid,
						Index:       expectedActualLRP.Index,
						Domain:      expectedActualLRP.Domain,
					}, &models.ActualLRPInstanceKey{
						InstanceGuid: instanceKey.InstanceGuid,
						CellId:       instanceKey.CellId,
					},
						&netInfo)
					Expect(err).NotTo(HaveOccurred())
				})

				Context("with the same cell and instance guid", func() {
					It("reverts the RUNNING actual lrp to the CLAIMED state", func() {
						_, _, err := sqlDB.ClaimActualLRP(logger, expectedActualLRP.ProcessGuid, expectedActualLRP.Index, instanceKey)
						Expect(err).NotTo(HaveOccurred())

						actualLRPs, err := sqlDB.ActualLRPs(logger, models.ActualLRPFilter{ProcessGuid: expectedActualLRP.ProcessGuid, Index: &expectedActualLRP.Index})
						Expect(err).NotTo(HaveOccurred())

						expectedActualLRP.ActualLRPInstanceKey = *instanceKey
						expectedActualLRP.State = models.ActualLRPStateClaimed
						expectedActualLRP.Since = fakeClock.Now().UnixNano()
						expectedActualLRP.ModificationTag.Increment()
						Expect(actualLRPs).To(ConsistOf(expectedActualLRP))
					})
				})

				Context("with a different cell id", func() {
					BeforeEach(func() {
						instanceKey.CellId = "another-cell"

						actualLRPs, err := sqlDB.ActualLRPs(logger, models.ActualLRPFilter{ProcessGuid: expectedActualLRP.ProcessGuid, Index: &expectedActualLRP.Index})
						Expect(err).NotTo(HaveOccurred())
						Expect(actualLRPs).To(HaveLen(1))
						expectedActualLRP = actualLRPs[0]
					})

					It("returns an error", func() {
						_, _, err := sqlDB.ClaimActualLRP(logger, expectedActualLRP.ProcessGuid, expectedActualLRP.Index, instanceKey)
						Expect(err).To(HaveOccurred())

						actualLRPs, err := sqlDB.ActualLRPs(logger, models.ActualLRPFilter{ProcessGuid: expectedActualLRP.ProcessGuid, Index: &expectedActualLRP.Index})
						Expect(err).NotTo(HaveOccurred())
						Expect(actualLRPs).To(ConsistOf(expectedActualLRP))
					})
				})

				Context("with a different instance guid", func() {
					BeforeEach(func() {
						instanceKey.InstanceGuid = "another-instance-guid"

						actualLRPs, err := sqlDB.ActualLRPs(logger, models.ActualLRPFilter{ProcessGuid: expectedActualLRP.ProcessGuid, Index: &expectedActualLRP.Index})
						Expect(err).NotTo(HaveOccurred())
						Expect(actualLRPs).To(HaveLen(1))
						expectedActualLRP = actualLRPs[0]
					})

					It("returns an error", func() {
						_, _, err := sqlDB.ClaimActualLRP(logger, expectedActualLRP.ProcessGuid, expectedActualLRP.Index, instanceKey)
						Expect(err).To(HaveOccurred())

						actualLRPs, err := sqlDB.ActualLRPs(logger, models.ActualLRPFilter{ProcessGuid: expectedActualLRP.ProcessGuid, Index: &expectedActualLRP.Index})
						Expect(err).NotTo(HaveOccurred())
						Expect(actualLRPs).To(ConsistOf(expectedActualLRP))
					})
				})
			})

			Context("and the actual lrp is CRASHED", func() {
				BeforeEach(func() {
					queryStr := `
						UPDATE actual_lrps SET state = ?
						WHERE process_guid = ? AND instance_index = ?`
					if test_helpers.UsePostgres() {
						queryStr = test_helpers.ReplaceQuestionMarks(queryStr)
					}
					_, err := db.Exec(queryStr,
						models.ActualLRPStateCrashed,
						expectedActualLRP.ProcessGuid,
						expectedActualLRP.Index,
					)
					Expect(err).NotTo(HaveOccurred())
				})

				It("returns an error", func() {
					_, _, err := sqlDB.ClaimActualLRP(logger, expectedActualLRP.ProcessGuid, expectedActualLRP.Index, instanceKey)
					Expect(err).To(HaveOccurred())

					actualLRPs, err := sqlDB.ActualLRPs(logger, models.ActualLRPFilter{ProcessGuid: expectedActualLRP.ProcessGuid, Index: &expectedActualLRP.Index})
					Expect(err).NotTo(HaveOccurred())
					Expect(actualLRPs).To(HaveLen(1))
					Expect(actualLRPs[0].State).To(Equal(models.ActualLRPStateCrashed))
				})
			})

			Context("and the actual lrp is evacuating", func() {
				BeforeEach(func() {
					queryStr := "UPDATE actual_lrps SET presence = ? WHERE process_guid = ? AND instance_index = ? AND presence = ?"
					if test_helpers.UsePostgres() {
						queryStr = test_helpers.ReplaceQuestionMarks(queryStr)
					}
					_, err := db.Exec(queryStr,
						models.ActualLRP_Evacuating,
						expectedActualLRP.ActualLRPKey.ProcessGuid,
						expectedActualLRP.ActualLRPKey.Index,
						models.ActualLRP_Ordinary,
					)
					Expect(err).NotTo(HaveOccurred())

					expectedActualLRP.State = models.ActualLRPStateUnclaimed
					expectedActualLRP.Since = lrpCreationTime.UnixNano()
					expectedActualLRP.Presence = models.ActualLRP_Evacuating
				})

				It("returns an error", func() {
					_, _, err := sqlDB.ClaimActualLRP(logger, expectedActualLRP.ProcessGuid, expectedActualLRP.Index, instanceKey)
					Expect(err).To(HaveOccurred())
					Expect(err).To(Equal(models.ErrResourceNotFound))

					actualLRPs, err := sqlDB.ActualLRPs(logger, models.ActualLRPFilter{ProcessGuid: expectedActualLRP.ProcessGuid, Index: &expectedActualLRP.Index})
					Expect(err).NotTo(HaveOccurred())
					Expect(actualLRPs).To(ConsistOf(expectedActualLRP))
				})
			})
		})

		Context("when the actual lrp does not exist", func() {
			BeforeEach(func() {
				key := models.ActualLRPKey{
					ProcessGuid: "the-right-guid",
					Index:       1,
					Domain:      "the-domain",
				}
				_, err := sqlDB.CreateUnclaimedActualLRP(logger, &key)
				Expect(err).NotTo(HaveOccurred())

				key = models.ActualLRPKey{
					ProcessGuid: "the-wrong-guid",
					Index:       0,
					Domain:      "the-domain",
				}
				_, err = sqlDB.CreateUnclaimedActualLRP(logger, &key)
				Expect(err).NotTo(HaveOccurred())
			})

			It("returns a ResourceNotFound error", func() {
				_, _, err := sqlDB.ClaimActualLRP(logger, "i-do-not-exist", 1, instanceKey)
				Expect(err).To(Equal(models.ErrResourceNotFound))
			})
		})
	})

	Describe("StartActualLRP", func() {
		Context("when the actual lrp exists", func() {
			var (
				instanceKey *models.ActualLRPInstanceKey
				netInfo     *models.ActualLRPNetInfo
				actualLRP   *models.ActualLRP
			)

			BeforeEach(func() {
				instanceKey = &models.ActualLRPInstanceKey{
					InstanceGuid: "the-instance-guid",
					CellId:       "the-cell-id",
				}

				netInfo = &models.ActualLRPNetInfo{
					Address:         "1.2.1.2",
					Ports:           []*models.PortMapping{{ContainerPort: 8080, HostPort: 9090}},
					InstanceAddress: "2.2.2.2",
				}

				actualLRP = &models.ActualLRP{
					ActualLRPKey: models.ActualLRPKey{
						ProcessGuid: "the-guid",
						Index:       1,
						Domain:      "the-domain",
					},
				}
				_, err := sqlDB.CreateUnclaimedActualLRP(logger, &actualLRP.ActualLRPKey)
				Expect(err).NotTo(HaveOccurred())
				fakeClock.Increment(time.Hour)
			})

			Context("and the actual lrp is UNCLAIMED", func() {
				It("transitions the state to RUNNING", func() {
					_, _, err := sqlDB.StartActualLRP(logger, &actualLRP.ActualLRPKey, instanceKey, netInfo)
					Expect(err).NotTo(HaveOccurred())

					actualLRPs, err := sqlDB.ActualLRPs(logger, models.ActualLRPFilter{ProcessGuid: actualLRP.ProcessGuid, Index: &actualLRP.Index})
					Expect(err).NotTo(HaveOccurred())

					expectedActualLRP := *actualLRP
					expectedActualLRP.ActualLRPInstanceKey = *instanceKey
					expectedActualLRP.State = models.ActualLRPStateRunning
					expectedActualLRP.ActualLRPNetInfo = *netInfo
					expectedActualLRP.Since = fakeClock.Now().UnixNano()
					expectedActualLRP.ModificationTag = models.ModificationTag{
						Epoch: "my-awesome-guid",
						Index: 1,
					}

					Expect(actualLRPs).To(ConsistOf(&expectedActualLRP))
				})

				It("returns the existing actual lrp", func() {
					beforeActualLRP, afterActualLRP, err := sqlDB.StartActualLRP(logger, &actualLRP.ActualLRPKey, instanceKey, netInfo)
					Expect(err).NotTo(HaveOccurred())
					expectedActualLRP := *actualLRP
					expectedActualLRP.State = models.ActualLRPStateUnclaimed
					expectedActualLRP.Since = fakeClock.Now().Add(-time.Hour).UnixNano()
					expectedActualLRP.ModificationTag = models.ModificationTag{
						Epoch: "my-awesome-guid",
						Index: 0,
					}
					Expect(beforeActualLRP).To(Equal(&expectedActualLRP))

					actualLRPs, err := sqlDB.ActualLRPs(logger, models.ActualLRPFilter{ProcessGuid: actualLRP.ProcessGuid, Index: &actualLRP.Index})
					Expect(err).NotTo(HaveOccurred())
					Expect(actualLRPs).To(ConsistOf(afterActualLRP))
				})
			})

			Context("and the actual lrp has been CLAIMED", func() {
				BeforeEach(func() {
					_, _, err := sqlDB.ClaimActualLRP(logger, actualLRP.ProcessGuid, actualLRP.Index, instanceKey)
					Expect(err).NotTo(HaveOccurred())
					fakeClock.Increment(time.Hour)
				})

				It("transitions the state to RUNNING", func() {
					_, _, err := sqlDB.StartActualLRP(logger, &actualLRP.ActualLRPKey, instanceKey, netInfo)
					Expect(err).NotTo(HaveOccurred())

					actualLRPs, err := sqlDB.ActualLRPs(logger, models.ActualLRPFilter{ProcessGuid: actualLRP.ProcessGuid, Index: &actualLRP.Index})
					Expect(err).NotTo(HaveOccurred())

					expectedActualLRP := *actualLRP
					expectedActualLRP.ActualLRPInstanceKey = *instanceKey
					expectedActualLRP.State = models.ActualLRPStateRunning
					expectedActualLRP.ActualLRPNetInfo = *netInfo
					expectedActualLRP.Since = fakeClock.Now().UnixNano()
					expectedActualLRP.ModificationTag = models.ModificationTag{
						Epoch: "my-awesome-guid",
						Index: 2,
					}

					Expect(actualLRPs).To(ConsistOf(&expectedActualLRP))
				})

				It("returns the existing actual lrp", func() {
					beforeActualLRP, afterActualLRP, err := sqlDB.StartActualLRP(logger, &actualLRP.ActualLRPKey, instanceKey, netInfo)
					Expect(err).NotTo(HaveOccurred())

					expectedActualLRP := *actualLRP
					expectedActualLRP.ActualLRPInstanceKey = *instanceKey
					expectedActualLRP.State = models.ActualLRPStateClaimed
					// claim doesn't set since
					expectedActualLRP.Since = fakeClock.Now().Add(-time.Hour).UnixNano()
					expectedActualLRP.ModificationTag = models.ModificationTag{
						Epoch: "my-awesome-guid",
						Index: 1,
					}

					actualLRPs, err := sqlDB.ActualLRPs(logger, models.ActualLRPFilter{ProcessGuid: actualLRP.ProcessGuid, Index: &actualLRP.Index})
					Expect(err).NotTo(HaveOccurred())

					Expect(beforeActualLRP).To(Equal(&expectedActualLRP))
					Expect(actualLRPs).To(ConsistOf(afterActualLRP))
				})

				Context("and the instance key is different", func() {
					It("transitions the state to RUNNING, updating the instance key", func() {
						otherInstanceKey := &models.ActualLRPInstanceKey{CellId: "some-other-cell", InstanceGuid: "some-other-instance-guid"}
						_, _, err := sqlDB.StartActualLRP(logger, &actualLRP.ActualLRPKey, otherInstanceKey, netInfo)
						Expect(err).NotTo(HaveOccurred())

						actualLRPs, err := sqlDB.ActualLRPs(logger, models.ActualLRPFilter{ProcessGuid: actualLRP.ProcessGuid, Index: &actualLRP.Index})
						Expect(err).NotTo(HaveOccurred())

						expectedActualLRP := *actualLRP
						expectedActualLRP.ActualLRPInstanceKey = *otherInstanceKey
						expectedActualLRP.State = models.ActualLRPStateRunning
						expectedActualLRP.ActualLRPNetInfo = *netInfo
						expectedActualLRP.Since = fakeClock.Now().UnixNano()
						expectedActualLRP.ModificationTag = models.ModificationTag{
							Epoch: "my-awesome-guid",
							Index: 2,
						}

						Expect(actualLRPs).To(ConsistOf(&expectedActualLRP))
					})
				})

				Context("and the actual lrp is RUNNING", func() {
					BeforeEach(func() {
						_, _, err := sqlDB.StartActualLRP(logger, &actualLRP.ActualLRPKey, instanceKey, netInfo)
						Expect(err).NotTo(HaveOccurred())
					})

					Context("and the instance key is the same", func() {
						Context("and the net info is the same", func() {
							It("does nothing", func() {
								beforeActualLRPs, err := sqlDB.ActualLRPs(logger, models.ActualLRPFilter{ProcessGuid: actualLRP.ProcessGuid, Index: &actualLRP.Index})
								Expect(err).NotTo(HaveOccurred())

								_, _, err = sqlDB.StartActualLRP(logger, &actualLRP.ActualLRPKey, instanceKey, netInfo)
								Expect(err).NotTo(HaveOccurred())

								afterActualLRPs, err := sqlDB.ActualLRPs(logger, models.ActualLRPFilter{ProcessGuid: actualLRP.ProcessGuid, Index: &actualLRP.Index})
								Expect(err).NotTo(HaveOccurred())

								Expect(beforeActualLRPs).To(BeEquivalentTo(afterActualLRPs))
							})

							It("returns the same actual lrp group for before and after", func() {
								beforeActualLRP, afterActualLRP, err := sqlDB.StartActualLRP(logger, &actualLRP.ActualLRPKey, instanceKey, netInfo)
								Expect(err).NotTo(HaveOccurred())
								Expect(beforeActualLRP).To(Equal(afterActualLRP))
							})
						})

						Context("and the net info is NOT the same", func() {
							var (
								expectedActualLRPs []*models.ActualLRP
								newNetInfo         *models.ActualLRPNetInfo
							)

							BeforeEach(func() {
								var err error
								expectedActualLRPs, err = sqlDB.ActualLRPs(logger, models.ActualLRPFilter{ProcessGuid: actualLRP.ProcessGuid, Index: &actualLRP.Index})
								Expect(err).NotTo(HaveOccurred())
								newNetInfo = &models.ActualLRPNetInfo{Address: "some-other-address"}
							})

							It("updates the net info", func() {
								beforeActualLRP, afterActualLRP, err := sqlDB.StartActualLRP(logger, &actualLRP.ActualLRPKey, instanceKey, newNetInfo)
								Expect(err).NotTo(HaveOccurred())

								Expect(expectedActualLRPs).To(ConsistOf(beforeActualLRP))

								actualLRPs, err := sqlDB.ActualLRPs(logger, models.ActualLRPFilter{ProcessGuid: actualLRP.ProcessGuid, Index: &actualLRP.Index})
								Expect(err).NotTo(HaveOccurred())

								expectedActualLRPs[0].ActualLRPNetInfo = *newNetInfo
								expectedActualLRPs[0].ModificationTag.Increment()
								Expect(actualLRPs).To(BeEquivalentTo(expectedActualLRPs))
								Expect(actualLRPs).To(ConsistOf(afterActualLRP))
							})
						})
					})

					Context("and the instance key is not the same", func() {
						It("returns an ErrActualLRPCannotBeStarted", func() {
							_, _, err := sqlDB.StartActualLRP(logger, &actualLRP.ActualLRPKey, &models.ActualLRPInstanceKey{CellId: "some-other-cell", InstanceGuid: "some-other-instance-guid"}, netInfo)
							Expect(err).To(Equal(models.ErrActualLRPCannotBeStarted))
						})
					})
				})

				Context("and the actual lrp is CRASHED", func() {
					BeforeEach(func() {
						queryStr := `
						UPDATE actual_lrps SET state = ?
						WHERE process_guid = ? AND instance_index = ?`
						if test_helpers.UsePostgres() {
							queryStr = test_helpers.ReplaceQuestionMarks(queryStr)
						}
						_, err := db.Exec(queryStr,
							models.ActualLRPStateCrashed,
							actualLRP.ProcessGuid,
							actualLRP.Index,
						)
						Expect(err).NotTo(HaveOccurred())
					})

					It("transitions the state to RUNNING", func() {
						beforeActualLRP, afterActualLRP, err := sqlDB.StartActualLRP(logger, &actualLRP.ActualLRPKey, instanceKey, netInfo)
						Expect(err).NotTo(HaveOccurred())

						Expect(err).NotTo(HaveOccurred())
						expectedBeforeActualLRP := *actualLRP
						expectedBeforeActualLRP.ActualLRPInstanceKey = *instanceKey
						expectedBeforeActualLRP.State = models.ActualLRPStateCrashed
						// we crash directly in the DB, so no since
						expectedBeforeActualLRP.Since = fakeClock.Now().Add(-time.Hour).UnixNano()
						expectedBeforeActualLRP.ModificationTag = models.ModificationTag{
							Epoch: "my-awesome-guid",
							Index: 1,
						}
						Expect(beforeActualLRP).To(Equal(&expectedBeforeActualLRP))

						fetchedActualLRPs, err := sqlDB.ActualLRPs(logger, models.ActualLRPFilter{ProcessGuid: actualLRP.ProcessGuid, Index: &actualLRP.Index})
						Expect(err).NotTo(HaveOccurred())

						expectedAfterActualLRP := *actualLRP
						expectedAfterActualLRP.ActualLRPInstanceKey = *instanceKey
						expectedAfterActualLRP.State = models.ActualLRPStateRunning
						expectedAfterActualLRP.ActualLRPNetInfo = *netInfo
						expectedAfterActualLRP.Since = fakeClock.Now().UnixNano()
						expectedAfterActualLRP.ModificationTag = models.ModificationTag{
							Epoch: "my-awesome-guid",
							Index: 2,
						}

						Expect(fetchedActualLRPs).To(ContainElement(afterActualLRP))
						Expect(fetchedActualLRPs).To(ContainElement(&expectedAfterActualLRP))
					})
				})
			})
		})

		Context("when the actual lrp does not exist", func() {
			var (
				instanceKey *models.ActualLRPInstanceKey
				netInfo     *models.ActualLRPNetInfo
				actualLRP   *models.ActualLRP
			)

			BeforeEach(func() {
				instanceKey = &models.ActualLRPInstanceKey{
					InstanceGuid: "the-instance-guid",
					CellId:       "the-cell-id",
				}

				netInfo = &models.ActualLRPNetInfo{
					Address: "1.2.1.2",
					Ports:   []*models.PortMapping{{ContainerPort: 8080, HostPort: 9090}},
				}

				actualLRP = &models.ActualLRP{
					ActualLRPKey: models.ActualLRPKey{
						ProcessGuid: "the-guid",
						Index:       1,
						Domain:      "the-domain",
					},
					ModificationTag: models.ModificationTag{
						Epoch: "my-awesome-guid",
						Index: 0,
					},
				}
			})

			It("creates the actual lrp", func() {
				beforeActualLRP, afterActualLRP, err := sqlDB.StartActualLRP(logger, &actualLRP.ActualLRPKey, instanceKey, netInfo)
				Expect(err).NotTo(HaveOccurred())
				Expect(beforeActualLRP).To(Equal(&models.ActualLRP{}))

				fetchedActualLRPs, err := sqlDB.ActualLRPs(logger, models.ActualLRPFilter{ProcessGuid: actualLRP.ProcessGuid, Index: &actualLRP.Index})
				Expect(err).NotTo(HaveOccurred())

				expectedActualLRP := *actualLRP
				expectedActualLRP.State = models.ActualLRPStateRunning
				expectedActualLRP.ActualLRPNetInfo = *netInfo
				expectedActualLRP.ActualLRPInstanceKey = *instanceKey
				expectedActualLRP.Since = fakeClock.Now().UnixNano()

				Expect(fetchedActualLRPs).To(ContainElement(afterActualLRP))
				Expect(fetchedActualLRPs).To(ContainElement(&expectedActualLRP))
			})

			Context("when there is only an evacuating actual LRP", func() {
				BeforeEach(func() {
					_, err := sqlDB.CreateUnclaimedActualLRP(logger, &actualLRP.ActualLRPKey)
					Expect(err).NotTo(HaveOccurred())
					queryStr := "UPDATE actual_lrps SET presence = ? WHERE process_guid = ? AND instance_index = ? AND presence = ?"
					if test_helpers.UsePostgres() {
						queryStr = test_helpers.ReplaceQuestionMarks(queryStr)
					}
					_, err = db.Exec(queryStr,
						models.ActualLRP_Evacuating,
						actualLRP.ActualLRPKey.ProcessGuid,
						actualLRP.ActualLRPKey.Index,
						models.ActualLRP_Ordinary,
					)
					Expect(err).NotTo(HaveOccurred())
				})

				It("creates a new actual LRP", func() {
					beforeActualLRP, afterActualLRP, err := sqlDB.StartActualLRP(logger, &actualLRP.ActualLRPKey, instanceKey, netInfo)
					Expect(err).NotTo(HaveOccurred())
					Expect(beforeActualLRP).To(Equal(&models.ActualLRP{}))

					fetchedActualLRPs, err := sqlDB.ActualLRPs(logger, models.ActualLRPFilter{ProcessGuid: actualLRP.ProcessGuid, Index: &actualLRP.Index})
					Expect(err).NotTo(HaveOccurred())

					expectedActualLRP := *actualLRP
					expectedActualLRP.State = models.ActualLRPStateRunning
					expectedActualLRP.ActualLRPNetInfo = *netInfo
					expectedActualLRP.ActualLRPInstanceKey = *instanceKey
					expectedActualLRP.Since = fakeClock.Now().UnixNano()

					Expect(fetchedActualLRPs).To(ContainElement(afterActualLRP))
					Expect(fetchedActualLRPs).To(ContainElement(&expectedActualLRP))
				})
			})
		})
	})

	Describe("CrashActualLRP", func() {
		Context("when the actual lrp exists", func() {
			var (
				instanceKey *models.ActualLRPInstanceKey
				netInfo     *models.ActualLRPNetInfo
				actualLRP   *models.ActualLRP
			)

			BeforeEach(func() {
				instanceKey = &models.ActualLRPInstanceKey{
					InstanceGuid: "the-instance-guid",
					CellId:       "the-cell-id",
				}

				netInfo = &models.ActualLRPNetInfo{
					Address:         "1.2.1.2",
					Ports:           []*models.PortMapping{{ContainerPort: 8080, HostPort: 9090}},
					InstanceAddress: "2.2.2.2",
				}

				actualLRP = &models.ActualLRP{
					ActualLRPKey: models.ActualLRPKey{
						ProcessGuid: "the-guid",
						Index:       1,
						Domain:      "the-domain",
					},
				}
				_, err := sqlDB.CreateUnclaimedActualLRP(logger, &actualLRP.ActualLRPKey)
				Expect(err).NotTo(HaveOccurred())
				actualLRP.ModificationTag.Epoch = "my-awesome-guid"
				fakeClock.Increment(time.Hour)
			})

			Context("and it is RUNNING", func() {
				BeforeEach(func() {
					_, _, err := sqlDB.StartActualLRP(logger, &actualLRP.ActualLRPKey, instanceKey, netInfo)
					Expect(err).NotTo(HaveOccurred())
					actualLRP.ModificationTag.Increment()
				})

				It("returns the before and after actual lrps", func() {
					beforeActualLRP, afterActualLRP, _, err := sqlDB.CrashActualLRP(logger, &actualLRP.ActualLRPKey, instanceKey, "because it didn't go well")
					Expect(err).NotTo(HaveOccurred())

					expectedActualLRP := *actualLRP
					expectedActualLRP.State = models.ActualLRPStateRunning
					expectedActualLRP.ActualLRPInstanceKey = *instanceKey
					expectedActualLRP.Since = fakeClock.Now().UnixNano()
					expectedActualLRP.ActualLRPNetInfo = *netInfo

					Expect(beforeActualLRP).To(Equal(&expectedActualLRP))

					actualLRPs, err := sqlDB.ActualLRPs(logger, models.ActualLRPFilter{ProcessGuid: actualLRP.ProcessGuid, Index: &actualLRP.Index})
					Expect(err).NotTo(HaveOccurred())

					Expect(actualLRPs).To(ConsistOf(afterActualLRP))
				})

				Context("and the crash reason is larger than 1K", func() {
					It("truncates the crash reason", func() {
						crashReason := strings.Repeat("x", 2*1024)
						_, _, _, err := sqlDB.CrashActualLRP(logger, &actualLRP.ActualLRPKey, instanceKey, crashReason)
						Expect(err).NotTo(HaveOccurred())

						actualLRPs, err := sqlDB.ActualLRPs(logger, models.ActualLRPFilter{ProcessGuid: actualLRP.ProcessGuid, Index: &actualLRP.Index})
						Expect(err).NotTo(HaveOccurred())

						expectedActualLRP := *actualLRP
						expectedActualLRP.State = models.ActualLRPStateUnclaimed
						expectedActualLRP.CrashCount = 1
						expectedActualLRP.CrashReason = crashReason[:1013] + "(truncated)"
						expectedActualLRP.ModificationTag.Increment()
						expectedActualLRP.Since = fakeClock.Now().UnixNano()

						Expect(actualLRPs).To(ConsistOf(&expectedActualLRP))
					})
				})

				Context("and it should be restarted", func() {
					It("updates the lrp and sets its state to UNCLAIMED", func() {
						_, _, shouldRestart, err := sqlDB.CrashActualLRP(logger, &actualLRP.ActualLRPKey, instanceKey, "because it didn't go well")
						Expect(err).NotTo(HaveOccurred())
						Expect(shouldRestart).To(BeTrue())

						actualLRPs, err := sqlDB.ActualLRPs(logger, models.ActualLRPFilter{ProcessGuid: actualLRP.ProcessGuid, Index: &actualLRP.Index})
						Expect(err).NotTo(HaveOccurred())

						expectedActualLRP := *actualLRP
						expectedActualLRP.State = models.ActualLRPStateUnclaimed
						expectedActualLRP.CrashCount = 1
						expectedActualLRP.CrashReason = "because it didn't go well"
						expectedActualLRP.ModificationTag.Increment()
						expectedActualLRP.Since = fakeClock.Now().UnixNano()

						Expect(actualLRPs).To(ConsistOf(&expectedActualLRP))
					})
				})

				Context("and it should NOT be restarted", func() {
					BeforeEach(func() {
						queryStr := `
					UPDATE actual_lrps SET crash_count = ?
					WHERE process_guid = ? AND instance_index = ?`
						if test_helpers.UsePostgres() {
							queryStr = test_helpers.ReplaceQuestionMarks(queryStr)
						}
						_, err := db.Exec(queryStr,
							models.DefaultImmediateRestarts+1,
							actualLRP.ProcessGuid,
							actualLRP.Index,
						)
						Expect(err).NotTo(HaveOccurred())
					})

					It("updates the lrp and sets its state to CRASHED", func() {
						_, _, shouldRestart, err := sqlDB.CrashActualLRP(logger, &actualLRP.ActualLRPKey, instanceKey, "because it didn't go well")
						Expect(err).NotTo(HaveOccurred())
						Expect(shouldRestart).To(BeFalse())

						actualLRPs, err := sqlDB.ActualLRPs(logger, models.ActualLRPFilter{ProcessGuid: actualLRP.ProcessGuid, Index: &actualLRP.Index})
						Expect(err).NotTo(HaveOccurred())

						expectedActualLRP := *actualLRP
						expectedActualLRP.State = models.ActualLRPStateCrashed
						expectedActualLRP.CrashCount = models.DefaultImmediateRestarts + 2
						expectedActualLRP.CrashReason = "because it didn't go well"
						expectedActualLRP.ModificationTag.Increment()
						expectedActualLRP.Since = fakeClock.Now().UnixNano()

						Expect(actualLRPs).To(ConsistOf(&expectedActualLRP))
					})

					Context("and it has NOT been updated recently", func() {
						BeforeEach(func() {
							queryStr := `
					UPDATE actual_lrps SET since = ?
					WHERE process_guid = ? AND instance_index = ?`
							if test_helpers.UsePostgres() {
								queryStr = test_helpers.ReplaceQuestionMarks(queryStr)
							}
							_, err := db.Exec(queryStr,
								fakeClock.Now().Add(-(models.CrashResetTimeout + 1*time.Second)).UnixNano(),
								actualLRP.ProcessGuid,
								actualLRP.Index,
							)
							Expect(err).NotTo(HaveOccurred())
						})

						It("resets the crash count to 1", func() {
							_, _, shouldRestart, err := sqlDB.CrashActualLRP(logger, &actualLRP.ActualLRPKey, instanceKey, "because it didn't go well")
							Expect(err).NotTo(HaveOccurred())
							Expect(shouldRestart).To(BeTrue())

							actualLRPs, err := sqlDB.ActualLRPs(logger, models.ActualLRPFilter{ProcessGuid: actualLRP.ProcessGuid, Index: &actualLRP.Index})
							Expect(err).NotTo(HaveOccurred())

							Expect(actualLRPs).To(HaveLen(1))
							Expect(actualLRPs[0].CrashCount).To(BeNumerically("==", 1))
						})
					})
				})
			})

			Context("and it's CLAIMED", func() {
				BeforeEach(func() {
					_, _, err := sqlDB.ClaimActualLRP(logger, actualLRP.ProcessGuid, actualLRP.Index, instanceKey)
					Expect(err).NotTo(HaveOccurred())
					fakeClock.Increment(time.Hour)
					actualLRP.ModificationTag.Increment()
				})

				It("returns the previous and current actual lrp", func() {
					beforeActualLRP, afterActualLRP, _, err := sqlDB.CrashActualLRP(logger, &actualLRP.ActualLRPKey, instanceKey, "because it didn't go well")
					Expect(err).NotTo(HaveOccurred())

					expectedActualLRP := *actualLRP
					expectedActualLRP.State = models.ActualLRPStateClaimed
					expectedActualLRP.ActualLRPInstanceKey = *instanceKey
					expectedActualLRP.Since = fakeClock.Now().Add(-time.Hour).UnixNano()

					Expect(beforeActualLRP).To(Equal(&expectedActualLRP))

					actualLRPs, err := sqlDB.ActualLRPs(logger, models.ActualLRPFilter{ProcessGuid: actualLRP.ProcessGuid, Index: &actualLRP.Index})
					Expect(err).NotTo(HaveOccurred())
					Expect(actualLRPs).To(ConsistOf(afterActualLRP))
				})

				Context("and it should be restarted", func() {
					BeforeEach(func() {
						queryStr := `
			UPDATE actual_lrps SET crash_count = ?
			WHERE process_guid = ? AND instance_index = ?`
						if test_helpers.UsePostgres() {
							queryStr = test_helpers.ReplaceQuestionMarks(queryStr)
						}
						_, err := db.Exec(queryStr,
							models.DefaultImmediateRestarts-2,
							actualLRP.ProcessGuid,
							actualLRP.Index,
						)
						Expect(err).NotTo(HaveOccurred())
					})

					It("updates the lrp and sets its state to UNCLAIMED", func() {
						_, _, shouldRestart, err := sqlDB.CrashActualLRP(logger, &actualLRP.ActualLRPKey, instanceKey, "because it didn't go well")
						Expect(err).NotTo(HaveOccurred())
						Expect(shouldRestart).To(BeTrue())

						actualLRPs, err := sqlDB.ActualLRPs(logger, models.ActualLRPFilter{ProcessGuid: actualLRP.ProcessGuid, Index: &actualLRP.Index})
						Expect(err).NotTo(HaveOccurred())

						expectedActualLRP := *actualLRP
						expectedActualLRP.State = models.ActualLRPStateUnclaimed
						expectedActualLRP.CrashCount = models.DefaultImmediateRestarts - 1
						expectedActualLRP.CrashReason = "because it didn't go well"
						expectedActualLRP.ModificationTag.Increment()
						expectedActualLRP.Since = fakeClock.Now().UnixNano()

						Expect(actualLRPs).To(ConsistOf(&expectedActualLRP))
					})
				})

				Context("and it should NOT be restarted", func() {
					BeforeEach(func() {
						queryStr := `
		UPDATE actual_lrps SET crash_count = ?
		WHERE process_guid = ? AND instance_index = ?`
						if test_helpers.UsePostgres() {
							queryStr = test_helpers.ReplaceQuestionMarks(queryStr)
						}
						_, err := db.Exec(queryStr,
							models.DefaultImmediateRestarts+2,
							actualLRP.ProcessGuid,
							actualLRP.Index,
						)
						Expect(err).NotTo(HaveOccurred())
					})

					It("updates the lrp and sets its state to CRASHED", func() {
						_, _, shouldRestart, err := sqlDB.CrashActualLRP(logger, &actualLRP.ActualLRPKey, instanceKey, "some other failure reason")
						Expect(err).NotTo(HaveOccurred())
						Expect(shouldRestart).To(BeFalse())

						actualLRPs, err := sqlDB.ActualLRPs(logger, models.ActualLRPFilter{ProcessGuid: actualLRP.ProcessGuid, Index: &actualLRP.Index})
						Expect(err).NotTo(HaveOccurred())

						expectedActualLRP := *actualLRP
						expectedActualLRP.State = models.ActualLRPStateCrashed
						expectedActualLRP.CrashCount = models.DefaultImmediateRestarts + 3
						expectedActualLRP.CrashReason = "some other failure reason"
						expectedActualLRP.ModificationTag.Increment()
						expectedActualLRP.Since = fakeClock.Now().UnixNano()

						Expect(actualLRPs).To(ConsistOf(&expectedActualLRP))
					})
				})
			})

			Context("and it's already CRASHED", func() {
				BeforeEach(func() {
					_, _, err := sqlDB.StartActualLRP(logger, &actualLRP.ActualLRPKey, instanceKey, netInfo)
					Expect(err).NotTo(HaveOccurred())
					actualLRP.ModificationTag.Increment()

					_, _, _, err = sqlDB.CrashActualLRP(logger, &actualLRP.ActualLRPKey, instanceKey, "because it didn't go well")
					Expect(err).NotTo(HaveOccurred())
					actualLRP.ModificationTag.Increment()
				})

				It("returns a cannot crash error", func() {
					_, _, _, err := sqlDB.CrashActualLRP(logger, &actualLRP.ActualLRPKey, instanceKey, "because it didn't go well")
					Expect(err).To(HaveOccurred())
					Expect(err).To(Equal(models.ErrActualLRPCannotBeCrashed))
				})
			})

			Context("and it's UNCLAIMED", func() {
				It("returns a cannot crash error", func() {
					_, _, _, err := sqlDB.CrashActualLRP(logger, &actualLRP.ActualLRPKey, instanceKey, "because it didn't go well")
					Expect(err).To(HaveOccurred())
					Expect(err).To(Equal(models.ErrActualLRPCannotBeCrashed))
				})
			})
			Context("and it's EVACUATING", func() {
				BeforeEach(func() {
					queryStr := `
							UPDATE actual_lrps SET crash_count = ?, presence = ?
							WHERE process_guid = ? AND instance_index = ?`
					if test_helpers.UsePostgres() {
						queryStr = test_helpers.ReplaceQuestionMarks(queryStr)
					}
					_, err := db.Exec(queryStr,
						models.DefaultImmediateRestarts+2,
						models.ActualLRP_Evacuating,
						actualLRP.ProcessGuid,
						actualLRP.Index,
					)
					Expect(err).NotTo(HaveOccurred())
				})

				It("returns a cannot crash error", func() {
					_, _, _, err := sqlDB.CrashActualLRP(logger, &actualLRP.ActualLRPKey, instanceKey, "because it didn't go well")
					Expect(err).To(HaveOccurred())
					Expect(err).To(Equal(models.ErrResourceNotFound))
				})
			})

			Context("When two actual LRPs exist and only one is evacuating", func() {
				BeforeEach(func() {
					instanceKey = &models.ActualLRPInstanceKey{
						InstanceGuid: "the-instance-guid",
						CellId:       "the-cell-id",
					}

					netInfo = &models.ActualLRPNetInfo{
						Address:         "1.2.1.2",
						Ports:           []*models.PortMapping{{ContainerPort: 8080, HostPort: 9090}},
						InstanceAddress: "2.2.2.2",
					}
					evacuatingInstanceKey := &models.ActualLRPInstanceKey{
						InstanceGuid: "evacuating-instance-guid",
						CellId:       "evac-cell-id",
					}
					evacuatingLRP := &models.ActualLRP{
						ActualLRPKey: models.ActualLRPKey{
							ProcessGuid: "the-unclaimed-guid",
							Index:       1,
							Domain:      "the-domain",
						},
						Presence:             models.ActualLRP_Evacuating,
						ActualLRPInstanceKey: *instanceKey,
					}
					_, err := sqlDB.CreateUnclaimedActualLRP(logger, &evacuatingLRP.ActualLRPKey)
					Expect(err).NotTo(HaveOccurred())

					_, _, err = sqlDB.StartActualLRP(logger, &evacuatingLRP.ActualLRPKey, evacuatingInstanceKey, netInfo)
					Expect(err).NotTo(HaveOccurred())
					queryStr := `
							UPDATE actual_lrps SET presence = ?
							WHERE process_guid = ? AND instance_index = ?`

					if test_helpers.UsePostgres() {
						queryStr = test_helpers.ReplaceQuestionMarks(queryStr)
					}

					_, err = db.Exec(queryStr,
						models.ActualLRP_Evacuating,
						evacuatingLRP.ProcessGuid,
						evacuatingLRP.Index,
					)
					Expect(err).NotTo(HaveOccurred())

					actualLRP = &models.ActualLRP{
						ActualLRPKey: models.ActualLRPKey{
							ProcessGuid: "the-unclaimed-guid",
							Index:       1,
							Domain:      "the-domain",
						},
						Presence:             models.ActualLRP_Ordinary,
						ActualLRPInstanceKey: *instanceKey,
					}
					_, err = sqlDB.CreateUnclaimedActualLRP(logger, &actualLRP.ActualLRPKey)
					Expect(err).NotTo(HaveOccurred())

					_, _, err = sqlDB.StartActualLRP(logger, &actualLRP.ActualLRPKey, instanceKey, netInfo)
					Expect(err).NotTo(HaveOccurred())

				})

				It("only update the non evacuating one", func() {
					_, crashedActualLRP, _, err := sqlDB.CrashActualLRP(logger, &actualLRP.ActualLRPKey, instanceKey, "because it didn't go well")
					Expect(err).ToNot(HaveOccurred())
					Expect(crashedActualLRP.CrashCount).To(Equal(actualLRP.CrashCount + 1))
				})
			})
		})

		Context("when the actual lrp does NOT exist", func() {
			It("returns a record not found error", func() {
				instanceKey := &models.ActualLRPInstanceKey{
					InstanceGuid: "the-instance-guid",
					CellId:       "the-cell-id",
				}

				key := &models.ActualLRPKey{
					ProcessGuid: "the-guid",
					Index:       1,
					Domain:      "the-domain",
				}

				_, _, _, err := sqlDB.CrashActualLRP(logger, key, instanceKey, "because it didn't go well")
				Expect(err).To(HaveOccurred())
				Expect(err).To(Equal(models.ErrResourceNotFound))
			})
		})
	})

	Describe("FailActualLRP", func() {
		var actualLRPKey = &models.ActualLRPKey{
			ProcessGuid: "the-guid",
			Index:       1,
			Domain:      "the-domain",
		}

		Context("when the actualLRP exists", func() {
			var actualLRP *models.ActualLRP

			BeforeEach(func() {
				actualLRP = &models.ActualLRP{
					ActualLRPKey: *actualLRPKey,
				}
				_, err := sqlDB.CreateUnclaimedActualLRP(logger, &actualLRP.ActualLRPKey)
				Expect(err).NotTo(HaveOccurred())
				fakeClock.Increment(time.Hour)
			})

			Context("and the state is UNCLAIMED", func() {
				It("fails the LRP", func() {
					_, _, err := sqlDB.FailActualLRP(logger, &actualLRP.ActualLRPKey, "failing the LRP")
					Expect(err).NotTo(HaveOccurred())

					actualLRPs, err := sqlDB.ActualLRPs(logger, models.ActualLRPFilter{ProcessGuid: actualLRP.ProcessGuid, Index: &actualLRP.Index})
					Expect(err).NotTo(HaveOccurred())

					expectedActualLRP := *actualLRP
					expectedActualLRP.State = models.ActualLRPStateUnclaimed
					expectedActualLRP.PlacementError = "failing the LRP"
					expectedActualLRP.Since = fakeClock.Now().UnixNano()
					expectedActualLRP.ModificationTag = models.ModificationTag{
						Epoch: "my-awesome-guid",
						Index: 1,
					}

					Expect(actualLRPs).To(ConsistOf(&expectedActualLRP))
				})

				Context("and the placement error is longer than 1K", func() {
					It("truncates the placement_error", func() {
						value := strings.Repeat("x", 2*1024)
						_, _, err := sqlDB.FailActualLRP(logger, &actualLRP.ActualLRPKey, value)
						Expect(err).NotTo(HaveOccurred())

						actualLRPs, err := sqlDB.ActualLRPs(logger, models.ActualLRPFilter{ProcessGuid: actualLRP.ProcessGuid, Index: &actualLRP.Index})
						Expect(err).NotTo(HaveOccurred())

						expectedActualLRP := *actualLRP
						expectedActualLRP.State = models.ActualLRPStateUnclaimed
						expectedActualLRP.PlacementError = value[:1013] + "(truncated)"
						expectedActualLRP.Since = fakeClock.Now().UnixNano()
						expectedActualLRP.ModificationTag = models.ModificationTag{
							Epoch: "my-awesome-guid",
							Index: 1,
						}

						Expect(actualLRPs).To(ConsistOf(&expectedActualLRP))
					})
				})

				It("returns the previous and current actual lrp", func() {
					beforeActualLRP, afterActualLRP, err := sqlDB.FailActualLRP(logger, &actualLRP.ActualLRPKey, "failing the LRP")
					Expect(err).NotTo(HaveOccurred())

					actualLRPs, err := sqlDB.ActualLRPs(logger, models.ActualLRPFilter{ProcessGuid: actualLRP.ProcessGuid, Index: &actualLRP.Index})
					Expect(err).NotTo(HaveOccurred())

					expectedActualLRP := *actualLRP
					expectedActualLRP.State = models.ActualLRPStateUnclaimed
					expectedActualLRP.Since = fakeClock.Now().Add(-time.Hour).UnixNano()
					expectedActualLRP.ModificationTag = models.ModificationTag{
						Epoch: "my-awesome-guid",
						Index: 0,
					}
					Expect(beforeActualLRP).To(Equal(&expectedActualLRP))
					Expect(actualLRPs).To(ConsistOf(afterActualLRP))
				})
			})

			Context("and the state is not UNCLAIMED", func() {
				BeforeEach(func() {
					instanceKey := &models.ActualLRPInstanceKey{
						InstanceGuid: "the-instance-guid",
						CellId:       "the-cell-id",
					}
					_, _, err := sqlDB.ClaimActualLRP(logger, actualLRP.ProcessGuid, actualLRP.Index, instanceKey)
					Expect(err).NotTo(HaveOccurred())
					fakeClock.Increment(time.Hour)
				})

				It("returns a cannot be failed error", func() {
					_, _, err := sqlDB.FailActualLRP(logger, &actualLRP.ActualLRPKey, "failing the LRP")
					Expect(err).To(HaveOccurred())
					Expect(err).To(Equal(models.ErrActualLRPCannotBeFailed))
				})
			})
		})

		Context("when the actualLRP does not exist", func() {
			It("returns a not found error", func() {
				_, _, err := sqlDB.FailActualLRP(logger, actualLRPKey, "failing the LRP")
				Expect(err).To(HaveOccurred())
				Expect(err).To(Equal(models.ErrResourceNotFound))
			})
		})
	})

	Describe("RemoveActualLRP", func() {
		var actualLRPKey = &models.ActualLRPKey{
			ProcessGuid: "the-guid",
			Index:       1,
			Domain:      "the-domain",
		}

		Context("when the actual LRP exists", func() {
			var actualLRP *models.ActualLRP
			var otherActualLRPKey = &models.ActualLRPKey{
				ProcessGuid: "other-guid",
				Index:       1,
				Domain:      "the-domain",
			}

			BeforeEach(func() {
				actualLRP = &models.ActualLRP{
					ActualLRPKey: *actualLRPKey,
				}
				_, err := sqlDB.CreateUnclaimedActualLRP(logger, &actualLRP.ActualLRPKey)
				Expect(err).NotTo(HaveOccurred())

				_, err = sqlDB.CreateUnclaimedActualLRP(logger, otherActualLRPKey)
				Expect(err).NotTo(HaveOccurred())
				fakeClock.Increment(time.Hour)
			})

			It("removes the actual lrp", func() {
				err := sqlDB.RemoveActualLRP(logger, actualLRP.ProcessGuid, actualLRP.Index, nil)
				Expect(err).NotTo(HaveOccurred())

				lrps, err := sqlDB.ActualLRPs(logger, models.ActualLRPFilter{ProcessGuid: actualLRP.ProcessGuid, Index: &actualLRP.Index})
				Expect(err).NotTo(HaveOccurred())
				Expect(lrps).To(BeEmpty())
			})

			It("keeps the other lrps around", func() {
				err := sqlDB.RemoveActualLRP(logger, actualLRP.ProcessGuid, actualLRP.Index, nil)
				Expect(err).NotTo(HaveOccurred())

				_, err = sqlDB.ActualLRPs(logger, models.ActualLRPFilter{ProcessGuid: otherActualLRPKey.ProcessGuid, Index: &otherActualLRPKey.Index})
				Expect(err).NotTo(HaveOccurred())
			})

			Context("when an instance key is provided", func() {
				var instanceKey models.ActualLRPInstanceKey

				BeforeEach(func() {
					instanceKey = models.NewActualLRPInstanceKey("instance-guid", "cell-id")

					_, _, err := sqlDB.ClaimActualLRP(logger, actualLRP.ProcessGuid, actualLRP.Index, &instanceKey)
					Expect(err).NotTo(HaveOccurred())
				})

				Context("and it matches the existing actual lrp", func() {
					It("removes the actual lrp", func() {
						err := sqlDB.RemoveActualLRP(logger, actualLRP.ProcessGuid, actualLRP.Index, &instanceKey)
						Expect(err).NotTo(HaveOccurred())

						lrps, err := sqlDB.ActualLRPs(logger, models.ActualLRPFilter{ProcessGuid: actualLRP.ProcessGuid, Index: &actualLRP.Index})
						Expect(err).NotTo(HaveOccurred())
						Expect(lrps).To(BeEmpty())
					})
				})

				Context("and it does not match the existing actual lrp", func() {
					It("returns an error", func() {
						instanceKey.CellId = "not the right cell id"
						err := sqlDB.RemoveActualLRP(logger, actualLRP.ProcessGuid, actualLRP.Index, &instanceKey)
						Expect(err).To(HaveOccurred())
					})
				})
			})
		})

		Context("when the actual lrp does NOT exist", func() {
			It("returns a resource not found error", func() {
				err := sqlDB.RemoveActualLRP(logger, actualLRPKey.ProcessGuid, actualLRPKey.Index, nil)
				Expect(err).To(HaveOccurred())
				Expect(err).To(Equal(models.ErrResourceNotFound))
			})
		})
	})

	Describe("UnclaimActualLRP", func() {
		var (
			actualLRP *models.ActualLRP
			guid      = "the-guid"
			index     = int32(1)

			actualLRPKey = &models.ActualLRPKey{
				ProcessGuid: guid,
				Index:       index,
				Domain:      "the-domain",
			}
		)

		Context("when the actual LRP exists", func() {
			Context("When the actual LRP is claimed", func() {
				var beforeActualLRP, afterActualLRP *models.ActualLRP

				BeforeEach(func() {
					actualLRP = &models.ActualLRP{
						ActualLRPKey: *actualLRPKey,
					}

					_, err := sqlDB.CreateUnclaimedActualLRP(logger, &actualLRP.ActualLRPKey)
					Expect(err).NotTo(HaveOccurred())
					_, _, err = sqlDB.ClaimActualLRP(logger, guid, index, &actualLRP.ActualLRPInstanceKey)
					Expect(err).NotTo(HaveOccurred())
				})

				JustBeforeEach(func() {
					var err error
					beforeActualLRP, afterActualLRP, err = sqlDB.UnclaimActualLRP(logger, actualLRPKey)
					Expect(err).ToNot(HaveOccurred())
				})

				It("unclaims the actual LRP", func() {
					actualLRPs, err := sqlDB.ActualLRPs(logger, models.ActualLRPFilter{ProcessGuid: guid, Index: &index})
					Expect(err).ToNot(HaveOccurred())
					Expect(actualLRPs).To(HaveLen(1))
					Expect(actualLRPs[0].State).To(Equal(models.ActualLRPStateUnclaimed))
				})

				It("it removes the net info from the actualLRP", func() {
					actualLRPs, err := sqlDB.ActualLRPs(logger, models.ActualLRPFilter{ProcessGuid: guid, Index: &index})
					Expect(err).ToNot(HaveOccurred())
					Expect(actualLRPs).To(HaveLen(1))
					Expect(actualLRPs[0].ActualLRPNetInfo).To(Equal(models.ActualLRPNetInfo{}))
				})

				It("it increments the modification tag on the actualLRP", func() {
					actualLRPs, err := sqlDB.ActualLRPs(logger, models.ActualLRPFilter{ProcessGuid: guid, Index: &index})
					Expect(err).ToNot(HaveOccurred())
					// +2 because of claim AND unclaim
					Expect(actualLRPs).To(HaveLen(1))
					Expect(actualLRPs[0].ModificationTag.Index).To(Equal(actualLRP.ModificationTag.Index + uint32(2)))
				})

				It("it clears the actualLRP's instance key", func() {
					actualLRPs, err := sqlDB.ActualLRPs(logger, models.ActualLRPFilter{ProcessGuid: guid, Index: &index})
					Expect(err).ToNot(HaveOccurred())
					Expect(actualLRPs).To(HaveLen(1))
					Expect(actualLRPs[0].ActualLRPInstanceKey).To(Equal(models.ActualLRPInstanceKey{}))
				})

				It("it updates the actualLRP's update at timestamp", func() {
					actualLRPs, err := sqlDB.ActualLRPs(logger, models.ActualLRPFilter{ProcessGuid: guid, Index: &index})
					Expect(err).ToNot(HaveOccurred())
					Expect(actualLRPs).To(HaveLen(1))
					Expect(actualLRPs[0].Since).To(BeNumerically(">", actualLRP.Since))
				})

				It("returns the previous and current actual lrp", func() {
					expectedActualLRP := *actualLRP
					expectedActualLRP.State = models.ActualLRPStateClaimed
					expectedActualLRP.Since = fakeClock.Now().UnixNano()
					expectedActualLRP.ModificationTag = models.ModificationTag{
						Epoch: "my-awesome-guid",
						Index: 1,
					}
					Expect(beforeActualLRP).To(BeEquivalentTo(&expectedActualLRP))

					actualLRPs, err := sqlDB.ActualLRPs(logger, models.ActualLRPFilter{ProcessGuid: guid, Index: &index})
					Expect(err).ToNot(HaveOccurred())
					Expect(actualLRPs).To(ConsistOf(afterActualLRP))
				})
			})

			Context("When the actual LRP is unclaimed", func() {
				BeforeEach(func() {
					actualLRP = &models.ActualLRP{
						ActualLRPKey: *actualLRPKey,
					}

					_, err := sqlDB.CreateUnclaimedActualLRP(logger, &actualLRP.ActualLRPKey)
					Expect(err).NotTo(HaveOccurred())
				})

				It("returns an error", func() {
					_, _, err := sqlDB.UnclaimActualLRP(logger, actualLRPKey)
					Expect(err).To(HaveOccurred())
					Expect(err).To(Equal(models.ErrActualLRPCannotBeUnclaimed))
				})
			})
		})

		Context("when the actual LRP doesn't exist", func() {
			It("returns a resource not found error", func() {
				_, _, err := sqlDB.UnclaimActualLRP(logger, actualLRPKey)
				Expect(err).To(HaveOccurred())
				Expect(err).To(Equal(models.ErrResourceNotFound))
			})
		})
	})
})
