package instance_index_injector_test

import (
	"context"
	"crypto/tls"
	"fmt"
	"net/http"
	"os"

	"code.cloudfoundry.org/eirini"
	"code.cloudfoundry.org/eirini/k8s/stset"
	"code.cloudfoundry.org/eirini/tests"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"
	arv1 "k8s.io/api/admissionregistration/v1"
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
		certDir        string
	)

	BeforeEach(func() {
		port := int32(tests.GetTelepresencePort())
		telepresenceService := tests.GetTelepresenceServiceName()
		telepresenceDomain := fmt.Sprintf("%s.default.svc", telepresenceService)
		fingerprint = "instance-id-" + tests.GenerateGUID()[:8]
		var caBundle []byte
		certDir, caBundle = tests.GenerateKeyPairDir("tls", telepresenceDomain)
		sideEffects := arv1.SideEffectClassNone
		scope := arv1.NamespacedScope

		_, err := fixture.Clientset.AdmissionregistrationV1().MutatingWebhookConfigurations().Create(context.Background(),
			&arv1.MutatingWebhookConfiguration{
				ObjectMeta: metav1.ObjectMeta{
					Name: fingerprint + "-mutating-hook",
				},
				Webhooks: []arv1.MutatingWebhook{
					{
						Name: fingerprint + "-mutating-webhook.cloudfoundry.org",
						ClientConfig: arv1.WebhookClientConfig{
							Service: &arv1.ServiceReference{
								Namespace: "default",
								Name:      telepresenceService,
								Port:      &port,
							},
							CABundle: caBundle,
						},
						SideEffects:             &sideEffects,
						AdmissionReviewVersions: []string{"v1beta1"},
						Rules: []arv1.RuleWithOperations{
							{
								Operations: []arv1.OperationType{arv1.Create},
								Rule: arv1.Rule{
									APIGroups:   []string{""},
									APIVersions: []string{"v1"},
									Resources:   []string{"pods"},
									Scope:       &scope,
								},
							},
						},
					},
				},
			}, metav1.CreateOptions{})
		Expect(err).NotTo(HaveOccurred())

		config = &eirini.InstanceIndexEnvInjectorConfig{
			Port: port,
			KubeConfig: eirini.KubeConfig{
				ConfigPath: fixture.KubeConfigPath,
			},
		}
		env := fmt.Sprintf("%s=%s", eirini.EnvInstanceEnvInjectorCertDir, certDir)
		hookSession, configFilePath = eiriniBins.InstanceIndexEnvInjector.Run(config, env)

		tr := &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true}, //nolint:gosec
		}
		client := &http.Client{Transport: tr}
		Eventually(func() (int, error) {
			resp, err := client.Get(fmt.Sprintf("https://%s:%d/", telepresenceDomain, port))
			if err != nil {
				printMessage(fmt.Sprintf("failed to call telepresence: %s", err.Error()))

				return 0, err
			}
			resp.Body.Close()

			return resp.StatusCode, nil
		}, "2m", "500ms").Should(Equal(http.StatusOK))

		pod = &corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name: "app-name-0",
				Labels: map[string]string{
					stset.LabelSourceType: "APP",
				},
			},
			Spec: corev1.PodSpec{
				Containers: []corev1.Container{
					{
						Name:  stset.OPIContainerName,
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

		Expect(os.RemoveAll(certDir)).To(Succeed())
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
		Eventually(func() string { return getCFInstanceIndex(pod, stset.OPIContainerName) }).Should(Equal("0"))
	})

	It("does not set CF_INSTANCE_INDEX on the non-opi container", func() {
		Expect(getCFInstanceIndex(pod, "not-opi")).To(Equal(""))
	})
})

func printMessage(str string) {
	_, err := GinkgoWriter.Write([]byte(str))
	Expect(err).NotTo(HaveOccurred())
}
