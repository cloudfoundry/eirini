package crash_test

import (
	"os"
	"testing"

	"code.cloudfoundry.org/eirini"
	"code.cloudfoundry.org/eirini/integration/util"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"

	_ "k8s.io/client-go/plugin/pkg/client/auth"
)

func TestCrash(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Crash Suite")
}

var (
	fixture            util.Fixture
	pathToCrashEmitter string
)

var _ = BeforeSuite(func() {
	var err error
	pathToCrashEmitter, err = gexec.Build("code.cloudfoundry.org/eirini/cmd/event-reporter")
	Expect(err).NotTo(HaveOccurred())

	fixture, err = util.NewFixture(GinkgoWriter)
	Expect(err).NotTo(HaveOccurred())
})

var _ = BeforeEach(func() {
	var err error
	fixture, err = fixture.SetUp()
	Expect(err).NotTo(HaveOccurred())
})

var _ = AfterEach(func() {
	Expect(fixture.TearDown()).To(Succeed())
})

var _ = AfterSuite(func() {
	gexec.CleanupBuildArtifacts()
})

func defaultEventReporterConfig() *eirini.EventReporterConfig {
	config := &eirini.EventReporterConfig{
		KubeConfig: eirini.KubeConfig{
			Namespace:  fixture.Namespace,
			ConfigPath: os.Getenv("INTEGRATION_KUBECONFIG"),
		},
		CcInternalAPI: "doesitmatter.com",
		CCCertPath:    util.PathToTestFixture("cert"),
		CCCAPath:      util.PathToTestFixture("cert"),
		CCKeyPath:     util.PathToTestFixture("key"),
	}

	return config
}
