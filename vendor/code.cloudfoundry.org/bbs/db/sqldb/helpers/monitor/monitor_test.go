package monitor_test

import (
	"database/sql"
	"errors"
	"sync"
	"time"

	"code.cloudfoundry.org/bbs/db/sqldb/helpers/monitor"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Monitor", func() {
	var mon monitor.Monitor

	BeforeEach(func() {
		mon = monitor.New()
	})

	Describe("#Monitor", func() {
		Context("when no errors occur", func() {
			It("increases total and succeeded counts by 1 and returns nil", func() {
				err := mon.Monitor(func() error {
					return nil
				})
				Expect(err).ToNot(HaveOccurred())

				Expect(mon.Total()).To(BeEquivalentTo(1))
				Expect(mon.Succeeded()).To(BeEquivalentTo(1))
				Expect(mon.Failed()).To(BeEquivalentTo(0))
			})
		})

		Context("when a generic error occurs", func() {
			It("increments the total and failed counts by 1 and returns the error", func() {
				err := mon.Monitor(func() error {
					return errors.New("boom!")
				})
				Expect(err).To(MatchError("boom!"))

				Expect(mon.Total()).To(BeEquivalentTo(1))
				Expect(mon.Succeeded()).To(BeEquivalentTo(0))
				Expect(mon.Failed()).To(BeEquivalentTo(1))
			})
		})

		Context("when a sql ErrNoRows occurs", func() {
			It("increments the total and succeeded counts by 1 and returns the error", func() {
				err := mon.Monitor(func() error {
					return sql.ErrNoRows
				})
				Expect(err).To(MatchError(sql.ErrNoRows))

				Expect(mon.Total()).To(BeEquivalentTo(1))
				Expect(mon.Succeeded()).To(BeEquivalentTo(1))
				Expect(mon.Failed()).To(BeEquivalentTo(0))
			})
		})

		Context("when a sql ErrTxDone occurs", func() {
			It("does not increment any counts and returns the error", func() {
				err := mon.Monitor(func() error {
					return sql.ErrTxDone
				})
				Expect(err).To(MatchError(sql.ErrTxDone))

				Expect(mon.Total()).To(BeEquivalentTo(0))
				Expect(mon.Succeeded()).To(BeEquivalentTo(0))
				Expect(mon.Failed()).To(BeEquivalentTo(0))
			})
		})

		It("records the max number of queries in flight", func() {
			cleanupCh := make(chan struct{})
			defer close(cleanupCh)

			wg := new(sync.WaitGroup)
			wg.Add(3)

			go mon.Monitor(func() error {
				wg.Done()
				<-cleanupCh
				return nil
			})

			go mon.Monitor(func() error {
				wg.Done()
				<-cleanupCh
				return nil
			})

			go mon.Monitor(func() error {
				wg.Done()
				<-cleanupCh
				return nil
			})

			wg.Wait()
			Expect(mon.ReadAndResetInFlightMax()).To(Equal(int64(3)))
		})

		It("records the max duration of queries", func() {
			mon.Monitor(func() error {
				time.Sleep(10 * time.Millisecond)
				return nil
			})

			mon.Monitor(func() error {
				time.Sleep(100 * time.Millisecond)
				return nil
			})

			mon.Monitor(func() error {
				time.Sleep(1 * time.Millisecond)
				return nil
			})

			Expect(mon.ReadAndResetDurationMax()).To(BeNumerically(">", 50*time.Millisecond))
		})

		It("doesn't cause any race conditions", func() {
			blockCh := make(chan struct{})

			updateFunc := func() error {
				<-blockCh
				return nil
			}

			go func() {
				mon.Monitor(updateFunc)
			}()
			// increase the chance of race condition happening
			go func() {
				mon.Monitor(updateFunc)
			}()

			Consistently(mon.ReadAndResetDurationMax).Should(BeNumerically("==", 0))
			close(blockCh)
			Eventually(mon.ReadAndResetDurationMax).Should(BeNumerically(">", 0))
		})
	})

	Describe("#Total", func() {
		It("returns the total number of queries ran", func() {
			mon.Monitor(func() error {
				return nil
			})
			mon.Monitor(func() error {
				return errors.New("foo")
			})
			mon.Monitor(func() error {
				return nil
			})
			Expect(mon.Total()).To(BeEquivalentTo(3))
		})
	})

	Describe("#Succeeded", func() {
		It("returns the number of queries succeeded", func() {
			mon.Monitor(func() error {
				return nil
			})
			mon.Monitor(func() error {
				return sql.ErrNoRows
			})
			mon.Monitor(func() error {
				return nil
			})
			Expect(mon.Total()).To(BeEquivalentTo(3))
		})
	})

	Describe("#Failed", func() {
		It("returns the number of queries failed", func() {
			mon.Monitor(func() error {
				return errors.New("foo")
			})
			mon.Monitor(func() error {
				return errors.New("bar")
			})
			mon.Monitor(func() error {
				return errors.New("baz")
			})
			Expect(mon.Total()).To(BeEquivalentTo(3))
		})
	})

	Describe("#ReadAndResetInFlightMax", func() {
		It("resets the max number of queries in flight to the current number of queries in flight", func() {
			blockCh1 := make(chan struct{})
			finishedCh1 := make(chan struct{})
			blockCh2 := make(chan struct{})
			finishedCh2 := make(chan struct{})

			wg := new(sync.WaitGroup)
			wg.Add(2)

			go func() {
				mon.Monitor(func() error {
					wg.Done()
					<-blockCh1
					return nil
				})
				close(finishedCh1)
			}()
			go func() {
				mon.Monitor(func() error {
					wg.Done()
					<-blockCh2
					return nil
				})
				close(finishedCh2)
			}()

			wg.Wait()

			Consistently(mon.ReadAndResetInFlightMax).Should(BeEquivalentTo(2))
			close(blockCh1)
			<-finishedCh1
			Expect(mon.ReadAndResetInFlightMax()).To(BeEquivalentTo(2))
			Consistently(mon.ReadAndResetInFlightMax).Should(BeEquivalentTo(1))
			close(blockCh2)
			<-finishedCh2
			Expect(mon.ReadAndResetInFlightMax()).To(BeEquivalentTo(1))
			Consistently(mon.ReadAndResetInFlightMax).Should(BeEquivalentTo(0))
		})
	})

	Describe("#ReadAndResetDurationMax", func() {
		It("resets the max duration of all queries ran since last reset to 0", func() {
			mon.Monitor(func() error {
				time.Sleep(10 * time.Millisecond)
				return nil
			})
			mon.Monitor(func() error {
				time.Sleep(100 * time.Millisecond)
				return nil
			})

			Expect(mon.ReadAndResetDurationMax()).To(BeNumerically(">", 50*time.Millisecond))
			Expect(mon.ReadAndResetDurationMax()).To(BeZero())
		})
	})
})
