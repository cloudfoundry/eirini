package tests

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"

	"code.cloudfoundry.org/cfhttp/v2"
	"code.cloudfoundry.org/eirini"
	"code.cloudfoundry.org/eirini/tests/eats/wiremock"

	// nolint:golint,stylecheck
	. "github.com/onsi/ginkgo"

	// nolint:golint,stylecheck
	. "github.com/onsi/gomega"
	"gopkg.in/yaml.v2"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/dynamic"
)

type EATSFixture struct {
	Fixture

	Wiremock         *wiremock.Wiremock
	DynamicClientset dynamic.Interface

	eiriniCertPath   string
	eiriniKeyPath    string
	eiriniHTTPClient *http.Client
}

func NewEATSFixture(baseFixture Fixture, dynamicClientset dynamic.Interface, wiremock *wiremock.Wiremock) *EATSFixture {
	return &EATSFixture{
		Fixture:          baseFixture,
		DynamicClientset: dynamicClientset,
		Wiremock:         wiremock,
	}
}

func (f *EATSFixture) SetUp() {
	if IsMultiNamespaceEnabled() {
		f.Fixture.SetUp()
	} else {
		f.Namespace = GetEiriniWorkloadsNamespace()
	}
}

func (f *EATSFixture) TearDown() {
	if f == nil {
		Fail("failed to initialize fixture")
	}

	f.Fixture.TearDown()

	if f.eiriniCertPath != "" {
		Expect(os.RemoveAll(f.eiriniCertPath)).To(Succeed())
	}

	if f.eiriniKeyPath != "" {
		Expect(os.RemoveAll(f.eiriniKeyPath)).To(Succeed())
	}
}

func (f *EATSFixture) GetEiriniHTTPClient() *http.Client {
	f.eiriniCertPath, f.eiriniKeyPath = f.downloadEiriniCertificates()

	if f.eiriniHTTPClient == nil {
		var err error
		f.eiriniHTTPClient, err = f.makeTestHTTPClient()
		Expect(err).ToNot(HaveOccurred())
	}

	return f.eiriniHTTPClient
}

func (f *EATSFixture) makeTestHTTPClient() (*http.Client, error) {
	bs, err := ioutil.ReadFile(f.eiriniCertPath)
	if err != nil {
		return nil, err
	}

	clientCert, err := tls.LoadX509KeyPair(f.eiriniCertPath, f.eiriniKeyPath)
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
		MinVersion:   tls.VersionTLS12,
	}
	client := cfhttp.NewClient(cfhttp.WithTLSConfig(tlsConfig))

	return client, nil
}

func (f *EATSFixture) downloadEiriniCertificates() (string, string) {
	certFile, err := ioutil.TempFile("", "cert-")
	Expect(err).NotTo(HaveOccurred())

	defer certFile.Close()

	eiriniSysNs := GetEiriniSystemNamespace()
	eiriniTLSSecretName := getEiriniTLSSecretName()

	_, err = certFile.WriteString(f.getSecret(eiriniSysNs, eiriniTLSSecretName, "tls.crt"))
	Expect(err).NotTo(HaveOccurred())

	keyFile, err := ioutil.TempFile("", "key-")
	Expect(err).NotTo(HaveOccurred())

	defer keyFile.Close()

	_, err = keyFile.WriteString(f.getSecret(eiriniSysNs, eiriniTLSSecretName, "tls.key"))
	Expect(err).NotTo(HaveOccurred())

	return certFile.Name(), keyFile.Name()
}

func (f *EATSFixture) getSecret(namespace, secretName, secretPath string) string {
	secret, err := f.Clientset.CoreV1().Secrets(namespace).Get(context.Background(), secretName, metav1.GetOptions{})
	Expect(err).NotTo(HaveOccurred())

	return string(secret.Data[secretPath])
}

func (f *EATSFixture) GetEiriniWorkloadsNamespace() string {
	cm, err := f.Clientset.CoreV1().ConfigMaps(GetEiriniSystemNamespace()).Get(context.Background(), "eirini", metav1.GetOptions{})
	Expect(err).NotTo(HaveOccurred())

	opiYml := cm.Data["opi.yml"]
	config := eirini.Config{}

	Expect(yaml.Unmarshal([]byte(opiYml), &config)).To(Succeed())

	return config.Properties.DefaultWorkloadsNamespace
}

func NewWiremock() *wiremock.Wiremock {
	// We assume wiremock is exposed using a ClusterIP service which listens on port 80
	wireMockHost := fmt.Sprintf("cc-wiremock.%s.svc.cluster.local", GetEiriniSystemNamespace())

	RetryResolveHost(wireMockHost, "Is wiremock running in the cluster?")

	return wiremock.New(fmt.Sprintf("http://%s", wireMockHost))
}
