package metrics_test

import (
	"os"
	"time"

	"code.cloudfoundry.org/bbs/metrics"
	mfakes "code.cloudfoundry.org/diego-logging-client/testhelpers"
	"code.cloudfoundry.org/lager/lagertest"
	"github.com/tedsuo/ifrit"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("BBSElectionMetronNotifier", func() {
	var (
		pmn              ifrit.Process
		fakeMetronClient *mfakes.FakeIngressClient
	)

	BeforeEach(func() {
		fakeMetronClient = new(mfakes.FakeIngressClient)
	})

	JustBeforeEach(func() {
		pmn = ifrit.Invoke(metrics.NewBBSElectionMetronNotifier(lagertest.NewTestLogger("test"), fakeMetronClient))
	})

	AfterEach(func() {
		pmn.Signal(os.Interrupt)
		Eventually(pmn.Wait(), 2*time.Second).Should(Receive())
	})

	Context("when the metron notifier starts up", func() {
		It("should emit an event that BBS has started", func() {
			Eventually(fakeMetronClient.SendMetricCallCount).Should(Equal(1))
			name, value := fakeMetronClient.SendMetricArgsForCall(0)
			Expect(name).To(Equal("BBSMasterElected"))
			Expect(value).To(Equal(1))
		})
	})
})
