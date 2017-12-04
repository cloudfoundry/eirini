package metrics_test

import (
	"os"
	"sync"
	"time"

	"code.cloudfoundry.org/bbs/metrics"
	"code.cloudfoundry.org/clock/fakeclock"
	mfakes "code.cloudfoundry.org/diego-logging-client/testhelpers"
	"code.cloudfoundry.org/lager/lagertest"
	"github.com/tedsuo/ifrit"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("PeriodicMetronCountNotifier", func() {
	var (
		fakeMetronClient *mfakes.FakeIngressClient
		counterMap       map[string]uint64
		durationMap      map[string]time.Duration
		metricsLock      sync.Mutex

		reportInterval time.Duration
		fakeClock      *fakeclock.FakeClock

		mn  *metrics.RequestStatMetronNotifier
		mnp ifrit.Process
	)

	BeforeEach(func() {
		counterMap = make(map[string]uint64)
		durationMap = make(map[string]time.Duration)
		fakeMetronClient = new(mfakes.FakeIngressClient)
		fakeMetronClient.IncrementCounterWithDeltaStub = func(name string, delta uint64) error {
			metricsLock.Lock()
			defer metricsLock.Unlock()
			counterMap[name] += delta
			return nil
		}

		fakeMetronClient.SendDurationStub = func(name string, value time.Duration) error {
			metricsLock.Lock()
			defer metricsLock.Unlock()
			durationMap[name] = value
			return nil
		}

		reportInterval = 100 * time.Millisecond

		fakeClock = fakeclock.NewFakeClock(time.Unix(123, 456))
	})

	JustBeforeEach(func() {
		ticker := fakeClock.NewTicker(reportInterval)
		mn = metrics.NewRequestStatMetronNotifier(lagertest.NewTestLogger("test"), ticker, fakeMetronClient)
		mnp = ifrit.Invoke(mn)
	})

	AfterEach(func() {
		mnp.Signal(os.Interrupt)
		Eventually(mnp.Wait(), 2*time.Second).Should(Receive())
	})

	It("should emit a request count event periodically", func() {
		mn.IncrementCounter(1)
		mn.UpdateLatency(time.Second)
		fakeClock.WaitForWatcherAndIncrement(reportInterval)

		Eventually(func() uint64 {
			metricsLock.Lock()
			defer metricsLock.Unlock()
			return counterMap["RequestCount"]
		}).Should(Equal(uint64(1)))

		Eventually(func() time.Duration {
			metricsLock.Lock()
			defer metricsLock.Unlock()
			return durationMap["RequestLatency"]
		}).Should(Equal(1 * time.Second))

		mn.IncrementCounter(1)
		mn.UpdateLatency(3 * time.Second)

		mn.IncrementCounter(1)
		mn.UpdateLatency(2 * time.Second)
		fakeClock.WaitForWatcherAndIncrement(reportInterval)

		Eventually(func() uint64 {
			metricsLock.Lock()
			defer metricsLock.Unlock()
			return counterMap["RequestCount"]
		}).Should(Equal(uint64(3)))

		Eventually(func() time.Duration {
			metricsLock.Lock()
			defer metricsLock.Unlock()
			return durationMap["RequestLatency"]
		}).Should(Equal(3 * time.Second))
	})
})
