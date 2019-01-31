package metrics_test

import (
	"errors"
	"fmt"
	"time"

	"code.cloudfoundry.org/bbs/metrics"
	"code.cloudfoundry.org/clock/fakeclock"
	"code.cloudfoundry.org/diego-logging-client/testhelpers"
	"code.cloudfoundry.org/go-loggregator"
	"code.cloudfoundry.org/lager/lagertest"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/tedsuo/ifrit"
	"github.com/tedsuo/ifrit/ginkgomon"
)

var _ = Describe("LRPStatMetronNotifier", func() {
	type metric struct {
		Name  string
		Value int
	}

	type counter struct {
		Name  string
		Value uint64
	}

	var (
		metricsCh    chan metric
		counterCh    chan counter
		fakeClock    *fakeclock.FakeClock
		metronClient *testhelpers.FakeIngressClient

		fakeLogger *lagertest.TestLogger

		notifier metrics.LRPStatMetronNotifier

		process ifrit.Process
	)

	BeforeEach(func() {
		metricsCh = make(chan metric, 100)
		counterCh = make(chan counter, 100)

		fakeClock = fakeclock.NewFakeClock(time.Now())
		metronClient = new(testhelpers.FakeIngressClient)
		metronClient.SendMetricStub = func(name string, value int, opts ...loggregator.EmitGaugeOption) error {
			metricsCh <- metric{name, value}
			return nil
		}
		metronClient.IncrementCounterWithDeltaStub = func(name string, value uint64) error {
			counterCh <- counter{name, value}
			return nil
		}

		fakeLogger = lagertest.NewTestLogger("lrp-stat-metron-notifier-test")
		notifier = metrics.NewLRPStatMetronNotifier(fakeLogger, fakeClock, metronClient)
		Expect(notifier).NotTo(BeNil())

		process = ginkgomon.Invoke(notifier)
	})

	AfterEach(func() {
		ginkgomon.Kill(process)
	})

	Describe("convergence runs metrics", func() {
		Context("when metrics are emitted for the first time and convergence has happened", func() {
			BeforeEach(func() {
				notifier.RecordConvergenceDuration(time.Second)
				notifier.RecordConvergenceDuration(3 * time.Second)
			})

			JustBeforeEach(func() {
				fakeClock.WaitForWatcherAndIncrement(metrics.DefaultEmitMetricsFrequency)
			})

			It("emits the number of convergence runs", func() {
				Eventually(counterCh).Should(Receive(Equal(counter{
					Name:  "ConvergenceLRPRuns",
					Value: 2,
				})))
			})

			It("emits the duration of the most recent convergence run", func() {
				Eventually(metronClient.SendDurationCallCount).Should(Equal(1))

				metricName, duration, _ := metronClient.SendDurationArgsForCall(0)
				Expect(metricName).To(Equal("ConvergenceLRPDuration"))
				Expect(duration).To(BeEquivalentTo(3 * time.Second))
			})

			Context("when metrics are emitted a second time and convergence has happened again", func() {
				JustBeforeEach(func() {
					// wait for previous set of metrics to be emitted then emit the next set
					Eventually(counterCh).Should(Receive())
					notifier.RecordConvergenceDuration(5 * time.Second)
					fakeClock.WaitForWatcherAndIncrement(metrics.DefaultEmitMetricsFrequency)
				})

				It("emits the number of runs between the first and second metric emissions", func() {
					Eventually(counterCh).Should(Receive(Equal(counter{
						Name:  "ConvergenceLRPRuns",
						Value: 1,
					})))
				})

				It("emits the duration of the most recent convergence run", func() {
					Eventually(metronClient.SendDurationCallCount).Should(Equal(2))

					metricName, duration, _ := metronClient.SendDurationArgsForCall(1)
					Expect(metricName).To(Equal("ConvergenceLRPDuration"))
					Expect(duration).To(BeEquivalentTo(5 * time.Second))
				})
			})

			Context("when metrics are emitted a second time and convergence has NOT happened again", func() {
				JustBeforeEach(func() {
					Eventually(counterCh).Should(Receive())
					fakeClock.WaitForWatcherAndIncrement(metrics.DefaultEmitMetricsFrequency)
				})

				It("does not emit convergence runs", func() {
					Consistently(counterCh).ShouldNot(Receive())
				})

				It("emits the cached duration of the last convergence run", func() {
					Eventually(metronClient.SendDurationCallCount).Should(Equal(2))

					metricName, duration, _ := metronClient.SendDurationArgsForCall(1)
					Expect(metricName).To(Equal("ConvergenceLRPDuration"))
					Expect(duration).To(BeEquivalentTo(3 * time.Second))
				})
			})

			Context("when there is an error sending number of convergence runs", func() {
				BeforeEach(func() {
					metronClient.IncrementCounterWithDeltaStub = func(name string, value uint64) error {
						return errors.New("boom")
					}
				})

				It("logs the error", func() {
					Eventually(metronClient.IncrementCounterWithDeltaCallCount).Should(Equal(1))
					Eventually(fakeLogger).Should(gbytes.Say("failed-sending-metric"))
					Eventually(fakeLogger).Should(gbytes.Say("boom"))
					Eventually(fakeLogger).Should(gbytes.Say("ConvergenceLRPRuns"))
				})

				It("continues trying to send the other metrics", func() {
					Eventually(metronClient.SendDurationCallCount).ShouldNot(BeZero())
				})
			})

			Context("when there is an error sending convergence duration", func() {
				BeforeEach(func() {
					metronClient.SendDurationReturns(errors.New("boom"))
				})

				It("logs the error", func() {
					Eventually(metronClient.SendDurationCallCount).Should(Equal(1))
					Eventually(fakeLogger).Should(gbytes.Say("failed-sending-metric"))
					Eventually(fakeLogger).Should(gbytes.Say("boom"))
					Eventually(fakeLogger).Should(gbytes.Say("ConvergenceLRPDuration"))
				})

				It("continues trying to send the other metrics", func() {
					Eventually(metronClient.SendMetricCallCount).ShouldNot(BeZero())
				})
			})
		})
	})

	Describe("all other metrics", func() {
		BeforeEach(func() {
			notifier.RecordFreshDomains([]string{"domain-1", "domain-2"})
			notifier.RecordLRPCounts(1, 2, 3, 4, 5, 6, 7, 8, 9, 10)
			notifier.RecordCellCounts(42, 3)
		})

		JustBeforeEach(func() {
			fakeClock.WaitForWatcherAndIncrement(metrics.DefaultEmitMetricsFrequency)
		})

		Context("when send metric returns an error", func() {
			allOtherMetrics := []string{
				"Domain.domain-1",
				"Domain.domain-2",
				metrics.LRPsUnclaimedMetric,
				metrics.LRPsClaimedMetric,
				metrics.LRPsRunningMetric,
				metrics.CrashedActualLRPsMetric,
				metrics.LRPsMissingMetric,
				metrics.LRPsExtraMetric,
				metrics.SuspectRunningLRPsMetric,
				metrics.SuspectClaimedLRPsMetric,
				metrics.LRPsDesiredMetric,
				metrics.CrashingDesiredLRPsMetric,
				metrics.PresentCellsMetric,
				metrics.SuspectCellsMetric,
			}

			for _, mn := range allOtherMetrics {
				Context(fmt.Sprintf("for %s", mn), func() {
					metricName := mn
					BeforeEach(func() {
						origSendMetricStub := metronClient.SendMetricStub

						metronClient.SendMetricStub = func(name string, value int, opts ...loggregator.EmitGaugeOption) error {
							if name == metricName {
								return errors.New("boom")
							}
							return origSendMetricStub(name, value, opts...)
						}
					})

					It("logs the error", func() {
						Eventually(fakeLogger).Should(gbytes.Say("failed-sending-metric"))
						Eventually(fakeLogger).Should(gbytes.Say("boom"))
						Eventually(fakeLogger).Should(gbytes.Say(metricName))
					})

					It("continues trying to send the other metrics", func() {
						Eventually(metronClient.SendMetricCallCount).Should(Equal(len(allOtherMetrics)))
					})
				})
			}
		})

		Context("when metrics are emitted for the first time and convergence has happened", func() {
			It("emits metrics", func() {
				Eventually(metricsCh).Should(Receive(Equal(metric{
					Name:  "Domain.domain-1",
					Value: 1,
				})))
				Eventually(metricsCh).Should(Receive(Equal(metric{
					Name:  "Domain.domain-2",
					Value: 1,
				})))

				Eventually(metricsCh).Should(Receive(Equal(metric{
					Name:  "LRPsUnclaimed",
					Value: 1,
				})))
				Eventually(metricsCh).Should(Receive(Equal(metric{
					Name:  "LRPsClaimed",
					Value: 2,
				})))
				Eventually(metricsCh).Should(Receive(Equal(metric{
					Name:  "LRPsRunning",
					Value: 3,
				})))
				Eventually(metricsCh).Should(Receive(Equal(metric{
					Name:  "CrashedActualLRPs",
					Value: 4,
				})))
				Eventually(metricsCh).Should(Receive(Equal(metric{
					Name:  "LRPsMissing",
					Value: 5,
				})))
				Eventually(metricsCh).Should(Receive(Equal(metric{
					Name:  "LRPsExtra",
					Value: 6,
				})))
				Eventually(metricsCh).Should(Receive(Equal(metric{
					Name:  "SuspectRunningActualLRPs",
					Value: 7,
				})))
				Eventually(metricsCh).Should(Receive(Equal(metric{
					Name:  "SuspectClaimedActualLRPs",
					Value: 8,
				})))
				Eventually(metricsCh).Should(Receive(Equal(metric{
					Name:  "LRPsDesired",
					Value: 9,
				})))
				Eventually(metricsCh).Should(Receive(Equal(metric{
					Name:  "CrashingDesiredLRPs",
					Value: 10,
				})))

				Eventually(metricsCh).Should(Receive(Equal(metric{
					Name:  "PresentCells",
					Value: 42,
				})))
				Eventually(metricsCh).Should(Receive(Equal(metric{
					Name:  "SuspectCells",
					Value: 3,
				})))
			})
		})

		Context("when metrics are emitted a second time and convergence has happened again", func() {
			BeforeEach(func() {
				notifier.RecordFreshDomains([]string{"domain-11", "domain-12"})
				notifier.RecordLRPCounts(11, 12, 13, 14, 15, 16, 17, 18, 19, 20)
				notifier.RecordCellCounts(40, 5)
			})

			JustBeforeEach(func() {
				fakeClock.WaitForWatcherAndIncrement(metrics.DefaultEmitMetricsFrequency)
			})

			It("emits the most recent values of these metrics", func() {
				Eventually(metricsCh).Should(Receive(Equal(metric{
					Name:  "Domain.domain-11",
					Value: 1,
				})))
				Eventually(metricsCh).Should(Receive(Equal(metric{
					Name:  "Domain.domain-12",
					Value: 1,
				})))

				Eventually(metricsCh).Should(Receive(Equal(metric{
					Name:  "LRPsUnclaimed",
					Value: 11,
				})))
				Eventually(metricsCh).Should(Receive(Equal(metric{
					Name:  "LRPsClaimed",
					Value: 12,
				})))
				Eventually(metricsCh).Should(Receive(Equal(metric{
					Name:  "LRPsRunning",
					Value: 13,
				})))
				Eventually(metricsCh).Should(Receive(Equal(metric{
					Name:  "CrashedActualLRPs",
					Value: 14,
				})))
				Eventually(metricsCh).Should(Receive(Equal(metric{
					Name:  "LRPsMissing",
					Value: 15,
				})))
				Eventually(metricsCh).Should(Receive(Equal(metric{
					Name:  "LRPsExtra",
					Value: 16,
				})))
				Eventually(metricsCh).Should(Receive(Equal(metric{
					Name:  "SuspectRunningActualLRPs",
					Value: 17,
				})))
				Eventually(metricsCh).Should(Receive(Equal(metric{
					Name:  "SuspectClaimedActualLRPs",
					Value: 18,
				})))
				Eventually(metricsCh).Should(Receive(Equal(metric{
					Name:  "LRPsDesired",
					Value: 19,
				})))
				Eventually(metricsCh).Should(Receive(Equal(metric{
					Name:  "CrashingDesiredLRPs",
					Value: 20,
				})))

				Eventually(metricsCh).Should(Receive(Equal(metric{
					Name:  "PresentCells",
					Value: 40,
				})))
				Eventually(metricsCh).Should(Receive(Equal(metric{
					Name:  "SuspectCells",
					Value: 5,
				})))
			})
		})

		Context("when metrics are emitted a second time and convergence has NOT happened", func() {
			JustBeforeEach(func() {
				fakeClock.WaitForWatcherAndIncrement(metrics.DefaultEmitMetricsFrequency)
			})

			It("emits the cached values of these metrics", func() {
				Eventually(metricsCh).Should(Receive(Equal(metric{
					Name:  "Domain.domain-1",
					Value: 1,
				})))
				Eventually(metricsCh).Should(Receive(Equal(metric{
					Name:  "Domain.domain-2",
					Value: 1,
				})))

				Eventually(metricsCh).Should(Receive(Equal(metric{
					Name:  "LRPsUnclaimed",
					Value: 1,
				})))
				Eventually(metricsCh).Should(Receive(Equal(metric{
					Name:  "LRPsClaimed",
					Value: 2,
				})))
				Eventually(metricsCh).Should(Receive(Equal(metric{
					Name:  "LRPsRunning",
					Value: 3,
				})))
				Eventually(metricsCh).Should(Receive(Equal(metric{
					Name:  "CrashedActualLRPs",
					Value: 4,
				})))
				Eventually(metricsCh).Should(Receive(Equal(metric{
					Name:  "LRPsMissing",
					Value: 5,
				})))
				Eventually(metricsCh).Should(Receive(Equal(metric{
					Name:  "LRPsExtra",
					Value: 6,
				})))
				Eventually(metricsCh).Should(Receive(Equal(metric{
					Name:  "SuspectRunningActualLRPs",
					Value: 7,
				})))
				Eventually(metricsCh).Should(Receive(Equal(metric{
					Name:  "SuspectClaimedActualLRPs",
					Value: 8,
				})))
				Eventually(metricsCh).Should(Receive(Equal(metric{
					Name:  "LRPsDesired",
					Value: 9,
				})))
				Eventually(metricsCh).Should(Receive(Equal(metric{
					Name:  "CrashingDesiredLRPs",
					Value: 10,
				})))

				Eventually(metricsCh).Should(Receive(Equal(metric{
					Name:  "PresentCells",
					Value: 42,
				})))
				Eventually(metricsCh).Should(Receive(Equal(metric{
					Name:  "SuspectCells",
					Value: 3,
				})))
			})
		})
	})
})
