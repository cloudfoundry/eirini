package opi_test

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"

	"code.cloudfoundry.org/eirini"
	"code.cloudfoundry.org/eirini/models/cf"
	"code.cloudfoundry.org/eirini/tests"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/ghttp"
	. "github.com/onsi/gomega/gstruct"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = Describe("Task Desire and Cancel", func() {
	var (
		request cf.TaskRequest
		jobs    *batchv1.JobList
	)

	const serviceAccountTokenMountPath = "/var/run/secrets/kubernetes.io/serviceaccount" //nolint:gosec

	JustBeforeEach(func() {
		body, err := json.Marshal(request)
		Expect(err).NotTo(HaveOccurred())

		httpRequest, err := http.NewRequest("POST", fmt.Sprintf("%s/tasks/%s", url, request.GUID), bytes.NewReader(body))
		Expect(err).NotTo(HaveOccurred())

		resp, err := httpClient.Do(httpRequest)
		Expect(err).NotTo(HaveOccurred())
		Expect(resp.StatusCode).To(Equal(http.StatusAccepted))
	})

	Context("buildpack tasks", func() {
		BeforeEach(func() {
			request = cf.TaskRequest{
				GUID:        tests.GenerateGUID(),
				AppName:     "my_app",
				Name:        "my_task",
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
				jobs, err = fixture.Clientset.BatchV1().Jobs(fixture.Namespace).List(context.Background(), metav1.ListOptions{})

				return jobs.Items, err
			}).Should(HaveLen(1))

			By("creating a job for the task", func() {
				Expect(jobs.Items).To(HaveLen(1))
				Expect(jobs.Items[0].Name).To(Equal("my-app-my-space-my-task"))
			})

			By("using the correct service account", func() {
				Expect(jobs.Items[0].Spec.Template.Spec.ServiceAccountName).To(Equal(tests.GetApplicationServiceAccount()))
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

			By("not mounting the service account token", func() {
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

				podMountPaths := []string{}
				for _, podMount := range pods.Items[0].Spec.Containers[0].VolumeMounts {
					podMountPaths = append(podMountPaths, podMount.MountPath)
				}
				Expect(podMountPaths).NotTo(ContainElement(serviceAccountTokenMountPath))
			})
		})
	})

	Context("docker tasks", func() {
		BeforeEach(func() {
			request = cf.TaskRequest{
				GUID:        tests.GenerateGUID(),
				AppName:     "my_app",
				Name:        "my_task",
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
				jobs, err = fixture.Clientset.BatchV1().Jobs(fixture.Namespace).List(context.Background(), metav1.ListOptions{})

				return jobs.Items, err
			}).Should(HaveLen(1))

			By("creating a job for the task", func() {
				Expect(jobs.Items).To(HaveLen(1))
				Expect(jobs.Items[0].Name).To(HavePrefix("my-app-my-space-my-task"))
			})

			By("specifying the right containers", func() {
				jobContainers := jobs.Items[0].Spec.Template.Spec.Containers
				Expect(jobContainers).To(HaveLen(1))
				Expect(jobContainers[0].Env).To(ContainElement(corev1.EnvVar{Name: "my-env", Value: "my-value"}))
				Expect(jobContainers[0].Image).To(Equal("busybox"))
				Expect(jobContainers[0].Command).To(ConsistOf("/bin/echo", "hello"))
			})

			By("not mounting the service account token", func() {
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

				podMountPaths := []string{}
				for _, podMount := range pods.Items[0].Spec.Containers[0].VolumeMounts {
					podMountPaths = append(podMountPaths, podMount.MountPath)
				}
				Expect(podMountPaths).NotTo(ContainElement(serviceAccountTokenMountPath))
			})

			By("completing the task", func() {
				Eventually(func() []batchv1.JobCondition {
					jobs, _ = fixture.Clientset.BatchV1().Jobs(fixture.Namespace).List(context.Background(), metav1.ListOptions{})

					return jobs.Items[0].Status.Conditions
				}).Should(ConsistOf(MatchFields(IgnoreExtras, Fields{
					"Type":   Equal(batchv1.JobComplete),
					"Status": Equal(corev1.ConditionTrue),
				})))
			})
		})

		When("the task uses a private Docker registry", func() {
			BeforeEach(func() {
				request.Lifecycle.DockerLifecycle.Image = "eiriniuser/notdora"
				request.Lifecycle.DockerLifecycle.RegistryUsername = "eiriniuser"
				request.Lifecycle.DockerLifecycle.RegistryPassword = tests.GetEiriniDockerHubPassword()
			})

			It("creates a new secret and points the job to it", func() {
				Eventually(func() ([]batchv1.Job, error) {
					var err error
					jobs, err = fixture.Clientset.BatchV1().Jobs(fixture.Namespace).List(context.Background(), metav1.ListOptions{})

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

				secret, err := fixture.Clientset.CoreV1().Secrets(fixture.Namespace).Get(context.Background(), registrySecretName, metav1.GetOptions{})
				Expect(err).NotTo(HaveOccurred())
				Expect(secret).NotTo(BeNil())
				Expect(secret.Data).To(
					HaveKeyWithValue(
						".dockerconfigjson",
						[]byte(fmt.Sprintf(
							`{"auths":{"index.docker.io/v1/":{"username":"eiriniuser","password":"%s","auth":"%s"}}}`,
							tests.GetEiriniDockerHubPassword(),
							base64.StdEncoding.EncodeToString([]byte(fmt.Sprintf("eiriniuser:%s", tests.GetEiriniDockerHubPassword()))),
						)),
					),
				)

				By("allowing the task to complete", func() {
					Eventually(func() []batchv1.JobCondition {
						jobs, _ = fixture.Clientset.BatchV1().Jobs(fixture.Namespace).List(context.Background(), metav1.ListOptions{})

						return jobs.Items[0].Status.Conditions
					}).Should(ConsistOf(MatchFields(IgnoreExtras, Fields{
						"Type":   Equal(batchv1.JobComplete),
						"Status": Equal(corev1.ConditionTrue),
					})))
				})
			})
		})

		When("unsafe_allow_automount_service_account_token is set", func() {
			var newConfigPath string

			BeforeEach(func() {
				newConfigPath = restartWithConfig(func(config eirini.Config) eirini.Config {
					config.Properties.UnsafeAllowAutomountServiceAccountToken = true

					return config
				})
			})

			AfterEach(func() {
				os.RemoveAll(newConfigPath)
			})

			getPods := func() []corev1.Pod {
				var podItems []corev1.Pod
				Eventually(func() ([]corev1.Pod, error) {
					var err error
					pods, err := fixture.Clientset.CoreV1().Pods(fixture.Namespace).List(context.Background(), metav1.ListOptions{})
					if err != nil {
						return nil, err
					}

					podItems = pods.Items

					return podItems, nil
				}).ShouldNot(BeEmpty())

				return podItems
			}

			It("mounts the service account token (because this is how K8S works by default)", func() {
				pods := getPods()
				Expect(pods).To(HaveLen(1))

				podMountPaths := []string{}
				for _, podMount := range pods[0].Spec.Containers[0].VolumeMounts {
					podMountPaths = append(podMountPaths, podMount.MountPath)
				}
				Expect(podMountPaths).To(ContainElement(serviceAccountTokenMountPath))
			})

			When("the app/task service account has its automountServiceAccountToken set to false", func() {
				BeforeEach(func() {
					appServiceAccount, err := fixture.Clientset.CoreV1().ServiceAccounts(fixture.Namespace).Get(context.Background(), tests.GetApplicationServiceAccount(), metav1.GetOptions{})
					Expect(err).NotTo(HaveOccurred())
					automountServiceAccountToken := false
					appServiceAccount.AutomountServiceAccountToken = &automountServiceAccountToken
					_, err = fixture.Clientset.CoreV1().ServiceAccounts(fixture.Namespace).Update(context.Background(), appServiceAccount, metav1.UpdateOptions{})
					Expect(err).NotTo(HaveOccurred())
				})

				It("does not mount the service account token", func() {
					pods := getPods()
					Expect(pods).To(HaveLen(1))

					podMountPaths := []string{}
					for _, podMount := range pods[0].Spec.Containers[0].VolumeMounts {
						podMountPaths = append(podMountPaths, podMount.MountPath)
					}
					Expect(podMountPaths).NotTo(ContainElement(serviceAccountTokenMountPath))
				})
			})
		})
	})

	Describe("cancelling", func() {
		var cloudControllerServer *ghttp.Server

		BeforeEach(func() {
			var err error
			cloudControllerServer, err = tests.CreateTestServer(certPath, keyPath, certPath)
			Expect(err).ToNot(HaveOccurred())
			cloudControllerServer.HTTPTestServer.StartTLS()

			guid := tests.GenerateGUID()

			cloudControllerServer.AppendHandlers(
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("POST", "/"),
					ghttp.VerifyJSONRepresenting(cf.TaskCompletedRequest{
						TaskGUID:      guid,
						Failed:        true,
						FailureReason: "task was cancelled",
					}),
				),
			)

			request = cf.TaskRequest{
				GUID:      guid,
				AppName:   "my_app",
				SpaceName: "my_space",
				Lifecycle: cf.Lifecycle{
					DockerLifecycle: &cf.DockerLifecycle{
						Image:   "busybox",
						Command: []string{"/bin/sleep", "100"},
					},
				},
				CompletionCallback: cloudControllerServer.URL(),
			}
		})

		JustBeforeEach(func() {
			// Ensure the job is created
			Eventually(func() ([]batchv1.Job, error) {
				var err error
				jobs, err = fixture.Clientset.BatchV1().Jobs(fixture.Namespace).List(context.Background(), metav1.ListOptions{})

				return jobs.Items, err
			}).Should(HaveLen(1))

			// Cancel the task
			httpRequest, err := http.NewRequest("DELETE", fmt.Sprintf("%s/tasks/%s", url, request.GUID), nil)
			Expect(err).NotTo(HaveOccurred())
			resp, err := httpClient.Do(httpRequest)
			Expect(err).NotTo(HaveOccurred())
			Expect(resp.StatusCode).To(Equal(http.StatusNoContent))
		})

		AfterEach(func() {
			cloudControllerServer.Close()
		})

		It("deletes the job and notifies the Cloud Controller", func() {
			// Ensure the job is deleted
			Eventually(func() ([]batchv1.Job, error) {
				var err error
				jobs, err = fixture.Clientset.BatchV1().Jobs(fixture.Namespace).List(context.Background(), metav1.ListOptions{})

				return jobs.Items, err
			}).Should(BeEmpty())

			Eventually(cloudControllerServer.ReceivedRequests).Should(HaveLen(1))
		})

		When("cancelling a task with docker lifecycle", func() {
			BeforeEach(func() {
				request.Lifecycle.DockerLifecycle.Image = "eiriniuser/notdora"
				request.Lifecycle.DockerLifecycle.RegistryUsername = "eiriniuser"
				request.Lifecycle.DockerLifecycle.RegistryPassword = tests.GetEiriniDockerHubPassword()
			})

			It("deletes the docker registry secret", func() {
				Eventually(func() ([]string, error) {
					secrets, err := fixture.Clientset.CoreV1().Secrets(fixture.Namespace).List(context.Background(), metav1.ListOptions{})
					if err != nil {
						return nil, err
					}

					secretNames := []string{}
					for _, secret := range secrets.Items {
						secretNames = append(secretNames, secret.Name)
					}

					return secretNames, nil
				}).Should(Not(ContainElement(HavePrefix("my-app-my-space-registry-secret-"))))
			})
		})

		When("CC TLS is disabled and CC certs not configured", func() {
			var newConfigPath string

			BeforeEach(func() {
				cloudControllerServer.Close()
				cloudControllerServer = ghttp.NewServer()
				cloudControllerServer.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("POST", "/"),
						ghttp.VerifyJSONRepresenting(cf.TaskCompletedRequest{
							TaskGUID:      request.GUID,
							Failed:        true,
							FailureReason: "task was cancelled",
						}),
					),
				)

				newConfigPath = restartWithConfig(func(cfg eirini.Config) eirini.Config {
					cfg.Properties.CCTLSDisabled = true
					cfg.Properties.CCCertPath = ""
					cfg.Properties.CCKeyPath = ""
					cfg.Properties.CCCAPath = ""

					return cfg
				})

				request.CompletionCallback = cloudControllerServer.URL()
			})

			AfterEach(func() {
				os.RemoveAll(newConfigPath)
			})

			It("sends the task completed message to the cloud controller", func() {
				Eventually(cloudControllerServer.ReceivedRequests).Should(HaveLen(1))
			})
		})
	})
})
