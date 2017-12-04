package main_test

import (
	"code.cloudfoundry.org/bbs/cmd/bbs/testrunner"
	"code.cloudfoundry.org/bbs/models"
	"code.cloudfoundry.org/bbs/models/test/model_helpers"
	"github.com/tedsuo/ifrit/ginkgomon"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Evacuation API", func() {
	var actual *models.ActualLRP

	BeforeEach(func() {
		bbsRunner = testrunner.New(bbsBinPath, bbsConfig)
		bbsProcess = ginkgomon.Invoke(bbsRunner)

		actual = model_helpers.NewValidActualLRP("some-process-guid", 1)
		actual.State = models.ActualLRPStateRunning
		desiredLRP := model_helpers.NewValidDesiredLRP(actual.ProcessGuid)
		desiredLRP.Instances = 2

		Expect(client.DesireLRP(logger, desiredLRP)).To(Succeed())
		Expect(client.ClaimActualLRP(logger, actual.ProcessGuid, 1, &actual.ActualLRPInstanceKey)).To(Succeed())
		_, err := client.ActualLRPGroupByProcessGuidAndIndex(logger, actual.ProcessGuid, int(actual.Index))
		Expect(err).NotTo(HaveOccurred())
	})

	Describe("RemoveEvacuatingActualLRP", func() {
		It("removes the evacuating actual_lrp", func() {
			_, err := client.EvacuateClaimedActualLRP(logger, &actual.ActualLRPKey, &actual.ActualLRPInstanceKey)
			Expect(err).NotTo(HaveOccurred())

			err = client.RemoveEvacuatingActualLRP(logger, &actual.ActualLRPKey, &actual.ActualLRPInstanceKey)
			Expect(err).NotTo(HaveOccurred())

			group, err := client.ActualLRPGroupByProcessGuidAndIndex(logger, actual.ProcessGuid, int(actual.Index))
			Expect(err).ToNot(HaveOccurred())
			Expect(group.Evacuating).To(BeNil())
		})
	})

	Describe("EvacuateClaimedActualLRP", func() {
		It("removes the claimed actual_lrp without evacuating", func() {
			keepContainer, evacuateErr := client.EvacuateClaimedActualLRP(logger, &actual.ActualLRPKey, &actual.ActualLRPInstanceKey)
			Expect(keepContainer).To(BeFalse())
			Expect(evacuateErr).NotTo(HaveOccurred())

			actualLRPGroup, err := client.ActualLRPGroupByProcessGuidAndIndex(logger, actual.ProcessGuid, int(actual.Index))
			Expect(err).NotTo(HaveOccurred())
			Expect(actualLRPGroup.Evacuating).To(BeNil())
			Expect(actualLRPGroup.Instance).NotTo(BeNil())
			Expect(actualLRPGroup.Instance.State).To(Equal(models.ActualLRPStateUnclaimed))
		})
	})

	Describe("EvacuateRunningActualLRP", func() {
		BeforeEach(func() {
			err := client.StartActualLRP(logger, &actual.ActualLRPKey, &actual.ActualLRPInstanceKey, &actual.ActualLRPNetInfo)
			Expect(err).NotTo(HaveOccurred())
		})

		It("runs the evacuating ActualLRP and unclaims the instance ActualLRP", func() {
			keepContainer, err := client.EvacuateRunningActualLRP(logger, &actual.ActualLRPKey, &actual.ActualLRPInstanceKey, &actual.ActualLRPNetInfo, uint64(10000))
			Expect(keepContainer).To(BeTrue())
			Expect(err).NotTo(HaveOccurred())

			actualLRPGroup, err := client.ActualLRPGroupByProcessGuidAndIndex(logger, actual.ProcessGuid, int(actual.Index))
			Expect(err).NotTo(HaveOccurred())
			Expect(actualLRPGroup.Evacuating).NotTo(BeNil())
			Expect(actualLRPGroup.Instance).NotTo(BeNil())
			Expect(actualLRPGroup.Evacuating.State).To(Equal(models.ActualLRPStateRunning))
			Expect(actualLRPGroup.Instance.State).To(Equal(models.ActualLRPStateUnclaimed))
		})
	})

	Describe("EvacuateStoppedActualLRP", func() {
		BeforeEach(func() {
			err := client.StartActualLRP(logger, &actual.ActualLRPKey, &actual.ActualLRPInstanceKey, &actual.ActualLRPNetInfo)
			Expect(err).NotTo(HaveOccurred())
		})

		It("deletes the container and both actualLRPs", func() {
			keepContainer, err := client.EvacuateStoppedActualLRP(logger, &actual.ActualLRPKey, &actual.ActualLRPInstanceKey)
			Expect(keepContainer).To(BeFalse())
			Expect(err).NotTo(HaveOccurred())
			_, err = client.ActualLRPGroupByProcessGuidAndIndex(logger, actual.ProcessGuid, int(actual.Index))
			Expect(err).To(Equal(models.ErrResourceNotFound))
		})
	})

	Describe("EvacuateCrashedActualLRP", func() {
		BeforeEach(func() {
			err := client.StartActualLRP(logger, &actual.ActualLRPKey, &actual.ActualLRPInstanceKey, &actual.ActualLRPNetInfo)
			Expect(err).NotTo(HaveOccurred())
		})

		It("removes the crashed evacuating LRP and unclaims the instance ActualLRP", func() {
			keepContainer, evacuateErr := client.EvacuateCrashedActualLRP(logger, &actual.ActualLRPKey, &actual.ActualLRPInstanceKey, "some-reason")
			Expect(keepContainer).Should(BeFalse())
			Expect(evacuateErr).NotTo(HaveOccurred())

			actualLRPGroup, err := client.ActualLRPGroupByProcessGuidAndIndex(logger, actual.ProcessGuid, int(actual.Index))
			Expect(err).NotTo(HaveOccurred())
			Expect(actualLRPGroup.Evacuating).To(BeNil())
			Expect(actualLRPGroup.Instance).ToNot(BeNil())
			Expect(actualLRPGroup.Instance.State).To(Equal(models.ActualLRPStateUnclaimed))
		})
	})
})
