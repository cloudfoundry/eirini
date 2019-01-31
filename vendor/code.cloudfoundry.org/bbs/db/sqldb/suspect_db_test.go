package sqldb_test

import (
	"code.cloudfoundry.org/bbs/models"
	"code.cloudfoundry.org/bbs/models/test/model_helpers"
	"code.cloudfoundry.org/bbs/test_helpers"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Suspect ActualLRPs", func() {
	var (
		actualLRP *models.ActualLRP
		guid      string
		index     int32
	)

	BeforeEach(func() {
		guid = "some-guid"
		index = int32(1)
		actualLRP = model_helpers.NewValidActualLRP(guid, index)
		actualLRP.CrashCount = 0
		actualLRP.CrashReason = ""
		actualLRP.Since = fakeClock.Now().UnixNano()
		actualLRP.ModificationTag = models.ModificationTag{}
		actualLRP.ModificationTag.Increment()
		actualLRP.ModificationTag.Increment()

		_, err := sqlDB.CreateUnclaimedActualLRP(logger, &actualLRP.ActualLRPKey)
		Expect(err).NotTo(HaveOccurred())
		_, _, err = sqlDB.ClaimActualLRP(logger, guid, index, &actualLRP.ActualLRPInstanceKey)
		Expect(err).NotTo(HaveOccurred())
		_, _, err = sqlDB.StartActualLRP(logger, &actualLRP.ActualLRPKey, &actualLRP.ActualLRPInstanceKey, &actualLRP.ActualLRPNetInfo)
		Expect(err).NotTo(HaveOccurred())
	})

	Describe("RemoveSuspectActualLRP", func() {
		Context("when there is a suspect actualLRP", func() {
			BeforeEach(func() {
				queryStr := "UPDATE actual_lrps SET presence = ? WHERE process_guid = ? AND instance_index = ? AND presence = ?"
				if test_helpers.UsePostgres() {
					queryStr = test_helpers.ReplaceQuestionMarks(queryStr)
				}
				_, err := db.Exec(queryStr, models.ActualLRP_Suspect, actualLRP.ProcessGuid, actualLRP.Index, models.ActualLRP_Ordinary)
				Expect(err).NotTo(HaveOccurred())
			})

			It("removes the suspect actual LRP", func() {
				beforeLRPs, err := sqlDB.ActualLRPs(logger, models.ActualLRPFilter{ProcessGuid: guid, Index: &index})
				Expect(err).NotTo(HaveOccurred())

				lrp, err := sqlDB.RemoveSuspectActualLRP(logger, &actualLRP.ActualLRPKey)
				Expect(err).ToNot(HaveOccurred())
				Expect(beforeLRPs).To(ConsistOf(lrp))

				afterLRPs, err := sqlDB.ActualLRPs(logger, models.ActualLRPFilter{ProcessGuid: guid, Index: &index})
				Expect(err).NotTo(HaveOccurred())
				Expect(afterLRPs).To(BeEmpty())
			})
		})

		Context("when the actualLRP does not exist", func() {
			// the only LRP in the database is the Ordinary one created in the
			// BeforeEach
			It("does not return an error", func() {
				before, err := sqlDB.ActualLRPs(logger, models.ActualLRPFilter{ProcessGuid: "some-guid"})
				Expect(err).NotTo(HaveOccurred())

				_, err = sqlDB.RemoveSuspectActualLRP(logger, &actualLRP.ActualLRPKey)
				Expect(err).NotTo(HaveOccurred())

				after, err := sqlDB.ActualLRPs(logger, models.ActualLRPFilter{ProcessGuid: "some-guid"})
				Expect(err).NotTo(HaveOccurred())

				Expect(after).To(Equal(before))
			})
		})
	})
})
