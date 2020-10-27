package tests

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io/ioutil"
	"math/rand"
	"net/http"
	"os"

	"code.cloudfoundry.org/cfhttp/v2"
	"code.cloudfoundry.org/eirini"
	"code.cloudfoundry.org/tlsconfig"
	ginkgoconfig "github.com/onsi/ginkgo/config"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/ghttp"
	"gopkg.in/yaml.v2"
	corev1 "k8s.io/api/core/v1"
	policyv1 "k8s.io/api/policy/v1beta1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

const randUpperBound = 100000000

func CreateRandomNamespace(clientset kubernetes.Interface) string {
	namespace := fmt.Sprintf("opi-integration-test-%d-%d", rand.Intn(randUpperBound), ginkgoconfig.GinkgoConfig.ParallelNode)
	for namespaceExists(namespace, clientset) {
		namespace = fmt.Sprintf("opi-integration-test-%d-%d", rand.Intn(randUpperBound), ginkgoconfig.GinkgoConfig.ParallelNode)
	}
	createNamespace(namespace, clientset)

	return namespace
}

func namespaceExists(namespace string, clientset kubernetes.Interface) bool {
	_, err := clientset.CoreV1().Namespaces().Get(context.Background(), namespace, metav1.GetOptions{})

	return err == nil
}

func createNamespace(namespace string, clientset kubernetes.Interface) {
	namespaceSpec := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: namespace}}

	_, err := clientset.CoreV1().Namespaces().Create(context.Background(), namespaceSpec, metav1.CreateOptions{})
	Expect(err).NotTo(HaveOccurred())
}

func CreatePodCreationPSP(namespace, pspName, serviceAccountName string, clientset kubernetes.Interface) error {
	_, err := clientset.PolicyV1beta1().PodSecurityPolicies().Create(context.Background(), &policyv1.PodSecurityPolicy{
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
				Rule: policyv1.SupplementalGroupsStrategyRunAsAny,
			},
			FSGroup: policyv1.FSGroupStrategyOptions{
				Rule: policyv1.FSGroupStrategyRunAsAny,
			},
			Volumes: []policyv1.FSType{
				policyv1.EmptyDir, policyv1.Projected, policyv1.Secret,
			},
		},
	}, metav1.CreateOptions{})
	if err != nil {
		return err
	}

	roleName := "use-psp"
	_, err = clientset.RbacV1().Roles(namespace).Create(context.Background(), &rbacv1.Role{
		ObjectMeta: metav1.ObjectMeta{
			Name:      roleName,
			Namespace: namespace,
		},
		Rules: []rbacv1.PolicyRule{
			{
				APIGroups:     []string{"policy"},
				Resources:     []string{"podsecuritypolicies"},
				Verbs:         []string{"use"},
				ResourceNames: []string{pspName},
			},
		},
	}, metav1.CreateOptions{})

	if err != nil {
		return err
	}

	_, err = clientset.CoreV1().ServiceAccounts(namespace).Create(context.Background(), &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      serviceAccountName,
			Namespace: namespace,
		},
	}, metav1.CreateOptions{})
	if err != nil {
		return err
	}

	_, err = clientset.RbacV1().RoleBindings(namespace).Create(context.Background(), &rbacv1.RoleBinding{
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
			Name:      serviceAccountName,
			Namespace: namespace,
		}},
	}, metav1.CreateOptions{})

	return err
}

func CreateEmptySecret(namespace, secretName string, clientset kubernetes.Interface) error {
	_, err := clientset.CoreV1().Secrets(namespace).Create(context.Background(), &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      secretName,
			Namespace: namespace,
		},
	}, metav1.CreateOptions{})

	return err
}

func CreateSecretWithStringData(namespace, secretName string, clientset kubernetes.Interface, stringData map[string]string) error {
	_, err := clientset.CoreV1().Secrets(namespace).Create(context.Background(), &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      secretName,
			Namespace: namespace,
		},
		StringData: stringData,
	}, metav1.CreateOptions{})

	return err
}

func DeleteNamespace(namespace string, clientset kubernetes.Interface) error {
	return clientset.CoreV1().Namespaces().Delete(context.Background(), namespace, metav1.DeleteOptions{})
}

func DeletePSP(name string, clientset kubernetes.Interface) error {
	return clientset.PolicyV1beta1().PodSecurityPolicies().Delete(context.Background(), name, metav1.DeleteOptions{})
}

func MakeTestHTTPClient() (*http.Client, error) {
	bs, err := ioutil.ReadFile(PathToTestFixture("cert"))
	if err != nil {
		return nil, err
	}

	clientCert, err := tls.LoadX509KeyPair(PathToTestFixture("cert"), PathToTestFixture("key"))
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

func DefaultEiriniConfig(namespace string, tlsPort int) *eirini.Config {
	return &eirini.Config{
		Properties: eirini.Properties{
			KubeConfig: eirini.KubeConfig{
				ConfigPath:                  GetKubeconfig(),
				Namespace:                   namespace,
				EnableMultiNamespaceSupport: false,
			},
			CCCAPath:       PathToTestFixture("cert"),
			CCCertPath:     PathToTestFixture("cert"),
			CCKeyPath:      PathToTestFixture("key"),
			ServerCertPath: PathToTestFixture("cert"),
			ServerKeyPath:  PathToTestFixture("key"),
			ClientCAPath:   PathToTestFixture("cert"),
			TLSPort:        tlsPort,

			DownloaderImage: "docker.io/eirini/integration_test_staging",
			ExecutorImage:   "docker.io/eirini/integration_test_staging",
			UploaderImage:   "docker.io/eirini/integration_test_staging",

			ApplicationServiceAccount: GetApplicationServiceAccount(),
			StagingServiceAccount:     "staging",
			RegistryAddress:           "registry",
			RegistrySecretName:        "registry-secret",
		},
	}
}

func CreateConfigFile(config interface{}) (*os.File, error) {
	yamlBytes, err := yaml.Marshal(config)
	if err != nil {
		return nil, err
	}

	configFile, err := ioutil.TempFile("", "config.yml")
	if err != nil {
		return nil, err
	}

	err = ioutil.WriteFile(configFile.Name(), yamlBytes, os.ModePerm)

	return configFile, err
}

func PathToTestFixture(relativePath string) string {
	cwd, err := os.Getwd()
	Expect(err).NotTo(HaveOccurred())

	return fmt.Sprintf("%s/../fixtures/%s", cwd, relativePath)
}

func CreateTestServer(certPath, keyPath, caCertPath string) (*ghttp.Server, error) {
	tlsConf, err := tlsconfig.Build(
		tlsconfig.WithInternalServiceDefaults(),
		tlsconfig.WithIdentityFromFile(certPath, keyPath),
	).Server(
		tlsconfig.WithClientAuthenticationFromFile(caCertPath),
	)
	if err != nil {
		return nil, err
	}

	testServer := ghttp.NewUnstartedServer()
	testServer.HTTPTestServer.TLS = tlsConf

	return testServer, nil
}
