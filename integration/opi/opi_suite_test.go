package opi_test

import (
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"testing"

	"code.cloudfoundry.org/eirini/integration/util"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"

	_ "k8s.io/client-go/plugin/pkg/client/auth"
)

func TestOpi(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Opi Suite")
}

const secretName = "certs-secret"

var (
	fixture          util.Fixture
	pathToOpi        string
	httpClient       *http.Client
	eiriniConfigFile *os.File
	session          *gexec.Session
	url              string
)

var _ = BeforeSuite(func() {
	var err error
	pathToOpi, err = gexec.Build("code.cloudfoundry.org/eirini/cmd/opi")
	Expect(err).NotTo(HaveOccurred())

	fixture, err = util.NewFixture(GinkgoWriter)
	Expect(err).NotTo(HaveOccurred())
})

var _ = AfterSuite(func() {
	gexec.CleanupBuildArtifacts()
})

var _ = BeforeEach(func() {
	var err error
	fixture, err = fixture.SetUp()
	Expect(err).NotTo(HaveOccurred())

	Expect(util.CreateEmptySecret(fixture.Namespace, secretName, fixture.Clientset)).To(Succeed())

	httpClient, err = util.MakeTestHTTPClient()
	Expect(err).ToNot(HaveOccurred())

	eiriniConfig := util.DefaultEiriniConfig(fixture.Namespace)
	eiriniConfigFile, err = util.CreateConfigFile(eiriniConfig)
	Expect(err).ToNot(HaveOccurred())

	command := exec.Command(pathToOpi, "connect", "-c", eiriniConfigFile.Name()) // #nosec G204
	session, err = gexec.Start(command, GinkgoWriter, GinkgoWriter)
	Expect(err).ToNot(HaveOccurred())

	url = fmt.Sprintf("https://localhost:%d/", eiriniConfig.Properties.TLSPort)
	Eventually(func() error {
		_, getErr := httpClient.Get(url)
		return getErr
	}, "10s").Should(Succeed())
})

var _ = AfterEach(func() {
	if eiriniConfigFile != nil {
		Expect(os.Remove(eiriniConfigFile.Name())).To(Succeed())
	}
	if session != nil {
		session.Kill()
	}

	Expect(fixture.TearDown()).To(Succeed())
})
