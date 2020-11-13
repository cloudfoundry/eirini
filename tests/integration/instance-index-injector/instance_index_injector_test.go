package instance_index_injector_test

import (
	"context"
	"crypto/tls"
	"fmt"
	"net/http"
	"os"

	"code.cloudfoundry.org/eirini"
	"code.cloudfoundry.org/eirini/k8s"
	"code.cloudfoundry.org/eirini/tests"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = Describe("InstanceIndexInjector", func() {
	var (
		config         *eirini.InstanceIndexEnvInjectorConfig
		configFilePath string
		hookSession    *gexec.Session
		pod            *corev1.Pod
		fingerprint    string
	)

	BeforeEach(func() {
		port := tests.GetTelepresencePort()
		telepresenceService := tests.GetTelepresenceServiceName()
		fingerprint = "instance-id-" + tests.GenerateGUID()[:8]

		config = &eirini.InstanceIndexEnvInjectorConfig{
			ServiceName:                telepresenceService,
			ServicePort:                int32(port),
			ServiceNamespace:           "default",
			EiriniXOperatorFingerprint: fingerprint,
			WorkloadsNamespace:         fixture.Namespace,
			KubeConfig: eirini.KubeConfig{
				ConfigPath: fixture.KubeConfigPath,
			},
		}
		hookSession, configFilePath = eiriniBins.InstanceIndexEnvInjector.Run(config)

		tr := &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true}, //nolint:gosec
		}
		client := &http.Client{Transport: tr}
		Eventually(func() (int, error) {
			resp, err := client.Get(fmt.Sprintf("https://%s.default.svc:%d/0", telepresenceService, port))
			if err != nil {
				printMessage(fmt.Sprintf("failed to call telepresence: %s" + err.Error()))

				return 0, err
			}
			resp.Body.Close()

			return resp.StatusCode, nil
		}, "2m", "500ms").Should(Equal(http.StatusOK))

		pod = &corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name: "app-name-0",
				Labels: map[string]string{
					k8s.LabelSourceType: "APP",
				},
			},
			Spec: corev1.PodSpec{
				Containers: []corev1.Container{
					{
						Name:  k8s.OPIContainerName,
						Image: "eirini/dorini",
					},
					{
						Name:  "not-opi",
						Image: "eirini/dorini",
					},
				},
			},
		}
	})

	AfterEach(func() {
		if configFilePath != "" {
			Expect(os.Remove(configFilePath)).To(Succeed())
		}
		if hookSession != nil {
			Eventually(hookSession.Kill()).Should(gexec.Exit())
		}
		err := fixture.Clientset.AdmissionregistrationV1().MutatingWebhookConfigurations().Delete(context.Background(), fingerprint+"-mutating-hook", metav1.DeleteOptions{})
		Expect(err).NotTo(HaveOccurred())
		err = fixture.Clientset.CoreV1().Secrets("default").Delete(context.Background(), fingerprint+"-setupcertificate", metav1.DeleteOptions{})
		Expect(err).NotTo(HaveOccurred())
	})

	JustBeforeEach(func() {
		var err error
		pod, err = fixture.Clientset.CoreV1().Pods(fixture.Namespace).Create(context.Background(), pod, metav1.CreateOptions{})
		Expect(err).NotTo(HaveOccurred())
	})

	getCFInstanceIndex := func(pod *corev1.Pod, containerName string) string {
		for _, container := range pod.Spec.Containers {
			if container.Name != containerName {
				continue
			}

			for _, e := range container.Env {
				if e.Name != eirini.EnvCFInstanceIndex {
					continue
				}

				return e.Value
			}
		}

		return ""
	}

	It("sets CF_INSTANCE_INDEX in the opi container environment", func() {
		Expect(getCFInstanceIndex(pod, k8s.OPIContainerName)).To(Equal("0"))
	})

	It("does not set CF_INSTANCE_INDEX on the non-opi container", func() {
		Expect(getCFInstanceIndex(pod, "not-opi")).To(Equal(""))
	})
})

func printMessage(str string) {
	_, err := GinkgoWriter.Write([]byte(str))
	Expect(err).NotTo(HaveOccurred())
}
