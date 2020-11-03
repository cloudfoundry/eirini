package events_test

import (
	"encoding/json"
	"testing"
	"time"

	"code.cloudfoundry.org/eirini/tests"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	_ "k8s.io/client-go/plugin/pkg/client/auth"
)

func TestStagingReporter(t *testing.T) {
	RegisterFailHandler(Fail)
	SetDefaultEventuallyTimeout(30 * time.Second)
	SetDefaultConsistentlyDuration(10 * time.Second)
	RunSpecs(t, "Events Suite")
}

var (
	fixture    *tests.Fixture
	eiriniBins tests.EiriniBinaries
)

var _ = SynchronizedBeforeSuite(func() []byte {
	eiriniBins = tests.NewEiriniBinaries()
	eiriniBins.EventsReporter.Build()

	data, err := json.Marshal(eiriniBins)
	Expect(err).NotTo(HaveOccurred())

	return data
}, func(data []byte) {
	err := json.Unmarshal(data, &eiriniBins)
	Expect(err).NotTo(HaveOccurred())

	fixture = tests.NewFixture(GinkgoWriter)
})

var _ = SynchronizedAfterSuite(func() {
	fixture.Destroy()
}, func() {
	eiriniBins.TearDown()
})

var _ = BeforeEach(func() {
	fixture.SetUp()
})

var _ = AfterEach(func() {
	fixture.TearDown()
})
