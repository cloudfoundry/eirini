package tests

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"io"
	"io/ioutil"
	"net/http"
	"os"

	"code.cloudfoundry.org/cfhttp/v2"
	"code.cloudfoundry.org/eirini"

	// nolint:golint,stylecheck
	. "github.com/onsi/gomega"
	"gopkg.in/yaml.v2"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type EATSFixture struct {
	Fixture

	eiriniCertPath   string
	eiriniKeyPath    string
	eiriniHTTPClient *http.Client
}

func NewEATSFixture(writer io.Writer) *EATSFixture {
	return &EATSFixture{
		Fixture: *NewFixture(writer),
	}
}

func (f *EATSFixture) TearDown() {
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
		MinVersion:   tls.VersionTLS13,
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

	return config.Properties.Namespace
}
