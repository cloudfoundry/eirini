package eats_test

import (
	"bytes"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"math/rand"
	"net/http"
	"os"
	"os/exec"
	"testing"
	"time"

	"code.cloudfoundry.org/cfhttp/v2"
	"code.cloudfoundry.org/eirini"
	"code.cloudfoundry.org/eirini/integration/util"
	"code.cloudfoundry.org/eirini/models/cf"
	"code.cloudfoundry.org/tlsconfig/certtest"
	. "github.com/onsi/ginkgo"
	ginkgoconfig "github.com/onsi/ginkgo/config"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"
	"k8s.io/client-go/rest"
)

func TestEats(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Eats Suite")
}

var (
	fixture *util.Fixture
)

var _ = BeforeSuite(func() {
	fixture = util.NewFixture(GinkgoWriter)

	SetDefaultEventuallyTimeout(5 * time.Second)
})

var _ = BeforeEach(func() {
	fixture.SetUp()
})

var _ = AfterEach(func() {
	fixture.TearDown()
})

func generateKeyPair(name string) (string, string) {
	authority, err := certtest.BuildCA(name)
	Expect(err).NotTo(HaveOccurred())
	cert, err := authority.BuildSignedCertificate(name, certtest.WithDomains(name))
	Expect(err).NotTo(HaveOccurred())

	certData, keyData, err := cert.CertificatePEMAndPrivateKey()
	Expect(err).NotTo(HaveOccurred())
	metricsCertPath := writeTempFile(certData, fmt.Sprintf("%s-cert", name))
	metricsKeyPath := writeTempFile(keyData, fmt.Sprintf("%s-key", name))

	return metricsCertPath, metricsKeyPath
}

func runOpi(certPath, keyPath string) (*gexec.Session, string, string) {
	pathToOpi, err := gexec.Build("code.cloudfoundry.org/eirini/cmd/opi")
	Expect(err).NotTo(HaveOccurred())

	eiriniConfig := &eirini.Config{
		Properties: eirini.Properties{
			KubeConfig: eirini.KubeConfig{
				ConfigPath: os.Getenv("INTEGRATION_KUBECONFIG"),
				Namespace:  fixture.Namespace,
			},
			CCCAPath:             certPath,
			CCCertPath:           certPath,
			CCKeyPath:            keyPath,
			ServerCertPath:       certPath,
			ServerKeyPath:        keyPath,
			ClientCAPath:         certPath,
			TLSPort:              61000 + rand.Intn(1000) + ginkgoconfig.GinkgoConfig.ParallelNode,
			CCUploaderSecretName: "cc-uploader-secret",
			CCUploaderCertPath:   "path-to-crt",
			CCUploaderKeyPath:    "path-to-key",

			ClientCertsSecretName: "eirini-client-secret",
			ClientKeyPath:         "path-to-key",
			ClientCertPath:        "path-to-crt",

			CACertSecretName: "global-ca-secret",
			CACertPath:       "path-to-ca",

			DownloaderImage: "docker.io/eirini/integration_test_staging",
			ExecutorImage:   "docker.io/eirini/integration_test_staging",
			UploaderImage:   "docker.io/eirini/integration_test_staging",

			ApplicationServiceAccount: "default",
		},
	}
	eiriniConfigFile, err := util.CreateConfigFile(eiriniConfig)
	Expect(err).ToNot(HaveOccurred())

	eiriniCommand := exec.Command(pathToOpi, "connect", "-c", eiriniConfigFile.Name()) // #nosec G204
	eiriniSession, err := gexec.Start(eiriniCommand, GinkgoWriter, GinkgoWriter)
	Expect(err).ToNot(HaveOccurred())

	opiURL := fmt.Sprintf("https://localhost:%d", eiriniConfig.Properties.TLSPort)

	return eiriniSession, eiriniConfigFile.Name(), opiURL
}

func makeTestHTTPClient(certPath, keyPath string) (*http.Client, error) {
	bs, err := ioutil.ReadFile(certPath)
	if err != nil {
		return nil, err
	}

	clientCert, err := tls.LoadX509KeyPair(certPath, keyPath)
	if err != nil {
		return nil, err
	}

	certPool := x509.NewCertPool()
	if !certPool.AppendCertsFromPEM(bs) {
		return nil, err
	}
	tlsConfig := &tls.Config{
		RootCAs:      certPool,
		Certificates: []tls.Certificate{clientCert},
	}
	httpClient := cfhttp.NewClient(cfhttp.WithTLSConfig(tlsConfig))

	return httpClient, nil
}

func waitOpiReady(httpClient rest.HTTPClient, opiURL string) {
	Eventually(func() error {
		desireAppReq, err := http.NewRequest("GET", fmt.Sprintf("%s/apps", opiURL), bytes.NewReader([]byte{}))
		Expect(err).ToNot(HaveOccurred())
		_, err = httpClient.Do(desireAppReq) //nolint:bodyclose
		return err
	}).Should(Succeed())
}

func desireLRP(httpClient rest.HTTPClient, opiURL string, lrpRequest cf.DesireLRPRequest) (*http.Response, error) {
	body, err := json.Marshal(lrpRequest)
	if err != nil {
		return nil, err
	}
	desireLrpReq, err := http.NewRequest("PUT", fmt.Sprintf("%s/apps/the-app-guid", opiURL), bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	return httpClient.Do(desireLrpReq)
}

func writeTempFile(content []byte, fileName string) string {
	configFile, err := ioutil.TempFile("", fileName)
	Expect(err).ToNot(HaveOccurred())
	defer configFile.Close()

	err = ioutil.WriteFile(configFile.Name(), content, os.ModePerm)
	Expect(err).ToNot(HaveOccurred())
	return configFile.Name()
}
