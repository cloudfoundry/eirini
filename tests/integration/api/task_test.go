package api_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"path/filepath"
	"strconv"

	"code.cloudfoundry.org/eirini"
	cmdcommons "code.cloudfoundry.org/eirini/cmd"
	"code.cloudfoundry.org/eirini/k8s/jobs"
	"code.cloudfoundry.org/eirini/k8s/shared"
	"code.cloudfoundry.org/eirini/models/cf"
	"code.cloudfoundry.org/eirini/tests"
	"code.cloudfoundry.org/eirini/tests/integration"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/ghttp"
	. "github.com/onsi/gomega/gstruct"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = Describe("Tasks", func() {
	var (
		request  cf.TaskRequest
		jobsList *batchv1.JobList
		response *http.Response
	)

	JustBeforeEach(func() {
		body, err := json.Marshal(request)
		Expect(err).NotTo(HaveOccurred())

		httpRequest, err := http.NewRequest("POST", fmt.Sprintf("%s/tasks/%s", url, request.GUID), bytes.NewReader(body))
		Expect(err).NotTo(HaveOccurred())

		response, err = httpClient.Do(httpRequest)
		Expect(err).NotTo(HaveOccurred())
	})

	Describe("desiring", func() {
		const serviceAccountTokenMountPath = "/var/run/secrets/kubernetes.io/serviceaccount"
		var taskGUID string

		BeforeEach(func() {
			taskGUID = tests.GenerateGUID()
			request = cf.TaskRequest{
				GUID:        taskGUID,
				AppName:     "my_app",
				AppGUID:     "guid-1234",
				Name:        "my_task",
				SpaceName:   "my_space",
				Namespace:   fixture.Namespace,
				Environment: []cf.EnvironmentVariable{{Name: "my-env", Value: "my-value"}},
				Lifecycle: cf.Lifecycle{
					DockerLifecycle: &cf.DockerLifecycle{
						Image:   "eirini/busybox",
						Command: []string{"/bin/echo", "hello"},
					},
				},
			}
		})

		It("creates the job successfully", func() {
			Expect(response.StatusCode).To(Equal(http.StatusAccepted))

			Eventually(func() ([]batchv1.Job, error) {
				var err error
				jobsList, err = fixture.Clientset.BatchV1().Jobs(fixture.Namespace).List(context.Background(), metav1.ListOptions{})

				return jobsList.Items, err
			}).Should(HaveLen(1))

			By("creating a job for the task", func() {
				Expect(jobsList.Items).To(HaveLen(1))
				Expect(jobsList.Items[0].Name).To(HavePrefix("my-app-my-space-my-task"))
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
					jobsList, _ = fixture.Clientset.BatchV1().Jobs(fixture.Namespace).List(context.Background(), metav1.ListOptions{})

					return jobsList.Items[0].Status.Conditions
				}).Should(ConsistOf(MatchFields(IgnoreExtras, Fields{
					"Type":   Equal(batchv1.JobComplete),
					"Status": Equal(corev1.ConditionTrue),
				})))
			})

			By("setting the latest migration index", func() {
				Expect(jobsList.Items[0].Annotations).To(
					HaveKeyWithValue(shared.AnnotationLatestMigration, strconv.Itoa(cmdcommons.GetLatestMigrationIndex())),
				)
			})
		})

		When("the task uses a private Docker registry", func() {
			BeforeEach(func() {
				request.Lifecycle.DockerLifecycle.Image = "eiriniuser/notdora"
				request.Lifecycle.DockerLifecycle.RegistryUsername = "eiriniuser"
				request.Lifecycle.DockerLifecycle.RegistryPassword = tests.GetEiriniDockerHubPassword()
			})

			It("creates a job that completes", func() {
				Eventually(func() ([]batchv1.Job, error) {
					var err error
					jobsList, err = fixture.Clientset.BatchV1().Jobs(fixture.Namespace).List(context.Background(), metav1.ListOptions{})

					return jobsList.Items, err
				}).Should(HaveLen(1))
				Expect(jobsList.Items[0].Labels).To(HaveKeyWithValue(jobs.LabelAppGUID, "guid-1234"))

				Eventually(func() []batchv1.JobCondition {
					jobsList, _ = fixture.Clientset.BatchV1().Jobs(fixture.Namespace).List(context.Background(), metav1.ListOptions{})

					return jobsList.Items[0].Status.Conditions
				}).Should(ConsistOf(MatchFields(IgnoreExtras, Fields{
					"Type":   Equal(batchv1.JobComplete),
					"Status": Equal(corev1.ConditionTrue),
				})))
			})
		})

		When("unsafe_allow_automount_service_account_token is set", func() {
			BeforeEach(func() {
				apiConfig.UnsafeAllowAutomountServiceAccountToken = true
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
					Eventually(func() error {
						appServiceAccount, err := fixture.Clientset.CoreV1().ServiceAccounts(fixture.Namespace).Get(context.Background(), tests.GetApplicationServiceAccount(), metav1.GetOptions{})
						if err != nil {
							return err
						}
						automountServiceAccountToken := false
						appServiceAccount.AutomountServiceAccountToken = &automountServiceAccountToken
						_, err = fixture.Clientset.CoreV1().ServiceAccounts(fixture.Namespace).Update(context.Background(), appServiceAccount, metav1.UpdateOptions{})
						if err != nil {
							return err
						}

						return nil
					}).Should(Succeed())
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

		When("no task namespaces is explicitly requested", func() {
			BeforeEach(func() {
				request = cf.TaskRequest{
					GUID:      tests.GenerateGUID(),
					Namespace: "",
					Lifecycle: cf.Lifecycle{
						DockerLifecycle: &cf.DockerLifecycle{
							Image:   "eirini/busybox",
							Command: []string{"/bin/echo", "hello"},
						},
					},
				}
			})

			It("creates create the task in the default namespace", func() {
				Expect(response.StatusCode).To(Equal(http.StatusAccepted))

				Eventually(func() ([]batchv1.Job, error) {
					var err error
					jobsList, err = fixture.Clientset.BatchV1().Jobs(fixture.Namespace).List(context.Background(), metav1.ListOptions{})

					return jobsList.Items, err
				}).Should(HaveLen(1))

				Expect(jobsList.Items).To(HaveLen(1))
				Expect(jobsList.Items[0].Name).To(Equal(request.GUID))
			})
		})
	})

	Describe("cancelling", func() {
		var cloudControllerServer *ghttp.Server

		BeforeEach(func() {
			var err error
			cloudControllerServer, err = integration.CreateTestServer(
				filepath.Join(eiriniBins.CertsPath, "tls.crt"),
				filepath.Join(eiriniBins.CertsPath, "tls.key"),
				filepath.Join(eiriniBins.CertsPath, "tls.ca"),
			)
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
				Namespace: fixture.Namespace,
				Lifecycle: cf.Lifecycle{
					DockerLifecycle: &cf.DockerLifecycle{
						Image:   "eirini/busybox",
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
				jobsList, err = fixture.Clientset.BatchV1().Jobs(fixture.Namespace).List(context.Background(), metav1.ListOptions{})

				return jobsList.Items, err
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
				jobsList, err = fixture.Clientset.BatchV1().Jobs(fixture.Namespace).List(context.Background(), metav1.ListOptions{})

				return jobsList.Items, err
			}).Should(BeEmpty())

			Eventually(cloudControllerServer.ReceivedRequests).Should(HaveLen(1))
		})

		When("CC TLS is disabled and CC certs not configured", func() {
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

				apiConfig.CCTLSDisabled = true
				apiEnvOverride = []string{fmt.Sprintf("%s=%s", eirini.EnvCCCertDir, "/does/not/exits")}

				request.CompletionCallback = cloudControllerServer.URL()
			})

			It("sends the task completed message to the cloud controller", func() {
				Eventually(cloudControllerServer.ReceivedRequests).Should(HaveLen(1))
			})
		})
	})

	Describe("listing", func() {
		BeforeEach(func() {
			guid := tests.GenerateGUID()

			request = cf.TaskRequest{
				GUID:      guid,
				AppName:   "my_app",
				SpaceName: "my_space",
				Namespace: fixture.Namespace,
				Lifecycle: cf.Lifecycle{
					DockerLifecycle: &cf.DockerLifecycle{
						Image:   "eirini/busybox",
						Command: []string{"/bin/sleep", "100"},
					},
				},
			}
		})

		JustBeforeEach(func() {
			Eventually(func() ([]batchv1.Job, error) {
				var err error
				jobsList, err = fixture.Clientset.BatchV1().Jobs(fixture.Namespace).List(context.Background(), metav1.ListOptions{})

				return jobsList.Items, err
			}).Should(HaveLen(1))
		})

		It("returns all tasks", func() {
			httpRequest, err := http.NewRequest("GET", fmt.Sprintf("%s/tasks", url), nil)
			Expect(err).NotTo(HaveOccurred())
			resp, err := httpClient.Do(httpRequest)
			Expect(err).NotTo(HaveOccurred())
			defer resp.Body.Close()
			Expect(resp.StatusCode).To(Equal(http.StatusOK))

			var tasks cf.TasksResponse
			err = json.NewDecoder(resp.Body).Decode(&tasks)
			Expect(err).NotTo(HaveOccurred())

			Expect(tasks).To(ConsistOf(cf.TaskResponse{GUID: request.GUID}))
		})
	})
})
