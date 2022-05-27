package task_reporter_test

import (
	"encoding/json"
	"os"
	"testing"
	"time"

	"code.cloudfoundry.org/eirini"
	"code.cloudfoundry.org/eirini/tests"
	"code.cloudfoundry.org/eirini/tests/integration"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"
	_ "k8s.io/client-go/plugin/pkg/client/auth"
)

func TestStagingReporter(t *testing.T) {
	SetDefaultEventuallyTimeout(2 * time.Minute)
	RegisterFailHandler(Fail)
	RunSpecs(t, "TaskReporter Suite")
}

var (
	fixture         *tests.Fixture
	eiriniBins      integration.EiriniBinaries
	envVarOverrides []string

	config     *eirini.TaskReporterConfig
	configFile string
	session    *gexec.Session
)

var _ = SynchronizedBeforeSuite(func() []byte {
	eiriniBins = integration.NewEiriniBinaries()
	eiriniBins.TaskReporter.Build()

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
	envVarOverrides = []string{}
	fixture.SetUp()

	Expect(integration.CreateSecretWithStringData(fixture.Namespace, "cc-uploader-secret", fixture.Clientset, map[string]string{"foo1": "val1", "bar1": "val2"})).To(Succeed())
	Expect(integration.CreateSecretWithStringData(fixture.Namespace, "eirini-client-secret", fixture.Clientset, map[string]string{"foo2": "val1", "bar2": "val2"})).To(Succeed())
	Expect(integration.CreateSecretWithStringData(fixture.Namespace, "ca-cert-secret", fixture.Clientset, map[string]string{"foo3": "val1", "bar3": "val2"})).To(Succeed())
})

var _ = JustBeforeEach(func() {
	session, configFile = eiriniBins.TaskReporter.Run(config, envVarOverrides...)
	Eventually(session).Should(gbytes.Say("Starting workers"))
})

var _ = AfterEach(func() {
	if session != nil {
		session.Kill()
	}
	Expect(os.Remove(configFile)).To(Succeed())

	fixture.TearDown()
})
