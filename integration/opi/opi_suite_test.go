package opi_test

import (
	"fmt"
	"os"
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
	pathToOpi string
	namespace string
	clientset kubernetes.Interface
	pspName   string
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
	config, err := clientcmd.BuildConfigFromFlags("", kubeConfigPath)
	Expect(err).ToNot(HaveOccurred())

	clientset, err = kubernetes.NewForConfig(config)
	Expect(err).ToNot(HaveOccurred())

	namespace = util.CreateRandomNamespace(clientset)
	pspName = fmt.Sprintf("%s-psp", namespace)
	Expect(util.CreatePodCreationPSP(namespace, pspName, clientset)).To(Succeed())
	Expect(util.CreateEmptySecret(namespace, secretName, clientset)).To(Succeed())
})

var _ = AfterEach(func() {
	Expect(util.DeleteNamespace(namespace, clientset)).To(Succeed())
	Expect(util.DeletePSP(pspName, clientset)).To(Succeed())
})
