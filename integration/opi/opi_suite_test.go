package opi_test

import (
	"crypto/tls"
	"crypto/x509"
	"io/ioutil"
	"net/http"
	"os"
	"sync"
	"testing"

	"code.cloudfoundry.org/cfhttp/v2"
	"code.cloudfoundry.org/eirini"
	. "github.com/onsi/ginkgo"
	ginkgoconfig "github.com/onsi/ginkgo/config"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"
	"gopkg.in/yaml.v2"
)

func TestOpi(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Opi Suite")
}

var (
	pathToOpi    string
	lastPortUsed int
	portLock     sync.Mutex
	once         sync.Once
	namespace    string
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
	// TODO
	namespace = "test"
})

func pathToTestFixture(relativePath string) string {
	cwd, err := os.Getwd()
	Expect(err).ToNot(HaveOccurred())
	return cwd + "/../fixtures/" + relativePath
}

func defaultEiriniConfig() *eirini.Config {
	kubeConfigPath := os.Getenv("INTEGRATION_KUBECONFIG")
	if kubeConfigPath == "" {
		Fail("INTEGRATION_KUBECONFIG is not provided")
	}
	config := &eirini.Config{
		Properties: eirini.Properties{
			KubeConfig: eirini.KubeConfig{
				ConfigPath: kubeConfigPath,
				Namespace:  namespace,
			},
			CCCAPath:       pathToTestFixture("cert"),
			CCCertPath:     pathToTestFixture("cert"),
			CCKeyPath:      pathToTestFixture("key"),
			ServerCertPath: pathToTestFixture("cert"),
			ServerKeyPath:  pathToTestFixture("key"),
			ClientCAPath:   pathToTestFixture("cert"),
			TLSPort:        int(nextAvailPort()),

			DownloaderImage: "docker.io/eirini/integration_test_staging",
			ExecutorImage:   "docker.io/eirini/integration_test_staging",
			UploaderImage:   "docker.io/eirini/integration_test_staging",
		},
	}

	return config
}

func createOpiConfigFromFixtures(config *eirini.Config) *os.File {
	bs, err := yaml.Marshal(config)
	Expect(err).ToNot(HaveOccurred())

	file, err := createConfigFile(bs)
	Expect(err).ToNot(HaveOccurred())
	return file
}

func createConfigFile(yamlBytes []byte) (*os.File, error) {
	configFile, err := ioutil.TempFile("", "config.yml")
	Expect(err).ToNot(HaveOccurred())

	err = ioutil.WriteFile(configFile.Name(), yamlBytes, os.ModePerm)
	Expect(err).ToNot(HaveOccurred())

	return configFile, err
}

func makeTestHTTPClient() *http.Client {
	bs, err := ioutil.ReadFile(pathToTestFixture("cert"))
	Expect(err).ToNot(HaveOccurred())

	certPool := x509.NewCertPool()
	Expect(certPool.AppendCertsFromPEM(bs)).To(BeTrue())
	tlsConfig := &tls.Config{
		RootCAs: certPool,
	}
	httpClient := cfhttp.NewClient(cfhttp.WithTLSConfig(tlsConfig))
	return httpClient
}

func nextAvailPort() uint16 {
	portLock.Lock()
	defer portLock.Unlock()

	if lastPortUsed == 0 {
		once.Do(func() {
			const portRangeStart = 61000
			lastPortUsed = portRangeStart + ginkgoconfig.GinkgoConfig.ParallelNode
		})
	}

	lastPortUsed += ginkgoconfig.GinkgoConfig.ParallelTotal
	return uint16(lastPortUsed)
}
