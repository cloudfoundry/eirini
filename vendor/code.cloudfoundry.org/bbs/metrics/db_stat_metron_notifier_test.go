package metrics_test

import (
	"time"

	"code.cloudfoundry.org/bbs/db/sqldb/helpers/monitor/monitorfakes"
	"code.cloudfoundry.org/bbs/metrics"
	"code.cloudfoundry.org/bbs/metrics/metricsfakes"
	"code.cloudfoundry.org/clock/fakeclock"
	mfakes "code.cloudfoundry.org/diego-logging-client/testhelpers"
	loggregator "code.cloudfoundry.org/go-loggregator"
	"code.cloudfoundry.org/lager/lagertest"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/tedsuo/ifrit"
	"github.com/tedsuo/ifrit/ginkgomon"
)

type FakeGauge struct {
	Name  string
	Value int
}

var _ = Describe("dbStatMetronNotifier", func() {
	var (
		logger           *lagertest.TestLogger
		fakeClock        *fakeclock.FakeClock
		fakeDBStats      *metricsfakes.FakeDBStats
		fakeMetronClient *mfakes.FakeIngressClient
		fakeMonitor      *monitorfakes.FakeMonitor

		metricsChan chan FakeGauge

		runner  ifrit.Runner
		process ifrit.Process
	)

	BeforeEach(func() {
		metricsChan = make(chan FakeGauge, 100)

		logger = lagertest.NewTestLogger("metrics")
		fakeClock = fakeclock.NewFakeClock(time.Now())

		fakeMetronClient = new(mfakes.FakeIngressClient)
		fakeMetronClient.SendMetricStub = func(name string, value int, opts ...loggregator.EmitGaugeOption) error {
			defer GinkgoRecover()
			Eventually(metricsChan).Should(BeSent(FakeGauge{name, value}))
			return nil
		}
		fakeMetronClient.SendDurationStub = func(name string, value time.Duration, opts ...loggregator.EmitGaugeOption) error {
			defer GinkgoRecover()
			Eventually(metricsChan).Should(BeSent(FakeGauge{name, int(value)}))
			return nil
		}

		fakeDBStats = new(metricsfakes.FakeDBStats)
		fakeDBStats.OpenConnectionsReturnsOnCall(0, 10)
		fakeDBStats.OpenConnectionsReturnsOnCall(1, 100)

		fakeMonitor = new(monitorfakes.FakeMonitor)
		fakeMonitor.TotalReturnsOnCall(0, 20)
		fakeMonitor.TotalReturnsOnCall(1, 200)
		fakeMonitor.SucceededReturnsOnCall(0, 15)
		fakeMonitor.SucceededReturnsOnCall(1, 150)
		fakeMonitor.FailedReturnsOnCall(0, 5)
		fakeMonitor.FailedReturnsOnCall(1, 50)
		fakeMonitor.ReadAndResetInFlightMaxReturnsOnCall(0, 8)
		fakeMonitor.ReadAndResetInFlightMaxReturnsOnCall(1, 80)
		fakeMonitor.ReadAndResetDurationMaxReturnsOnCall(0, time.Second)
		fakeMonitor.ReadAndResetDurationMaxReturnsOnCall(1, 10*time.Second)
	})

	JustBeforeEach(func() {
		runner = metrics.NewDBStatMetronNotifier(
			logger,
			fakeClock,
			fakeDBStats,
			fakeMetronClient,
			fakeMonitor,
		)
		process = ifrit.Background(runner)
		Eventually(process.Ready()).Should(BeClosed())

		fakeClock.Increment(metrics.DefaultEmitFrequency)
	})

	AfterEach(func() {
		ginkgomon.Interrupt(process)
	})

	It("emits a metric for the number of open database connections", func() {
		Eventually(metricsChan).Should(Receive(Equal(FakeGauge{"DBOpenConnections", 10})))
		fakeClock.Increment(metrics.DefaultEmitFrequency)
		Eventually(metricsChan).Should(Receive(Equal(FakeGauge{"DBOpenConnections", 100})))
	})

	It("emits a metric for the number of total queries against the database", func() {
		Eventually(metricsChan).Should(Receive(Equal(FakeGauge{"DBQueriesTotal", 20})))
		fakeClock.Increment(metrics.DefaultEmitFrequency)
		Eventually(metricsChan).Should(Receive(Equal(FakeGauge{"DBQueriesTotal", 200})))
	})

	It("emits a metric for the number of queries succeeded against the database", func() {
		Eventually(metricsChan).Should(Receive(Equal(FakeGauge{"DBQueriesSucceeded", 15})))
		fakeClock.Increment(metrics.DefaultEmitFrequency)
		Eventually(metricsChan).Should(Receive(Equal(FakeGauge{"DBQueriesSucceeded", 150})))
	})

	It("emits a metric for the number of queries failed against the database", func() {
		Eventually(metricsChan).Should(Receive(Equal(FakeGauge{"DBQueriesFailed", 5})))
		fakeClock.Increment(metrics.DefaultEmitFrequency)
		Eventually(metricsChan).Should(Receive(Equal(FakeGauge{"DBQueriesFailed", 50})))
	})

	It("emits a metric for the max number of queries in flight against the database", func() {
		Eventually(metricsChan).Should(Receive(Equal(FakeGauge{"DBQueriesInFlight", 8})))
		fakeClock.Increment(metrics.DefaultEmitFrequency)
		Eventually(metricsChan).Should(Receive(Equal(FakeGauge{"DBQueriesInFlight", 80})))
	})

	It("emits a metric for the max duration of queries", func() {
		Eventually(metricsChan).Should(Receive(Equal(FakeGauge{"DBQueryDurationMax", int(time.Second)})))
		fakeClock.Increment(metrics.DefaultEmitFrequency)
		Eventually(metricsChan).Should(Receive(Equal(FakeGauge{"DBQueryDurationMax", int(10 * time.Second)})))
	})
})
