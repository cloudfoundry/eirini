package api_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"code.cloudfoundry.org/eirini/k8s/utils/dockerutils"
	"code.cloudfoundry.org/eirini/models/cf"
	"code.cloudfoundry.org/eirini/tests"
	"code.cloudfoundry.org/eirini/tests/integration"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = Describe("Desire App", func() {
	var (
		lrp      cf.DesireLRPRequest
		response *http.Response
	)

	isStatefulSetReady := func() bool {
		stset := integration.GetStatefulSet(fixture.Clientset, fixture.Namespace, lrp.GUID, lrp.Version)

		return stset.Status.ReadyReplicas == *stset.Spec.Replicas
	}

	BeforeEach(func() {
		lrp = cf.DesireLRPRequest{
			GUID:         "the-app-guid",
			Version:      "0.0.0",
			Namespace:    fixture.Namespace,
			Ports:        []int32{8080},
			NumInstances: 1,
			DiskMB:       512,
			Lifecycle: cf.Lifecycle{
				DockerLifecycle: &cf.DockerLifecycle{
					Image:   "eirini/busybox",
					Command: []string{"/bin/sleep", "100"},
				},
			},
		}
	})

	JustBeforeEach(func() {
		lrpRequestBytes, err := json.Marshal(lrp)
		Expect(err).NotTo(HaveOccurred())

		desireAppReq, err := http.NewRequest("PUT", fmt.Sprintf("%s/apps/the-app-guid", eiriniAPIUrl), bytes.NewReader(lrpRequestBytes))
		Expect(err).NotTo(HaveOccurred())

		response, err = httpClient.Do(desireAppReq)
		Expect(err).NotTo(HaveOccurred())
	})

	It("should return a 202 Accepted HTTP code", func() {
		Expect(response.StatusCode).To(Equal(http.StatusAccepted))
	})

	It("successfully runs the lrp", func() {
		Eventually(isStatefulSetReady).Should(BeTrue())
	})

	When("disk_mb is specified as 0", func() {
		BeforeEach(func() {
			lrp.DiskMB = 0
		})

		It("should return a 400 Bad Request HTTP code", func() {
			Expect(response.StatusCode).To(Equal(http.StatusBadRequest))
		})
	})

	When("no app namespace is explicitly requested", func() {
		BeforeEach(func() {
			lrp.Namespace = ""
		})

		It("creates create the app in the default namespace", func() {
			statefulsets, err := fixture.Clientset.AppsV1().StatefulSets(fixture.Namespace).List(context.Background(), metav1.ListOptions{})
			Expect(err).ToNot(HaveOccurred())

			Expect(statefulsets.Items).To(HaveLen(1))
			Expect(statefulsets.Items[0].Name).To(ContainSubstring("the-app-guid"))
		})
	})

	When("a registry secret name is configured", func() {
		BeforeEach(func() {
			generateRegistryCredsSecret("registry-secret", "https://index.docker.io/v1/", "eiriniuser", tests.GetEiriniDockerHubPassword())
			apiConfig.RegistrySecretName = "registry-secret"
			lrp.Lifecycle.DockerLifecycle.Image = "eiriniuser/notdora"
		})

		It("should return a 202 Accepted HTTP code", func() {
			Expect(response.StatusCode).To(Equal(http.StatusAccepted))
		})

		It("can desire apps that use private images from that registry", func() {
			statefulsets, err := fixture.Clientset.AppsV1().StatefulSets(fixture.Namespace).List(context.Background(), metav1.ListOptions{})
			Expect(err).ToNot(HaveOccurred())

			Expect(statefulsets.Items).To(HaveLen(1))
			Expect(statefulsets.Items[0].Name).To(ContainSubstring("the-app-guid"))
			Expect(statefulsets.Items[0].Spec.Template.Spec.ImagePullSecrets).To(ConsistOf(corev1.LocalObjectReference{Name: "registry-secret"}))
		})
	})

	When("AllowRunImageAsRoot is true", func() {
		BeforeEach(func() {
			lrp.Lifecycle.DockerLifecycle.Image = "eirini/busybox-root"
			apiConfig.AllowRunImageAsRoot = true
		})

		It("successfully runs the lrp", func() {
			Eventually(isStatefulSetReady).Should(BeTrue())
		})
	})

	Describe("automounting serviceacccount token", func() {
		const serviceAccountTokenMountPath = "/var/run/secrets/kubernetes.io/serviceaccount"
		var serviceName string

		BeforeEach(func() {
			lrp.Lifecycle.DockerLifecycle = &cf.DockerLifecycle{
				Image: "eirini/dorini",
			}
		})

		JustBeforeEach(func() {
			serviceName = tests.ExposeAsService(fixture.Clientset, fixture.Namespace, lrp.GUID, 8080, "/")
		})

		It("does not mount the service account token", func() {
			result, err := tests.RequestServiceFn(fixture.Namespace, serviceName, 8080, fmt.Sprintf("/ls?path=%s", serviceAccountTokenMountPath))()
			Expect(err).To(MatchError(ContainSubstring("Internal Server Error")))
			Expect(result).To(ContainSubstring("no such file or directory"))
		})

		When("unsafe_allow_automount_service_account_token is set", func() {
			BeforeEach(func() {
				apiConfig.UnsafeAllowAutomountServiceAccountToken = true
			})

			It("mounts the service account token (because this is how K8S works by default)", func() {
				_, err := tests.RequestServiceFn(fixture.Namespace, serviceName, 8080, fmt.Sprintf("/ls?path=%s", serviceAccountTokenMountPath))()
				Expect(err).NotTo(HaveOccurred())
			})

			When("the app service account has its automountServiceAccountToken set to false", func() {
				updateServiceaccount := func() error {
					appServiceAccount, err := fixture.Clientset.CoreV1().ServiceAccounts(fixture.Namespace).Get(context.Background(), tests.GetApplicationServiceAccount(), metav1.GetOptions{})
					Expect(err).NotTo(HaveOccurred())
					automountServiceAccountToken := false
					appServiceAccount.AutomountServiceAccountToken = &automountServiceAccountToken
					_, err = fixture.Clientset.CoreV1().ServiceAccounts(fixture.Namespace).Update(context.Background(), appServiceAccount, metav1.UpdateOptions{})

					return err
				}

				BeforeEach(func() {
					Eventually(updateServiceaccount, "5s").Should(Succeed())
				})

				It("does not mount the service account token", func() {
					result, err := tests.RequestServiceFn(fixture.Namespace, serviceName, 8080, fmt.Sprintf("/ls?path=%s", serviceAccountTokenMountPath))()
					Expect(err).To(MatchError(ContainSubstring("Internal Server Error")))
					Expect(result).To(ContainSubstring("no such file or directory"))
				})
			})
		})
	})
})

func generateRegistryCredsSecret(name, server, username, password string) {
	dockerConfig := dockerutils.NewDockerConfig(server, username, password)

	dockerConfigJSON, err := dockerConfig.JSON()
	Expect(err).NotTo(HaveOccurred())

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		Type: corev1.SecretTypeDockerConfigJson,
		StringData: map[string]string{
			dockerutils.DockerConfigKey: dockerConfigJSON,
		},
	}
	_, err = fixture.Clientset.CoreV1().Secrets(fixture.Namespace).Create(context.Background(), secret, metav1.CreateOptions{})
	Expect(err).NotTo(HaveOccurred())
}
