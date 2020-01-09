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

	"k8s.io/client-go/kubernetes"
	_ "k8s.io/client-go/plugin/pkg/client/auth"
	"k8s.io/client-go/tools/clientcmd"
)

func TestOpi(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Opi Suite")
}

const secretName = "certs-secret"

var (
	pathToOpi        string
	namespace        string
	clientset        kubernetes.Interface
	pspName          string
	httpClient       *http.Client
	eiriniConfigFile *os.File
	session          *gexec.Session
	url              string
)

var _ = BeforeSuite(func() {
	var err error
	pathToOpi, err = gexec.Build("code.cloudfoundry.org/eirini/cmd/opi")
	Expect(err).NotTo(HaveOccurred())
})

var _ = AfterSuite(func() {
	gexec.CleanupBuildArtifacts()
})

var _ = BeforeEach(func() {
	kubeConfigPath := os.Getenv("INTEGRATION_KUBECONFIG")
	if kubeConfigPath == "" {
		Fail("INTEGRATION_KUBECONFIG is not provided")
	}
	kubeConfig, err := clientcmd.BuildConfigFromFlags("", kubeConfigPath)
	Expect(err).ToNot(HaveOccurred())

	clientset, err = kubernetes.NewForConfig(kubeConfig)
	Expect(err).ToNot(HaveOccurred())

	namespace = util.CreateRandomNamespace(clientset)
	pspName = fmt.Sprintf("%s-psp", namespace)
	Expect(util.CreatePodCreationPSP(namespace, pspName, clientset)).To(Succeed())
	Expect(util.CreateEmptySecret(namespace, secretName, clientset)).To(Succeed())

	httpClient, err = util.MakeTestHTTPClient()
	Expect(err).ToNot(HaveOccurred())

	eiriniConfig := util.DefaultEiriniConfig(namespace, secretName)
	eiriniConfigFile, err = util.CreateOpiConfigFromFixtures(eiriniConfig)
	Expect(err).ToNot(HaveOccurred())

	command := exec.Command(pathToOpi, "connect", "-c", eiriniConfigFile.Name()) // #nosec G204
	session, err = gexec.Start(command, GinkgoWriter, GinkgoWriter)
	Expect(err).ToNot(HaveOccurred())

	url = fmt.Sprintf("https://localhost:%d/", eiriniConfig.Properties.TLSPort)
	Eventually(func() error {
		_, getErr := httpClient.Get(url)
		return getErr
	}, "5s").Should(Succeed())
})

var _ = AfterEach(func() {
	if eiriniConfigFile != nil {
		Expect(os.Remove(eiriniConfigFile.Name())).To(Succeed())
	}
	if session != nil {
		session.Kill()
	}

	Expect(util.DeleteNamespace(namespace, clientset)).To(Succeed())
	Expect(util.DeletePSP(pspName, clientset)).To(Succeed())
})
