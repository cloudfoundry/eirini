package integration

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"code.cloudfoundry.org/cfhttp/v2"
	"code.cloudfoundry.org/eirini"
	"code.cloudfoundry.org/eirini/k8s/jobs"
	"code.cloudfoundry.org/eirini/k8s/stset"
	eiriniv1 "code.cloudfoundry.org/eirini/pkg/apis/eirini/v1"
	eiriniclient "code.cloudfoundry.org/eirini/pkg/generated/clientset/versioned"
	"code.cloudfoundry.org/eirini/tests"
	"code.cloudfoundry.org/tlsconfig"
	"github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/ghttp"
	"gopkg.in/yaml.v2"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	policyv1 "k8s.io/api/policy/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

func ListJobs(clientset kubernetes.Interface, namespace, taskGUID string) func() []batchv1.Job {
	return func() []batchv1.Job {
		jobs, err := clientset.BatchV1().
			Jobs(namespace).
			List(context.Background(), metav1.ListOptions{LabelSelector: fmt.Sprintf("%s=%s", jobs.LabelGUID, taskGUID)})

		Expect(err).NotTo(HaveOccurred())

		return jobs.Items
	}
}

func GetTaskJobConditions(clientset kubernetes.Interface, namespace, taskGUID string) func() []batchv1.JobCondition {
	return func() []batchv1.JobCondition {
		jobs := ListJobs(clientset, namespace, taskGUID)()

		return jobs[0].Status.Conditions
	}
}

func GetRegistrySecretName(clientset kubernetes.Interface, namespace, taskGUID, secretName string) string {
	jobs := ListJobs(clientset, namespace, taskGUID)()
	imagePullSecrets := jobs[0].Spec.Template.Spec.ImagePullSecrets

	var registrySecretName string

	for _, imagePullSecret := range imagePullSecrets {
		if strings.HasPrefix(imagePullSecret.Name, secretName) {
			registrySecretName = imagePullSecret.Name
		}
	}

	Expect(registrySecretName).NotTo(BeEmpty())

	return registrySecretName
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

func MakeTestHTTPClient(certsPath string) (*http.Client, error) {
	bs, err := ioutil.ReadFile(filepath.Join(certsPath, "tls.ca"))
	if err != nil {
		return nil, err
	}

	clientCert, err := tls.LoadX509KeyPair(filepath.Join(certsPath, "tls.crt"), filepath.Join(certsPath, "tls.key"))
	if err != nil {
		return nil, err
	}

	certPool := x509.NewCertPool()
	if !certPool.AppendCertsFromPEM(bs) {
		return nil, err
	}

	tlsConfig := &tls.Config{
		MinVersion:   tls.VersionTLS12,
		RootCAs:      certPool,
		Certificates: []tls.Certificate{clientCert},
	}
	httpClient := cfhttp.NewClient(cfhttp.WithTLSConfig(tlsConfig))

	return httpClient, nil
}

func DefaultAPIConfig(namespace string, tlsPort int) *eirini.APIConfig {
	return &eirini.APIConfig{
		CommonConfig: eirini.CommonConfig{
			KubeConfig: eirini.KubeConfig{
				ConfigPath: tests.GetKubeconfig(),
			},

			ApplicationServiceAccount: tests.GetApplicationServiceAccount(),
			RegistrySecretName:        "registry-secret",
			WorkloadsNamespace:        namespace,
		},
		DefaultWorkloadsNamespace: namespace,
		TLSPort:                   tlsPort,
	}
}

func DefaultControllerConfig(namespace string) *eirini.ControllerConfig {
	return &eirini.ControllerConfig{
		CommonConfig: eirini.CommonConfig{
			KubeConfig: eirini.KubeConfig{
				ConfigPath: tests.GetKubeconfig(),
			},
			ApplicationServiceAccount: tests.GetApplicationServiceAccount(),
			RegistrySecretName:        "registry-secret",
			WorkloadsNamespace:        namespace,
		},
		TaskTTLSeconds:          5,
		LeaderElectionID:        fmt.Sprintf("test-eirini-%d", ginkgo.GinkgoParallelNode()),
		LeaderElectionNamespace: namespace,
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

func GetPDBItems(clientset kubernetes.Interface, namespace, lrpGUID, lrpVersion string) ([]policyv1.PodDisruptionBudget, error) {
	pdbList, err := clientset.PolicyV1beta1().PodDisruptionBudgets(namespace).List(context.Background(), metav1.ListOptions{
		LabelSelector: fmt.Sprintf("%s=%s,%s=%s", stset.LabelGUID, lrpGUID, stset.LabelVersion, lrpVersion),
	})
	if err != nil {
		return nil, err
	}

	return pdbList.Items, nil
}

func GetPDB(clientset kubernetes.Interface, namespace, lrpGUID, lrpVersion string) policyv1.PodDisruptionBudget {
	var pdbs []policyv1.PodDisruptionBudget

	Eventually(func() ([]policyv1.PodDisruptionBudget, error) {
		var err error
		pdbs, err = GetPDBItems(clientset, namespace, lrpGUID, lrpVersion)

		return pdbs, err
	}).Should(HaveLen(1))

	Consistently(func() ([]policyv1.PodDisruptionBudget, error) {
		var err error
		pdbs, err = GetPDBItems(clientset, namespace, lrpGUID, lrpVersion)

		return pdbs, err
	}, "5s").Should(HaveLen(1))

	return pdbs[0]
}

func GetStatefulSet(clientset kubernetes.Interface, namespace, guid, version string) *appsv1.StatefulSet {
	appListOpts := metav1.ListOptions{
		LabelSelector: fmt.Sprintf("%s=%s,%s=%s", stset.LabelGUID, guid, stset.LabelVersion, version),
	}

	stsList, err := clientset.
		AppsV1().
		StatefulSets(namespace).
		List(context.Background(), appListOpts)

	Expect(err).NotTo(HaveOccurred())

	if len(stsList.Items) == 0 {
		return nil
	}

	Expect(stsList.Items).To(HaveLen(1))

	return &stsList.Items[0]
}

func GetLRP(clientset eiriniclient.Interface, namespace, lrpName string) *eiriniv1.LRP {
	l, err := clientset.
		EiriniV1().
		LRPs(namespace).
		Get(context.Background(), lrpName, metav1.GetOptions{})

	Expect(err).NotTo(HaveOccurred())

	return l
}

func GetTaskExecutionStatus(clientset eiriniclient.Interface, namespace, taskName string) func() eiriniv1.ExecutionStatus {
	return func() eiriniv1.ExecutionStatus {
		task, err := clientset.
			EiriniV1().
			Tasks(namespace).
			Get(context.Background(), taskName, metav1.GetOptions{})
		Expect(err).NotTo(HaveOccurred())

		return task.Status.ExecutionStatus
	}
}
