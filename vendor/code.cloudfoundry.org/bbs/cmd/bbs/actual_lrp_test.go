package main_test

import (
	"fmt"

	"code.cloudfoundry.org/bbs/cmd/bbs/testrunner"
	"code.cloudfoundry.org/bbs/models"
	"code.cloudfoundry.org/bbs/models/test/model_helpers"
	"code.cloudfoundry.org/bbs/test_helpers"
	"code.cloudfoundry.org/localip"
	"github.com/tedsuo/ifrit"
	"github.com/tedsuo/ifrit/ginkgomon"

	locketconfig "code.cloudfoundry.org/locket/cmd/locket/config"
	locketrunner "code.cloudfoundry.org/locket/cmd/locket/testrunner"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("ActualLRP API", func() {
	const (
		cellID          = "cell-id"
		otherCellID     = "other-cell-id"
		noExpirationTTL = 0

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

		baseIndex       = 0
		otherIndex      = 0
		evacuatingIndex = 0
		unclaimedIndex  = 0
		crashingIndex   = 0
	)

	var (
		actualActualLRPGroups []*models.ActualLRPGroup

		baseLRP               *models.ActualLRP
		otherLRP              *models.ActualLRP
		evacuatingLRP         *models.ActualLRP
		evacuatingInstanceLRP *models.ActualLRP
		unclaimedLRP          *models.ActualLRP
		crashingLRP           *models.ActualLRP

		baseLRPKey         models.ActualLRPKey
		baseLRPInstanceKey models.ActualLRPInstanceKey

		evacuatingLRPKey         models.ActualLRPKey
		evacuatingLRPInstanceKey models.ActualLRPInstanceKey

		otherLRPKey         models.ActualLRPKey
		otherLRPInstanceKey models.ActualLRPInstanceKey

		crashingLRPKey         models.ActualLRPKey
		crashingLRPInstanceKey models.ActualLRPInstanceKey

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
		otherLRPKey = models.NewActualLRPKey(otherProcessGuid, otherIndex, otherDomain)
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
		}

		evacuatingInstanceLRP = &models.ActualLRP{
			ActualLRPKey: evacuatingLRPKey,
			State:        models.ActualLRPStateUnclaimed,
		}

		otherLRP = &models.ActualLRP{
			ActualLRPKey:         otherLRPKey,
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

		var err error

		baseDesiredLRP := model_helpers.NewValidDesiredLRP(baseLRP.ProcessGuid)
		baseDesiredLRP.Domain = baseDomain
		err = client.DesireLRP(logger, baseDesiredLRP)
		Expect(err).NotTo(HaveOccurred())
		err = client.StartActualLRP(logger, &baseLRPKey, &baseLRPInstanceKey, &netInfo)
		Expect(err).NotTo(HaveOccurred())

		otherDesiredLRP := model_helpers.NewValidDesiredLRP(otherLRP.ProcessGuid)
		otherDesiredLRP.Domain = otherDomain
		Expect(client.DesireLRP(logger, otherDesiredLRP)).To(Succeed())
		err = client.StartActualLRP(logger, &otherLRPKey, &otherLRPInstanceKey, &netInfo)
		Expect(err).NotTo(HaveOccurred())

		evacuatingDesiredLRP := model_helpers.NewValidDesiredLRP(evacuatingLRP.ProcessGuid)
		evacuatingDesiredLRP.Domain = evacuatingDomain
		err = client.DesireLRP(logger, evacuatingDesiredLRP)
		Expect(err).NotTo(HaveOccurred())
		err = client.StartActualLRP(logger, &evacuatingLRPKey, &evacuatingLRPInstanceKey, &netInfo)
		Expect(err).NotTo(HaveOccurred())
		_, err = client.EvacuateRunningActualLRP(logger, &evacuatingLRPKey, &evacuatingLRPInstanceKey, &netInfo, noExpirationTTL)
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
					test_helpers.MatchActualLRPGroup(&models.ActualLRPGroup{Instance: otherLRP}),
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

		JustBeforeEach(func() {
			actualLRPGroup, getErr = client.ActualLRPGroupByProcessGuidAndIndex(logger, baseProcessGuid, baseIndex)
		})

		It("responds without error", func() {
			Expect(getErr).NotTo(HaveOccurred())
		})

		It("returns all actual lrps from the bbs", func() {
			expectedActualLRPGroup = &models.ActualLRPGroup{Instance: baseLRP}
			Expect(actualLRPGroup).To(test_helpers.MatchActualLRPGroup(expectedActualLRPGroup))
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
			claimErr = client.ClaimActualLRP(logger, unclaimedProcessGuid, unclaimedIndex, &instanceKey)
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

			fetchedActualLRP, _ := fetchedActualLRPGroup.Resolve()
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

			fetchedActualLRP, _ := fetchedActualLRPGroup.Resolve()
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
				locketPort, err := localip.LocalPort()
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
			removeErr = client.RemoveActualLRP(logger, otherProcessGuid, otherIndex, instanceKey)
		})

		Describe("when the instance key isn't preset", func() {
			BeforeEach(func() {
				instanceKey = nil
			})

			It("removes the actual_lrp", func() {
				Expect(removeErr).NotTo(HaveOccurred())

				_, err := client.ActualLRPGroupByProcessGuidAndIndex(logger, otherProcessGuid, otherIndex)
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

				_, err := client.ActualLRPGroupByProcessGuidAndIndex(logger, otherProcessGuid, otherIndex)
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
