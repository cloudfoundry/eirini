package crash_test

import (
	"fmt"
	"io/ioutil"
	"os"
	"testing"

	"code.cloudfoundry.org/eirini"
	"code.cloudfoundry.org/eirini/integration/util"
	"gopkg.in/yaml.v2"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"

	"k8s.io/client-go/kubernetes"
	_ "k8s.io/client-go/plugin/pkg/client/auth"
	"k8s.io/client-go/tools/clientcmd"
)

func TestCrash(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Crash Suite")
}

var (
	namespace          string
	clientset          kubernetes.Interface
	kubeConfigPath     string
	pspName            string
	pathToCrashEmitter string
)

var _ = BeforeSuite(func() {
	var err error
	pathToCrashEmitter, err = gexec.Build("code.cloudfoundry.org/eirini/cmd/event-reporter")
	Expect(err).NotTo(HaveOccurred())

	kubeConfigPath = os.Getenv("INTEGRATION_KUBECONFIG")
	if kubeConfigPath == "" {
		Fail("INTEGRATION_KUBECONFIG is not provided")
	}
	config, err := clientcmd.BuildConfigFromFlags("", os.Getenv("INTEGRATION_KUBECONFIG"))
	Expect(err).ToNot(HaveOccurred())

	clientset, err = kubernetes.NewForConfig(config)
	Expect(err).ToNot(HaveOccurred())
})

var _ = BeforeEach(func() {
	namespace = util.CreateRandomNamespace(clientset)
	pspName = fmt.Sprintf("%s-psp", namespace)
	Expect(util.CreatePodCreationPSP(namespace, pspName, clientset)).To(Succeed())
})

var _ = AfterEach(func() {
	Expect(util.DeleteNamespace(namespace, clientset)).To(Succeed())
	Expect(util.DeletePSP(pspName, clientset)).To(Succeed())
})

var _ = AfterSuite(func() {
	gexec.CleanupBuildArtifacts()
})

func defaultEventReporterConfig() *eirini.EventReporterConfig {
	config := &eirini.EventReporterConfig{
		KubeConfig: eirini.KubeConfig{
			ConfigPath: os.Getenv("INTEGRATION_KUBECONFIG"),
		},
		CcInternalAPI: "doesitmatter.com",
		CCCertPath:    util.PathToTestFixture("cert"),
		CCCAPath:      util.PathToTestFixture("cert"),
		CCKeyPath:     util.PathToTestFixture("key"),
	}

	return config
}

func createEventReporterConfigFile(config *eirini.EventReporterConfig) (*os.File, error) {
	bs, err := yaml.Marshal(config)
	Expect(err).ToNot(HaveOccurred())

	return createConfigFile(bs)
}
func createConfigFile(yamlBytes []byte) (*os.File, error) {
	configFile, err := ioutil.TempFile("", "config.yml")
	Expect(err).ToNot(HaveOccurred())

	err = ioutil.WriteFile(configFile.Name(), yamlBytes, os.ModePerm)
	Expect(err).ToNot(HaveOccurred())

	return configFile, err
}
