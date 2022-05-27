package migration_test

import (
	"encoding/json"
	"os"
	"testing"

	"code.cloudfoundry.org/eirini"
	"code.cloudfoundry.org/eirini/tests"
	"code.cloudfoundry.org/eirini/tests/integration"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"
	_ "k8s.io/client-go/plugin/pkg/client/auth"
)

func TestMigration(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Migration Suite")
}

var (
	eiriniBins integration.EiriniBinaries
	fixture    *tests.Fixture
)

var _ = SynchronizedBeforeSuite(func() []byte {
	eiriniBins = integration.NewEiriniBinaries()
	eiriniBins.Migration.Build()

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

var _ = JustBeforeEach(func() {
	migrationConfig := eirini.MigrationConfig{
		WorkloadsNamespace: fixture.Namespace,
		KubeConfig: eirini.KubeConfig{
			ConfigPath: tests.GetKubeconfig(),
		},
	}
	session, configFilePath := eiriniBins.Migration.Run(migrationConfig)
	Eventually(session, "5s").Should(gexec.Exit(0))
	Expect(os.Remove(configFilePath)).To(Succeed())
})

var _ = AfterEach(func() {
	fixture.TearDown()
})
