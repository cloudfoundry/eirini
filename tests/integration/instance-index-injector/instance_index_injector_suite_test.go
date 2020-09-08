package instance_index_injector_test

import (
	"encoding/json"
	"io/ioutil"
	"os"
	"testing"

	"code.cloudfoundry.org/eirini/tests"
	. "github.com/onsi/ginkgo"
	"github.com/onsi/ginkgo/config"
	. "github.com/onsi/gomega"
	_ "k8s.io/client-go/plugin/pkg/client/auth"
)

func TestInstanceIndexInjector(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "InstanceIndexInjector Suite")
}

var (
	fixture             *tests.Fixture
	eiriniBins          tests.EiriniBinaries
	binsPath            string
	telepresenceRunner  *tests.TelepresenceRunner
	telepresenceService string
)

const (
	startPort = 20000
)

var _ = SynchronizedBeforeSuite(func() []byte {
	var err error

	telepresenceService = "local-binaries-" + tests.GenerateGUID()[:8]
	telepresenceRunner, err = tests.StartTelepresence(telepresenceService, startPort, config.GinkgoConfig.ParallelTotal)
	Expect(err).NotTo(HaveOccurred())

	binsPath, err = ioutil.TempDir("", "bins")
	Expect(err).NotTo(HaveOccurred())

	eiriniBins = tests.NewEiriniBinaries(binsPath)
	eiriniBins.TelepresenceService = telepresenceService

	data, err := json.Marshal(eiriniBins)
	Expect(err).NotTo(HaveOccurred())

	return data
}, func(data []byte) {
	err := json.Unmarshal(data, &eiriniBins)
	Expect(err).NotTo(HaveOccurred())

	telepresenceService = eiriniBins.TelepresenceService

	fixture = tests.NewFixture(GinkgoWriter)
})

var _ = SynchronizedAfterSuite(func() {
	fixture.Destroy()
}, func() {
	eiriniBins.TearDown()
	Expect(os.RemoveAll(binsPath)).To(Succeed())
	telepresenceRunner.Stop()
})

var _ = BeforeEach(func() {
	Skip("This test requires telepresence, which can only be run in privileged containers. This test is skipped until we enable runnig privileged workloads in concourse")
	fixture.SetUp()
})

var _ = AfterEach(func() {
	fixture.TearDown()
})
