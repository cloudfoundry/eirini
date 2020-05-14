package staging_reporter_test

import (
	"testing"

	"code.cloudfoundry.org/eirini/integration/util"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"
	_ "k8s.io/client-go/plugin/pkg/client/auth/oidc"
)

func TestStagingReporter(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "TaskReporter Suite")
}

var (
	fixture            *util.Fixture
	pathToTaskReporter string
)

var _ = SynchronizedBeforeSuite(func() []byte {
	var err error
	pathToTaskReporter, err = gexec.Build("code.cloudfoundry.org/eirini/cmd/task-reporter")
	Expect(err).NotTo(HaveOccurred())
	return []byte(pathToTaskReporter)
}, func(data []byte) {
	pathToTaskReporter = string(data)
	fixture = util.NewFixture(GinkgoWriter)
})

var _ = SynchronizedAfterSuite(func() {}, func() {
	gexec.CleanupBuildArtifacts()
})

var _ = BeforeEach(func() {
	fixture.SetUp()

	Expect(util.CreateSecretWithStringData(fixture.Namespace, "cc-uploader-secret", fixture.Clientset, map[string]string{"foo1": "val1", "bar1": "val2"})).To(Succeed())
	Expect(util.CreateSecretWithStringData(fixture.Namespace, "eirini-client-secret", fixture.Clientset, map[string]string{"foo2": "val1", "bar2": "val2"})).To(Succeed())
	Expect(util.CreateSecretWithStringData(fixture.Namespace, "ca-cert-secret", fixture.Clientset, map[string]string{"foo3": "val1", "bar3": "val2"})).To(Succeed())
})

var _ = AfterEach(func() {
	fixture.TearDown()
})
