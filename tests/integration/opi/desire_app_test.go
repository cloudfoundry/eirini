package opi_test

import (
	"bytes"
	"context"
	"fmt"
	"net/http"

	"code.cloudfoundry.org/eirini/tests"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = Describe("Desire App", func() {
	var (
		body     string
		response *http.Response
	)

	BeforeEach(func() {
		body = `{
			"guid": "the-app-guid",
			"version": "0.0.0",
			"ports" : [8080],
			"disk_mb": 512,
			"lifecycle": {
				"docker_lifecycle": {
				"image": "busybox",
				"command": ["/bin/sleep", "100"]
				}
			},
			"instances": 1
		}`
	})

	JustBeforeEach(func() {
		desireAppReq, err := http.NewRequest("PUT", fmt.Sprintf("%s/apps/the-app-guid", url), bytes.NewReader([]byte(body)))
		Expect(err).NotTo(HaveOccurred())
		response, err = httpClient.Do(desireAppReq)
		Expect(err).NotTo(HaveOccurred())
	})

	It("should return a 202 Accepted HTTP code", func() {
		Expect(response.StatusCode).To(Equal(http.StatusAccepted))
	})

	It("should create a stateful set for the app", func() {
		statefulsets, err := fixture.Clientset.AppsV1().StatefulSets(fixture.Namespace).List(context.Background(), metav1.ListOptions{})
		Expect(err).ToNot(HaveOccurred())

		Expect(statefulsets.Items).To(HaveLen(1))
		Expect(statefulsets.Items[0].Name).To(ContainSubstring("the-app-guid"))
		Expect(statefulsets.Items[0].Spec.Template.Spec.ImagePullSecrets).To(ConsistOf(corev1.LocalObjectReference{Name: "registry-secret"}))
	})

	Context("when the app has user defined annotations", func() {
		BeforeEach(func() {
			body = `{
			"guid": "the-app-guid",
			"version": "0.0.0",
			"ports" : [8080],
			"disk_mb": 400,
		  "lifecycle": {
				"docker_lifecycle": {
				  "image": "foo",
					"command": ["bar", "baz"]
				}
			},
			"user_defined_annotations": {
			  "prometheus.io/scrape": "yes, please"
			}
		}`
		})

		It("should set the annotations to the pod template", func() {
			statefulsets, err := fixture.Clientset.AppsV1().StatefulSets(fixture.Namespace).List(context.Background(), metav1.ListOptions{})
			Expect(err).ToNot(HaveOccurred())
			Expect(statefulsets.Items[0].Spec.Template.Annotations).To(HaveKeyWithValue("prometheus.io/scrape", "yes, please"))
		})
	})

	When("disk_mb isn't specified", func() {
		BeforeEach(func() {
			body = `{
			"guid": "the-app-guid",
			"version": "0.0.0",
			"ports" : [8080],
		  "lifecycle": {
				"docker_lifecycle": {
				  "image": "foo",
					"command": ["bar", "baz"]
				}
			},
			"user_defined_annotations": {
			  "prometheus.io/scrape": "yes, please"
			}
		}`
		})

		It("should return a 400 Bad Request HTTP code", func() {
			Expect(response.StatusCode).To(Equal(http.StatusBadRequest))
		})
	})

	Describe("automounting serviceacccount token", func() {
		const serviceAccountTokenMountPath = "/var/run/secrets/kubernetes.io/serviceaccount" //nolint:gosec
		var podMountPaths []string

		JustBeforeEach(func() {
			Eventually(func() ([]corev1.Pod, error) {
				pods, err := fixture.Clientset.CoreV1().Pods(fixture.Namespace).List(context.Background(), metav1.ListOptions{})
				if err != nil {
					return nil, err
				}

				return pods.Items, nil
			}).ShouldNot(BeEmpty())

			pods, err := fixture.Clientset.CoreV1().Pods(fixture.Namespace).List(context.Background(), metav1.ListOptions{})
			Expect(err).NotTo(HaveOccurred())
			Expect(pods.Items).To(HaveLen(1))

			podMountPaths = []string{}
			for _, podMount := range pods.Items[0].Spec.Containers[0].VolumeMounts {
				podMountPaths = append(podMountPaths, podMount.MountPath)
			}
		})

		It("does not mount the service account token", func() {
			Expect(podMountPaths).NotTo(ContainElement(serviceAccountTokenMountPath))
		})

		When("unsafe_allow_automount_service_account_token is set", func() {
			BeforeEach(func() {
				eiriniConfig.Properties.UnsafeAllowAutomountServiceAccountToken = true
			})

			It("mounts the service account token (because this is how K8S works by default)", func() {
				Expect(podMountPaths).To(ContainElement(serviceAccountTokenMountPath))
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
					Expect(podMountPaths).NotTo(ContainElement(serviceAccountTokenMountPath))
				})
			})
		})
	})
})
