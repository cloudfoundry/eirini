package util_test

import (
	"sync"
	"sync/atomic"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/pkg/errors"

	. "code.cloudfoundry.org/eirini/util"
	"code.cloudfoundry.org/lager/lagertest"
)

var _ = Describe("Scheduler", func() {

	Describe("TickerTaskScheduler", func() {
		Context("When task is Scheduled", func() {

			var (
				ticker   *time.Ticker
				duration time.Duration
				count    int32
				logger   *lagertest.TestLogger
			)

			BeforeEach(func() {
				duration = time.Duration(20) * time.Millisecond
				logger = lagertest.NewTestLogger("scheduler-test")
				ticker = time.NewTicker(duration)
			})

			It("should call the provided function on each tick", func() {
				scheduler := &TickerTaskScheduler{Ticker: ticker, Logger: logger}
				task := func() error {
					atomic.AddInt32(&count, 1)
					return nil
				}
				go scheduler.Schedule(task)
				time.Sleep(50 * time.Millisecond)
				ticker.Stop()

				Expect(atomic.LoadInt32(&count)).To(Equal(int32(2)))
			})

			Context("when the function returns an error", func() {
				It("should provide a helpful log message", func() {
					scheduler := &TickerTaskScheduler{Ticker: ticker, Logger: logger}
					task := func() error {
						return errors.New("task failure")
					}
					go scheduler.Schedule(task)
					time.Sleep(50 * time.Millisecond)
					ticker.Stop()

					Eventually(func() int {
						return len(logger.Logs())
					}).Should(BeNumerically(">", 0))
					log := logger.Logs()[0]
					Expect(log.Message).To(Equal("scheduler-test.task-failed"))
					Expect(log.Data).To(HaveKeyWithValue("error", "task failure"))
				})
			})
		})
	})

	Describe("SimpleLoopScheduler", func() {
		var (
			workChan   chan int
			cancelChan chan struct{}
			wg         sync.WaitGroup
			logger     *lagertest.TestLogger
			scheduler  SimpleLoopScheduler
			task       Task
		)

		AfterEach(func() {
			close(cancelChan)
			wg.Wait()
			close(workChan)
		})

		BeforeEach(func() {
			task = func() error {
				time.Sleep(4 * time.Millisecond)
				workChan <- 42
				return nil
			}
		})

		JustBeforeEach(func() {
			cancelChan = make(chan struct{}, 1)
			workChan = make(chan int, 100)
			logger = lagertest.NewTestLogger("scheduler-test")
			scheduler = SimpleLoopScheduler{CancelChan: cancelChan, Logger: logger}
			wg = sync.WaitGroup{}

			wg.Add(1)
			go func() {
				scheduler.Schedule(task)
				wg.Done()
			}()
		})

		It("should call the provided function until the stop chanel is closed", func() {
			Eventually(workChan).Should(Receive())
			Consistently(workChan, "150ms", "5s").Should(Receive())
		})

		Context("when the task fails", func() {
			BeforeEach(func() {
				task = func() error {
					return errors.New("failed to task")
				}
			})

			It("should log an error when the task fails", func() {
				Eventually(func() int {
					return len(logger.Logs())
				}).Should(BeNumerically(">", 0))

				log := logger.Logs()[0]
				Expect(log.Message).To(Equal("scheduler-test.task-failed"))
				Expect(log.Data).To(HaveKeyWithValue("error", "failed to task"))
			})

		})

	})
})
