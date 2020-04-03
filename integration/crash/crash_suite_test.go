package crash_test

import (
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
	fixture            *util.Fixture
	pathToCrashEmitter string
)

var _ = BeforeSuite(func() {
	var err error
	pathToCrashEmitter, err = gexec.Build("code.cloudfoundry.org/eirini/cmd/event-reporter")
	Expect(err).NotTo(HaveOccurred())

	fixture = util.NewFixture(GinkgoWriter)
})

var _ = BeforeEach(func() {
	fixture.SetUp()
})

var _ = AfterEach(func() {
	fixture.TearDown()
})

var _ = AfterSuite(func() {
	gexec.CleanupBuildArtifacts()
})

func defaultEventReporterConfig() *eirini.EventReporterConfig {
	config := &eirini.EventReporterConfig{
		KubeConfig: eirini.KubeConfig{
			Namespace:  fixture.Namespace,
			ConfigPath: fixture.KubeConfigPath,
		},
		CcInternalAPI: "doesitmatter.com",
		CCCertPath:    util.PathToTestFixture("cert"),
		CCCAPath:      util.PathToTestFixture("cert"),
		CCKeyPath:     util.PathToTestFixture("key"),
	}

	return config
}
