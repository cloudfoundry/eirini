package metrics_test

import (
	"errors"
	"fmt"
	"time"

	"code.cloudfoundry.org/bbs/metrics"
	"code.cloudfoundry.org/clock/fakeclock"
	"code.cloudfoundry.org/diego-logging-client/testhelpers"
	"code.cloudfoundry.org/go-loggregator"
	"code.cloudfoundry.org/go-loggregator/rpc/loggregator_v2"
	"code.cloudfoundry.org/lager/lagertest"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gstruct"
	"github.com/onsi/gomega/types"
	"github.com/tedsuo/ifrit"
	"github.com/tedsuo/ifrit/ginkgomon"
)

var _ = Describe("TaskStatMetronNotifier", func() {
	type metric struct {
		Name  string
		Value int
		Opts  []loggregator.EmitGaugeOption
	}

	type counter struct {
		Name  string
		Value uint64
	}

	var (
		taskStatMetronNotifier metrics.TaskStatMetronNotifier
		fakeClock              *fakeclock.FakeClock
		metronClient           *testhelpers.FakeIngressClient
		metricsCh              chan metric
		counterCh              chan counter
		process                ifrit.Process
		fakeLogger             *lagertest.TestLogger
	)

	BeforeEach(func() {
		metricsCh = make(chan metric, 100)
		counterCh = make(chan counter, 100)

		metronClient = &testhelpers.FakeIngressClient{}
		metronClient.SendMetricStub = func(name string, value int, opts ...loggregator.EmitGaugeOption) error {
			metricsCh <- metric{name, value, opts}
			return nil
		}
		metronClient.IncrementCounterWithDeltaStub = func(name string, value uint64) error {
			counterCh <- counter{name, value}
			return nil
		}

		fakeClock = fakeclock.NewFakeClock(time.Now())
		fakeLogger = lagertest.NewTestLogger("task-state-metron-notifier-test")
		taskStatMetronNotifier = metrics.NewTaskStatMetronNotifier(fakeLogger, fakeClock, metronClient)
		Expect(taskStatMetronNotifier).NotTo(BeNil())

		process = ginkgomon.Invoke(taskStatMetronNotifier)
	})

	AfterEach(func() {
		ginkgomon.Kill(process)
	})

	Context("when a task is started and convergence has happened", func() {
		BeforeEach(func() {
			taskStatMetronNotifier.RecordTaskStarted("cell-1")
			fakeClock.Increment(metrics.DefaultTaskEmitMetricsFrequency)
		})

		It("emits the metric", func() {
			Eventually(metricsCh).Should(Receive(gstruct.MatchAllFields(gstruct.Fields{
				"Name":  Equal("TasksStarted"),
				"Value": Equal(1),
				"Opts":  haveTag("cell-id", "cell-1"),
			})))
		})

		Context("when metrics were emitted a second time and convergence has NOT happened again", func() {
			BeforeEach(func() {
				Eventually(metricsCh).Should(Receive())
				fakeClock.Increment(metrics.DefaultTaskEmitMetricsFrequency)
			})

			It("emits the same metric again", func() {
				Eventually(metricsCh).Should(Receive(gstruct.MatchAllFields(gstruct.Fields{
					"Name":  Equal("TasksStarted"),
					"Value": Equal(1),
					"Opts":  haveTag("cell-id", "cell-1"),
				})))
			})
		})
	})

	Context("when a task succeeds and convergence has happened", func() {
		BeforeEach(func() {
			taskStatMetronNotifier.RecordTaskSucceeded("cell-1")
			fakeClock.Increment(metrics.DefaultTaskEmitMetricsFrequency)
		})

		It("emits the metric with the proper tag", func() {
			Eventually(metricsCh).Should(Receive(gstruct.MatchAllFields(gstruct.Fields{
				"Name":  Equal("TasksSucceeded"),
				"Value": Equal(1),
				"Opts":  haveTag("cell-id", "cell-1"),
			})))
		})
	})

	Context("when a task fails and convergence has happened", func() {
		BeforeEach(func() {
			taskStatMetronNotifier.RecordTaskFailed("cell-1")
			taskStatMetronNotifier.RecordTaskFailed("cell-1")
			fakeClock.Increment(metrics.DefaultTaskEmitMetricsFrequency)
		})

		It("emits the metric with the proper tag", func() {
			Eventually(metricsCh).Should(Receive(gstruct.MatchAllFields(gstruct.Fields{
				"Name":  Equal("TasksFailed"),
				"Value": Equal(2),
				"Opts":  haveTag("cell-id", "cell-1"),
			})))
		})
	})

	Context("when tasks on multiple cells are started and convergence has happened", func() {
		BeforeEach(func() {
			taskStatMetronNotifier.RecordTaskFailed("cell-1")
			taskStatMetronNotifier.RecordTaskFailed("cell-2")
			fakeClock.Increment(metrics.DefaultTaskEmitMetricsFrequency)
		})

		It("emits the metric for the second cell with the proper tag", func() {
			Eventually(metricsCh).Should(Receive(gstruct.MatchAllFields(gstruct.Fields{
				"Name":  Equal("TasksFailed"),
				"Value": Equal(1),
				"Opts":  haveTag("cell-id", "cell-2"),
			})))
		})

		It("emits the metric for the first cell with the proper tag", func() {
			Eventually(metricsCh).Should(Receive(gstruct.MatchAllFields(gstruct.Fields{
				"Name":  Equal("TasksFailed"),
				"Value": Equal(1),
				"Opts":  haveTag("cell-id", "cell-1"),
			})))
		})
	})

	Describe("convergence runs metrics", func() {
		Context("when metrics are emitted and convergence has happened", func() {
			BeforeEach(func() {
				taskStatMetronNotifier.RecordConvergenceDuration(10 * time.Second)
				taskStatMetronNotifier.RecordConvergenceDuration(5 * time.Second)
				taskStatMetronNotifier.RecordConvergenceDuration(1 * time.Second)
			})

			JustBeforeEach(func() {
				fakeClock.Increment(metrics.DefaultTaskEmitMetricsFrequency)
			})

			It("emits the number of convergence runs since the last time metrics were emitted", func() {
				Eventually(counterCh).Should(Receive(Equal(counter{
					Name:  "ConvergenceTaskRuns",
					Value: 3,
				})))
			})

			It("emits the duration of the last convergence run", func() {
				Eventually(metronClient.SendDurationCallCount).Should(Equal(1))

				metricName, duration, _ := metronClient.SendDurationArgsForCall(0)
				Expect(metricName).To(Equal("ConvergenceTaskDuration"))
				Expect(duration).To(BeEquivalentTo(1 * time.Second))
			})

			Context("when sending convergence duration errors", func() {
				BeforeEach(func() {
					metronClient.SendDurationStub = func(name string, value time.Duration, opts ...loggregator.EmitGaugeOption) error {
						return errors.New("boom")
					}
				})

				It("logs the error", func() {
					Eventually(fakeLogger).Should(gbytes.Say("failed-sending-metric"))
					Eventually(fakeLogger).Should(gbytes.Say("boom"))
					Eventually(fakeLogger).Should(gbytes.Say("ConvergenceTaskDuration"))
				})
			})

			Context("when metrics are emitted a second time and convergence has happened again", func() {
				JustBeforeEach(func() {
					// wait for previous set of metrics to be emitted then emit the next set
					Eventually(counterCh).Should(Receive())

					taskStatMetronNotifier.RecordConvergenceDuration(2 * time.Second)

					fakeClock.Increment(metrics.DefaultTaskEmitMetricsFrequency)
				})

				It("resets the value and emits the number of runs since the last time metrics were emitted", func() {
					Eventually(counterCh).Should(Receive(Equal(counter{
						Name:  "ConvergenceTaskRuns",
						Value: 1,
					})))
				})

				It("emits the duration of the new convergence run", func() {
					Eventually(metronClient.SendDurationCallCount).Should(Equal(2))

					metricName, duration, _ := metronClient.SendDurationArgsForCall(1)
					Expect(metricName).To(Equal("ConvergenceTaskDuration"))
					Expect(duration).To(BeEquivalentTo(2 * time.Second))
				})
			})

			Context("when metrics are emitted a second time but convergence has NOT happened again", func() {
				JustBeforeEach(func() {
					Eventually(counterCh).Should(Receive())
					fakeClock.Increment(metrics.DefaultTaskEmitMetricsFrequency)
				})

				It("doesn't update the converge runs counter", func() {
					Consistently(counterCh).ShouldNot(Receive(gstruct.MatchFields(gstruct.IgnoreExtras, gstruct.Fields{
						"Name": Equal("TasksFailed"),
					})))
				})

				It("emits the duration of the last convergence run", func() {
					Eventually(metronClient.SendDurationCallCount).Should(Equal(2))

					metricName, duration, _ := metronClient.SendDurationArgsForCall(1)
					Expect(metricName).To(Equal("ConvergenceTaskDuration"))
					Expect(duration).To(BeEquivalentTo(1 * time.Second))
				})
			})

			Context("when there is an error sending number of convergence runs", func() {
				BeforeEach(func() {
					metronClient.IncrementCounterWithDeltaStub = func(name string, value uint64) error {
						return errors.New("boom")
					}
				})

				It("logs the error", func() {
					Eventually(fakeLogger).Should(gbytes.Say("failed-sending-metric"))
					Eventually(fakeLogger).Should(gbytes.Say("boom"))
					Eventually(fakeLogger).Should(gbytes.Say("ConvergenceTaskRuns"))
				})

				It("continues trying to send the other metrics", func() {
					Eventually(metronClient.SendDurationCallCount).ShouldNot(BeZero())
				})
			})
		})
	})

	Describe("task count metrics", func() {
		Context("when metrics are emitted and convergence has happened", func() {
			BeforeEach(func() {
				taskStatMetronNotifier.RecordTaskStarted("cell-1")
				taskStatMetronNotifier.RecordTaskFailed("cell-1")
				taskStatMetronNotifier.RecordTaskSucceeded("cell-1")
				taskStatMetronNotifier.RecordTaskCounts(1, 2, 3, 4, 5, 6)
			})

			JustBeforeEach(func() {
				fakeClock.WaitForWatcherAndIncrement(metrics.DefaultEmitMetricsFrequency)
			})

			Context("when send metric returns an error", func() {
				allOtherMetrics := []string{
					"TasksStarted",
					"TasksSucceeded",
					"TasksFailed",
					"TasksPending",
					"TasksRunning",
					"TasksCompleted",
					"TasksResolving",
				}

				otherDurationMetrics := []string{
					"ConvergenceTasksPruned",
					"ConvergenceTasksKicked",
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

				for _, mn := range otherDurationMetrics {
					Context(fmt.Sprintf("for %s", mn), func() {
						metricName := mn
						BeforeEach(func() {
							origSendMetricStub := metronClient.IncrementCounterWithDeltaStub

							metronClient.IncrementCounterWithDeltaStub = func(name string, value uint64) error {
								if name == metricName {
									return errors.New("boom")
								}
								return origSendMetricStub(name, value)
							}
						})

						It("logs the error", func() {
							Eventually(fakeLogger).Should(gbytes.Say("failed-sending-metric"))
							Eventually(fakeLogger).Should(gbytes.Say("boom"))
							Eventually(fakeLogger).Should(gbytes.Say(metricName))
						})

						It("continues trying to send the other metrics", func() {
							Eventually(metronClient.IncrementCounterWithDeltaCallCount).Should(Equal(len(otherDurationMetrics)))
						})
					})
				}
			})

			It("emits the number of pending, running, completed, resolving, pruned, and kicked tasks", func() {
				Eventually(metricsCh).Should(Receive(Equal(metric{
					Name:  "TasksPending",
					Value: 1,
				})))

				Eventually(metricsCh).Should(Receive(Equal(metric{
					Name:  "TasksRunning",
					Value: 2,
				})))

				Eventually(metricsCh).Should(Receive(Equal(metric{
					Name:  "TasksCompleted",
					Value: 3,
				})))

				Eventually(metricsCh).Should(Receive(Equal(metric{
					Name:  "TasksResolving",
					Value: 4,
				})))

				Eventually(counterCh).Should(Receive(Equal(counter{
					Name:  "ConvergenceTasksPruned",
					Value: uint64(5),
				})))

				Eventually(counterCh).Should(Receive(Equal(counter{
					Name:  "ConvergenceTasksKicked",
					Value: uint64(6),
				})))
			})

			Context("when metrics have been emitted a second time and convergence has happened again", func() {
				BeforeEach(func() {
					taskStatMetronNotifier.RecordTaskCounts(5, 6, 7, 8, 9, 10)
					fakeClock.Increment(metrics.DefaultTaskEmitMetricsFrequency)
				})

				It("emits the new value for the metric", func() {
					Eventually(metricsCh).Should(Receive(Equal(metric{
						Name:  "TasksPending",
						Value: 5,
					})))

					Eventually(metricsCh).Should(Receive(Equal(metric{
						Name:  "TasksRunning",
						Value: 6,
					})))

					Eventually(metricsCh).Should(Receive(Equal(metric{
						Name:  "TasksCompleted",
						Value: 7,
					})))

					Eventually(metricsCh).Should(Receive(Equal(metric{
						Name:  "TasksResolving",
						Value: 8,
					})))

					Eventually(counterCh).Should(Receive(Equal(counter{
						Name:  "ConvergenceTasksPruned",
						Value: uint64(9),
					})))

					Eventually(counterCh).Should(Receive(Equal(counter{
						Name:  "ConvergenceTasksKicked",
						Value: uint64(10),
					})))
				})
			})

			Context("when metrics are emitted a second time but convergence has NOT happened again", func() {
				BeforeEach(func() {
					fakeClock.Increment(metrics.DefaultTaskEmitMetricsFrequency)
				})

				It("emits the last cached value of the metric", func() {
					Eventually(metricsCh).Should(Receive(Equal(metric{
						Name:  "TasksPending",
						Value: 1,
					})))

					Eventually(metricsCh).Should(Receive(Equal(metric{
						Name:  "TasksRunning",
						Value: 2,
					})))

					Eventually(metricsCh).Should(Receive(Equal(metric{
						Name:  "TasksCompleted",
						Value: 3,
					})))

					Eventually(metricsCh).Should(Receive(Equal(metric{
						Name:  "TasksResolving",
						Value: 4,
					})))

					Eventually(counterCh).Should(Receive(Equal(counter{
						Name:  "ConvergenceTasksPruned",
						Value: uint64(5),
					})))

					Eventually(counterCh).Should(Receive(Equal(counter{
						Name:  "ConvergenceTasksKicked",
						Value: uint64(6),
					})))
				})
			})
		})
	})
})

func haveTag(name, value string) types.GomegaMatcher {
	return WithTransform(func(opts []loggregator.EmitGaugeOption) map[string]string {
		envelope := &loggregator_v2.Envelope{
			Tags: make(map[string]string),
		}
		for _, opt := range opts {
			opt(envelope)
		}
		return envelope.Tags
	}, Equal(map[string]string{name: value}))
}
