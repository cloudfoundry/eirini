package bulk_test

import (
	"code.cloudfoundry.org/bbs/models"
	"code.cloudfoundry.org/lager/lagertest"
	"code.cloudfoundry.org/nsync/bulk"
	"code.cloudfoundry.org/runtimeschema/cc_messages"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Differ", func() {
	var (
		existingSchedulingInfo    *models.DesiredLRPSchedulingInfo
		existingSchedulingInfoMap map[string]*models.DesiredLRPSchedulingInfo
		existingAppFingerprint    cc_messages.CCDesiredAppFingerprint

		cancelChan  chan struct{}
		desiredChan chan []cc_messages.CCDesiredAppFingerprint

		staleChan   <-chan []cc_messages.CCDesiredAppFingerprint
		missingChan <-chan []cc_messages.CCDesiredAppFingerprint
		deletedChan <-chan []string

		errorsChan <-chan error

		logger *lagertest.TestLogger
		differ bulk.AppDiffer
	)

	BeforeEach(func() {
		logger = lagertest.NewTestLogger("test")

		existingSchedulingInfo = &models.DesiredLRPSchedulingInfo{
			DesiredLRPKey: models.NewDesiredLRPKey("process-guid", "domain", "log-guid"),
			Instances:     1,
			Annotation:    "some-etag-1",
		}

		existingAppFingerprint = cc_messages.CCDesiredAppFingerprint{
			ProcessGuid: existingSchedulingInfo.ProcessGuid,
			ETag:        existingSchedulingInfo.Annotation,
		}

		desiredChan = make(chan []cc_messages.CCDesiredAppFingerprint, 1)
		cancelChan = make(chan struct{})
	})

	JustBeforeEach(func() {
		existingSchedulingInfoMap = map[string]*models.DesiredLRPSchedulingInfo{
			existingSchedulingInfo.ProcessGuid: existingSchedulingInfo,
		}
		differ = bulk.NewAppDiffer(existingSchedulingInfoMap)

		staleChan = differ.Stale()
		missingChan = differ.Missing()
		deletedChan = differ.Deleted()

		errorsChan = differ.Diff(logger, cancelChan, desiredChan)
	})

	AfterEach(func() {
		Expect(existingSchedulingInfoMap).To(Equal(existingSchedulingInfoMap))
		Eventually(staleChan).Should(BeClosed())
		Eventually(missingChan).Should(BeClosed())
		Eventually(deletedChan).Should(BeClosed())
		Eventually(errorsChan).Should(BeClosed())
	})

	Context("when desired apps come in from CC", func() {
		var desiredAppFingerprints []cc_messages.CCDesiredAppFingerprint

		BeforeEach(func() {
			desiredAppFingerprints = []cc_messages.CCDesiredAppFingerprint{
				existingAppFingerprint,
			}
		})

		Context("existing desired LRPs and desired apps are consistent", func() {
			BeforeEach(func() {
				desiredChan <- desiredAppFingerprints
				close(desiredChan)
			})

			It("sends nothing to downstream channels", func() {
				Consistently(staleChan).ShouldNot(Receive())
				Consistently(missingChan).ShouldNot(Receive())
				Consistently(deletedChan).ShouldNot(Receive())
			})
		})

		Context("and some are missing from the existing desired LRPs set", func() {
			var missingAppFingerprints []cc_messages.CCDesiredAppFingerprint

			BeforeEach(func() {
				missingAppFingerprints = []cc_messages.CCDesiredAppFingerprint{
					cc_messages.CCDesiredAppFingerprint{
						ProcessGuid: "missing-guid-1",
						ETag:        "missing-etag-1",
					},
					cc_messages.CCDesiredAppFingerprint{
						ProcessGuid: "missing-guid-2",
						ETag:        "missing-etag-2",
					},
				}

				desiredAppFingerprints := []cc_messages.CCDesiredAppFingerprint{
					existingAppFingerprint,
					missingAppFingerprints[0],
					missingAppFingerprints[1],
				}

				desiredChan <- desiredAppFingerprints
				close(desiredChan)
			})

			It("sends a slice of missing fingerprints across the missing channel", func() {
				Eventually(missingChan).Should(Receive(ConsistOf(missingAppFingerprints)))

				Consistently(staleChan).ShouldNot(Receive())
				Consistently(deletedChan).ShouldNot(Receive())
			})
		})

		Context("and an existing desired LRP is not a desired app", func() {
			BeforeEach(func() {
				close(desiredChan)
			})

			It("sends a slice of process guids to the deleted channel that includes the excess LRP", func() {
				Eventually(deletedChan).Should(Receive(ConsistOf(existingSchedulingInfo.ProcessGuid)))

				Consistently(staleChan).ShouldNot(Receive())
				Consistently(missingChan).ShouldNot(Receive())
			})
		})

		Context("and an existing desired LRP has a stale ETag", func() {
			var fingerprint cc_messages.CCDesiredAppFingerprint

			BeforeEach(func() {
				fingerprint = existingAppFingerprint
				fingerprint.ETag = "updated-etag"

				desiredChan <- []cc_messages.CCDesiredAppFingerprint{fingerprint}
				close(desiredChan)
			})

			It("includes the fingerprint of the stale LRP in the slice sent on the stale channel", func() {
				Eventually(staleChan).Should(Receive(ConsistOf(fingerprint)))

				Consistently(staleChan).ShouldNot(Receive())
				Consistently(deletedChan).ShouldNot(Receive())
			})
		})
	})

	Context("while the desired app channel remains open", func() {
		AfterEach(func() {
			close(desiredChan)
		})

		It("continues to process the apps in batches", func() {
			fingerprint := cc_messages.CCDesiredAppFingerprint{
				ProcessGuid: "missing-process-guid",
				ETag:        "missing-etag",
			}
			desiredAppFingerprints := []cc_messages.CCDesiredAppFingerprint{fingerprint}

			Eventually(desiredChan).Should(BeSent(desiredAppFingerprints))
			Eventually(missingChan).Should(Receive(ConsistOf(desiredAppFingerprints)))

			desiredAppFingerprints = []cc_messages.CCDesiredAppFingerprint{}
			Eventually(desiredChan).Should(BeSent(desiredAppFingerprints))
		})

		It("does not close the deletedChan", func() {
			Consistently(deletedChan).ShouldNot(BeClosed())
		})
	})

	Describe("cancelling", func() {
		Context("when waiting for desired fingerprints", func() {
			It("closes the output channels", func() {
				close(cancelChan)

				Eventually(staleChan).Should(BeClosed())
				Eventually(missingChan).Should(BeClosed())
				Eventually(deletedChan).Should(BeClosed())
				Eventually(errorsChan).Should(BeClosed())
			})
		})

		Context("when waiting to send missing fingerprints", func() {
			BeforeEach(func() {
				Eventually(desiredChan).Should(BeSent([]cc_messages.CCDesiredAppFingerprint{{
					ProcessGuid: "missing-process-guid",
					ETag:        "missing-process-etag",
				}}))
			})

			It("closes the output channels", func() {
				close(cancelChan)

				Eventually(staleChan).Should(BeClosed())
				Eventually(missingChan).Should(BeClosed())
				Eventually(deletedChan).Should(BeClosed())
				Eventually(errorsChan).Should(BeClosed())
			})
		})

		Context("when waiting to send stale fingerprints", func() {
			BeforeEach(func() {
				existingAppFingerprint.ETag = "updated-etag"
				Eventually(desiredChan).Should(BeSent([]cc_messages.CCDesiredAppFingerprint{
					existingAppFingerprint,
				}))
			})

			It("closes the output channels", func() {
				close(cancelChan)

				Eventually(staleChan).Should(BeClosed())
				Eventually(missingChan).Should(BeClosed())
				Eventually(deletedChan).Should(BeClosed())
				Eventually(errorsChan).Should(BeClosed())
			})
		})
	})
})
