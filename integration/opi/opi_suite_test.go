package opi_test

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io/ioutil"
	"math/rand"
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

	corev1 "k8s.io/api/core/v1"
	policyv1 "k8s.io/api/policy/v1beta1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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
	pathToOpi    string
	lastPortUsed int
	portLock     sync.Mutex
	once         sync.Once
	namespace    string
	clientset    kubernetes.Interface
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
	namespace = fmt.Sprintf("opi-integration-test-%d", rand.Intn(100000000))

	for namespaceExists(namespace) {
		namespace = fmt.Sprintf("opi-integration-test-%d", rand.Intn(100000000))
	}
	createNamespace(namespace)
	createPSPForPodCreation(namespace)
	createSecret(namespace)
})

var _ = AfterEach(func() {
	err := clientset.CoreV1().Namespaces().Delete(namespace, &metav1.DeleteOptions{})
	Expect(err).ToNot(HaveOccurred())

	pspName := fmt.Sprintf("%s-psp", namespace)
	err = clientset.PolicyV1beta1().PodSecurityPolicies().Delete(pspName, &metav1.DeleteOptions{})
	Expect(err).ToNot(HaveOccurred())
})

func namespaceExists(namespace string) bool {
	_, err := clientset.CoreV1().Namespaces().Get(namespace, metav1.GetOptions{})
	return err == nil
}

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
			CCCAPath:          pathToTestFixture("cert"),
			CCCertPath:        pathToTestFixture("cert"),
			CCKeyPath:         pathToTestFixture("key"),
			ServerCertPath:    pathToTestFixture("cert"),
			ServerKeyPath:     pathToTestFixture("key"),
			ClientCAPath:      pathToTestFixture("cert"),
			TLSPort:           int(nextAvailPort()),
			CCCertsSecretName: secretName,

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

	clientCert, err := tls.LoadX509KeyPair(pathToTestFixture("cert"), pathToTestFixture("key"))
	Expect(err).ToNot(HaveOccurred())
	certPool := x509.NewCertPool()
	Expect(certPool.AppendCertsFromPEM(bs)).To(BeTrue())
	tlsConfig := &tls.Config{
		RootCAs:      certPool,
		Certificates: []tls.Certificate{clientCert},
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

func createNamespace(namespace string) {
	namespaceSpec := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: namespace}}

	if _, err := clientset.CoreV1().Namespaces().Create(namespaceSpec); err != nil {
		panic(err)
	}
}

// aka allowPodCreation
func createPSPForPodCreation(namespace string) {
	pspName := fmt.Sprintf("%s-psp", namespace)
	roleName := "use-psp"

	_, err := clientset.PolicyV1beta1().PodSecurityPolicies().Create(&policyv1.PodSecurityPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name: pspName,
			Annotations: map[string]string{
				"seccomp.security.alpha.kubernetes.io/allowedProfileNames": "runtime/default",
				"seccomp.security.alpha.kubernetes.io/defaultProfileName":  "runtime/default",
			},
		},
		Spec: policyv1.PodSecurityPolicySpec{
			Privileged: false,
			RunAsUser: policyv1.RunAsUserStrategyOptions{
				Rule: policyv1.RunAsUserStrategyRunAsAny,
			},
			SELinux: policyv1.SELinuxStrategyOptions{
				Rule: policyv1.SELinuxStrategyRunAsAny,
			},
			SupplementalGroups: policyv1.SupplementalGroupsStrategyOptions{
				Rule: policyv1.SupplementalGroupsStrategyMustRunAs,
				Ranges: []policyv1.IDRange{{
					Min: 1,
					Max: 65535,
				}},
			},
			FSGroup: policyv1.FSGroupStrategyOptions{
				Rule: policyv1.FSGroupStrategyMustRunAs,
				Ranges: []policyv1.IDRange{{
					Min: 1,
					Max: 65535,
				}},
			},
		},
	})
	if err != nil {
		panic(err)
	}

	_, err = clientset.RbacV1().Roles(namespace).Create(&rbacv1.Role{
		ObjectMeta: metav1.ObjectMeta{
			Name:      roleName,
			Namespace: namespace,
		},
		Rules: []rbacv1.PolicyRule{
			{
				APIGroups:     []string{"policy"},
				Resources:     []string{"podsecuritypolicies"},
				ResourceNames: []string{pspName},
				Verbs:         []string{"use"},
			},
		},
	})
	if err != nil {
		panic(err)
	}

	_, err = clientset.RbacV1().RoleBindings(namespace).Create(&rbacv1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "default-account-psp",
			Namespace: namespace,
		},
		RoleRef: rbacv1.RoleRef{
			APIGroup: "rbac.authorization.k8s.io",
			Kind:     "Role",
			Name:     roleName,
		},
		Subjects: []rbacv1.Subject{{
			Kind:      rbacv1.ServiceAccountKind,
			Name:      "default",
			Namespace: namespace,
		}},
	})
	if err != nil {
		panic(err)
	}
}

func createSecret(namespace string) {
	_, err := clientset.CoreV1().Secrets(namespace).Create(&corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      secretName,
			Namespace: namespace,
		},
	})
	Expect(err).NotTo(HaveOccurred())
}
