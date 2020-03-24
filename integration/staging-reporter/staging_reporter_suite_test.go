package staging_reporter_test

import (
	"os"
	"testing"

	"code.cloudfoundry.org/eirini"
	"code.cloudfoundry.org/eirini/integration/util"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"
	_ "k8s.io/client-go/plugin/pkg/client/auth/oidc"
)

func TestStagingReporter(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "StagingReporter Suite")
}

var (
	fixture               util.Fixture
	pathToStagingReporter string
)

var _ = BeforeSuite(func() {
	var err error
	pathToStagingReporter, err = gexec.Build("code.cloudfoundry.org/eirini/cmd/staging-reporter")
	Expect(err).NotTo(HaveOccurred())

	fixture, err = util.NewFixture()
	Expect(err).NotTo(HaveOccurred())
})

var _ = BeforeEach(func() {
	var err error
	fixture, err = fixture.SetUp()
	Expect(err).NotTo(HaveOccurred())

	Expect(util.CreateSecretWithStringData(fixture.Namespace, "cc-uploader-secret", fixture.Clientset, map[string]string{"foo1": "val1", "bar1": "val2"})).To(Succeed())
	Expect(util.CreateSecretWithStringData(fixture.Namespace, "eirini-client-secret", fixture.Clientset, map[string]string{"foo2": "val1", "bar2": "val2"})).To(Succeed())
	Expect(util.CreateSecretWithStringData(fixture.Namespace, "ca-cert-secret", fixture.Clientset, map[string]string{"foo3": "val1", "bar3": "val2"})).To(Succeed())
})

var _ = AfterEach(func() {
	Expect(fixture.TearDown()).To(Succeed())
})

var _ = AfterSuite(func() {
	gexec.CleanupBuildArtifacts()
})

func defaultStagingReporterConfig() *eirini.StagingReporterConfig {
	config := &eirini.StagingReporterConfig{
		KubeConfig: eirini.KubeConfig{
			Namespace:  fixture.Namespace,
			ConfigPath: os.Getenv("INTEGRATION_KUBECONFIG"),
		},
		EiriniCertPath: util.PathToTestFixture("cert"),
		CAPath:         util.PathToTestFixture("cert"),
		EiriniKeyPath:  util.PathToTestFixture("key"),
	}

	return config
}
