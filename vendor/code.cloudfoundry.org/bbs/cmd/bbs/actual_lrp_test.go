package main_test

import (
	"fmt"

	"code.cloudfoundry.org/bbs/cmd/bbs/testrunner"
	"code.cloudfoundry.org/bbs/models"
	"code.cloudfoundry.org/bbs/models/test/model_helpers"
	"code.cloudfoundry.org/bbs/test_helpers"
	"github.com/tedsuo/ifrit"
	"github.com/tedsuo/ifrit/ginkgomon"

	locketconfig "code.cloudfoundry.org/locket/cmd/locket/config"
	locketrunner "code.cloudfoundry.org/locket/cmd/locket/testrunner"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("ActualLRP API", func() {
	const (
		cellID      = "cell-id"
		otherCellID = "other-cell-id"

		baseProcessGuid  = "base-process-guid"
		baseDomain       = "base-domain"
		baseInstanceGuid = "base-instance-guid"

		evacuatingProcessGuid  = "evacuating-process-guid"
		evacuatingDomain       = "evacuating-domain"
		evacuatingInstanceGuid = "evacuating-instance-guid"

		otherProcessGuid  = "other-process-guid"
		otherDomain       = "other-domain"
		otherInstanceGuid = "other-instance-guid"

		unclaimedProcessGuid = "unclaimed-process-guid"
		unclaimedDomain      = "unclaimed-domain"

		crashingProcessGuid  = "crashing-process-guid"
		crashingDomain       = "crashing-domain"
		crashingInstanceGuid = "crashing-instance-guid"

		retiredProcessGuid  = "retired-process-guid"
		retiredDomain       = "retired-domain"
		retiredInstanceGuid = "retired-instance-guid"

		baseIndex       = 0
		otherIndex0     = 0
		otherIndex1     = 1
		evacuatingIndex = 0
		unclaimedIndex  = 0
		crashingIndex   = 0
		retiredIndex    = 0
	)

	var (
		actualActualLRPGroups []*models.ActualLRPGroup

		baseLRP               *models.ActualLRP
		otherLRP0             *models.ActualLRP
		otherLRP1             *models.ActualLRP
		evacuatingLRP         *models.ActualLRP
		evacuatingInstanceLRP *models.ActualLRP
		unclaimedLRP          *models.ActualLRP
		crashingLRP           *models.ActualLRP
		retiredLRP            *models.ActualLRP

		baseLRPKey         models.ActualLRPKey
		baseLRPInstanceKey models.ActualLRPInstanceKey

		evacuatingLRPKey         models.ActualLRPKey
		evacuatingLRPInstanceKey models.ActualLRPInstanceKey

		otherLRP0Key        models.ActualLRPKey
		otherLRP1Key        models.ActualLRPKey
		otherLRPInstanceKey models.ActualLRPInstanceKey

		crashingLRPKey         models.ActualLRPKey
		crashingLRPInstanceKey models.ActualLRPInstanceKey

		retiredLRPKey         models.ActualLRPKey
		retiredLRPInstanceKey models.ActualLRPInstanceKey

		netInfo         models.ActualLRPNetInfo
		unclaimedLRPKey models.ActualLRPKey

		filter models.ActualLRPFilter

		getErr error
	)

	BeforeEach(func() {
		filter = models.ActualLRPFilter{}
	})

	JustBeforeEach(func() {
		bbsRunner = testrunner.New(bbsBinPath, bbsConfig)
		bbsProcess = ginkgomon.Invoke(bbsRunner)

		actualActualLRPGroups = []*models.ActualLRPGroup{}

		baseLRPKey = models.NewActualLRPKey(baseProcessGuid, baseIndex, baseDomain)
		baseLRPInstanceKey = models.NewActualLRPInstanceKey(baseInstanceGuid, cellID)

		evacuatingLRPKey = models.NewActualLRPKey(evacuatingProcessGuid, evacuatingIndex, evacuatingDomain)
		evacuatingLRPInstanceKey = models.NewActualLRPInstanceKey(evacuatingInstanceGuid, cellID)

		retiredLRPKey = models.NewActualLRPKey(retiredProcessGuid, retiredIndex, retiredDomain)
		retiredLRPInstanceKey = models.NewActualLRPInstanceKey(retiredInstanceGuid, cellID)

		otherLRP0Key = models.NewActualLRPKey(otherProcessGuid, otherIndex0, otherDomain)
		otherLRP1Key = models.NewActualLRPKey(otherProcessGuid, otherIndex1, otherDomain)
		otherLRPInstanceKey = models.NewActualLRPInstanceKey(otherInstanceGuid, otherCellID)

		netInfo = models.NewActualLRPNetInfo("127.0.0.1", "10.10.10.10", models.NewPortMapping(8080, 80))

		unclaimedLRPKey = models.NewActualLRPKey(unclaimedProcessGuid, unclaimedIndex, unclaimedDomain)

		crashingLRPKey = models.NewActualLRPKey(crashingProcessGuid, crashingIndex, crashingDomain)
		crashingLRPInstanceKey = models.NewActualLRPInstanceKey(crashingInstanceGuid, otherCellID)

		baseLRP = &models.ActualLRP{
			ActualLRPKey:         baseLRPKey,
			ActualLRPInstanceKey: baseLRPInstanceKey,
			ActualLRPNetInfo:     netInfo,
			State:                models.ActualLRPStateRunning,
		}

		evacuatingLRP = &models.ActualLRP{
			ActualLRPKey:         evacuatingLRPKey,
			ActualLRPInstanceKey: evacuatingLRPInstanceKey,
			ActualLRPNetInfo:     netInfo,
			State:                models.ActualLRPStateRunning,
			Presence:             models.ActualLRP_Evacuating,
		}

		evacuatingInstanceLRP = &models.ActualLRP{
			ActualLRPKey: evacuatingLRPKey,
			State:        models.ActualLRPStateUnclaimed,
		}

		otherLRP0 = &models.ActualLRP{
			ActualLRPKey:         otherLRP0Key,
			ActualLRPInstanceKey: otherLRPInstanceKey,
			ActualLRPNetInfo:     netInfo,
			State:                models.ActualLRPStateRunning,
		}

		otherLRP1 = &models.ActualLRP{
			ActualLRPKey:         otherLRP1Key,
			ActualLRPInstanceKey: otherLRPInstanceKey,
			ActualLRPNetInfo:     netInfo,
			State:                models.ActualLRPStateRunning,
		}

		unclaimedLRP = &models.ActualLRP{
			ActualLRPKey: unclaimedLRPKey,
			State:        models.ActualLRPStateUnclaimed,
		}

		crashingLRP = &models.ActualLRP{
			ActualLRPKey: crashingLRPKey,
			State:        models.ActualLRPStateCrashed,
			CrashReason:  "crash",
			CrashCount:   3,
		}

		retiredLRP = &models.ActualLRP{
			ActualLRPKey: retiredLRPKey,
			State:        models.ActualLRPStateRunning,
		}

		var err error

		baseDesiredLRP := model_helpers.NewValidDesiredLRP(baseLRP.ProcessGuid)
		baseDesiredLRP.Domain = baseDomain
		err = client.DesireLRP(logger, baseDesiredLRP)
		Expect(err).NotTo(HaveOccurred())
		err = client.StartActualLRP(logger, &baseLRPKey, &baseLRPInstanceKey, &netInfo)
		Expect(err).NotTo(HaveOccurred())

		otherDesiredLRP := model_helpers.NewValidDesiredLRP(otherLRP0.ProcessGuid)
		otherDesiredLRP.Domain = otherDomain
		Expect(client.DesireLRP(logger, otherDesiredLRP)).To(Succeed())
		err = client.StartActualLRP(logger, &otherLRP0Key, &otherLRPInstanceKey, &netInfo)
		Expect(err).NotTo(HaveOccurred())
		err = client.StartActualLRP(logger, &otherLRP1Key, &otherLRPInstanceKey, &netInfo)
		Expect(err).NotTo(HaveOccurred())

		evacuatingDesiredLRP := model_helpers.NewValidDesiredLRP(evacuatingLRP.ProcessGuid)
		evacuatingDesiredLRP.Domain = evacuatingDomain
		err = client.DesireLRP(logger, evacuatingDesiredLRP)
		Expect(err).NotTo(HaveOccurred())
		err = client.StartActualLRP(logger, &evacuatingLRPKey, &evacuatingLRPInstanceKey, &netInfo)
		Expect(err).NotTo(HaveOccurred())
		_, err = client.EvacuateRunningActualLRP(logger, &evacuatingLRPKey, &evacuatingLRPInstanceKey, &netInfo)
		Expect(err).NotTo(HaveOccurred())

		unclaimedDesiredLRP := model_helpers.NewValidDesiredLRP(unclaimedLRP.ProcessGuid)
		unclaimedDesiredLRP.Domain = unclaimedDomain
		err = client.DesireLRP(logger, unclaimedDesiredLRP)
		Expect(err).NotTo(HaveOccurred())

		crashingDesiredLRP := model_helpers.NewValidDesiredLRP(crashingLRP.ProcessGuid)
		crashingDesiredLRP.Domain = crashingDomain
		Expect(client.DesireLRP(logger, crashingDesiredLRP)).To(Succeed())
		for i := 0; i < 3; i++ {
			err = client.StartActualLRP(logger, &crashingLRPKey, &crashingLRPInstanceKey, &netInfo)
			Expect(err).NotTo(HaveOccurred())
			err = client.CrashActualLRP(logger, &crashingLRPKey, &crashingLRPInstanceKey, "crash")
			Expect(err).NotTo(HaveOccurred())
		}

		retiredDesiredLRP := model_helpers.NewValidDesiredLRP(retiredLRP.ProcessGuid)
		retiredDesiredLRP.Domain = retiredDomain
		err = client.DesireLRP(logger, retiredDesiredLRP)
		Expect(err).NotTo(HaveOccurred())
		err = client.StartActualLRP(logger, &retiredLRPKey, &retiredLRPInstanceKey, &netInfo)
		Expect(err).NotTo(HaveOccurred())
		retireErr := client.RetireActualLRP(logger, &retiredLRPKey)
		Expect(retireErr).NotTo(HaveOccurred())
	})

	Describe("ActualLRPs", func() {
		var actualActualLRPs []*models.ActualLRP

		It("responds without error", func() {
			actualActualLRPs, getErr = client.ActualLRPs(logger, filter)
			Expect(getErr).NotTo(HaveOccurred())
		})

		Context("when not filtering", func() {
			It("returns all actual lrps from the bbs", func() {
				actualActualLRPs, getErr = client.ActualLRPs(logger, filter)
				Expect(getErr).NotTo(HaveOccurred())

				Expect(actualActualLRPs).To(ConsistOf(
					test_helpers.MatchActualLRP(baseLRP),
					test_helpers.MatchActualLRP(evacuatingInstanceLRP),
					test_helpers.MatchActualLRP(evacuatingLRP),
					test_helpers.MatchActualLRP(otherLRP0),
					test_helpers.MatchActualLRP(otherLRP1),
					test_helpers.MatchActualLRP(unclaimedLRP),
					test_helpers.MatchActualLRP(crashingLRP),
				))
			})
		})

		Context("when filtering by domain", func() {
			BeforeEach(func() {
				filter = models.ActualLRPFilter{Domain: baseDomain}
			})

			It("returns actual lrps from the requested domain", func() {
				actualActualLRPs, getErr = client.ActualLRPs(logger, filter)
				Expect(getErr).NotTo(HaveOccurred())

				Expect(actualActualLRPs).To(ConsistOf(test_helpers.MatchActualLRP(baseLRP)))
			})
		})

		Context("when filtering by cell", func() {
			BeforeEach(func() {
				filter = models.ActualLRPFilter{CellID: cellID}
			})

			It("returns actual lrps from the requested cell", func() {
				actualActualLRPs, getErr = client.ActualLRPs(logger, filter)
				Expect(getErr).NotTo(HaveOccurred())
				Expect(actualActualLRPs).To(ConsistOf(
					test_helpers.MatchActualLRP(baseLRP),
					test_helpers.MatchActualLRP(evacuatingLRP),
				))
			})
		})

		Context("when filtering by process GUID", func() {
			BeforeEach(func() {
				filter = models.ActualLRPFilter{ProcessGuid: otherProcessGuid}
			})

			It("returns the actual lrps with the requested process GUID", func() {
				actualActualLRPs, getErr = client.ActualLRPs(logger, filter)
				Expect(getErr).NotTo(HaveOccurred())
				Expect(actualActualLRPs).To(ConsistOf(
					test_helpers.MatchActualLRP(otherLRP0),
					test_helpers.MatchActualLRP(otherLRP1),
				))
			})
		})

		Context("when filtering by index", func() {
			BeforeEach(func() {
				Expect(otherIndex1).NotTo(Equal(baseIndex))
				filterIdx := int32(otherIndex1)
				filter = models.ActualLRPFilter{Index: &filterIdx}
			})

			It("returns the actual lrps with the requested index", func() {
				actualActualLRPs, getErr = client.ActualLRPs(logger, filter)
				Expect(getErr).NotTo(HaveOccurred())
				Expect(actualActualLRPs).To(ConsistOf(
					test_helpers.MatchActualLRP(otherLRP1),
				))
			})
		})

		Context("when filtering by cell ID and index", func() {
			BeforeEach(func() {
				Expect(otherIndex1).NotTo(Equal(baseIndex))
				filterIdx := int32(otherIndex1)
				filter = models.ActualLRPFilter{
					CellID: cellID,
					Index:  &filterIdx,
				}
			})

			It("returns the actual lrps that matches the filter combination", func() {
				actualActualLRPs, getErr = client.ActualLRPs(logger, filter)
				Expect(getErr).NotTo(HaveOccurred())
				Expect(actualActualLRPs).To(BeEmpty())
			})
		})

		Context("with a TLS-enabled actual LRP", func() {
			const (
				tlsEnabledProcessGuid  = "tlsEnabled-process-guid"
				tlsEnabledDomain       = "tlsEnabled-domain"
				tlsEnabledInstanceGuid = "tlsEnabled-instance-guid"
				tlsEnabledIndex        = 0
			)
			var (
				tlsEnabledLRP            *models.ActualLRP
				tlsEnabledLRPKey         models.ActualLRPKey
				tlsEnabledLRPInstanceKey models.ActualLRPInstanceKey
				tlsNetInfo               models.ActualLRPNetInfo
			)

			JustBeforeEach(func() {
				tlsEnabledLRPKey = models.NewActualLRPKey(tlsEnabledProcessGuid, tlsEnabledIndex, tlsEnabledDomain)
				tlsEnabledLRPInstanceKey = models.NewActualLRPInstanceKey(tlsEnabledInstanceGuid, cellID)
				tlsNetInfo = models.NewActualLRPNetInfo("127.0.0.1", "10.10.10.10", models.NewPortMappingWithTLSProxy(8080, 80, 60042, 443))

				tlsEnabledLRP = &models.ActualLRP{
					ActualLRPKey:         tlsEnabledLRPKey,
					ActualLRPInstanceKey: tlsEnabledLRPInstanceKey,
					ActualLRPNetInfo:     tlsNetInfo,
					State:                models.ActualLRPStateRunning,
				}

				tlsEnabledDesiredLRP := model_helpers.NewValidDesiredLRP(tlsEnabledLRP.ProcessGuid)
				tlsEnabledDesiredLRP.Domain = tlsEnabledDomain

				err := client.DesireLRP(logger, tlsEnabledDesiredLRP)
				Expect(err).NotTo(HaveOccurred())
				err = client.StartActualLRP(logger, &tlsEnabledLRPKey, &tlsEnabledLRPInstanceKey, &tlsNetInfo)
				Expect(err).NotTo(HaveOccurred())
			})

			It("returns the TLS host and container port", func() {
				actualLRPs, err := client.ActualLRPs(logger, filter)
				Expect(err).NotTo(HaveOccurred())

				Expect(actualLRPs).To(ContainElement(test_helpers.MatchActualLRP(tlsEnabledLRP)))
			})
		})
	})

	Describe("ActualLRPGroups", func() {
		It("responds without error", func() {
			actualActualLRPGroups, getErr = client.ActualLRPGroups(logger, filter)
			Expect(getErr).NotTo(HaveOccurred())
		})

		Context("when not filtering", func() {
			It("returns all actual lrps from the bbs", func() {
				actualActualLRPGroups, getErr = client.ActualLRPGroups(logger, filter)
				Expect(getErr).NotTo(HaveOccurred())

				Expect(actualActualLRPGroups).To(ConsistOf(
					test_helpers.MatchActualLRPGroup(&models.ActualLRPGroup{Instance: baseLRP}),
					test_helpers.MatchActualLRPGroup(&models.ActualLRPGroup{Instance: evacuatingInstanceLRP, Evacuating: evacuatingLRP}),
					test_helpers.MatchActualLRPGroup(&models.ActualLRPGroup{Instance: otherLRP0}),
					test_helpers.MatchActualLRPGroup(&models.ActualLRPGroup{Instance: otherLRP1}),
					test_helpers.MatchActualLRPGroup(&models.ActualLRPGroup{Instance: unclaimedLRP}),
					test_helpers.MatchActualLRPGroup(&models.ActualLRPGroup{Instance: crashingLRP}),
				))
			})
		})

		Context("when filtering by domain", func() {
			BeforeEach(func() {
				filter = models.ActualLRPFilter{Domain: baseDomain}
			})

			It("returns actual lrps from the requested domain", func() {
				actualActualLRPGroups, getErr = client.ActualLRPGroups(logger, filter)
				Expect(getErr).NotTo(HaveOccurred())

				expectedActualLRPGroup := &models.ActualLRPGroup{Instance: baseLRP}
				Expect(actualActualLRPGroups).To(ConsistOf(test_helpers.MatchActualLRPGroup(expectedActualLRPGroup)))
			})
		})

		Context("when filtering by cell", func() {
			BeforeEach(func() {
				filter = models.ActualLRPFilter{CellID: cellID}
			})

			It("returns actual lrps from the requested cell", func() {
				actualActualLRPGroups, getErr = client.ActualLRPGroups(logger, filter)
				Expect(getErr).NotTo(HaveOccurred())
				Expect(actualActualLRPGroups).To(ConsistOf(
					test_helpers.MatchActualLRPGroup(&models.ActualLRPGroup{Instance: baseLRP}),
					test_helpers.MatchActualLRPGroup(&models.ActualLRPGroup{Evacuating: evacuatingLRP}),
				))
			})
		})

		Context("with a TLS-enabled actual LRP", func() {
			const (
				tlsEnabledProcessGuid  = "tlsEnabled-process-guid"
				tlsEnabledDomain       = "tlsEnabled-domain"
				tlsEnabledInstanceGuid = "tlsEnabled-instance-guid"
				tlsEnabledIndex        = 0
			)
			var (
				tlsEnabledLRP            *models.ActualLRP
				tlsEnabledLRPKey         models.ActualLRPKey
				tlsEnabledLRPInstanceKey models.ActualLRPInstanceKey
				tlsNetInfo               models.ActualLRPNetInfo
			)

			JustBeforeEach(func() {
				tlsEnabledLRPKey = models.NewActualLRPKey(tlsEnabledProcessGuid, tlsEnabledIndex, tlsEnabledDomain)
				tlsEnabledLRPInstanceKey = models.NewActualLRPInstanceKey(tlsEnabledInstanceGuid, cellID)
				tlsNetInfo = models.NewActualLRPNetInfo("127.0.0.1", "10.10.10.10", models.NewPortMappingWithTLSProxy(8080, 80, 60042, 443))

				tlsEnabledLRP = &models.ActualLRP{
					ActualLRPKey:         tlsEnabledLRPKey,
					ActualLRPInstanceKey: tlsEnabledLRPInstanceKey,
					ActualLRPNetInfo:     tlsNetInfo,
					State:                models.ActualLRPStateRunning,
				}

				tlsEnabledDesiredLRP := model_helpers.NewValidDesiredLRP(tlsEnabledLRP.ProcessGuid)
				tlsEnabledDesiredLRP.Domain = tlsEnabledDomain

				err := client.DesireLRP(logger, tlsEnabledDesiredLRP)
				Expect(err).NotTo(HaveOccurred())
				err = client.StartActualLRP(logger, &tlsEnabledLRPKey, &tlsEnabledLRPInstanceKey, &tlsNetInfo)
				Expect(err).NotTo(HaveOccurred())
			})

			It("returns the TLS host and container port", func() {
				actualLRPGroups, err := client.ActualLRPGroups(logger, filter)
				Expect(err).NotTo(HaveOccurred())

				tlsGroup := &models.ActualLRPGroup{Instance: tlsEnabledLRP}
				Expect(actualLRPGroups).To(ContainElement(test_helpers.MatchActualLRPGroup(tlsGroup)))
			})
		})
	})

	Describe("ActualLRPGroupsByProcessGuid", func() {
		JustBeforeEach(func() {
			actualActualLRPGroups, getErr = client.ActualLRPGroupsByProcessGuid(logger, baseProcessGuid)
		})

		It("returns the specific actual lrp from the bbs", func() {
			Expect(getErr).NotTo(HaveOccurred())
			Expect(actualActualLRPGroups).To(HaveLen(1))

			fetchedActualLRPGroup := actualActualLRPGroups[0]
			Expect(fetchedActualLRPGroup).To(
				test_helpers.MatchActualLRPGroup(&models.ActualLRPGroup{Instance: baseLRP}),
			)
		})
	})

	Describe("ActualLRPGroupByProcessGuidAndIndex", func() {
		var (
			actualLRPGroup         *models.ActualLRPGroup
			expectedActualLRPGroup *models.ActualLRPGroup
		)

		It("responds without error", func() {
			actualLRPGroup, getErr = client.ActualLRPGroupByProcessGuidAndIndex(logger, baseProcessGuid, baseIndex)
			Expect(getErr).NotTo(HaveOccurred())
		})

		It("returns all actual lrps from the bbs", func() {
			actualLRPGroup, getErr = client.ActualLRPGroupByProcessGuidAndIndex(logger, baseProcessGuid, baseIndex)
			expectedActualLRPGroup = &models.ActualLRPGroup{Instance: baseLRP}
			Expect(actualLRPGroup).To(test_helpers.MatchActualLRPGroup(expectedActualLRPGroup))
		})

		Context("when no ActualLRP group matches the process guid and index", func() {
			It("returns an error", func() {
				_, err := client.ActualLRPGroupByProcessGuidAndIndex(logger, retiredProcessGuid, retiredIndex)
				Expect(err).To(Equal(models.ErrResourceNotFound))
			})
		})
	})

	Describe("ClaimActualLRP", func() {
		var (
			instanceKey models.ActualLRPInstanceKey
			claimErr    error
		)

		JustBeforeEach(func() {
			instanceKey = models.ActualLRPInstanceKey{
				CellId:       "my-cell-id",
				InstanceGuid: "my-instance-guid",
			}
			claimErr = client.ClaimActualLRP(logger, &unclaimedLRPKey, &instanceKey)
		})

		It("claims the actual_lrp", func() {
			Expect(claimErr).NotTo(HaveOccurred())

			expectedActualLRP := *unclaimedLRP
			expectedActualLRP.State = models.ActualLRPStateClaimed
			expectedActualLRP.ActualLRPInstanceKey = instanceKey

			fetchedActualLRPGroup, err := client.ActualLRPGroupByProcessGuidAndIndex(logger, unclaimedProcessGuid, unclaimedIndex)
			Expect(err).NotTo(HaveOccurred())

			Expect(fetchedActualLRPGroup).To(test_helpers.MatchActualLRPGroup(
				&models.ActualLRPGroup{Instance: &expectedActualLRP}),
			)
		})
	})

	Describe("StartActualLRP", func() {
		var (
			instanceKey models.ActualLRPInstanceKey
			startErr    error
		)

		JustBeforeEach(func() {
			instanceKey = models.ActualLRPInstanceKey{
				CellId:       "my-cell-id",
				InstanceGuid: "my-instance-guid",
			}
			startErr = client.StartActualLRP(logger, &unclaimedLRPKey, &instanceKey, &netInfo)
		})

		It("starts the actual_lrp", func() {
			Expect(startErr).NotTo(HaveOccurred())

			expectedActualLRP := *unclaimedLRP
			expectedActualLRP.State = models.ActualLRPStateRunning
			expectedActualLRP.ActualLRPInstanceKey = instanceKey
			expectedActualLRP.ActualLRPNetInfo = netInfo

			fetchedActualLRPGroup, err := client.ActualLRPGroupByProcessGuidAndIndex(logger, unclaimedProcessGuid, unclaimedIndex)
			Expect(err).NotTo(HaveOccurred())

			Expect(fetchedActualLRPGroup).To(test_helpers.MatchActualLRPGroup(
				&models.ActualLRPGroup{Instance: &expectedActualLRP}),
			)
		})
	})

	Describe("FailActualLRP", func() {
		var (
			errorMessage string
			failErr      error
		)

		JustBeforeEach(func() {
			errorMessage = "some bad ocurred"
			failErr = client.FailActualLRP(logger, &unclaimedLRPKey, errorMessage)
		})

		It("fails the actual_lrp", func() {
			Expect(failErr).NotTo(HaveOccurred())

			fetchedActualLRPGroup, err := client.ActualLRPGroupByProcessGuidAndIndex(logger, unclaimedProcessGuid, unclaimedIndex)
			Expect(err).NotTo(HaveOccurred())

			fetchedActualLRP, _, resolveError := fetchedActualLRPGroup.Resolve()
			Expect(resolveError).NotTo(HaveOccurred())
			Expect(fetchedActualLRP.PlacementError).To(Equal(errorMessage))
		})
	})

	Describe("CrashActualLRP", func() {
		var (
			errorMessage string
			crashErr     error
		)

		JustBeforeEach(func() {
			errorMessage = "some bad ocurred"
			crashErr = client.CrashActualLRP(logger, &baseLRPKey, &baseLRPInstanceKey, errorMessage)
		})

		It("crashes the actual_lrp", func() {
			Expect(crashErr).NotTo(HaveOccurred())

			fetchedActualLRPGroup, err := client.ActualLRPGroupByProcessGuidAndIndex(logger, baseProcessGuid, baseIndex)
			Expect(err).NotTo(HaveOccurred())

			fetchedActualLRP, _, resolveError := fetchedActualLRPGroup.Resolve()
			Expect(resolveError).NotTo(HaveOccurred())
			Expect(fetchedActualLRP.State).To(Equal(models.ActualLRPStateUnclaimed))
			Expect(fetchedActualLRP.CrashCount).To(Equal(int32(1)))
			Expect(fetchedActualLRP.CrashReason).To(Equal(errorMessage))
		})
	})

	Describe("RetireActualLRP", func() {
		var (
			retireErr error
		)

		JustBeforeEach(func() {
			retireErr = client.RetireActualLRP(logger, &unclaimedLRPKey)
		})

		It("retires the actual_lrp", func() {
			Expect(retireErr).NotTo(HaveOccurred())

			_, err := client.ActualLRPGroupByProcessGuidAndIndex(logger, unclaimedProcessGuid, unclaimedIndex)
			Expect(err).To(Equal(models.ErrResourceNotFound))
		})

		Context("when using locket cell presences", func() {
			var (
				locketProcess ifrit.Process
			)

			BeforeEach(func() {
				locketPort, err := portAllocator.ClaimPorts(1)
				Expect(err).NotTo(HaveOccurred())

				locketAddress := fmt.Sprintf("localhost:%d", locketPort)

				locketRunner := locketrunner.NewLocketRunner(locketBinPath, func(cfg *locketconfig.LocketConfig) {
					cfg.ConsulCluster = consulRunner.ConsulCluster()
					cfg.DatabaseConnectionString = sqlRunner.ConnectionString()
					cfg.DatabaseDriver = sqlRunner.DriverName()
					cfg.ListenAddress = locketAddress
				})

				locketProcess = ginkgomon.Invoke(locketRunner)
				bbsConfig.ClientLocketConfig = locketrunner.ClientLocketConfig()
				bbsConfig.LocketAddress = locketAddress

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
			})

			AfterEach(func() {
				ginkgomon.Interrupt(locketProcess)
			})

			It("retires an actual LRP when not found in locket", func() {
				retireErr = client.RetireActualLRP(logger, &baseLRPKey)
				Expect(retireErr).NotTo(HaveOccurred())

				_, err := client.ActualLRPGroupByProcessGuidAndIndex(logger, baseProcessGuid, baseIndex)
				Expect(err).To(Equal(models.ErrResourceNotFound))
			})
		})
	})

	Describe("RemoveActualLRP", func() {
		var (
			removeErr   error
			instanceKey *models.ActualLRPInstanceKey
		)

		JustBeforeEach(func() {
			removeErr = client.RemoveActualLRP(logger, &otherLRP0Key, instanceKey)
		})

		Describe("when the instance key isn't preset", func() {
			BeforeEach(func() {
				instanceKey = nil
			})

			It("removes the actual_lrp", func() {
				Expect(removeErr).NotTo(HaveOccurred())

				_, err := client.ActualLRPGroupByProcessGuidAndIndex(logger, otherProcessGuid, otherIndex0)
				Expect(err).To(HaveOccurred())
				Expect(err).To(Equal(models.ErrResourceNotFound))
			})
		})

		Describe("when the instance key is equal to the current instance key", func() {
			BeforeEach(func() {
				instanceKey = &otherLRPInstanceKey
			})

			It("removes the actual_lrp", func() {
				Expect(removeErr).NotTo(HaveOccurred())

				_, err := client.ActualLRPGroupByProcessGuidAndIndex(logger, otherProcessGuid, otherIndex0)
				Expect(err).To(HaveOccurred())
				Expect(err).To(Equal(models.ErrResourceNotFound))
			})
		})

		Describe("when the instance key is not equal to the current instance key", func() {
			BeforeEach(func() {
				instanceKey = &baseLRPInstanceKey
			})

			It("returns an error", func() {
				Expect(removeErr).To(Equal(models.ErrResourceNotFound))
			})
		})
	})
})
