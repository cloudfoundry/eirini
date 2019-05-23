package cmd_test

import (
	"crypto/tls"
	"crypto/x509"
	"io/ioutil"
	"net/http"
	"os"
	"sync"
	"testing"

	cfhttp "code.cloudfoundry.org/cfhttp/v2"
	"code.cloudfoundry.org/eirini"
	natsserver "github.com/nats-io/gnatsd/server"
	. "github.com/onsi/ginkgo"
	ginkgoconfig "github.com/onsi/ginkgo/config"
	. "github.com/onsi/gomega"
	yaml "gopkg.in/yaml.v2"
	_ "k8s.io/client-go/plugin/pkg/client/auth"
)

var (
	lastPortUsed int
	portLock     sync.Mutex
	once         sync.Once
)

func TestIntegration(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "cmd Suite")
}

func pathToTestFixture(relativePath string) string {
	cwd, err := os.Getwd()
	Expect(err).ToNot(HaveOccurred())
	return cwd + "/fixtures/" + relativePath
}

func defaultEiriniConfig(natsServerOpts natsserver.Options) *eirini.Config {
	config := &eirini.Config{
		Properties: eirini.Properties{
			KubeConfigPath:      pathToTestFixture("kube.conf"),
			CCCAPath:            pathToTestFixture("cert"),
			CCCertPath:          pathToTestFixture("cert"),
			CCKeyPath:           pathToTestFixture("key"),
			NatsIP:              natsServerOpts.Host,
			NatsPort:            natsServerOpts.Port,
			NatsPassword:        natsServerOpts.Password,
			LoggregatorCAPath:   pathToTestFixture("cert"),
			LoggregatorCertPath: pathToTestFixture("cert"),
			LoggregatorKeyPath:  pathToTestFixture("key"),
		},
	}

	return config
}

func createOpiConfigFromFixtures(config *eirini.Config) (*os.File, error) {
	configFile, err := ioutil.TempFile("", "opi-config.yml")
	Expect(err).ToNot(HaveOccurred())

	bs, err := yaml.Marshal(config)
	Expect(err).ToNot(HaveOccurred())
	err = ioutil.WriteFile(configFile.Name(), bs, os.ModePerm)
	Expect(err).ToNot(HaveOccurred())

	return configFile, err
}

func makeTestHttpClient() *http.Client {
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
