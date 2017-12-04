package sqldb_test

import (
	"errors"
	"strings"
	"time"

	"code.cloudfoundry.org/bbs/format"
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
			actualLRPGroup, err := sqlDB.CreateUnclaimedActualLRP(logger, key)
			Expect(err).NotTo(HaveOccurred())

			actualLRP := models.NewUnclaimedActualLRP(*key, fakeClock.Now().UnixNano())
			actualLRP.ModificationTag.Epoch = "my-awesome-guid"
			actualLRP.ModificationTag.Index = 0

			group, err := sqlDB.ActualLRPGroupByProcessGuidAndIndex(logger, key.ProcessGuid, key.Index)
			Expect(err).NotTo(HaveOccurred())
			Expect(group).NotTo(BeNil())
			Expect(group.Instance).To(BeEquivalentTo(actualLRP))
			Expect(group.Evacuating).To(BeNil())

			Expect(actualLRPGroup).To(Equal(group))
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

	Describe("ActualLRPGroupByProcessGuidAndIndex", func() {
		var actualLRP *models.ActualLRP

		BeforeEach(func() {
			actualLRP = &models.ActualLRP{
				ActualLRPKey: models.NewActualLRPKey("some-guid", 0, "some-domain"),
				State:        models.ActualLRPStateUnclaimed,
				ModificationTag: models.ModificationTag{
					Epoch: "my-awesome-guid",
					Index: 0,
				},
			}
			_, err := sqlDB.CreateUnclaimedActualLRP(logger, &actualLRP.ActualLRPKey)
			Expect(err).NotTo(HaveOccurred())
		})

		It("returns the existing actual lrp group", func() {
			actualLRP.Since = fakeClock.Now().UnixNano()

			group, err := sqlDB.ActualLRPGroupByProcessGuidAndIndex(logger, actualLRP.ProcessGuid, actualLRP.Index)
			Expect(err).NotTo(HaveOccurred())
			Expect(group).NotTo(BeNil())
			Expect(group.Instance).To(BeEquivalentTo(actualLRP))
			Expect(group.Evacuating).To(BeNil())
		})

		Context("when there's just an evacuating LRP", func() {
			BeforeEach(func() {
				queryStr := "UPDATE actual_lrps SET evacuating = ? WHERE process_guid = ? AND instance_index = ? AND evacuating = ?"
				if test_helpers.UsePostgres() {
					queryStr = test_helpers.ReplaceQuestionMarks(queryStr)
				}
				_, err := db.Exec(queryStr, true, actualLRP.ProcessGuid, actualLRP.Index, false)
				Expect(err).NotTo(HaveOccurred())
			})

			It("returns the existing actual lrp group", func() {
				actualLRP.Since = fakeClock.Now().UnixNano()

				group, err := sqlDB.ActualLRPGroupByProcessGuidAndIndex(logger, actualLRP.ProcessGuid, actualLRP.Index)
				Expect(err).NotTo(HaveOccurred())
				Expect(group).NotTo(BeNil())
				Expect(group.Instance).To(BeNil())
				Expect(group.Evacuating).To(BeEquivalentTo(actualLRP))
			})
		})

		Context("when there are both instance and evacuating LRPs", func() {
			BeforeEach(func() {
				queryStr := "UPDATE actual_lrps SET evacuating = true WHERE process_guid = ?"
				if test_helpers.UsePostgres() {
					queryStr = test_helpers.ReplaceQuestionMarks(queryStr)
				}
				_, err := db.Exec(queryStr, actualLRP.ProcessGuid)
				Expect(err).NotTo(HaveOccurred())
				_, err = sqlDB.CreateUnclaimedActualLRP(logger, &actualLRP.ActualLRPKey)
				Expect(err).NotTo(HaveOccurred())
			})

			It("returns the existing actual lrp group", func() {
				actualLRP.Since = fakeClock.Now().UnixNano()

				group, err := sqlDB.ActualLRPGroupByProcessGuidAndIndex(logger, actualLRP.ProcessGuid, actualLRP.Index)
				Expect(err).NotTo(HaveOccurred())
				Expect(group).NotTo(BeNil())
				Expect(group.Instance).To(BeEquivalentTo(actualLRP))
				Expect(group.Evacuating).To(BeEquivalentTo(actualLRP))
			})
		})

		Context("when the actual LRP does not exist", func() {
			It("returns a resource not found error", func() {
				group, err := sqlDB.ActualLRPGroupByProcessGuidAndIndex(logger, "nope", 0)
				Expect(err).To(Equal(models.ErrResourceNotFound))
				Expect(group).To(BeNil())
			})
		})
	})

	Describe("ActualLRPGroups", func() {
		var allActualLRPGroups []*models.ActualLRPGroup

		BeforeEach(func() {
			allActualLRPGroups = []*models.ActualLRPGroup{}
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
			allActualLRPGroups = append(allActualLRPGroups, &models.ActualLRPGroup{
				Instance: &models.ActualLRP{
					ActualLRPKey:         *actualLRPKey1,
					ActualLRPInstanceKey: *instanceKey1,
					State:                models.ActualLRPStateClaimed,
					Since:                fakeClock.Now().UnixNano(),
					ModificationTag: models.ModificationTag{
						Epoch: "mod-tag-guid",
						Index: 1,
					},
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
			allActualLRPGroups = append(allActualLRPGroups, &models.ActualLRPGroup{
				Instance: &models.ActualLRP{
					ActualLRPKey:         *actualLRPKey2,
					ActualLRPInstanceKey: *instanceKey2,
					State:                models.ActualLRPStateClaimed,
					Since:                fakeClock.Now().UnixNano(),
					ModificationTag: models.ModificationTag{
						Epoch: "mod-tag-guid",
						Index: 1,
					},
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
			allActualLRPGroups = append(allActualLRPGroups, &models.ActualLRPGroup{
				Instance: &models.ActualLRP{
					ActualLRPKey:         *actualLRPKey3,
					ActualLRPInstanceKey: *instanceKey3,
					State:                models.ActualLRPStateClaimed,
					Since:                fakeClock.Now().UnixNano(),
					ModificationTag: models.ModificationTag{
						Epoch: "mod-tag-guid",
						Index: 1,
					},
				},
			})

			actualLRPKey4 := &models.ActualLRPKey{
				ProcessGuid: "guid4",
				Index:       1,
				Domain:      "domain2",
			}
			_, err = sqlDB.CreateUnclaimedActualLRP(logger, actualLRPKey4)
			Expect(err).NotTo(HaveOccurred())
			allActualLRPGroups = append(allActualLRPGroups, &models.ActualLRPGroup{
				Instance: &models.ActualLRP{
					ActualLRPKey: *actualLRPKey4,
					State:        models.ActualLRPStateUnclaimed,
					Since:        fakeClock.Now().UnixNano(),
					ModificationTag: models.ModificationTag{
						Epoch: "mod-tag-guid",
						Index: 0,
					},
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
			queryStr := "UPDATE actual_lrps SET evacuating = ? WHERE process_guid = ? AND instance_index = ? AND evacuating = ?"
			if test_helpers.UsePostgres() {
				queryStr = test_helpers.ReplaceQuestionMarks(queryStr)
			}
			_, err = db.Exec(queryStr, true, actualLRPKey5.ProcessGuid, actualLRPKey5.Index, false)
			Expect(err).NotTo(HaveOccurred())
			allActualLRPGroups = append(allActualLRPGroups, &models.ActualLRPGroup{
				Evacuating: &models.ActualLRP{
					ActualLRPKey:         *actualLRPKey5,
					ActualLRPInstanceKey: *instanceKey5,
					State:                models.ActualLRPStateClaimed,
					Since:                fakeClock.Now().UnixNano(),
					ModificationTag: models.ModificationTag{
						Epoch: "mod-tag-guid",
						Index: 1,
					},
				},
			})

			actualLRPKey6 := &models.ActualLRPKey{
				ProcessGuid: "guid6",
				Index:       1,
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
			queryStr = "UPDATE actual_lrps SET evacuating = ? WHERE process_guid = ? AND instance_index = ? AND evacuating = ?"
			if test_helpers.UsePostgres() {
				queryStr = test_helpers.ReplaceQuestionMarks(queryStr)
			}
			_, err = db.Exec(queryStr, true, actualLRPKey6.ProcessGuid, actualLRPKey6.Index, false)

			_, err = sqlDB.CreateUnclaimedActualLRP(logger, actualLRPKey6)
			Expect(err).NotTo(HaveOccurred())
			_, _, err = sqlDB.ClaimActualLRP(logger, actualLRPKey6.ProcessGuid, actualLRPKey6.Index, instanceKey6)
			Expect(err).NotTo(HaveOccurred())

			Expect(err).NotTo(HaveOccurred())
			allActualLRPGroups = append(allActualLRPGroups, &models.ActualLRPGroup{
				Instance: &models.ActualLRP{
					ActualLRPKey:         *actualLRPKey6,
					ActualLRPInstanceKey: *instanceKey6,
					State:                models.ActualLRPStateClaimed,
					Since:                fakeClock.Now().UnixNano(),
					ModificationTag: models.ModificationTag{
						Epoch: "mod-tag-guid",
						Index: 1,
					},
				},
				Evacuating: &models.ActualLRP{
					ActualLRPKey:         *actualLRPKey6,
					ActualLRPInstanceKey: *instanceKey6,
					State:                models.ActualLRPStateClaimed,
					Since:                fakeClock.Now().UnixNano(),
					ModificationTag: models.ModificationTag{
						Epoch: "mod-tag-guid",
						Index: 1,
					},
				},
			})
		})

		It("returns all the actual lrp groups", func() {
			actualLRPGroups, err := sqlDB.ActualLRPGroups(logger, models.ActualLRPFilter{})
			Expect(err).NotTo(HaveOccurred())

			Expect(actualLRPGroups).To(ConsistOf(allActualLRPGroups))
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

			actualLRPGroups, err := sqlDB.ActualLRPGroups(logger, models.ActualLRPFilter{})
			Expect(err).NotTo(HaveOccurred())

			Expect(actualLRPGroups).NotTo(ContainElement(actualLRPWithInvalidData))
		})

		Context("when filtering on domains", func() {
			It("returns the actual lrp groups in the domain", func() {
				filter := models.ActualLRPFilter{
					Domain: "domain2",
				}
				actualLRPGroups, err := sqlDB.ActualLRPGroups(logger, filter)
				Expect(err).NotTo(HaveOccurred())

				Expect(actualLRPGroups).To(HaveLen(3))
				Expect(actualLRPGroups).To(ContainElement(allActualLRPGroups[1]))
				Expect(actualLRPGroups).To(ContainElement(allActualLRPGroups[3]))
				Expect(actualLRPGroups).To(ContainElement(allActualLRPGroups[4]))
			})
		})

		Context("when filtering on cell", func() {
			It("returns the actual lrp groups claimed by the cell", func() {
				filter := models.ActualLRPFilter{
					CellID: "cell1",
				}
				actualLRPGroups, err := sqlDB.ActualLRPGroups(logger, filter)
				Expect(err).NotTo(HaveOccurred())

				Expect(actualLRPGroups).To(HaveLen(2))
				Expect(actualLRPGroups).To(ContainElement(allActualLRPGroups[0]))
				Expect(actualLRPGroups).To(ContainElement(allActualLRPGroups[1]))
			})
		})

		Context("when filtering on domain and cell", func() {
			It("returns the actual lrp groups in the domain and claimed by the cell", func() {
				filter := models.ActualLRPFilter{
					Domain: "domain1",
					CellID: "cell2",
				}
				actualLRPGroups, err := sqlDB.ActualLRPGroups(logger, filter)
				Expect(err).NotTo(HaveOccurred())

				Expect(actualLRPGroups).To(HaveLen(2))
				Expect(actualLRPGroups).To(ContainElement(allActualLRPGroups[2]))
				Expect(actualLRPGroups).To(ContainElement(allActualLRPGroups[5]))
			})
		})
	})

	Describe("ActualLRPGroupsByProcessGuid", func() {
		var allActualLRPGroups []*models.ActualLRPGroup

		BeforeEach(func() {
			allActualLRPGroups = []*models.ActualLRPGroup{}
			fakeGUIDProvider.NextGUIDReturns("mod-tag-guid", nil)

			actualLRPKey1 := &models.ActualLRPKey{
				ProcessGuid: "guid1",
				Index:       0,
				Domain:      "domain1",
			}
			_, err := sqlDB.CreateUnclaimedActualLRP(logger, actualLRPKey1)
			Expect(err).NotTo(HaveOccurred())
			allActualLRPGroups = append(allActualLRPGroups, &models.ActualLRPGroup{
				Instance: &models.ActualLRP{
					ActualLRPKey: *actualLRPKey1,
					State:        models.ActualLRPStateUnclaimed,
					Since:        fakeClock.Now().UnixNano(),
					ModificationTag: models.ModificationTag{
						Epoch: "mod-tag-guid",
						Index: 0,
					},
				},
			})

			actualLRPKey2 := &models.ActualLRPKey{
				ProcessGuid: "guid1",
				Index:       1,
				Domain:      "domain1",
			}
			fakeClock.Increment(time.Hour)
			_, err = sqlDB.CreateUnclaimedActualLRP(logger, actualLRPKey2)
			Expect(err).NotTo(HaveOccurred())
			queryStr := "UPDATE actual_lrps SET evacuating = ? WHERE process_guid = ? AND instance_index = ? AND evacuating = ?"
			if test_helpers.UsePostgres() {
				queryStr = test_helpers.ReplaceQuestionMarks(queryStr)
			}
			_, err = db.Exec(queryStr, true, actualLRPKey2.ProcessGuid, actualLRPKey2.Index, false)

			_, err = sqlDB.CreateUnclaimedActualLRP(logger, actualLRPKey2)
			Expect(err).NotTo(HaveOccurred())

			Expect(err).NotTo(HaveOccurred())
			allActualLRPGroups = append(allActualLRPGroups, &models.ActualLRPGroup{
				Instance: &models.ActualLRP{
					ActualLRPKey: *actualLRPKey2,
					State:        models.ActualLRPStateUnclaimed,
					Since:        fakeClock.Now().UnixNano(),
					ModificationTag: models.ModificationTag{
						Epoch: "mod-tag-guid",
						Index: 0,
					},
				},
				Evacuating: &models.ActualLRP{
					ActualLRPKey: *actualLRPKey2,
					State:        models.ActualLRPStateUnclaimed,
					Since:        fakeClock.Now().UnixNano(),
					ModificationTag: models.ModificationTag{
						Epoch: "mod-tag-guid",
						Index: 0,
					},
				},
			})

			actualLRPKey3 := &models.ActualLRPKey{
				ProcessGuid: "guid2",
				Index:       0,
				Domain:      "domain1",
			}
			fakeClock.Increment(time.Hour)
			_, err = sqlDB.CreateUnclaimedActualLRP(logger, actualLRPKey3)
			Expect(err).NotTo(HaveOccurred())
			Expect(err).NotTo(HaveOccurred())
			allActualLRPGroups = append(allActualLRPGroups, &models.ActualLRPGroup{
				Instance: &models.ActualLRP{
					ActualLRPKey: *actualLRPKey3,
					State:        models.ActualLRPStateClaimed,
					Since:        fakeClock.Now().UnixNano(),
					ModificationTag: models.ModificationTag{
						Epoch: "mod-tag-guid",
						Index: 0,
					},
				},
			})
		})

		It("returns all the actual lrp groups for the chosen process guid", func() {
			actualLRPGroups, err := sqlDB.ActualLRPGroupsByProcessGuid(logger, "guid1")
			Expect(err).NotTo(HaveOccurred())

			Expect(actualLRPGroups).To(HaveLen(2))
			Expect(actualLRPGroups).To(ContainElement(allActualLRPGroups[0]))
			Expect(actualLRPGroups).To(ContainElement(allActualLRPGroups[1]))
		})

		Context("when no actual lrps exist for the process guid", func() {
			It("returns an empty slice", func() {
				actualLRPGroups, err := sqlDB.ActualLRPGroupsByProcessGuid(logger, "guid3")
				Expect(err).NotTo(HaveOccurred())

				Expect(actualLRPGroups).To(HaveLen(0))
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

					actualLRPGroup, err := sqlDB.ActualLRPGroupByProcessGuidAndIndex(logger, expectedActualLRP.ProcessGuid, expectedActualLRP.Index)
					Expect(err).NotTo(HaveOccurred())
					Expect(actualLRPGroup.Instance).To(BeEquivalentTo(expectedActualLRP))
					Expect(actualLRPGroup.Evacuating).To(BeNil())
				})

				It("returns the existing actual lrp", func() {
					beforeActualLRPGroup, afterActualLRPGroup, err := sqlDB.ClaimActualLRP(logger, expectedActualLRP.ProcessGuid, expectedActualLRP.Index, instanceKey)
					Expect(err).NotTo(HaveOccurred())

					expectedActualLRP.State = models.ActualLRPStateUnclaimed
					expectedActualLRP.Since = lrpCreationTime.UnixNano()
					Expect(beforeActualLRPGroup).To(Equal(&models.ActualLRPGroup{Instance: expectedActualLRP}))

					actualLRPGroup, err := sqlDB.ActualLRPGroupByProcessGuidAndIndex(logger, expectedActualLRP.ProcessGuid, expectedActualLRP.Index)
					Expect(err).NotTo(HaveOccurred())
					Expect(afterActualLRPGroup).To(Equal(actualLRPGroup))
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

						actualLRPGroup, err := sqlDB.ActualLRPGroupByProcessGuidAndIndex(logger, expectedActualLRP.ProcessGuid, expectedActualLRP.Index)
						Expect(err).NotTo(HaveOccurred())
						Expect(actualLRPGroup.Instance).To(BeEquivalentTo(expectedActualLRP))
						Expect(actualLRPGroup.Evacuating).To(BeNil())
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
						Expect(afterActualLRP).To(Equal(&models.ActualLRPGroup{Instance: expectedActualLRP}))

						actualLRPGroup, err := sqlDB.ActualLRPGroupByProcessGuidAndIndex(logger, expectedActualLRP.ProcessGuid, expectedActualLRP.Index)
						Expect(err).NotTo(HaveOccurred())
						Expect(actualLRPGroup.Instance).To(BeEquivalentTo(expectedActualLRP))
						Expect(actualLRPGroup.Evacuating).To(BeNil())
					})
				})

				Context("when the actual lrp is claimed by another cell", func() {
					BeforeEach(func() {
						_, _, err := sqlDB.ClaimActualLRP(logger, expectedActualLRP.ProcessGuid, expectedActualLRP.Index, instanceKey)
						Expect(err).NotTo(HaveOccurred())

						group, err := sqlDB.ActualLRPGroupByProcessGuidAndIndex(logger, expectedActualLRP.ProcessGuid, expectedActualLRP.Index)
						Expect(err).NotTo(HaveOccurred())
						Expect(group).NotTo(BeNil())
						Expect(group.Instance).NotTo(BeNil())
						expectedActualLRP = group.Instance
					})

					It("returns an error", func() {
						instanceKey = &models.ActualLRPInstanceKey{
							InstanceGuid: "different-instance",
							CellId:       "different-cell",
						}

						_, _, err := sqlDB.ClaimActualLRP(logger, expectedActualLRP.ProcessGuid, expectedActualLRP.Index, instanceKey)
						Expect(err).To(HaveOccurred())

						actualLRPGroup, err := sqlDB.ActualLRPGroupByProcessGuidAndIndex(logger, expectedActualLRP.ProcessGuid, expectedActualLRP.Index)
						Expect(err).NotTo(HaveOccurred())
						Expect(actualLRPGroup.Instance).To(BeEquivalentTo(expectedActualLRP))
						Expect(actualLRPGroup.Evacuating).To(BeNil())
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

					netInfoData, err := serializer.Marshal(logger, format.ENCODED_PROTO, &netInfo)
					Expect(err).NotTo(HaveOccurred())

					queryStr := `
				UPDATE actual_lrps SET state = ?, net_info = ?, cell_id = ?, instance_guid = ?
				WHERE process_guid = ? AND instance_index = ?`
					if test_helpers.UsePostgres() {
						queryStr = test_helpers.ReplaceQuestionMarks(queryStr)
					}
					_, err = db.Exec(queryStr,
						models.ActualLRPStateRunning,
						netInfoData,
						instanceKey.CellId,
						instanceKey.InstanceGuid,
						expectedActualLRP.ProcessGuid,
						expectedActualLRP.Index,
					)
					Expect(err).NotTo(HaveOccurred())
				})

				Context("with the same cell and instance guid", func() {
					It("reverts the RUNNING actual lrp to the CLAIMED state", func() {
						_, _, err := sqlDB.ClaimActualLRP(logger, expectedActualLRP.ProcessGuid, expectedActualLRP.Index, instanceKey)
						Expect(err).NotTo(HaveOccurred())

						actualLRPGroup, err := sqlDB.ActualLRPGroupByProcessGuidAndIndex(logger, expectedActualLRP.ProcessGuid, expectedActualLRP.Index)
						Expect(err).NotTo(HaveOccurred())

						expectedActualLRP.ActualLRPInstanceKey = *instanceKey
						expectedActualLRP.State = models.ActualLRPStateClaimed
						expectedActualLRP.Since = fakeClock.Now().UnixNano()
						expectedActualLRP.ModificationTag.Increment()
						Expect(actualLRPGroup.Instance).To(BeEquivalentTo(expectedActualLRP))
						Expect(actualLRPGroup.Evacuating).To(BeNil())
					})
				})

				Context("with a different cell id", func() {
					BeforeEach(func() {
						instanceKey.CellId = "another-cell"

						group, err := sqlDB.ActualLRPGroupByProcessGuidAndIndex(logger, expectedActualLRP.ProcessGuid, expectedActualLRP.Index)
						Expect(err).NotTo(HaveOccurred())
						Expect(group).NotTo(BeNil())
						Expect(group.Instance).NotTo(BeNil())
						expectedActualLRP = group.Instance
					})

					It("returns an error", func() {
						_, _, err := sqlDB.ClaimActualLRP(logger, expectedActualLRP.ProcessGuid, expectedActualLRP.Index, instanceKey)
						Expect(err).To(HaveOccurred())

						actualLRPGroup, err := sqlDB.ActualLRPGroupByProcessGuidAndIndex(logger, expectedActualLRP.ProcessGuid, expectedActualLRP.Index)
						Expect(err).NotTo(HaveOccurred())
						Expect(actualLRPGroup.Instance).To(BeEquivalentTo(expectedActualLRP))
					})
				})

				Context("with a different instance guid", func() {
					BeforeEach(func() {
						instanceKey.InstanceGuid = "another-instance-guid"

						group, err := sqlDB.ActualLRPGroupByProcessGuidAndIndex(logger, expectedActualLRP.ProcessGuid, expectedActualLRP.Index)
						Expect(err).NotTo(HaveOccurred())
						Expect(group).NotTo(BeNil())
						Expect(group.Instance).NotTo(BeNil())
						expectedActualLRP = group.Instance
					})

					It("returns an error", func() {
						_, _, err := sqlDB.ClaimActualLRP(logger, expectedActualLRP.ProcessGuid, expectedActualLRP.Index, instanceKey)
						Expect(err).To(HaveOccurred())

						actualLRPGroup, err := sqlDB.ActualLRPGroupByProcessGuidAndIndex(logger, expectedActualLRP.ProcessGuid, expectedActualLRP.Index)
						Expect(err).NotTo(HaveOccurred())
						Expect(actualLRPGroup.Instance).To(BeEquivalentTo(expectedActualLRP))
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

					actualLRPGroup, err := sqlDB.ActualLRPGroupByProcessGuidAndIndex(logger, expectedActualLRP.ProcessGuid, expectedActualLRP.Index)
					Expect(err).NotTo(HaveOccurred())
					Expect(actualLRPGroup.Instance.State).To(Equal(models.ActualLRPStateCrashed))
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

					actualLRPGroup, err := sqlDB.ActualLRPGroupByProcessGuidAndIndex(logger, actualLRP.ProcessGuid, actualLRP.Index)
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

					Expect(*actualLRPGroup.Instance).To(BeEquivalentTo(expectedActualLRP))
				})

				It("returns the existing actual lrp", func() {
					beforeActualLRPGroup, afterActualLRPGroup, err := sqlDB.StartActualLRP(logger, &actualLRP.ActualLRPKey, instanceKey, netInfo)
					Expect(err).NotTo(HaveOccurred())
					expectedActualLRP := *actualLRP
					expectedActualLRP.State = models.ActualLRPStateUnclaimed
					expectedActualLRP.Since = fakeClock.Now().Add(-time.Hour).UnixNano()
					expectedActualLRP.ModificationTag = models.ModificationTag{
						Epoch: "my-awesome-guid",
						Index: 0,
					}
					Expect(beforeActualLRPGroup).To(Equal(&models.ActualLRPGroup{Instance: &expectedActualLRP}))

					actualLRPGroup, err := sqlDB.ActualLRPGroupByProcessGuidAndIndex(logger, actualLRP.ProcessGuid, actualLRP.Index)
					Expect(err).NotTo(HaveOccurred())
					Expect(afterActualLRPGroup).To(Equal(actualLRPGroup))
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

					actualLRPGroup, err := sqlDB.ActualLRPGroupByProcessGuidAndIndex(logger, actualLRP.ProcessGuid, actualLRP.Index)
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

					Expect(*actualLRPGroup.Instance).To(BeEquivalentTo(expectedActualLRP))
				})

				It("returns the existing actual lrp", func() {
					beforeActualLRPGroup, afterActualLRPGroup, err := sqlDB.StartActualLRP(logger, &actualLRP.ActualLRPKey, instanceKey, netInfo)
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

					actualLRPGroup, err := sqlDB.ActualLRPGroupByProcessGuidAndIndex(logger, actualLRP.ProcessGuid, actualLRP.Index)
					Expect(err).NotTo(HaveOccurred())

					Expect(beforeActualLRPGroup).To(Equal(&models.ActualLRPGroup{Instance: &expectedActualLRP}))
					Expect(afterActualLRPGroup).To(Equal(actualLRPGroup))
				})

				Context("and the instance key is different", func() {
					It("transitions the state to RUNNING, updating the instance key", func() {
						otherInstanceKey := &models.ActualLRPInstanceKey{CellId: "some-other-cell", InstanceGuid: "some-other-instance-guid"}
						_, _, err := sqlDB.StartActualLRP(logger, &actualLRP.ActualLRPKey, otherInstanceKey, netInfo)
						Expect(err).NotTo(HaveOccurred())

						actualLRPGroup, err := sqlDB.ActualLRPGroupByProcessGuidAndIndex(logger, actualLRP.ProcessGuid, actualLRP.Index)
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

						Expect(*actualLRPGroup.Instance).To(BeEquivalentTo(expectedActualLRP))
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
								beforeActualLRPGroup, err := sqlDB.ActualLRPGroupByProcessGuidAndIndex(logger, actualLRP.ProcessGuid, actualLRP.Index)
								Expect(err).NotTo(HaveOccurred())

								_, _, err = sqlDB.StartActualLRP(logger, &actualLRP.ActualLRPKey, instanceKey, netInfo)
								Expect(err).NotTo(HaveOccurred())

								afterActualLRPGroup, err := sqlDB.ActualLRPGroupByProcessGuidAndIndex(logger, actualLRP.ProcessGuid, actualLRP.Index)
								Expect(err).NotTo(HaveOccurred())

								Expect(beforeActualLRPGroup).To(BeEquivalentTo(afterActualLRPGroup))
							})

							It("returns the same actual lrp group for before and after", func() {
								beforeActualLRPGroup, afterActualLRPGroup, err := sqlDB.StartActualLRP(logger, &actualLRP.ActualLRPKey, instanceKey, netInfo)
								Expect(err).NotTo(HaveOccurred())
								Expect(beforeActualLRPGroup).To(Equal(afterActualLRPGroup))
							})
						})

						Context("and the net info is NOT the same", func() {
							var (
								expectedActualLRPGroup *models.ActualLRPGroup
								newNetInfo             *models.ActualLRPNetInfo
							)

							BeforeEach(func() {
								var err error
								expectedActualLRPGroup, err = sqlDB.ActualLRPGroupByProcessGuidAndIndex(logger, actualLRP.ProcessGuid, actualLRP.Index)
								Expect(err).NotTo(HaveOccurred())
								newNetInfo = &models.ActualLRPNetInfo{Address: "some-other-address"}
							})

							It("updates the net info", func() {
								beforeActualLRPGroup, afterActualLRPGroup, err := sqlDB.StartActualLRP(logger, &actualLRP.ActualLRPKey, instanceKey, newNetInfo)
								Expect(err).NotTo(HaveOccurred())

								Expect(beforeActualLRPGroup).To(Equal(expectedActualLRPGroup))

								actualLRPGroup, err := sqlDB.ActualLRPGroupByProcessGuidAndIndex(logger, actualLRP.ProcessGuid, actualLRP.Index)
								Expect(err).NotTo(HaveOccurred())

								expectedActualLRPGroup.Instance.ActualLRPNetInfo = *newNetInfo
								expectedActualLRPGroup.Instance.ModificationTag.Increment()
								Expect(actualLRPGroup).To(BeEquivalentTo(expectedActualLRPGroup))
								Expect(afterActualLRPGroup).To(Equal(actualLRPGroup))
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
						beforeActualLRPGroup, afterActualLRPGroup, err := sqlDB.StartActualLRP(logger, &actualLRP.ActualLRPKey, instanceKey, netInfo)
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
						Expect(beforeActualLRPGroup).To(Equal(&models.ActualLRPGroup{Instance: &expectedBeforeActualLRP}))

						fetchedActualLRPGroup, err := sqlDB.ActualLRPGroupByProcessGuidAndIndex(logger, actualLRP.ProcessGuid, actualLRP.Index)
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

						Expect(fetchedActualLRPGroup.Instance).To(BeEquivalentTo(&expectedAfterActualLRP))
						Expect(afterActualLRPGroup).To(BeEquivalentTo(fetchedActualLRPGroup))
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
				beforeActualLRPGroup, afterActualLRPGroup, err := sqlDB.StartActualLRP(logger, &actualLRP.ActualLRPKey, instanceKey, netInfo)
				Expect(err).NotTo(HaveOccurred())
				Expect(beforeActualLRPGroup).To(Equal(&models.ActualLRPGroup{Instance: &models.ActualLRP{}}))

				fetchedActualLRPGroup, err := sqlDB.ActualLRPGroupByProcessGuidAndIndex(logger, actualLRP.ProcessGuid, actualLRP.Index)
				Expect(err).NotTo(HaveOccurred())

				expectedActualLRP := *actualLRP
				expectedActualLRP.State = models.ActualLRPStateRunning
				expectedActualLRP.ActualLRPNetInfo = *netInfo
				expectedActualLRP.ActualLRPInstanceKey = *instanceKey
				expectedActualLRP.Since = fakeClock.Now().UnixNano()

				Expect(*fetchedActualLRPGroup.Instance).To(BeEquivalentTo(expectedActualLRP))
				Expect(afterActualLRPGroup).To(BeEquivalentTo(fetchedActualLRPGroup))
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
					beforeActualLRPGroup, afterActualLRPGroup, _, err := sqlDB.CrashActualLRP(logger, &actualLRP.ActualLRPKey, instanceKey, "because it didn't go well")
					Expect(err).NotTo(HaveOccurred())

					expectedActualLRP := *actualLRP
					expectedActualLRP.State = models.ActualLRPStateRunning
					expectedActualLRP.ActualLRPInstanceKey = *instanceKey
					expectedActualLRP.Since = fakeClock.Now().UnixNano()
					expectedActualLRP.ActualLRPNetInfo = *netInfo

					Expect(beforeActualLRPGroup).To(Equal(&models.ActualLRPGroup{Instance: &expectedActualLRP}))

					actualLRPGroup, err := sqlDB.ActualLRPGroupByProcessGuidAndIndex(logger, actualLRP.ProcessGuid, actualLRP.Index)
					Expect(err).NotTo(HaveOccurred())

					Expect(afterActualLRPGroup).To(Equal(actualLRPGroup))
				})

				Context("and the crash reason is larger than 1K", func() {
					It("truncates the crash reason", func() {
						crashReason := strings.Repeat("x", 2*1024)
						_, _, _, err := sqlDB.CrashActualLRP(logger, &actualLRP.ActualLRPKey, instanceKey, crashReason)
						Expect(err).NotTo(HaveOccurred())

						actualLRPGroup, err := sqlDB.ActualLRPGroupByProcessGuidAndIndex(logger, actualLRP.ProcessGuid, actualLRP.Index)
						Expect(err).NotTo(HaveOccurred())

						expectedActualLRP := *actualLRP
						expectedActualLRP.State = models.ActualLRPStateUnclaimed
						expectedActualLRP.CrashCount = 1
						expectedActualLRP.CrashReason = crashReason[:1024]
						expectedActualLRP.ModificationTag.Increment()
						expectedActualLRP.Since = fakeClock.Now().UnixNano()

						Expect(*actualLRPGroup.Instance).To(BeEquivalentTo(expectedActualLRP))
					})
				})

				Context("and it should be restarted", func() {
					It("updates the lrp and sets its state to UNCLAIMED", func() {
						_, _, shouldRestart, err := sqlDB.CrashActualLRP(logger, &actualLRP.ActualLRPKey, instanceKey, "because it didn't go well")
						Expect(err).NotTo(HaveOccurred())
						Expect(shouldRestart).To(BeTrue())

						actualLRPGroup, err := sqlDB.ActualLRPGroupByProcessGuidAndIndex(logger, actualLRP.ProcessGuid, actualLRP.Index)
						Expect(err).NotTo(HaveOccurred())

						expectedActualLRP := *actualLRP
						expectedActualLRP.State = models.ActualLRPStateUnclaimed
						expectedActualLRP.CrashCount = 1
						expectedActualLRP.CrashReason = "because it didn't go well"
						expectedActualLRP.ModificationTag.Increment()
						expectedActualLRP.Since = fakeClock.Now().UnixNano()

						Expect(*actualLRPGroup.Instance).To(BeEquivalentTo(expectedActualLRP))
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

						actualLRPGroup, err := sqlDB.ActualLRPGroupByProcessGuidAndIndex(logger, actualLRP.ProcessGuid, actualLRP.Index)
						Expect(err).NotTo(HaveOccurred())

						expectedActualLRP := *actualLRP
						expectedActualLRP.State = models.ActualLRPStateCrashed
						expectedActualLRP.CrashCount = models.DefaultImmediateRestarts + 2
						expectedActualLRP.CrashReason = "because it didn't go well"
						expectedActualLRP.ModificationTag.Increment()
						expectedActualLRP.Since = fakeClock.Now().UnixNano()

						Expect(*actualLRPGroup.Instance).To(BeEquivalentTo(expectedActualLRP))
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

							actualLRPGroup, err := sqlDB.ActualLRPGroupByProcessGuidAndIndex(logger, actualLRP.ProcessGuid, actualLRP.Index)
							Expect(err).NotTo(HaveOccurred())

							Expect(actualLRPGroup.Instance.CrashCount).To(BeNumerically("==", 1))
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
					beforeActualLRPGroup, afterActualLRPGroup, _, err := sqlDB.CrashActualLRP(logger, &actualLRP.ActualLRPKey, instanceKey, "because it didn't go well")
					Expect(err).NotTo(HaveOccurred())

					expectedActualLRP := *actualLRP
					expectedActualLRP.State = models.ActualLRPStateClaimed
					expectedActualLRP.ActualLRPInstanceKey = *instanceKey
					expectedActualLRP.Since = fakeClock.Now().Add(-time.Hour).UnixNano()

					Expect(beforeActualLRPGroup).To(Equal(&models.ActualLRPGroup{Instance: &expectedActualLRP}))

					actualLRPGroup, err := sqlDB.ActualLRPGroupByProcessGuidAndIndex(logger, actualLRP.ProcessGuid, actualLRP.Index)
					Expect(err).NotTo(HaveOccurred())
					Expect(afterActualLRPGroup).To(Equal(actualLRPGroup))
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

						actualLRPGroup, err := sqlDB.ActualLRPGroupByProcessGuidAndIndex(logger, actualLRP.ProcessGuid, actualLRP.Index)
						Expect(err).NotTo(HaveOccurred())

						expectedActualLRP := *actualLRP
						expectedActualLRP.State = models.ActualLRPStateUnclaimed
						expectedActualLRP.CrashCount = models.DefaultImmediateRestarts - 1
						expectedActualLRP.CrashReason = "because it didn't go well"
						expectedActualLRP.ModificationTag.Increment()
						expectedActualLRP.Since = fakeClock.Now().UnixNano()

						Expect(*actualLRPGroup.Instance).To(BeEquivalentTo(expectedActualLRP))
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

						actualLRPGroup, err := sqlDB.ActualLRPGroupByProcessGuidAndIndex(logger, actualLRP.ProcessGuid, actualLRP.Index)
						Expect(err).NotTo(HaveOccurred())

						expectedActualLRP := *actualLRP
						expectedActualLRP.State = models.ActualLRPStateCrashed
						expectedActualLRP.CrashCount = models.DefaultImmediateRestarts + 3
						expectedActualLRP.CrashReason = "some other failure reason"
						expectedActualLRP.ModificationTag.Increment()
						expectedActualLRP.Since = fakeClock.Now().UnixNano()

						Expect(*actualLRPGroup.Instance).To(BeEquivalentTo(expectedActualLRP))
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

					actualLRPGroup, err := sqlDB.ActualLRPGroupByProcessGuidAndIndex(logger, actualLRP.ProcessGuid, actualLRP.Index)
					Expect(err).NotTo(HaveOccurred())

					expectedActualLRP := *actualLRP
					expectedActualLRP.State = models.ActualLRPStateUnclaimed
					expectedActualLRP.PlacementError = "failing the LRP"
					expectedActualLRP.Since = fakeClock.Now().UnixNano()
					expectedActualLRP.ModificationTag = models.ModificationTag{
						Epoch: "my-awesome-guid",
						Index: 1,
					}

					Expect(*actualLRPGroup.Instance).To(BeEquivalentTo(expectedActualLRP))
				})

				Context("and the placement error is longer than 1K", func() {
					It("truncates the placement_error", func() {
						value := strings.Repeat("x", 2*1024)
						_, _, err := sqlDB.FailActualLRP(logger, &actualLRP.ActualLRPKey, value)
						Expect(err).NotTo(HaveOccurred())

						actualLRPGroup, err := sqlDB.ActualLRPGroupByProcessGuidAndIndex(logger, actualLRP.ProcessGuid, actualLRP.Index)
						Expect(err).NotTo(HaveOccurred())

						expectedActualLRP := *actualLRP
						expectedActualLRP.State = models.ActualLRPStateUnclaimed
						expectedActualLRP.PlacementError = value[:1024]
						expectedActualLRP.Since = fakeClock.Now().UnixNano()
						expectedActualLRP.ModificationTag = models.ModificationTag{
							Epoch: "my-awesome-guid",
							Index: 1,
						}

						Expect(*actualLRPGroup.Instance).To(BeEquivalentTo(expectedActualLRP))
					})
				})

				It("returns the previous and current actual lrp", func() {
					beforeActualLRP, afterActualLRP, err := sqlDB.FailActualLRP(logger, &actualLRP.ActualLRPKey, "failing the LRP")
					Expect(err).NotTo(HaveOccurred())

					actualLRPGroup, err := sqlDB.ActualLRPGroupByProcessGuidAndIndex(logger, actualLRP.ProcessGuid, actualLRP.Index)
					Expect(err).NotTo(HaveOccurred())

					expectedActualLRP := *actualLRP
					expectedActualLRP.State = models.ActualLRPStateUnclaimed
					expectedActualLRP.Since = fakeClock.Now().Add(-time.Hour).UnixNano()
					expectedActualLRP.ModificationTag = models.ModificationTag{
						Epoch: "my-awesome-guid",
						Index: 0,
					}
					Expect(beforeActualLRP).To(Equal(&models.ActualLRPGroup{Instance: &expectedActualLRP}))
					Expect(afterActualLRP).To(Equal(actualLRPGroup))
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

				_, err = sqlDB.ActualLRPGroupByProcessGuidAndIndex(logger, actualLRP.ProcessGuid, actualLRP.Index)
				Expect(err).To(HaveOccurred())
				Expect(err).To(Equal(models.ErrResourceNotFound))
			})

			It("keeps the other lrps around", func() {
				err := sqlDB.RemoveActualLRP(logger, actualLRP.ProcessGuid, actualLRP.Index, nil)
				Expect(err).NotTo(HaveOccurred())

				_, err = sqlDB.ActualLRPGroupByProcessGuidAndIndex(logger, otherActualLRPKey.ProcessGuid, otherActualLRPKey.Index)
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

						_, err = sqlDB.ActualLRPGroupByProcessGuidAndIndex(logger, actualLRP.ProcessGuid, actualLRP.Index)
						Expect(err).To(HaveOccurred())
						Expect(err).To(Equal(models.ErrResourceNotFound))
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
				var beforeActualLRPGroup, afterActualLRPGroup *models.ActualLRPGroup

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
					beforeActualLRPGroup, afterActualLRPGroup, err = sqlDB.UnclaimActualLRP(logger, actualLRPKey)
					Expect(err).ToNot(HaveOccurred())
				})

				It("unclaims the actual LRP", func() {
					actualLRPGroup, err := sqlDB.ActualLRPGroupByProcessGuidAndIndex(logger, guid, index)
					Expect(err).ToNot(HaveOccurred())
					Expect(actualLRPGroup.Instance.State).To(Equal(models.ActualLRPStateUnclaimed))
				})

				It("it removes the net info from the actualLRP", func() {
					actualLRPGroup, err := sqlDB.ActualLRPGroupByProcessGuidAndIndex(logger, guid, index)
					Expect(err).ToNot(HaveOccurred())
					Expect(actualLRPGroup.Instance.ActualLRPNetInfo).To(Equal(models.ActualLRPNetInfo{}))
				})

				It("it increments the modification tag on the actualLRP", func() {
					actualLRPGroup, err := sqlDB.ActualLRPGroupByProcessGuidAndIndex(logger, guid, index)
					Expect(err).ToNot(HaveOccurred())
					// +2 because of claim AND unclaim
					Expect(actualLRPGroup.Instance.ModificationTag.Index).To(Equal(actualLRP.ModificationTag.Index + uint32(2)))
				})

				It("it clears the actualLRP's instance key", func() {
					actualLRPGroup, err := sqlDB.ActualLRPGroupByProcessGuidAndIndex(logger, guid, index)
					Expect(err).ToNot(HaveOccurred())
					Expect(actualLRPGroup.Instance.ActualLRPInstanceKey).To(Equal(models.ActualLRPInstanceKey{}))
				})

				It("it updates the actualLRP's update at timestamp", func() {
					actualLRPGroup, err := sqlDB.ActualLRPGroupByProcessGuidAndIndex(logger, guid, index)
					Expect(err).ToNot(HaveOccurred())
					Expect(actualLRPGroup.Instance.Since).To(BeNumerically(">", actualLRP.Since))
				})

				It("returns the previous and current actual lrp", func() {
					expectedActualLRP := *actualLRP
					expectedActualLRP.State = models.ActualLRPStateClaimed
					expectedActualLRP.Since = fakeClock.Now().UnixNano()
					expectedActualLRP.ModificationTag = models.ModificationTag{
						Epoch: "my-awesome-guid",
						Index: 1,
					}
					Expect(beforeActualLRPGroup).To(BeEquivalentTo(&models.ActualLRPGroup{Instance: &expectedActualLRP}))

					actualLRPGroup, err := sqlDB.ActualLRPGroupByProcessGuidAndIndex(logger, guid, index)
					Expect(err).ToNot(HaveOccurred())
					Expect(afterActualLRPGroup).To(BeEquivalentTo(actualLRPGroup))
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
