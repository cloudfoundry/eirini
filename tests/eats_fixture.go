package tests

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"net/http"
	"os"

	"code.cloudfoundry.org/cfhttp/v2"
	"code.cloudfoundry.org/eirini"
	"code.cloudfoundry.org/eirini/tests/eats/wiremock"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"gopkg.in/yaml.v2"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
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
	f.Fixture.SetUp()
	CopyRolesAndBindings(f.Namespace, f.Fixture.Clientset)
}

func (f *EATSFixture) TearDown() {
	if f == nil {
		Fail("failed to initialize fixture")

		return
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
	bs, err := os.ReadFile(f.eiriniCertPath)
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
	certFile, err := os.CreateTemp("", "cert-")
	Expect(err).NotTo(HaveOccurred())

	defer certFile.Close()

	eiriniSysNs := GetEiriniSystemNamespace()
	eiriniTLSSecretName := getEiriniTLSSecretName()

	_, err = certFile.WriteString(f.getSecret(eiriniSysNs, eiriniTLSSecretName, "tls.crt"))
	Expect(err).NotTo(HaveOccurred())

	keyFile, err := os.CreateTemp("", "key-")
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
	cm, err := f.Clientset.CoreV1().ConfigMaps(GetEiriniSystemNamespace()).Get(context.Background(), "api", metav1.GetOptions{})
	Expect(err).NotTo(HaveOccurred())

	apiYml := cm.Data["api.yml"]
	config := eirini.APIConfig{}

	Expect(yaml.Unmarshal([]byte(apiYml), &config)).To(Succeed())

	return config.DefaultWorkloadsNamespace
}

func (f *EATSFixture) GetNATSPassword() string {
	secret, err := f.Clientset.CoreV1().Secrets(GetEiriniSystemNamespace()).Get(context.Background(), "nats-secret", metav1.GetOptions{})
	Expect(err).NotTo(HaveOccurred())

	return string(secret.Data["nats-password"])
}

func NewWiremock() *wiremock.Wiremock {
	// We assume wiremock is exposed using a ClusterIP service which listens on port 80
	wireMockHost := fmt.Sprintf("cc-wiremock.%s.svc.cluster.local", GetEiriniSystemNamespace())

	RetryResolveHost(wireMockHost, "Is wiremock running in the cluster?")

	return wiremock.New(wireMockHost)
}

func CopyRolesAndBindings(namespace string, clientset kubernetes.Interface) {
	from := GetEiriniWorkloadsNamespace()

	roleList, err := clientset.RbacV1().Roles(from).List(context.Background(), metav1.ListOptions{})
	Expect(err).NotTo(HaveOccurred())

	for _, role := range roleList.Items {
		newRole := new(rbacv1.Role)
		newRole.Namespace = namespace
		newRole.Name = role.Name
		newRole.Rules = role.Rules
		_, err = clientset.RbacV1().Roles(namespace).Create(context.Background(), newRole, metav1.CreateOptions{})
		Expect(err).NotTo(HaveOccurred())
	}

	bindingList, err := clientset.RbacV1().RoleBindings(from).List(context.Background(), metav1.ListOptions{})
	Expect(err).NotTo(HaveOccurred())

	for _, binding := range bindingList.Items {
		newBinding := new(rbacv1.RoleBinding)
		newBinding.Namespace = namespace
		newBinding.Name = binding.Name
		newBinding.Subjects = binding.Subjects

		if binding.Name == "eirini-workloads-app-rolebinding" {
			newBinding.Subjects[0].Namespace = namespace
		}

		newBinding.RoleRef = binding.RoleRef
		_, err := clientset.RbacV1().RoleBindings(namespace).Create(context.Background(), newBinding, metav1.CreateOptions{})
		Expect(err).NotTo(HaveOccurred())
	}
}
