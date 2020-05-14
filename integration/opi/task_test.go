package opi_test

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"code.cloudfoundry.org/eirini/integration/util"
	"code.cloudfoundry.org/eirini/models/cf"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = Describe("Task Desire and Complete", func() {
	var (
		request cf.TaskRequest
		jobs    *batchv1.JobList
	)

	JustBeforeEach(func() {
		body, err := json.Marshal(request)
		Expect(err).NotTo(HaveOccurred())

		httpRequest, err := http.NewRequest("POST", fmt.Sprintf("%s/tasks/the-task-guid", url), bytes.NewReader(body))
		Expect(err).NotTo(HaveOccurred())

		resp, err := httpClient.Do(httpRequest)
		Expect(err).NotTo(HaveOccurred())
		Expect(resp.StatusCode).To(Equal(http.StatusAccepted))
	})

	Context("buildpack tasks", func() {
		BeforeEach(func() {
			request = cf.TaskRequest{
				AppName:     "my_app",
				SpaceName:   "my_space",
				Environment: []cf.EnvironmentVariable{{Name: "my-env", Value: "my-value"}},
				Lifecycle: cf.Lifecycle{
					BuildpackLifecycle: &cf.BuildpackLifecycle{
						DropletHash:  "foo",
						DropletGUID:  "bar",
						StartCommand: "some command",
					},
				},
			}
		})

		It("should create a valid job for the task", func() {
			Eventually(func() ([]batchv1.Job, error) {
				var err error
				jobs, err = fixture.Clientset.BatchV1().Jobs(fixture.Namespace).List(metav1.ListOptions{})
				return jobs.Items, err
			}).Should(HaveLen(1))

			By("creating a job for the task", func() {
				Expect(jobs.Items).To(HaveLen(1))
				Expect(jobs.Items[0].Name).To(HavePrefix("my-app-my-space-"))
			})

			By("setting the registry secret name", func() {
				podSpec := jobs.Items[0].Spec.Template.Spec
				Expect(podSpec.ImagePullSecrets).To(ConsistOf(corev1.LocalObjectReference{Name: "registry-secret"}))
			})

			By("specifying the right containers", func() {
				jobContainers := jobs.Items[0].Spec.Template.Spec.Containers
				Expect(jobContainers).To(HaveLen(1))
				Expect(jobContainers[0].Env).To(ContainElement(corev1.EnvVar{Name: "my-env", Value: "my-value"}))
				Expect(jobContainers[0].Env).To(ContainElement(corev1.EnvVar{Name: "START_COMMAND", Value: "some command"}))
				Expect(jobContainers[0].Image).To(Equal("registry/cloudfoundry/bar:foo"))
				Expect(jobContainers[0].Command).To(ConsistOf("/lifecycle/launch"))
			})
		})
	})

	Context("docker tasks", func() {
		BeforeEach(func() {
			request = cf.TaskRequest{
				AppName:     "my_app",
				SpaceName:   "my_space",
				Environment: []cf.EnvironmentVariable{{Name: "my-env", Value: "my-value"}},
				Lifecycle: cf.Lifecycle{
					DockerLifecycle: &cf.DockerLifecycle{
						Image:   "busybox",
						Command: []string{"/bin/echo", "hello"},
					},
				},
			}
		})

		It("creates the job successfully", func() {
			Eventually(func() ([]batchv1.Job, error) {
				var err error
				jobs, err = fixture.Clientset.BatchV1().Jobs(fixture.Namespace).List(metav1.ListOptions{})
				return jobs.Items, err
			}).Should(HaveLen(1))

			By("creating a job for the task", func() {
				Expect(jobs.Items).To(HaveLen(1))
				Expect(jobs.Items[0].Name).To(HavePrefix("my-app-my-space-"))
			})

			By("specifying the right containers", func() {
				jobContainers := jobs.Items[0].Spec.Template.Spec.Containers
				Expect(jobContainers).To(HaveLen(1))
				Expect(jobContainers[0].Env).To(ContainElement(corev1.EnvVar{Name: "my-env", Value: "my-value"}))
				Expect(jobContainers[0].Image).To(Equal("busybox"))
				Expect(jobContainers[0].Command).To(ConsistOf("/bin/echo", "hello"))
			})

			By("completing the task", func() {
				Eventually(func() []batchv1.JobCondition {
					jobs, _ = fixture.Clientset.BatchV1().Jobs(fixture.Namespace).List(metav1.ListOptions{})
					return jobs.Items[0].Status.Conditions
				}, "5s").Should(ConsistOf(MatchFields(IgnoreExtras, Fields{
					"Type":   Equal(batchv1.JobComplete),
					"Status": Equal(corev1.ConditionTrue),
				})))
			})
		})

		When("the completed handler is called", func() {
			It("removes the job", func() {
				// Ensure the job is created
				Eventually(func() ([]batchv1.Job, error) {
					var err error
					jobs, err = fixture.Clientset.BatchV1().Jobs(fixture.Namespace).List(metav1.ListOptions{})
					return jobs.Items, err
				}).Should(HaveLen(1))

				// Complete the task
				httpRequest, err := http.NewRequest("PUT", fmt.Sprintf("%s/tasks/the-task-guid/completed", url), nil)
				Expect(err).NotTo(HaveOccurred())
				resp, err := httpClient.Do(httpRequest)
				Expect(err).NotTo(HaveOccurred())
				Expect(resp.StatusCode).To(Equal(http.StatusOK))

				// Ensure the job is deleted
				Eventually(func() ([]batchv1.Job, error) {
					var err error
					jobs, err = fixture.Clientset.BatchV1().Jobs(fixture.Namespace).List(metav1.ListOptions{})
					return jobs.Items, err
				}).Should(BeEmpty())
			})

		})

		When("the task uses a private Docker registry", func() {
			BeforeEach(func() {
				request.Lifecycle.DockerLifecycle.Image = "eiriniuser/notdora"
				request.Lifecycle.DockerLifecycle.RegistryUsername = "eiriniuser"
				request.Lifecycle.DockerLifecycle.RegistryPassword = util.GetEiriniDockerHubPassword()
			})

			It("creates a new secret and points the job to it", func() {
				Eventually(func() ([]batchv1.Job, error) {
					var err error
					jobs, err = fixture.Clientset.BatchV1().Jobs(fixture.Namespace).List(metav1.ListOptions{})
					return jobs.Items, err
				}).Should(HaveLen(1))

				imagePullSecrets := jobs.Items[0].Spec.Template.Spec.ImagePullSecrets
				var registrySecretName string
				for _, imagePullSecret := range imagePullSecrets {
					if strings.HasPrefix(imagePullSecret.Name, "my-app-my-space-registry-secret-") {
						registrySecretName = imagePullSecret.Name
					}
				}
				Expect(registrySecretName).NotTo(BeEmpty())

				secret, err := fixture.Clientset.CoreV1().Secrets(fixture.Namespace).Get(registrySecretName, metav1.GetOptions{})
				Expect(err).NotTo(HaveOccurred())
				Expect(secret).NotTo(BeNil())
				Expect(secret.Data).To(
					HaveKeyWithValue(
						".dockerconfigjson",
						[]byte(fmt.Sprintf(
							`{"auths":{"index.docker.io/v1/":{"username":"eiriniuser","password":"%s","auth":"%s"}}}`,
							util.GetEiriniDockerHubPassword(),
							base64.StdEncoding.EncodeToString([]byte(fmt.Sprintf("eiriniuser:%s", util.GetEiriniDockerHubPassword()))),
						)),
					),
				)

				By("completing the task", func() {
					Eventually(func() []batchv1.JobCondition {
						jobs, _ = fixture.Clientset.BatchV1().Jobs(fixture.Namespace).List(metav1.ListOptions{})
						return jobs.Items[0].Status.Conditions
					}, "5s").Should(ConsistOf(MatchFields(IgnoreExtras, Fields{
						"Type":   Equal(batchv1.JobComplete),
						"Status": Equal(corev1.ConditionTrue),
					})))
				})
			})
		})
	})
})
