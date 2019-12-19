package util

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io/ioutil"
	"math/rand"
	"net/http"
	"os"

	"code.cloudfoundry.org/cfhttp/v2"
	"code.cloudfoundry.org/eirini"
	ginkgoconfig "github.com/onsi/ginkgo/config"
	"gopkg.in/yaml.v2"
	corev1 "k8s.io/api/core/v1"
	policyv1 "k8s.io/api/policy/v1beta1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

func CreateRandomNamespace(clientset kubernetes.Interface) string {
	namespace := fmt.Sprintf("opi-integration-test-%d-%d", rand.Intn(100000000), ginkgoconfig.GinkgoConfig.ParallelNode)
	for namespaceExists(namespace, clientset) {
		namespace = fmt.Sprintf("opi-integration-test-%d-%d", rand.Intn(100000000), ginkgoconfig.GinkgoConfig.ParallelNode)
	}
	createNamespace(namespace, clientset)
	return namespace
}

func namespaceExists(namespace string, clientset kubernetes.Interface) bool {
	_, err := clientset.CoreV1().Namespaces().Get(namespace, metav1.GetOptions{})
	return err == nil
}

func createNamespace(namespace string, clientset kubernetes.Interface) {
	namespaceSpec := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: namespace}}

	if _, err := clientset.CoreV1().Namespaces().Create(namespaceSpec); err != nil {
		panic(err)
	}
}

func CreatePodCreationPSP(namespace, pspName string, clientset kubernetes.Interface) error {
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
		return err
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
		return err
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
	return err
}

func CreateEmptySecret(namespace, secretName string, clientset kubernetes.Interface) error {
	_, err := clientset.CoreV1().Secrets(namespace).Create(&corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      secretName,
			Namespace: namespace,
		},
	})
	return err
}

func DeleteNamespace(namespace string, clientset kubernetes.Interface) error {
	return clientset.CoreV1().Namespaces().Delete(namespace, &metav1.DeleteOptions{})
}

func DeletePSP(name string, clientset kubernetes.Interface) error {
	return clientset.PolicyV1beta1().PodSecurityPolicies().Delete(name, &metav1.DeleteOptions{})
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

func DefaultEiriniConfig(namespace, secretName string) *eirini.Config {
	return &eirini.Config{
		Properties: eirini.Properties{
			KubeConfig: eirini.KubeConfig{
				ConfigPath: os.Getenv("INTEGRATION_KUBECONFIG"),
				Namespace:  namespace,
			},
			CCCAPath:          PathToTestFixture("cert"),
			CCCertPath:        PathToTestFixture("cert"),
			CCKeyPath:         PathToTestFixture("key"),
			ServerCertPath:    PathToTestFixture("cert"),
			ServerKeyPath:     PathToTestFixture("key"),
			ClientCAPath:      PathToTestFixture("cert"),
			TLSPort:           61000 + rand.Intn(1000) + ginkgoconfig.GinkgoConfig.ParallelNode,
			CCCertsSecretName: secretName,

			DownloaderImage: "docker.io/eirini/integration_test_staging",
			ExecutorImage:   "docker.io/eirini/integration_test_staging",
			UploaderImage:   "docker.io/eirini/integration_test_staging",
		},
	}
}

func CreateOpiConfigFromFixtures(config *eirini.Config) (*os.File, error) {
	bs, err := yaml.Marshal(config)
	if err != nil {
		return nil, err
	}

	return createConfigFile(bs)
}

func createConfigFile(yamlBytes []byte) (*os.File, error) {
	configFile, err := ioutil.TempFile("", "config.yml")
	if err != nil {
		return nil, err
	}

	err = ioutil.WriteFile(configFile.Name(), yamlBytes, os.ModePerm)
	return configFile, err
}

func PathToTestFixture(relativePath string) string {
	cwd, err := os.Getwd()
	if err != nil {
		panic(err)
	}
	return fmt.Sprintf("%s/../fixtures/%s", cwd, relativePath)
}
