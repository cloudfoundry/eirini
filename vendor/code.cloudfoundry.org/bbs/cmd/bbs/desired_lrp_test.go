package main_test

import (
	"fmt"
	"time"

	"code.cloudfoundry.org/bbs"
	"code.cloudfoundry.org/bbs/cmd/bbs/testrunner"
	"code.cloudfoundry.org/bbs/models"
	"code.cloudfoundry.org/bbs/models/test/model_helpers"
	"github.com/tedsuo/ifrit/ginkgomon"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("DesiredLRP API", func() {
	var (
		desiredLRPs         map[string][]*models.DesiredLRP
		schedulingInfos     []*models.DesiredLRPSchedulingInfo
		expectedDesiredLRPs []*models.DesiredLRP
		actualDesiredLRPs   []*models.DesiredLRP

		filter models.DesiredLRPFilter

		getErr error
	)

	BeforeEach(func() {
		bbsRunner = testrunner.New(bbsBinPath, bbsConfig)
		bbsProcess = ginkgomon.Invoke(bbsRunner)
		filter = models.DesiredLRPFilter{}
		expectedDesiredLRPs = []*models.DesiredLRP{}
		actualDesiredLRPs = []*models.DesiredLRP{}
		desiredLRPs = createDesiredLRPsInDomains(client, map[string]int{
			"domain-1": 2,
			"domain-2": 3,
		})
	})

	Describe("DesiredLRPs", func() {
		JustBeforeEach(func() {
			actualDesiredLRPs, getErr = client.DesiredLRPs(logger, filter)
			for _, lrp := range actualDesiredLRPs {
				lrp.ModificationTag.Epoch = "epoch"
			}
		})

		It("responds without error", func() {
			Expect(getErr).NotTo(HaveOccurred())
		})

		It("has the correct number of responses", func() {
			Expect(actualDesiredLRPs).To(HaveLen(5))
		})

		Context("when not filtering", func() {
			It("returns all desired lrps from the bbs", func() {
				for _, domainLRPs := range desiredLRPs {
					for _, lrp := range domainLRPs {
						expectedDesiredLRPs = append(expectedDesiredLRPs, lrp)
					}
				}
				Expect(actualDesiredLRPs).To(ConsistOf(expectedDesiredLRPs))
			})
		})

		Context("when filtering by domain", func() {
			var domain string
			BeforeEach(func() {
				domain = "domain-1"
				filter = models.DesiredLRPFilter{Domain: domain}
			})

			It("has the correct number of responses", func() {
				Expect(actualDesiredLRPs).To(HaveLen(2))
			})

			It("returns only the desired lrps in the requested domain", func() {
				for _, lrp := range desiredLRPs[domain] {
					expectedDesiredLRPs = append(expectedDesiredLRPs, lrp)
				}
				Expect(actualDesiredLRPs).To(ConsistOf(expectedDesiredLRPs))
			})
		})

		Context("when filtering by process guids", func() {
			BeforeEach(func() {
				guids := []string{
					"guid-1-for-domain-1",
					"guid-2-for-domain-2",
				}

				filter = models.DesiredLRPFilter{ProcessGuids: guids}
			})

			It("returns only the scheduling infos in the requested process guids", func() {
				Expect(actualDesiredLRPs).To(HaveLen(2))

				expectedDesiredLRPs := []*models.DesiredLRP{
					desiredLRPs["domain-1"][1],
					desiredLRPs["domain-2"][2],
				}
				Expect(actualDesiredLRPs).To(ConsistOf(expectedDesiredLRPs))
			})
		})
	})

	Describe("DesiredLRPByProcessGuid", func() {
		var (
			desiredLRP         *models.DesiredLRP
			expectedDesiredLRP *models.DesiredLRP
		)

		JustBeforeEach(func() {
			expectedDesiredLRP = desiredLRPs["domain-1"][0]
			desiredLRP, getErr = client.DesiredLRPByProcessGuid(logger, expectedDesiredLRP.GetProcessGuid())
			desiredLRP.ModificationTag.Epoch = "epoch"
		})

		It("responds without error", func() {
			Expect(getErr).NotTo(HaveOccurred())
		})

		It("returns all desired lrps from the bbs", func() {
			Expect(desiredLRP).To(Equal(expectedDesiredLRP))
		})
	})

	Describe("DesiredLRPSchedulingInfos", func() {
		JustBeforeEach(func() {
			schedulingInfos, getErr = client.DesiredLRPSchedulingInfos(logger, filter)
			for _, schedulingInfo := range schedulingInfos {
				schedulingInfo.ModificationTag.Epoch = "epoch"
			}
		})

		It("responds without error", func() {
			Expect(getErr).NotTo(HaveOccurred())
		})

		It("has the correct number of responses", func() {
			Expect(schedulingInfos).To(HaveLen(5))
		})

		Context("when not filtering", func() {
			It("returns all scheduling infos from the bbs", func() {
				expectedSchedulingInfos := []*models.DesiredLRPSchedulingInfo{}
				for _, domainLRPs := range desiredLRPs {
					for _, lrp := range domainLRPs {
						schedulingInfo := lrp.DesiredLRPSchedulingInfo()
						expectedSchedulingInfos = append(expectedSchedulingInfos, &schedulingInfo)
					}
				}
				Expect(schedulingInfos).To(ConsistOf(expectedSchedulingInfos))
			})
		})

		Context("when filtering by domain", func() {
			var domain string
			BeforeEach(func() {
				domain = "domain-1"
				filter = models.DesiredLRPFilter{Domain: domain}
			})

			It("has the correct number of responses", func() {
				Expect(schedulingInfos).To(HaveLen(2))
			})

			It("returns only the scheduling infos in the requested domain", func() {
				expectedSchedulingInfos := []*models.DesiredLRPSchedulingInfo{}
				for _, lrp := range desiredLRPs[domain] {
					schedulingInfo := lrp.DesiredLRPSchedulingInfo()
					expectedSchedulingInfos = append(expectedSchedulingInfos, &schedulingInfo)
				}
				Expect(schedulingInfos).To(ConsistOf(expectedSchedulingInfos))
			})
		})

		Context("when filtering by process guids", func() {
			BeforeEach(func() {
				guids := []string{
					"guid-1-for-domain-1",
					"guid-2-for-domain-2",
				}

				filter = models.DesiredLRPFilter{ProcessGuids: guids}
			})

			It("returns only the scheduling infos in the requested process guids", func() {
				Expect(schedulingInfos).To(HaveLen(2))

				desiredLRP1 := desiredLRPs["domain-1"][1].DesiredLRPSchedulingInfo()
				desiredLRP2 := desiredLRPs["domain-2"][2].DesiredLRPSchedulingInfo()
				expectedSchedulingInfos := []*models.DesiredLRPSchedulingInfo{
					&desiredLRP1,
					&desiredLRP2,
				}
				Expect(schedulingInfos).To(ConsistOf(expectedSchedulingInfos))
			})
		})
	})

	Describe("DesireLRP", func() {
		var (
			desiredLRP *models.DesiredLRP
			desireErr  error
		)

		BeforeEach(func() {
			desiredLRP = model_helpers.NewValidDesiredLRP("super-lrp")
		})

		JustBeforeEach(func() {
			desireErr = client.DesireLRP(logger, desiredLRP)
		})

		It("creates the desired LRP in the system", func() {
			Expect(desireErr).NotTo(HaveOccurred())
			persistedDesiredLRP, err := client.DesiredLRPByProcessGuid(logger, "super-lrp")
			Expect(err).NotTo(HaveOccurred())
			Expect(persistedDesiredLRP.DesiredLRPKey()).To(Equal(desiredLRP.DesiredLRPKey()))
			Expect(persistedDesiredLRP.DesiredLRPResource()).To(Equal(desiredLRP.DesiredLRPResource()))
			Expect(persistedDesiredLRP.Annotation).To(Equal(desiredLRP.Annotation))
			Expect(persistedDesiredLRP.Instances).To(Equal(desiredLRP.Instances))
			Expect(persistedDesiredLRP.DesiredLRPRunInfo(time.Unix(42, 0))).To(Equal(desiredLRP.DesiredLRPRunInfo(time.Unix(42, 0))))
			Expect(persistedDesiredLRP.Action.RunAction.SuppressLogOutput).To(BeFalse())
			Expect(persistedDesiredLRP.CertificateProperties).NotTo(BeNil())
			Expect(persistedDesiredLRP.CertificateProperties.OrganizationalUnit).NotTo(BeEmpty())
			Expect(persistedDesiredLRP.CertificateProperties.OrganizationalUnit).To(Equal(desiredLRP.CertificateProperties.OrganizationalUnit))
			Expect(persistedDesiredLRP.ImageUsername).To(Equal(desiredLRP.ImageUsername))
			Expect(persistedDesiredLRP.ImagePassword).To(Equal(desiredLRP.ImagePassword))
		})

		Context("when suppressing log output", func() {
			BeforeEach(func() {
				desiredLRP.Action.RunAction.SuppressLogOutput = true
			})

			It("has an action with SuppressLogOutput set to true", func() {
				Expect(desireErr).NotTo(HaveOccurred())
				persistedDesiredLRP, err := client.DesiredLRPByProcessGuid(logger, "super-lrp")
				Expect(err).NotTo(HaveOccurred())
				Expect(persistedDesiredLRP.Action.RunAction.SuppressLogOutput).To(BeTrue())
			})
		})

		Context("when not suppressing log output", func() {
			BeforeEach(func() {
				desiredLRP.Action.RunAction.SuppressLogOutput = false
			})

			It("has an action with SuppressLogOutput set to false", func() {
				Expect(desireErr).NotTo(HaveOccurred())
				persistedDesiredLRP, err := client.DesiredLRPByProcessGuid(logger, "super-lrp")
				Expect(err).NotTo(HaveOccurred())
				Expect(persistedDesiredLRP.Action.RunAction.SuppressLogOutput).To(BeFalse())
			})
		})
	})

	Describe("RemoveDesiredLRP", func() {
		var (
			desiredLRP *models.DesiredLRP

			removeErr error
		)

		JustBeforeEach(func() {
			desiredLRP = model_helpers.NewValidDesiredLRP("super-lrp")
			err := client.DesireLRP(logger, desiredLRP)
			Expect(err).NotTo(HaveOccurred())
			removeErr = client.RemoveDesiredLRP(logger, "super-lrp")
		})

		It("creates the desired LRP in the system", func() {
			Expect(removeErr).NotTo(HaveOccurred())
			_, err := client.DesiredLRPByProcessGuid(logger, "super-lrp")
			Expect(err).To(HaveOccurred())
			Expect(err).To(Equal(models.ErrResourceNotFound))
		})
	})

	Describe("UpdateDesiredLRP", func() {
		var (
			desiredLRP *models.DesiredLRP

			updateErr error
		)

		JustBeforeEach(func() {
			desiredLRP = model_helpers.NewValidDesiredLRP("super-lrp")
			err := client.DesireLRP(logger, desiredLRP)
			Expect(err).NotTo(HaveOccurred())
			three := int32(3)
			updateErr = client.UpdateDesiredLRP(logger, "super-lrp", &models.DesiredLRPUpdate{Instances: &three})
		})

		It("creates the desired LRP in the system", func() {
			Expect(updateErr).NotTo(HaveOccurred())
			persistedDesiredLRP, err := client.DesiredLRPByProcessGuid(logger, "super-lrp")
			Expect(err).NotTo(HaveOccurred())
			Expect(persistedDesiredLRP.Instances).To(Equal(int32(3)))
		})
	})
})

func createDesiredLRPsInDomains(client bbs.InternalClient, domainCounts map[string]int) map[string][]*models.DesiredLRP {
	createdDesiredLRPs := map[string][]*models.DesiredLRP{}

	for domain, count := range domainCounts {
		createdDesiredLRPs[domain] = []*models.DesiredLRP{}

		for i := 0; i < count; i++ {
			guid := fmt.Sprintf("guid-%d-for-%s", i, domain)
			desiredLRP := model_helpers.NewValidDesiredLRP(guid)
			desiredLRP.Domain = domain
			err := client.DesireLRP(logger, desiredLRP)
			Expect(err).NotTo(HaveOccurred())

			createdDesiredLRPs[domain] = append(createdDesiredLRPs[domain], desiredLRP)
		}
	}

	return createdDesiredLRPs
}
