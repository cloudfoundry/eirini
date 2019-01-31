package sqldb_test

import (
	"fmt"
	"time"

	"code.cloudfoundry.org/bbs/models"
	"code.cloudfoundry.org/bbs/models/test/model_helpers"
	"code.cloudfoundry.org/bbs/test_helpers"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
)

var _ = Describe("LRPConvergence", func() {
	actualLRPKeyWithSchedulingInfo := func(desiredLRP *models.DesiredLRP, index int) *models.ActualLRPKeyWithSchedulingInfo {
		schedulingInfo := desiredLRP.DesiredLRPSchedulingInfo()
		lrpKey := models.NewActualLRPKey(desiredLRP.ProcessGuid, int32(index), desiredLRP.Domain)

		lrp := &models.ActualLRPKeyWithSchedulingInfo{
			Key:            &lrpKey,
			SchedulingInfo: &schedulingInfo,
		}
		return lrp
	}

	var (
		cellSet models.CellSet
	)

	BeforeEach(func() {
		cellSet = models.NewCellSetFromList([]*models.CellPresence{
			{CellId: "existing-cell"},
		})
	})

	Describe("pruning evacuating lrps", func() {
		var (
			processGuid, domain string
		)

		BeforeEach(func() {
			domain = "some-domain"
			processGuid = "desired-with-evacuating-actual"
			desiredLRP := model_helpers.NewValidDesiredLRP(processGuid)
			desiredLRP.Domain = domain
			desiredLRP.Instances = 2
			err := sqlDB.DesireLRP(logger, desiredLRP)
			Expect(err).NotTo(HaveOccurred())
			_, err = sqlDB.CreateUnclaimedActualLRP(logger, &models.ActualLRPKey{ProcessGuid: processGuid, Index: 0, Domain: domain})
			Expect(err).NotTo(HaveOccurred())
			_, _, err = sqlDB.ClaimActualLRP(logger, processGuid, 0, &models.ActualLRPInstanceKey{InstanceGuid: "ig-1", CellId: "existing-cell"})
			Expect(err).NotTo(HaveOccurred())

			_, err = db.Exec(fmt.Sprintf(`UPDATE actual_lrps SET presence = %d`, models.ActualLRP_Evacuating))
			Expect(err).NotTo(HaveOccurred())
		})

		Context("when the cell is present", func() {
			It("keeps evacuating actual lrps with available cells", func() {
				sqlDB.ConvergeLRPs(logger, cellSet)

				lrps, err := sqlDB.ActualLRPs(logger, models.ActualLRPFilter{ProcessGuid: processGuid})
				Expect(err).NotTo(HaveOccurred())

				Expect(lrps).To(HaveLen(1))
			})
		})

		Context("when the cell isn't present", func() {
			BeforeEach(func() {
				cellSet = models.NewCellSet()
			})

			It("clears out evacuating actual lrps with missing cells", func() {
				sqlDB.ConvergeLRPs(logger, cellSet)

				lrps, err := sqlDB.ActualLRPs(logger, models.ActualLRPFilter{ProcessGuid: processGuid})
				Expect(err).NotTo(HaveOccurred())
				Expect(lrps).To(BeEmpty())
			})

			It("return an ActualLRPRemovedEvent", func() {
				actualLRPs, err := sqlDB.ActualLRPs(logger, models.ActualLRPFilter{ProcessGuid: processGuid})
				Expect(err).NotTo(HaveOccurred())

				Expect(actualLRPs).To(HaveLen(1))

				result := sqlDB.ConvergeLRPs(logger, cellSet)
				Expect(result.Events).To(ContainElement(models.NewActualLRPRemovedEvent(actualLRPs[0].ToActualLRPGroup())))
				Expect(result.InstanceEvents).To(ContainElement(models.NewActualLRPInstanceRemovedEvent(actualLRPs[0])))
			})
		})
	})

	Context("when there are expired domains", func() {
		var (
			expiredDomain = "expired-domain"
		)

		BeforeEach(func() {
			fakeClock.Increment(-10 * time.Second)
			sqlDB.UpsertDomain(logger, expiredDomain, 5)
			fakeClock.Increment(10 * time.Second)
		})

		It("clears out expired domains", func() {
			fetchDomains := func() []string {
				rows, err := db.Query("SELECT domain FROM domains")
				Expect(err).NotTo(HaveOccurred())
				defer rows.Close()

				var domain string
				var results []string
				for rows.Next() {
					err = rows.Scan(&domain)
					Expect(err).NotTo(HaveOccurred())
					results = append(results, domain)
				}
				return results
			}

			Expect(fetchDomains()).To(ContainElement(expiredDomain))

			sqlDB.ConvergeLRPs(logger, cellSet)

			Expect(fetchDomains()).NotTo(ContainElement(expiredDomain))
		})

		It("logs the expired domains", func() {
			sqlDB.ConvergeLRPs(logger, cellSet)
			Eventually(logger).Should(gbytes.Say("pruning-domain.*expired-domain"))
		})
	})

	Context("when there are unclaimed LRPs", func() {
		var (
			domain      string
			processGuid string
		)

		BeforeEach(func() {
			domain = "some-domain"
			processGuid = "desired-with-unclaimed-actuals"
			desiredLRPWithStaleActuals := model_helpers.NewValidDesiredLRP(processGuid)
			desiredLRPWithStaleActuals.Domain = domain
			desiredLRPWithStaleActuals.Instances = 1
			err := sqlDB.DesireLRP(logger, desiredLRPWithStaleActuals)
			Expect(err).NotTo(HaveOccurred())
			_, err = sqlDB.CreateUnclaimedActualLRP(logger, &models.ActualLRPKey{ProcessGuid: processGuid, Index: 0, Domain: domain})
			Expect(err).NotTo(HaveOccurred())
		})

		Context("when the domain is fresh", func() {
			BeforeEach(func() {
				Expect(sqlDB.UpsertDomain(logger, domain, 5)).To(Succeed())
			})

			It("does not touch the ActualLRPs in the database", func() {
				lrpsBefore, err := sqlDB.ActualLRPs(logger, models.ActualLRPFilter{ProcessGuid: processGuid})
				Expect(err).NotTo(HaveOccurred())

				sqlDB.ConvergeLRPs(logger, cellSet)

				lrpsAfter, err := sqlDB.ActualLRPs(logger, models.ActualLRPFilter{ProcessGuid: processGuid})
				Expect(err).NotTo(HaveOccurred())

				Expect(lrpsAfter).To(Equal(lrpsBefore))
			})

			It("returns an empty convergence result", func() {
				result := sqlDB.ConvergeLRPs(logger, cellSet)
				Expect(result).To(BeZero())
			})
		})

		Context("when the ActualLRP's presence is set to evacuating", func() {
			BeforeEach(func() {
				Expect(sqlDB.UpsertDomain(logger, domain, 5)).To(Succeed())

				queryStr := `UPDATE actual_lrps SET presence = ? WHERE process_guid = ?`
				if test_helpers.UsePostgres() {
					queryStr = test_helpers.ReplaceQuestionMarks(queryStr)
				}
				_, err := db.Exec(queryStr, models.ActualLRP_Evacuating, processGuid)
				Expect(err).NotTo(HaveOccurred())
			})

			It("ignores the evacuating LRPs and sets missing LRPs to the correct value", func() {
				schedulingInfos, err := sqlDB.DesiredLRPSchedulingInfos(logger, models.DesiredLRPFilter{ProcessGuids: []string{processGuid}})
				Expect(err).NotTo(HaveOccurred())

				results := sqlDB.ConvergeLRPs(logger, cellSet)
				Expect(results.MissingLRPKeys).To(ConsistOf(
					&models.ActualLRPKeyWithSchedulingInfo{
						Key:            &models.ActualLRPKey{ProcessGuid: processGuid, Index: 0, Domain: domain},
						SchedulingInfo: schedulingInfos[0],
					},
				))
			})

			It("removes the evacuating lrps", func() {
				sqlDB.ConvergeLRPs(logger, cellSet)

				actualLRPs, err := sqlDB.ActualLRPs(logger, models.ActualLRPFilter{ProcessGuid: processGuid})
				Expect(err).NotTo(HaveOccurred())
				Expect(actualLRPs).To(BeEmpty())
			})

			It("return ActualLRPRemoveEvent", func() {
				actualLRPs, err := sqlDB.ActualLRPs(logger, models.ActualLRPFilter{ProcessGuid: processGuid})
				Expect(err).NotTo(HaveOccurred())

				Expect(actualLRPs).To(HaveLen(1))

				result := sqlDB.ConvergeLRPs(logger, cellSet)
				Expect(result.Events).To(ConsistOf(models.NewActualLRPRemovedEvent(actualLRPs[0].ToActualLRPGroup())))
				Expect(result.InstanceEvents).To(ConsistOf(models.NewActualLRPInstanceRemovedEvent(actualLRPs[0])))
			})
		})
	})

	Context("when the cellset is empty", func() {
		var (
			processGuid, domain string
			lrpKey              models.ActualLRPKey
		)

		BeforeEach(func() {
			// add suspect and ordinary lrps that are running on different cells
			domain = "some-domain"
			processGuid = "desired-with-suspect-and-running-actual"
			desiredLRP := model_helpers.NewValidDesiredLRP(processGuid)
			desiredLRP.Domain = domain
			err := sqlDB.DesireLRP(logger, desiredLRP)
			Expect(err).NotTo(HaveOccurred())

			// create the suspect lrp
			actualLRPNetInfo := models.NewActualLRPNetInfo("some-address", "container-address", models.NewPortMapping(2222, 4444))
			lrpKey = models.NewActualLRPKey(processGuid, 0, domain)
			_, _, err = sqlDB.StartActualLRP(logger, &lrpKey, &models.ActualLRPInstanceKey{InstanceGuid: "ig-1", CellId: "suspect-cell"}, &actualLRPNetInfo)
			Expect(err).NotTo(HaveOccurred())
			_, _, err = sqlDB.ChangeActualLRPPresence(logger, &lrpKey, models.ActualLRP_Ordinary, models.ActualLRP_Suspect)
			Expect(err).NotTo(HaveOccurred())
		})

		It("returns no LRP key in the SuspectKeysWithExistingCells", func() {
			result := sqlDB.ConvergeLRPs(logger, models.NewCellSet())
			Expect(result.SuspectKeysWithExistingCells).To(BeEmpty())
		})

		Context("and there is an unclaimed Ordinary LRP", func() {
			BeforeEach(func() {
				_, err := sqlDB.CreateUnclaimedActualLRP(logger, &lrpKey)
				Expect(err).NotTo(HaveOccurred())
			})

			It("returns no KeysWithMissingCells", func() {
				result := sqlDB.ConvergeLRPs(logger, models.NewCellSet())
				Expect(result.KeysWithMissingCells).To(BeEmpty())

			})
		})
	})

	Context("when there is a suspect LRP with an existing cell", func() {
		var (
			processGuid, domain string
			lrpKey              models.ActualLRPKey
			lrpKey2             models.ActualLRPKey
		)

		BeforeEach(func() {
			// add suspect and ordinary lrps that are running on different cells
			domain = "some-domain"
			processGuid = "desired-with-suspect-and-running-actual"
			desiredLRP := model_helpers.NewValidDesiredLRP(processGuid)
			desiredLRP.Domain = domain
			desiredLRP.Instances = 2
			err := sqlDB.DesireLRP(logger, desiredLRP)
			Expect(err).NotTo(HaveOccurred())

			// create the suspect lrp
			actualLRPNetInfo := models.NewActualLRPNetInfo("some-address", "container-address", models.NewPortMapping(2222, 4444))
			lrpKey = models.NewActualLRPKey(processGuid, 0, domain)
			_, _, err = sqlDB.StartActualLRP(logger, &lrpKey, &models.ActualLRPInstanceKey{InstanceGuid: "ig-1", CellId: "existing-cell"}, &actualLRPNetInfo)
			Expect(err).NotTo(HaveOccurred())
			_, _, err = sqlDB.ChangeActualLRPPresence(logger, &lrpKey, models.ActualLRP_Ordinary, models.ActualLRP_Suspect)
			Expect(err).NotTo(HaveOccurred())

			// create the second suspect lrp
			lrpKey2 = models.NewActualLRPKey(processGuid, 1, domain)
			_, _, err = sqlDB.StartActualLRP(logger, &lrpKey2, &models.ActualLRPInstanceKey{InstanceGuid: "ig-2", CellId: "suspect-cell"}, &actualLRPNetInfo)
			Expect(err).NotTo(HaveOccurred())
			_, _, err = sqlDB.ChangeActualLRPPresence(logger, &lrpKey2, models.ActualLRP_Ordinary, models.ActualLRP_Suspect)
			Expect(err).NotTo(HaveOccurred())
		})

		It("returns the LRP key with the existing cell in the SuspectKeysWithExistingCells", func() {
			result := sqlDB.ConvergeLRPs(logger, cellSet)
			Expect(result.SuspectKeysWithExistingCells).To(ConsistOf(&lrpKey))
		})

		It("returns all suspect running LRP keys in the SuspectRunningKeys", func() {
			result := sqlDB.ConvergeLRPs(logger, cellSet)
			Expect(result.SuspectRunningKeys).To(ConsistOf(&lrpKey, &lrpKey2))
		})
	})

	Context("when there is a suspect LRP and ordinary LRP present", func() {
		var (
			processGuid, domain string
			lrpKey              models.ActualLRPKey
			lrpKey2             models.ActualLRPKey
		)

		BeforeEach(func() {
			cellSet = models.NewCellSetFromList([]*models.CellPresence{
				{CellId: "existing-cell"},
			})

			// add suspect and ordinary lrps that are running on different cells
			domain = "some-domain"
			processGuid = "desired-with-suspect-and-running-actual"
			desiredLRP := model_helpers.NewValidDesiredLRP(processGuid)
			desiredLRP.Domain = domain
			err := sqlDB.DesireLRP(logger, desiredLRP)
			Expect(err).NotTo(HaveOccurred())

			// create the suspect lrp
			actualLRPNetInfo := models.NewActualLRPNetInfo("some-address", "container-address", models.NewPortMapping(2222, 4444))
			lrpKey = models.NewActualLRPKey(processGuid, 0, domain)
			_, _, err = sqlDB.StartActualLRP(logger, &lrpKey, &models.ActualLRPInstanceKey{InstanceGuid: "ig-1", CellId: "suspect-cell"}, &actualLRPNetInfo)
			Expect(err).NotTo(HaveOccurred())
			_, err = db.Exec(fmt.Sprintf(`UPDATE actual_lrps SET presence = %d`, models.ActualLRP_Suspect))
			Expect(err).NotTo(HaveOccurred())

			// create the ordinary lrp
			_, _, err = sqlDB.StartActualLRP(logger, &lrpKey, &models.ActualLRPInstanceKey{InstanceGuid: "ig-2", CellId: "existing-cell"}, &actualLRPNetInfo)
			Expect(err).NotTo(HaveOccurred())

			// create the unrelated suspect lrp
			processGuid2 := "other-process-guid"
			desiredLRP2 := model_helpers.NewValidDesiredLRP(processGuid2)
			desiredLRP.Domain = domain
			err = sqlDB.DesireLRP(logger, desiredLRP2)
			Expect(err).NotTo(HaveOccurred())
			lrpKey2 = models.NewActualLRPKey(processGuid2, 1, domain)
			_, _, err = sqlDB.StartActualLRP(logger, &lrpKey2, &models.ActualLRPInstanceKey{InstanceGuid: "ig-2", CellId: "suspect-cell"}, &actualLRPNetInfo)
			Expect(err).NotTo(HaveOccurred())
			_, _, err = sqlDB.ChangeActualLRPPresence(logger, &lrpKey2, models.ActualLRP_Ordinary, models.ActualLRP_Suspect)
			Expect(err).NotTo(HaveOccurred())
		})

		It("should return the suspect lrp key in the SuspectLRPKeysToRetire", func() {
			result := sqlDB.ConvergeLRPs(logger, cellSet)
			Expect(result.SuspectLRPKeysToRetire).To(ConsistOf(&lrpKey))
		})

		It("includes the suspect lrp's cell id in the MissingCellIds", func() {
			result := sqlDB.ConvergeLRPs(logger, cellSet)
			Expect(result.MissingCellIds).To(ContainElement("suspect-cell"))
		})

		It("logs the missing cell", func() {
			sqlDB.ConvergeLRPs(logger, cellSet)
			Expect(logger).To(gbytes.Say(`detected-missing-cells.*cell_ids":\["suspect-cell"\]`))
		})

		It("returns all suspect running LRP keys in the SuspectKeys", func() {
			result := sqlDB.ConvergeLRPs(logger, cellSet)
			Expect(result.SuspectRunningKeys).To(ConsistOf(&lrpKey, &lrpKey2))
		})

		Context("if the ordinary lrp is not running", func() {
			BeforeEach(func() {
				_, _, _, err := sqlDB.CrashActualLRP(logger, &lrpKey, &models.ActualLRPInstanceKey{CellId: "existing-cell", InstanceGuid: "ig-2"}, "booooom!")
				Expect(err).NotTo(HaveOccurred())
			})

			It("does not retire the Suspect LRP", func() {
				result := sqlDB.ConvergeLRPs(logger, cellSet)
				Expect(result.SuspectLRPKeysToRetire).To(BeEmpty())
			})
		})
	})

	Context("when there are orphaned suspect LRPs", func() {
		var (
			lrpKey, lrpKey2, lrpKey3 models.ActualLRPKey
		)

		BeforeEach(func() {
			cellSet = models.NewCellSetFromList([]*models.CellPresence{
				{CellId: "suspect-cell"},
			})

			domain := "some-domain"
			Expect(sqlDB.UpsertDomain(logger, domain, 5)).To(Succeed())

			var err error

			// create the suspect LRP
			actualLRPNetInfo := models.NewActualLRPNetInfo("some-address", "container-address", models.NewPortMapping(2222, 4444))
			processGuid := "orphaned-suspect-lrp-1"
			lrpKey = models.NewActualLRPKey(processGuid, 0, domain)
			_, _, err = sqlDB.StartActualLRP(logger, &lrpKey, &models.ActualLRPInstanceKey{InstanceGuid: "ig-1", CellId: "suspect-cell"}, &actualLRPNetInfo)
			Expect(err).NotTo(HaveOccurred())

			otherProcessGuid := "orphaned-suspect-lrp-2"
			lrpKey2 = models.NewActualLRPKey(otherProcessGuid, 0, domain)
			_, _, err = sqlDB.StartActualLRP(logger, &lrpKey2, &models.ActualLRPInstanceKey{InstanceGuid: "ig-2", CellId: "suspect-cell"}, &actualLRPNetInfo)
			Expect(err).NotTo(HaveOccurred())

			_, err = db.Exec(fmt.Sprintf(`UPDATE actual_lrps SET presence = %d`, models.ActualLRP_Suspect))
			Expect(err).NotTo(HaveOccurred())

			// create suspect LRP that is not orphaned
			notOrphanedProcessGuid := "suspect-lrp-that-is-not-orphaned"
			desiredLRP2 := model_helpers.NewValidDesiredLRP(notOrphanedProcessGuid)
			err = sqlDB.DesireLRP(logger, desiredLRP2)
			Expect(err).NotTo(HaveOccurred())
			lrpKey3 = models.NewActualLRPKey(notOrphanedProcessGuid, 0, domain)
			_, _, err = sqlDB.StartActualLRP(logger, &lrpKey3, &models.ActualLRPInstanceKey{InstanceGuid: "ig-3", CellId: "suspect-cell"}, &actualLRPNetInfo)
			Expect(err).NotTo(HaveOccurred())
			_, _, err = sqlDB.ChangeActualLRPPresence(logger, &lrpKey3, models.ActualLRP_Ordinary, models.ActualLRP_Suspect)
			Expect(err).NotTo(HaveOccurred())
		})

		It("should only return the orphaned suspect lrp key in the SuspectLRPKeysToRetire", func() {
			result := sqlDB.ConvergeLRPs(logger, cellSet)
			Expect(result.SuspectLRPKeysToRetire).To(ConsistOf(&lrpKey, &lrpKey2))
		})
	})

	Context("when there are claimed LRPs", func() {
		var (
			domain string
			lrpKey models.ActualLRPKey
		)

		BeforeEach(func() {
			domain = "some-domain"
			lrpKey = models.ActualLRPKey{ProcessGuid: "desired-with-claimed-actuals", Index: 0, Domain: domain}
			desiredLRPWithStaleActuals := model_helpers.NewValidDesiredLRP(lrpKey.ProcessGuid)
			desiredLRPWithStaleActuals.Domain = domain
			desiredLRPWithStaleActuals.Instances = 1
			err := sqlDB.DesireLRP(logger, desiredLRPWithStaleActuals)
			Expect(err).NotTo(HaveOccurred())
			_, err = sqlDB.CreateUnclaimedActualLRP(logger, &lrpKey)
			Expect(err).NotTo(HaveOccurred())
			_, _, err = sqlDB.ClaimActualLRP(logger, lrpKey.ProcessGuid, lrpKey.Index, &models.ActualLRPInstanceKey{InstanceGuid: "instance-guid", CellId: "existing-cell"})
			Expect(err).NotTo(HaveOccurred())
		})

		Context("when the domain is fresh", func() {
			BeforeEach(func() {
				Expect(sqlDB.UpsertDomain(logger, domain, 5)).To(Succeed())
			})

			It("does not retire the extra lrps", func() {
				result := sqlDB.ConvergeLRPs(logger, cellSet)
				Expect(result.KeysToRetire).To(BeEmpty())
			})

			It("does not touch the ActualLRPs in the database", func() {
				lrpsBefore, err := sqlDB.ActualLRPs(logger, models.ActualLRPFilter{ProcessGuid: lrpKey.ProcessGuid})
				Expect(err).NotTo(HaveOccurred())

				sqlDB.ConvergeLRPs(logger, cellSet)

				lrpsAfter, err := sqlDB.ActualLRPs(logger, models.ActualLRPFilter{ProcessGuid: lrpKey.ProcessGuid})
				Expect(err).NotTo(HaveOccurred())

				Expect(lrpsAfter).To(Equal(lrpsBefore))
			})

			Context("when the LRP is suspect", func() {
				BeforeEach(func() {
					_, _, err := sqlDB.ChangeActualLRPPresence(logger, &lrpKey, models.ActualLRP_Ordinary, models.ActualLRP_Suspect)
					Expect(err).NotTo(HaveOccurred())
				})
				It("returns the suspect claimed ActualLRP in SuspectClaimedKeys", func() {
					result := sqlDB.ConvergeLRPs(logger, cellSet)
					Expect(result.SuspectClaimedKeys).To(ConsistOf(&lrpKey))
				})
			})
		})
	})

	Context("when there are stale unclaimed LRPs", func() {
		var (
			domain      string
			processGuid string
		)

		BeforeEach(func() {
			domain = "some-domain"
			processGuid = "desired-with-stale-actuals"
			desiredLRPWithStaleActuals := model_helpers.NewValidDesiredLRP(processGuid)
			desiredLRPWithStaleActuals.Domain = domain
			desiredLRPWithStaleActuals.Instances = 2
			err := sqlDB.DesireLRP(logger, desiredLRPWithStaleActuals)
			Expect(err).NotTo(HaveOccurred())
			fakeClock.Increment(-models.StaleUnclaimedActualLRPDuration)
			_, err = sqlDB.CreateUnclaimedActualLRP(logger, &models.ActualLRPKey{ProcessGuid: processGuid, Index: 0, Domain: domain})
			Expect(err).NotTo(HaveOccurred())
			_, err = sqlDB.CreateUnclaimedActualLRP(logger, &models.ActualLRPKey{ProcessGuid: processGuid, Index: 1, Domain: domain})
			Expect(err).NotTo(HaveOccurred())
			fakeClock.Increment(models.StaleUnclaimedActualLRPDuration + 2)
		})

		Context("when the domain is fresh", func() {
			BeforeEach(func() {
				Expect(sqlDB.UpsertDomain(logger, domain, 5)).To(Succeed())
			})

			It("returns start requests", func() {
				result := sqlDB.ConvergeLRPs(logger, cellSet)
				unstartedLRPKeys := result.UnstartedLRPKeys
				Expect(unstartedLRPKeys).NotTo(BeEmpty())
				Expect(logger).To(gbytes.Say("creating-start-request.*reason\":\"stale-unclaimed-lrp"))

				desiredLRP, err := sqlDB.DesiredLRPByProcessGuid(logger, processGuid)
				Expect(err).NotTo(HaveOccurred())

				Expect(unstartedLRPKeys).To(ContainElement(actualLRPKeyWithSchedulingInfo(desiredLRP, 0)))
				Expect(unstartedLRPKeys).To(ContainElement(actualLRPKeyWithSchedulingInfo(desiredLRP, 1)))
			})

			It("does not touch the ActualLRPs in the database", func() {
				lrpsBefore, err := sqlDB.ActualLRPs(logger, models.ActualLRPFilter{ProcessGuid: processGuid})
				Expect(err).NotTo(HaveOccurred())

				sqlDB.ConvergeLRPs(logger, cellSet)

				lrpsAfter, err := sqlDB.ActualLRPs(logger, models.ActualLRPFilter{ProcessGuid: processGuid})
				Expect(err).NotTo(HaveOccurred())

				Expect(lrpsAfter).To(Equal(lrpsBefore))
			})

			It("should have the correct number of unclaimed LRP instances", func() {
				sqlDB.ConvergeLRPs(logger, cellSet)
				_, unclaimed, _, _, _ := sqlDB.CountActualLRPsByState(logger)
				Expect(unclaimed).To(Equal(2))
			})
		})

		Context("when the domain is expired", func() {
			BeforeEach(func() {
				fakeClock.Increment(-10 * time.Second)
				Expect(sqlDB.UpsertDomain(logger, domain, 5)).To(Succeed())
				fakeClock.Increment(10 * time.Second)
			})

			It("returns start requests", func() {
				result := sqlDB.ConvergeLRPs(logger, cellSet)
				unstartedLRPKeys := result.UnstartedLRPKeys
				Expect(unstartedLRPKeys).NotTo(BeEmpty())
				Expect(logger).To(gbytes.Say("creating-start-request.*reason\":\"stale-unclaimed-lrp"))

				desiredLRP, err := sqlDB.DesiredLRPByProcessGuid(logger, processGuid)
				Expect(err).NotTo(HaveOccurred())

				Expect(unstartedLRPKeys).To(ContainElement(actualLRPKeyWithSchedulingInfo(desiredLRP, 0)))
				Expect(unstartedLRPKeys).To(ContainElement(actualLRPKeyWithSchedulingInfo(desiredLRP, 1)))
			})

			It("does not touch the ActualLRPs in the database", func() {
				lrpsBefore, err := sqlDB.ActualLRPs(logger, models.ActualLRPFilter{ProcessGuid: processGuid})
				Expect(err).NotTo(HaveOccurred())

				sqlDB.ConvergeLRPs(logger, cellSet)

				lrpsAfter, err := sqlDB.ActualLRPs(logger, models.ActualLRPFilter{ProcessGuid: processGuid})
				Expect(err).NotTo(HaveOccurred())

				Expect(lrpsAfter).To(Equal(lrpsBefore))
			})

			It("should have the correct number of unclaimed LRP instances", func() {
				sqlDB.ConvergeLRPs(logger, cellSet)
				_, unclaimed, _, _, _ := sqlDB.CountActualLRPsByState(logger)
				Expect(unclaimed).To(Equal(2))
			})
		})

		Context("when the ActualLRPs presence is set to evacuating", func() {
			BeforeEach(func() {
				Expect(sqlDB.UpsertDomain(logger, domain, 5)).To(Succeed())

				queryStr := `UPDATE actual_lrps SET presence = ?`
				if test_helpers.UsePostgres() {
					queryStr = test_helpers.ReplaceQuestionMarks(queryStr)
				}
				_, err := db.Exec(queryStr, models.ActualLRP_Evacuating)
				Expect(err).NotTo(HaveOccurred())
			})

			It("ignores the evacuating LRPs and should have the correct number of missing LRPs", func() {
				schedulingInfos, err := sqlDB.DesiredLRPSchedulingInfos(logger, models.DesiredLRPFilter{ProcessGuids: []string{processGuid}})
				Expect(err).NotTo(HaveOccurred())

				results := sqlDB.ConvergeLRPs(logger, cellSet)
				Expect(results.MissingLRPKeys).To(ConsistOf(
					&models.ActualLRPKeyWithSchedulingInfo{
						Key:            &models.ActualLRPKey{ProcessGuid: processGuid, Index: 0, Domain: domain},
						SchedulingInfo: schedulingInfos[0],
					},
					&models.ActualLRPKeyWithSchedulingInfo{
						Key:            &models.ActualLRPKey{ProcessGuid: processGuid, Index: 1, Domain: domain},
						SchedulingInfo: schedulingInfos[0],
					},
				))
			})

			It("returns the lrp keys in the MissingLRPKeys", func() {
				result := sqlDB.ConvergeLRPs(logger, cellSet)

				desiredLRP, err := sqlDB.DesiredLRPByProcessGuid(logger, processGuid)
				Expect(err).NotTo(HaveOccurred())

				expectedSched := desiredLRP.DesiredLRPSchedulingInfo()
				Expect(result.MissingLRPKeys).To(ContainElement(&models.ActualLRPKeyWithSchedulingInfo{
					Key:            &models.ActualLRPKey{ProcessGuid: processGuid, Index: 0, Domain: domain},
					SchedulingInfo: &expectedSched,
				}))
			})

			// it is the responsibility of the caller to create new LRPs
			It("prune the evacuating LRPs and does not create new ones", func() {
				sqlDB.ConvergeLRPs(logger, cellSet)

				lrps, err := sqlDB.ActualLRPs(logger, models.ActualLRPFilter{ProcessGuid: processGuid})
				Expect(err).NotTo(HaveOccurred())
				Expect(lrps).To(BeEmpty())
			})

			It("return ActualLRPRemovedEvent for the removed evacuating LRPs", func() {
				actualLRPs, err := sqlDB.ActualLRPs(logger, models.ActualLRPFilter{ProcessGuid: processGuid})
				Expect(err).NotTo(HaveOccurred())
				Expect(actualLRPs).To(HaveLen(2))

				result := sqlDB.ConvergeLRPs(logger, cellSet)
				Expect(result.Events).To(ConsistOf(
					models.NewActualLRPRemovedEvent(actualLRPs[0].ToActualLRPGroup()),
					models.NewActualLRPRemovedEvent(actualLRPs[1].ToActualLRPGroup()),
				))
				Expect(result.InstanceEvents).To(ConsistOf(
					models.NewActualLRPInstanceRemovedEvent(actualLRPs[0]),
					models.NewActualLRPInstanceRemovedEvent(actualLRPs[1]),
				))
			})
		})
	})

	Context("when there is an ActualLRP on a missing cell", func() {
		var (
			domain      string
			processGuid string
		)

		BeforeEach(func() {
			domain = "some-domain"
			processGuid = "desired-with-missing-cell-actuals"
			desiredLRPWithMissingCellActuals := model_helpers.NewValidDesiredLRP(processGuid)
			desiredLRPWithMissingCellActuals.Domain = domain
			err := sqlDB.DesireLRP(logger, desiredLRPWithMissingCellActuals)
			Expect(err).NotTo(HaveOccurred())
			_, err = sqlDB.CreateUnclaimedActualLRP(logger, &models.ActualLRPKey{ProcessGuid: processGuid, Index: 0, Domain: domain})
			Expect(err).NotTo(HaveOccurred())
			_, _, err = sqlDB.ClaimActualLRP(logger, processGuid, 0, &models.ActualLRPInstanceKey{InstanceGuid: "actual-with-missing-cell", CellId: "other-cell"})
			Expect(err).NotTo(HaveOccurred())
		})

		Context("when the domain is fresh", func() {
			BeforeEach(func() {
				Expect(sqlDB.UpsertDomain(logger, domain, 5)).To(Succeed())
			})

			It("returns the start requests, actual lrp keys for actuals with missing cells and missing cell ids", func() {
				result := sqlDB.ConvergeLRPs(logger, cellSet)
				keysWithMissingCells := result.KeysWithMissingCells
				missingCellIds := result.MissingCellIds

				desiredLRP, err := sqlDB.DesiredLRPByProcessGuid(logger, processGuid)
				Expect(err).NotTo(HaveOccurred())

				index := int32(0)
				actualLRPs, err := sqlDB.ActualLRPs(logger, models.ActualLRPFilter{ProcessGuid: processGuid, Index: &index})
				Expect(err).NotTo(HaveOccurred())
				expectedSched := desiredLRP.DesiredLRPSchedulingInfo()
				Expect(actualLRPs).To(HaveLen(1))
				Expect(keysWithMissingCells).To(ContainElement(&models.ActualLRPKeyWithSchedulingInfo{
					Key:            &actualLRPs[0].ActualLRPKey,
					SchedulingInfo: &expectedSched,
				}))
				Expect(missingCellIds).To(Equal([]string{"other-cell"}))
			})

			It("does not touch the ActualLRPs in the database", func() {
				lrpsBefore, err := sqlDB.ActualLRPs(logger, models.ActualLRPFilter{ProcessGuid: processGuid})
				Expect(err).NotTo(HaveOccurred())

				sqlDB.ConvergeLRPs(logger, cellSet)

				lrpsAfter, err := sqlDB.ActualLRPs(logger, models.ActualLRPFilter{ProcessGuid: processGuid})
				Expect(err).NotTo(HaveOccurred())

				Expect(lrpsAfter).To(Equal(lrpsBefore))
			})
		})

		Context("when the domain is expired", func() {
			BeforeEach(func() {
				fakeClock.Increment(-10 * time.Second)
				Expect(sqlDB.UpsertDomain(logger, domain, 5)).To(Succeed())
				fakeClock.Increment(10 * time.Second)
			})

			It("return ActualLRPKeys for actuals with missing cells", func() {
				result := sqlDB.ConvergeLRPs(logger, cellSet)
				keysWithMissingCells := result.KeysWithMissingCells

				desiredLRP, err := sqlDB.DesiredLRPByProcessGuid(logger, processGuid)
				Expect(err).NotTo(HaveOccurred())

				index := int32(0)
				actualLRPs, err := sqlDB.ActualLRPs(logger, models.ActualLRPFilter{ProcessGuid: processGuid, Index: &index})
				Expect(err).NotTo(HaveOccurred())
				expectedSched := desiredLRP.DesiredLRPSchedulingInfo()
				Expect(actualLRPs).To(HaveLen(1))
				Expect(keysWithMissingCells).To(ContainElement(&models.ActualLRPKeyWithSchedulingInfo{
					Key:            &actualLRPs[0].ActualLRPKey,
					SchedulingInfo: &expectedSched,
				}))
			})

			It("does not touch the ActualLRPs in the database", func() {
				lrpsBefore, err := sqlDB.ActualLRPs(logger, models.ActualLRPFilter{ProcessGuid: processGuid})
				Expect(err).NotTo(HaveOccurred())

				sqlDB.ConvergeLRPs(logger, cellSet)

				lrpsAfter, err := sqlDB.ActualLRPs(logger, models.ActualLRPFilter{ProcessGuid: processGuid})
				Expect(err).NotTo(HaveOccurred())

				Expect(lrpsAfter).To(Equal(lrpsBefore))
			})
		})

		Context("when the lrp is evacuating", func() {
			BeforeEach(func() {
				Expect(sqlDB.UpsertDomain(logger, domain, 5)).To(Succeed())

				queryStr := `UPDATE actual_lrps SET presence = ?`
				if test_helpers.UsePostgres() {
					queryStr = test_helpers.ReplaceQuestionMarks(queryStr)
				}
				_, err := db.Exec(queryStr, models.ActualLRP_Evacuating)
				Expect(err).NotTo(HaveOccurred())
			})

			It("ignores the evacuating LRPs and should have the correct number of missing LRPs", func() {
				schedulingInfos, err := sqlDB.DesiredLRPSchedulingInfos(logger, models.DesiredLRPFilter{ProcessGuids: []string{processGuid}})
				Expect(err).NotTo(HaveOccurred())

				results := sqlDB.ConvergeLRPs(logger, cellSet)
				Expect(results.MissingLRPKeys).To(ConsistOf(&models.ActualLRPKeyWithSchedulingInfo{
					Key:            &models.ActualLRPKey{ProcessGuid: processGuid, Index: 0, Domain: domain},
					SchedulingInfo: schedulingInfos[0],
				},
				))
			})

			It("returns the start requests and actual lrp keys for actuals with missing cells", func() {
				result := sqlDB.ConvergeLRPs(logger, cellSet)

				desiredLRP, err := sqlDB.DesiredLRPByProcessGuid(logger, processGuid)
				Expect(err).NotTo(HaveOccurred())

				expectedSched := desiredLRP.DesiredLRPSchedulingInfo()
				Expect(result.MissingLRPKeys).To(ContainElement(&models.ActualLRPKeyWithSchedulingInfo{
					Key:            &models.ActualLRPKey{ProcessGuid: processGuid, Index: 0, Domain: domain},
					SchedulingInfo: &expectedSched,
				}))
			})

			It("removes the evacuating lrp", func() {
				sqlDB.ConvergeLRPs(logger, cellSet)

				lrps, err := sqlDB.ActualLRPs(logger, models.ActualLRPFilter{ProcessGuid: processGuid})
				Expect(err).NotTo(HaveOccurred())
				Expect(lrps).To(BeEmpty())
			})
		})

		It("logs the missing cells", func() {
			sqlDB.ConvergeLRPs(logger, cellSet)
			Expect(logger).To(gbytes.Say(`detected-missing-cells.*cell_ids":\["other-cell"\]`))
		})

		Context("when there are no missing cells", func() {
			BeforeEach(func() {
				cellSet = models.NewCellSetFromList([]*models.CellPresence{
					{CellId: "existing-cell"},
					{CellId: "other-cell"},
				})
			})

			It("does not log missing cells", func() {
				sqlDB.ConvergeLRPs(logger, cellSet)
				Expect(logger).ToNot(gbytes.Say("detected-missing-cells"))
			})
		})
	})

	Context("when there are extra ActualLRPs for a DesiredLRP", func() {
		var (
			domain      string
			processGuid string
		)

		BeforeEach(func() {
			domain = "some-domain"
			processGuid = "desired-with-extra-actuals"
			desiredLRPWithExtraActuals := model_helpers.NewValidDesiredLRP(processGuid)
			desiredLRPWithExtraActuals.Domain = domain
			desiredLRPWithExtraActuals.Instances = 1
			err := sqlDB.DesireLRP(logger, desiredLRPWithExtraActuals)
			Expect(err).NotTo(HaveOccurred())
			_, err = sqlDB.CreateUnclaimedActualLRP(logger, &models.ActualLRPKey{ProcessGuid: processGuid, Index: 0, Domain: domain})
			Expect(err).NotTo(HaveOccurred())
			_, err = sqlDB.CreateUnclaimedActualLRP(logger, &models.ActualLRPKey{ProcessGuid: processGuid, Index: 4, Domain: domain})
			Expect(err).NotTo(HaveOccurred())
			_, _, err = sqlDB.ClaimActualLRP(logger, processGuid, 0, &models.ActualLRPInstanceKey{InstanceGuid: "not-extra-actual", CellId: "existing-cell"})
			Expect(err).NotTo(HaveOccurred())
			_, _, err = sqlDB.ClaimActualLRP(logger, processGuid, 4, &models.ActualLRPInstanceKey{InstanceGuid: "extra-actual", CellId: "existing-cell"})
			Expect(err).NotTo(HaveOccurred())
		})

		Context("when the domain is fresh", func() {
			BeforeEach(func() {
				Expect(sqlDB.UpsertDomain(logger, domain, 5)).To(Succeed())
			})

			It("returns extra ActualLRPs to be retired", func() {
				result := sqlDB.ConvergeLRPs(logger, cellSet)
				keysToRetire := result.KeysToRetire

				actualLRPKey := models.ActualLRPKey{ProcessGuid: processGuid, Index: 4, Domain: domain}
				Expect(keysToRetire).To(ContainElement(&actualLRPKey))
			})

			It("does not touch the ActualLRPs in the database", func() {
				lrpsBefore, err := sqlDB.ActualLRPs(logger, models.ActualLRPFilter{ProcessGuid: processGuid})
				Expect(err).NotTo(HaveOccurred())

				sqlDB.ConvergeLRPs(logger, cellSet)

				lrpsAfter, err := sqlDB.ActualLRPs(logger, models.ActualLRPFilter{ProcessGuid: processGuid})
				Expect(err).NotTo(HaveOccurred())

				Expect(lrpsAfter).To(Equal(lrpsBefore))
			})

			It("should have the correct number of extra LRPs instances", func() {
				results := sqlDB.ConvergeLRPs(logger, cellSet)
				Expect(results.KeysToRetire).To(ConsistOf(&models.ActualLRPKey{ProcessGuid: processGuid, Index: 4, Domain: domain}))
			})
		})

		Context("when the domain is expired", func() {
			BeforeEach(func() {
				fakeClock.Increment(-10 * time.Second)
				Expect(sqlDB.UpsertDomain(logger, domain, 5)).To(Succeed())
				fakeClock.Increment(10 * time.Second)
			})

			It("returns an empty convergence result", func() {
				result := sqlDB.ConvergeLRPs(logger, cellSet)
				Expect(result).To(BeZero())
			})

			It("does not touch the ActualLRPs in the database", func() {
				lrpsBefore, err := sqlDB.ActualLRPs(logger, models.ActualLRPFilter{ProcessGuid: processGuid})
				Expect(err).NotTo(HaveOccurred())

				sqlDB.ConvergeLRPs(logger, cellSet)

				lrpsAfter, err := sqlDB.ActualLRPs(logger, models.ActualLRPFilter{ProcessGuid: processGuid})
				Expect(err).NotTo(HaveOccurred())

				Expect(lrpsAfter).To(Equal(lrpsBefore))
			})

			It("should not have any extra LRP instances", func() {
				results := sqlDB.ConvergeLRPs(logger, cellSet)
				Expect(results.KeysToRetire).To(BeEmpty())
			})
		})

		Context("when the ActualLRP's presence is set to evacuating", func() {
			BeforeEach(func() {
				Expect(sqlDB.UpsertDomain(logger, domain, 5)).To(Succeed())

				queryStr := `UPDATE actual_lrps SET presence = ? WHERE process_guid = ?`
				if test_helpers.UsePostgres() {
					queryStr = test_helpers.ReplaceQuestionMarks(queryStr)
				}
				_, err := db.Exec(queryStr, models.ActualLRP_Evacuating, processGuid)
				Expect(err).NotTo(HaveOccurred())
			})

			It("returns the lrp key to be started", func() {
				schedulingInfos, err := sqlDB.DesiredLRPSchedulingInfos(logger, models.DesiredLRPFilter{ProcessGuids: []string{processGuid}})
				Expect(err).NotTo(HaveOccurred())

				Expect(schedulingInfos).To(HaveLen(1))

				result := sqlDB.ConvergeLRPs(logger, cellSet)
				Expect(result.MissingLRPKeys).To(ConsistOf(&models.ActualLRPKeyWithSchedulingInfo{
					Key:            &models.ActualLRPKey{ProcessGuid: processGuid, Index: 0, Domain: domain},
					SchedulingInfo: schedulingInfos[0],
				}))
			})
		})
	})

	Context("when there are no ActualLRPs for a DesiredLRP", func() {
		var (
			domain      string
			processGuid string
		)

		BeforeEach(func() {
			processGuid = "desired-with-missing-all-actuals" + "-" + domain
			desiredLRPWithMissingAllActuals := model_helpers.NewValidDesiredLRP(processGuid)
			desiredLRPWithMissingAllActuals.Domain = domain
			desiredLRPWithMissingAllActuals.Instances = 1
			err := sqlDB.DesireLRP(logger, desiredLRPWithMissingAllActuals)
			Expect(err).NotTo(HaveOccurred())
		})

		Context("and the domain is fresh", func() {
			BeforeEach(func() {
				Expect(sqlDB.UpsertDomain(logger, domain, 5)).To(Succeed())
			})

			It("should have the correct number of missing LRP instances", func() {
				schedulingInfos, err := sqlDB.DesiredLRPSchedulingInfos(logger, models.DesiredLRPFilter{ProcessGuids: []string{processGuid}})
				Expect(err).NotTo(HaveOccurred())

				Expect(schedulingInfos).To(HaveLen(1))

				result := sqlDB.ConvergeLRPs(logger, cellSet)
				Expect(result.MissingLRPKeys).To(ConsistOf(&models.ActualLRPKeyWithSchedulingInfo{
					Key:            &models.ActualLRPKey{ProcessGuid: processGuid, Index: 0, Domain: domain},
					SchedulingInfo: schedulingInfos[0],
				}))
			})

			It("return ActualLRPKeys for missing actuals", func() {
				result := sqlDB.ConvergeLRPs(logger, cellSet)

				desiredLRP, err := sqlDB.DesiredLRPByProcessGuid(logger, processGuid)
				Expect(err).NotTo(HaveOccurred())

				expectedSched := desiredLRP.DesiredLRPSchedulingInfo()
				Expect(result.MissingLRPKeys).To(ContainElement(&models.ActualLRPKeyWithSchedulingInfo{
					Key:            &models.ActualLRPKey{ProcessGuid: processGuid, Index: 0, Domain: domain},
					SchedulingInfo: &expectedSched,
				}))
			})
		})

		Context("and the domain is expired", func() {
			BeforeEach(func() {
				fakeClock.Increment(-10 * time.Second)
				Expect(sqlDB.UpsertDomain(logger, domain, 5)).To(Succeed())
				fakeClock.Increment(10 * time.Second)
			})

			It("should have the correct number of missing LRP instances", func() {
				schedulingInfos, err := sqlDB.DesiredLRPSchedulingInfos(logger, models.DesiredLRPFilter{ProcessGuids: []string{processGuid}})
				Expect(err).NotTo(HaveOccurred())

				Expect(schedulingInfos).To(HaveLen(1))

				result := sqlDB.ConvergeLRPs(logger, cellSet)
				Expect(result.MissingLRPKeys).To(ConsistOf(&models.ActualLRPKeyWithSchedulingInfo{
					Key:            &models.ActualLRPKey{ProcessGuid: processGuid, Index: 0, Domain: domain},
					SchedulingInfo: schedulingInfos[0],
				}))
			})

			It("return ActualLRPKeys for missing actuals", func() {
				result := sqlDB.ConvergeLRPs(logger, cellSet)

				desiredLRP, err := sqlDB.DesiredLRPByProcessGuid(logger, processGuid)
				Expect(err).NotTo(HaveOccurred())

				expectedSched := desiredLRP.DesiredLRPSchedulingInfo()
				Expect(result.MissingLRPKeys).To(ContainElement(&models.ActualLRPKeyWithSchedulingInfo{
					Key:            &models.ActualLRPKey{ProcessGuid: processGuid, Index: 0, Domain: domain},
					SchedulingInfo: &expectedSched,
				}))
			})
		})
	})

	Context("when the ActualLRPs are crashed and restartable", func() {
		var (
			domain      string
			processGuid string
		)

		BeforeEach(func() {
			processGuid = "desired-with-restartable-crashed-actuals" + "-" + domain
			desiredLRPWithRestartableCrashedActuals := model_helpers.NewValidDesiredLRP(processGuid)
			desiredLRPWithRestartableCrashedActuals.Domain = domain
			desiredLRPWithRestartableCrashedActuals.Instances = 2
			err := sqlDB.DesireLRP(logger, desiredLRPWithRestartableCrashedActuals)
			Expect(err).NotTo(HaveOccurred())

			for i := int32(0); i < 2; i++ {
				crashedActualLRPKey := models.NewActualLRPKey(processGuid, i, domain)
				_, err = sqlDB.CreateUnclaimedActualLRP(logger, &crashedActualLRPKey)
				Expect(err).NotTo(HaveOccurred())
				instanceGuid := "restartable-crashed-actual" + "-" + domain
				_, _, err = sqlDB.ClaimActualLRP(logger, processGuid, i, &models.ActualLRPInstanceKey{InstanceGuid: instanceGuid, CellId: "existing-cell"})
				Expect(err).NotTo(HaveOccurred())
				actualLRPNetInfo := models.NewActualLRPNetInfo("some-address", "container-address", models.NewPortMapping(2222, 4444))
				_, _, err = sqlDB.StartActualLRP(logger, &crashedActualLRPKey, &models.ActualLRPInstanceKey{InstanceGuid: instanceGuid, CellId: "existing-cell"}, &actualLRPNetInfo)
				Expect(err).NotTo(HaveOccurred())
				_, _, _, err = sqlDB.CrashActualLRP(logger, &crashedActualLRPKey, &models.ActualLRPInstanceKey{InstanceGuid: instanceGuid, CellId: "existing-cell"}, "whatever")
				Expect(err).NotTo(HaveOccurred())
			}

			// we cannot use CrashedActualLRPs, otherwise it will transition the LRP
			// to unclaimed since ShouldRestartCrash will return true on the first
			// crash
			queryStr := `
				UPDATE actual_lrps
				SET state = ?
			`
			if test_helpers.UsePostgres() {
				queryStr = test_helpers.ReplaceQuestionMarks(queryStr)
			}
			_, err = db.Exec(queryStr, models.ActualLRPStateCrashed)
			Expect(err).NotTo(HaveOccurred())
		})

		Context("when the domain is fresh", func() {
			BeforeEach(func() {
				Expect(sqlDB.UpsertDomain(logger, domain, 5)).To(Succeed())
			})

			It("should have the correct number of crashed LRP instances", func() {
				sqlDB.ConvergeLRPs(logger, cellSet)
				_, _, _, crashed, _ := sqlDB.CountActualLRPsByState(logger)
				Expect(crashed).To(Equal(2))
			})

			It("add the keys to UnstartedLRPKeys", func() {
				result := sqlDB.ConvergeLRPs(logger, cellSet)

				desiredLRP, err := sqlDB.DesiredLRPByProcessGuid(logger, processGuid)
				Expect(err).NotTo(HaveOccurred())

				expectedSched := desiredLRP.DesiredLRPSchedulingInfo()
				Expect(result.UnstartedLRPKeys).To(ContainElement(&models.ActualLRPKeyWithSchedulingInfo{
					Key:            &models.ActualLRPKey{ProcessGuid: processGuid, Index: 0, Domain: domain},
					SchedulingInfo: &expectedSched,
				}))
			})
		})

		Context("when the domain is expired", func() {
			BeforeEach(func() {
				fakeClock.Increment(-10 * time.Second)
				Expect(sqlDB.UpsertDomain(logger, domain, 5)).To(Succeed())
				fakeClock.Increment(10 * time.Second)
			})

			It("should have the correct number of crashed LRP instances", func() {
				sqlDB.ConvergeLRPs(logger, cellSet)
				_, _, _, crashed, _ := sqlDB.CountActualLRPsByState(logger)
				Expect(crashed).To(Equal(2))
			})

			It("add the keys to UnstartedLRPKeys", func() {
				result := sqlDB.ConvergeLRPs(logger, cellSet)

				desiredLRP, err := sqlDB.DesiredLRPByProcessGuid(logger, processGuid)
				Expect(err).NotTo(HaveOccurred())

				expectedSched := desiredLRP.DesiredLRPSchedulingInfo()
				Expect(result.UnstartedLRPKeys).To(ContainElement(&models.ActualLRPKeyWithSchedulingInfo{
					Key:            &models.ActualLRPKey{ProcessGuid: processGuid, Index: 0, Domain: domain},
					SchedulingInfo: &expectedSched,
				}))
			})
		})

		Context("when the the lrps are evacuating", func() {
			BeforeEach(func() {
				Expect(sqlDB.UpsertDomain(logger, domain, 5)).To(Succeed())

				queryStr := `UPDATE actual_lrps SET presence = ?`
				if test_helpers.UsePostgres() {
					queryStr = test_helpers.ReplaceQuestionMarks(queryStr)
				}
				_, err := db.Exec(queryStr, models.ActualLRP_Evacuating)
				Expect(err).NotTo(HaveOccurred())
			})

			It("should have the correct number of missing LRP instances", func() {
				schedulingInfos, err := sqlDB.DesiredLRPSchedulingInfos(logger, models.DesiredLRPFilter{ProcessGuids: []string{processGuid}})
				Expect(err).NotTo(HaveOccurred())

				Expect(schedulingInfos).To(HaveLen(1))

				result := sqlDB.ConvergeLRPs(logger, cellSet)
				Expect(result.MissingLRPKeys).To(ConsistOf(
					&models.ActualLRPKeyWithSchedulingInfo{
						Key:            &models.ActualLRPKey{ProcessGuid: processGuid, Index: 0, Domain: domain},
						SchedulingInfo: schedulingInfos[0],
					},
					&models.ActualLRPKeyWithSchedulingInfo{
						Key:            &models.ActualLRPKey{ProcessGuid: processGuid, Index: 1, Domain: domain},
						SchedulingInfo: schedulingInfos[0],
					},
				))
			})

			// it is the responsibility of the caller to create new LRPs
			It("prune the evacuating LRPs and does not create new ones", func() {
				sqlDB.ConvergeLRPs(logger, cellSet)

				lrps, err := sqlDB.ActualLRPs(logger, models.ActualLRPFilter{ProcessGuid: processGuid})
				Expect(err).NotTo(HaveOccurred())
				Expect(lrps).To(BeEmpty())
			})

			It("return ActualLRPKeys for missing actuals", func() {
				result := sqlDB.ConvergeLRPs(logger, cellSet)

				desiredLRP, err := sqlDB.DesiredLRPByProcessGuid(logger, processGuid)
				Expect(err).NotTo(HaveOccurred())

				expectedSched := desiredLRP.DesiredLRPSchedulingInfo()
				Expect(result.MissingLRPKeys).To(ContainElement(&models.ActualLRPKeyWithSchedulingInfo{
					Key:            &models.ActualLRPKey{ProcessGuid: processGuid, Index: 0, Domain: domain},
					SchedulingInfo: &expectedSched,
				}))
			})

			It("return ActualLRPRemovedEvent for the removed evacuating LRPs", func() {
				actualLRPs, err := sqlDB.ActualLRPs(logger, models.ActualLRPFilter{ProcessGuid: processGuid})
				Expect(err).NotTo(HaveOccurred())
				Expect(actualLRPs).To(HaveLen(2))

				result := sqlDB.ConvergeLRPs(logger, cellSet)
				Expect(result.Events).To(ConsistOf(
					models.NewActualLRPRemovedEvent(actualLRPs[0].ToActualLRPGroup()),
					models.NewActualLRPRemovedEvent(actualLRPs[1].ToActualLRPGroup()),
				))
				Expect(result.InstanceEvents).To(ConsistOf(
					models.NewActualLRPInstanceRemovedEvent(actualLRPs[0]),
					models.NewActualLRPInstanceRemovedEvent(actualLRPs[1]),
				))
			})
		})
	})

	Context("when the ActualLRPs are crashed and non-restartable", func() {
		var (
			domain      string
			processGuid string
		)

		BeforeEach(func() {
			processGuid = "desired-with-non-restartable-crashed-actuals" + "-" + domain
			desiredLRPWithRestartableCrashedActuals := model_helpers.NewValidDesiredLRP(processGuid)
			desiredLRPWithRestartableCrashedActuals.Domain = domain
			desiredLRPWithRestartableCrashedActuals.Instances = 2
			err := sqlDB.DesireLRP(logger, desiredLRPWithRestartableCrashedActuals)
			Expect(err).NotTo(HaveOccurred())

			for i := int32(0); i < 2; i++ {
				crashedActualLRPKey := models.NewActualLRPKey(processGuid, i, domain)
				_, err = sqlDB.CreateUnclaimedActualLRP(logger, &crashedActualLRPKey)
				Expect(err).NotTo(HaveOccurred())
				instanceGuid := "restartable-crashed-actual" + "-" + domain
				_, _, err = sqlDB.ClaimActualLRP(logger, processGuid, i, &models.ActualLRPInstanceKey{InstanceGuid: instanceGuid, CellId: "existing-cell"})
				Expect(err).NotTo(HaveOccurred())
				actualLRPNetInfo := models.NewActualLRPNetInfo("some-address", "container-address", models.NewPortMapping(2222, 4444))
				_, _, err = sqlDB.StartActualLRP(logger, &crashedActualLRPKey, &models.ActualLRPInstanceKey{InstanceGuid: instanceGuid, CellId: "existing-cell"}, &actualLRPNetInfo)
				Expect(err).NotTo(HaveOccurred())
			}

			// we cannot use CrashedActualLRPs, otherwise it will transition the LRP
			// to unclaimed since ShouldRestartCrash will return true on the first
			// crash
			queryStr := `
			UPDATE actual_lrps
			SET crash_count = ?, state = ?
			`
			if test_helpers.UsePostgres() {
				queryStr = test_helpers.ReplaceQuestionMarks(queryStr)
			}
			_, err = db.Exec(queryStr, models.DefaultMaxRestarts+1, models.ActualLRPStateCrashed)
			Expect(err).NotTo(HaveOccurred())
		})

		Context("when the domain is fresh", func() {
			BeforeEach(func() {
				Expect(sqlDB.UpsertDomain(logger, domain, 5)).To(Succeed())
			})

			It("should have the correct number of crashed LRP instances", func() {
				sqlDB.ConvergeLRPs(logger, cellSet)
				_, _, _, crashed, _ := sqlDB.CountActualLRPsByState(logger)
				Expect(crashed).To(Equal(2))
			})

			It("returns an empty convergence result", func() {
				result := sqlDB.ConvergeLRPs(logger, cellSet)
				Expect(result).To(BeZero())
			})
		})

		Context("when the domain is expired", func() {
			BeforeEach(func() {
				fakeClock.Increment(-10 * time.Second)
				Expect(sqlDB.UpsertDomain(logger, domain, 5)).To(Succeed())
				fakeClock.Increment(10 * time.Second)
			})

			It("should have the correct number of crashed LRP instances", func() {
				sqlDB.ConvergeLRPs(logger, cellSet)
				_, _, _, crashed, _ := sqlDB.CountActualLRPsByState(logger)
				Expect(crashed).To(Equal(2))
			})

			It("does not add the keys to UnstartedLRPKeys", func() {
				result := sqlDB.ConvergeLRPs(logger, cellSet)
				Expect(result.UnstartedLRPKeys).To(BeEmpty())
			})
		})
	})

	Context("there is an ActualLRP without a corresponding DesiredLRP", func() {
		var (
			processGuid, domain string
		)

		BeforeEach(func() {
			domain = "some-domain"
			processGuid = "actual-with-no-desired"
			actualLRPWithNoDesired := &models.ActualLRPKey{ProcessGuid: processGuid, Index: 0, Domain: domain}
			_, err := sqlDB.CreateUnclaimedActualLRP(logger, actualLRPWithNoDesired)
			Expect(err).NotTo(HaveOccurred())
		})

		Context("when the domain is fresh", func() {
			BeforeEach(func() {
				Expect(sqlDB.UpsertDomain(logger, domain, 5)).To(Succeed())
			})

			It("returns extra ActualLRPs to be retired", func() {
				result := sqlDB.ConvergeLRPs(logger, cellSet)
				keysToRetire := result.KeysToRetire

				actualLRPKey := models.ActualLRPKey{ProcessGuid: processGuid, Index: 0, Domain: domain}
				Expect(keysToRetire).To(ContainElement(&actualLRPKey))
			})

			It("returns the no lrp keys to be started", func() {
				result := sqlDB.ConvergeLRPs(logger, cellSet)
				Expect(result.UnstartedLRPKeys).To(BeEmpty())
				Expect(result.MissingLRPKeys).To(BeEmpty())
			})

			It("does not touch the ActualLRPs in the database", func() {
				lrpsBefore, err := sqlDB.ActualLRPs(logger, models.ActualLRPFilter{ProcessGuid: processGuid})
				Expect(err).NotTo(HaveOccurred())

				sqlDB.ConvergeLRPs(logger, cellSet)

				lrpsAfter, err := sqlDB.ActualLRPs(logger, models.ActualLRPFilter{ProcessGuid: processGuid})
				Expect(err).NotTo(HaveOccurred())

				Expect(lrpsAfter).To(Equal(lrpsBefore))
			})

			It("should have the correct number of extra LRP instances", func() {
				results := sqlDB.ConvergeLRPs(logger, cellSet)
				Expect(results.KeysToRetire).To(ConsistOf(&models.ActualLRPKey{ProcessGuid: processGuid, Index: 0, Domain: domain}))
			})
		})

		Context("when the domain is expired", func() {
			BeforeEach(func() {
				fakeClock.Increment(-10 * time.Second)
				Expect(sqlDB.UpsertDomain(logger, domain, 5)).To(Succeed())
				fakeClock.Increment(10 * time.Second)
			})

			It("does not return extra ActualLRPs to be retired", func() {
				result := sqlDB.ConvergeLRPs(logger, cellSet)
				Expect(result.KeysToRetire).To(BeEmpty())
			})

			It("returns the no lrp keys to be started", func() {
				result := sqlDB.ConvergeLRPs(logger, cellSet)
				Expect(result.UnstartedLRPKeys).To(BeEmpty())
				Expect(result.MissingLRPKeys).To(BeEmpty())
			})

			It("does not touch the ActualLRPs in the database", func() {
				lrpsBefore, err := sqlDB.ActualLRPs(logger, models.ActualLRPFilter{ProcessGuid: processGuid})
				Expect(err).NotTo(HaveOccurred())

				sqlDB.ConvergeLRPs(logger, cellSet)

				lrpsAfter, err := sqlDB.ActualLRPs(logger, models.ActualLRPFilter{ProcessGuid: processGuid})
				Expect(err).NotTo(HaveOccurred())

				Expect(lrpsAfter).To(Equal(lrpsBefore))
			})

			It("should not have any extra LRP instances", func() {
				results := sqlDB.ConvergeLRPs(logger, cellSet)
				Expect(results.KeysToRetire).To(BeEmpty())
			})
		})

		Context("when the the lrps are evacuating", func() {
			BeforeEach(func() {
				Expect(sqlDB.UpsertDomain(logger, domain, 5)).To(Succeed())

				queryStr := `UPDATE actual_lrps SET presence = ?`
				if test_helpers.UsePostgres() {
					queryStr = test_helpers.ReplaceQuestionMarks(queryStr)
				}
				_, err := db.Exec(queryStr, models.ActualLRP_Evacuating)
				Expect(err).NotTo(HaveOccurred())
			})

			It("returns the no lrp keys to be started", func() {
				result := sqlDB.ConvergeLRPs(logger, cellSet)
				Expect(result.UnstartedLRPKeys).To(BeEmpty())
				Expect(result.MissingLRPKeys).To(BeEmpty())
			})

			It("removes the evacuating LRPs", func() {
				sqlDB.ConvergeLRPs(logger, cellSet)

				actualLRPs, err := sqlDB.ActualLRPs(logger, models.ActualLRPFilter{ProcessGuid: processGuid})
				Expect(err).NotTo(HaveOccurred())
				Expect(actualLRPs).To(BeEmpty())
			})

			It("return an ActualLRPRemoved Event", func() {
				actualLRPs, err := sqlDB.ActualLRPs(logger, models.ActualLRPFilter{ProcessGuid: processGuid})
				Expect(err).NotTo(HaveOccurred())

				Expect(actualLRPs).To(HaveLen(1))

				result := sqlDB.ConvergeLRPs(logger, cellSet)
				Expect(result.Events).To(ConsistOf(models.NewActualLRPRemovedEvent(actualLRPs[0].ToActualLRPGroup())))
				Expect(result.InstanceEvents).To(ConsistOf(models.NewActualLRPInstanceRemovedEvent(actualLRPs[0])))
			})

			It("should not have any extra LRP instances", func() {
				results := sqlDB.ConvergeLRPs(logger, cellSet)
				Expect(results.KeysToRetire).To(BeEmpty())
			})
		})
	})
})
