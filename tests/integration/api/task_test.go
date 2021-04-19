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
	"code.cloudfoundry.org/eirini/k8s/shared"
	"code.cloudfoundry.org/eirini/models/cf"
	"code.cloudfoundry.org/eirini/tests"
	"code.cloudfoundry.org/eirini/tests/integration"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/ghttp"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = Describe("Tasks", func() {
	var (
		request          cf.TaskRequest
		response         *http.Response
		serviceName      string
		serviceNameSpace string
		servicePort      int32
		taskGUID         string
	)

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
					Image: "eirini/dorini",
				},
			},
		}
		serviceNameSpace = fixture.Namespace
		servicePort = 8080
	})

	JustBeforeEach(func() {
		body, err := json.Marshal(request)
		Expect(err).NotTo(HaveOccurred())

		httpRequest, err := http.NewRequest("POST", fmt.Sprintf("%s/tasks/%s", eiriniAPIUrl, request.GUID), bytes.NewReader(body))
		Expect(err).NotTo(HaveOccurred())

		response, err = httpClient.Do(httpRequest)
		Expect(err).NotTo(HaveOccurred())

		serviceName = tests.ExposeAsService(fixture.Clientset, serviceNameSpace, request.GUID, servicePort, "/")
	})

	Describe("desiring", func() {
		const serviceAccountTokenMountPath = "/var/run/secrets/kubernetes.io/serviceaccount"

		It("starts the task", func() {
			Expect(response.StatusCode).To(Equal(http.StatusAccepted))
			out, err := tests.RequestServiceFn(fixture.Namespace, serviceName, servicePort, "/")()
			Expect(err).NotTo(HaveOccurred())
			Expect(out).To(ContainSubstring("not Dora"))
		})

		It("sets the correct latest migration index", func() {
			jobsList, err := fixture.Clientset.BatchV1().Jobs(fixture.Namespace).List(context.Background(), metav1.ListOptions{})
			Expect(err).NotTo(HaveOccurred())

			Expect(jobsList.Items[0].Annotations).To(
				HaveKeyWithValue(shared.AnnotationLatestMigration, strconv.Itoa(cmdcommons.GetLatestMigrationIndex())),
			)
		})

		When("the task uses a private Docker registry", func() {
			BeforeEach(func() {
				request.Lifecycle.DockerLifecycle.Image = "eiriniuser/notdora"
				request.Lifecycle.DockerLifecycle.RegistryUsername = "eiriniuser"
				request.Lifecycle.DockerLifecycle.RegistryPassword = tests.GetEiriniDockerHubPassword()
				servicePort = 8888
			})

			It("starts the task", func() {
				Eventually(tests.RequestServiceFn(fixture.Namespace, serviceName, servicePort, "/")).Should(ContainSubstring("not Dora"))
			})
		})

		Describe("mounting service account tokens", func() {
			It("does not mount the service account token", func() {
				result, err := tests.RequestServiceFn(fixture.Namespace, serviceName, servicePort, fmt.Sprintf("/ls?path=%s", serviceAccountTokenMountPath))()
				Expect(err).To(MatchError(ContainSubstring("Internal Server Error")))
				Expect(result).To(ContainSubstring("no such file or directory"))
			})

			When("unsafe_allow_automount_service_account_token is set", func() {
				BeforeEach(func() {
					apiConfig.UnsafeAllowAutomountServiceAccountToken = true
				})

				It("mounts the service account token (because this is how K8S works by default)", func() {
					_, err := tests.RequestServiceFn(fixture.Namespace, serviceName, servicePort, fmt.Sprintf("/ls?path=%s", serviceAccountTokenMountPath))()
					Expect(err).NotTo(HaveOccurred())
				})

				When("the service account has its automountServiceAccountToken set to false", func() {
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
						result, err := tests.RequestServiceFn(fixture.Namespace, serviceName, servicePort, fmt.Sprintf("/ls?path=%s", serviceAccountTokenMountPath))()
						Expect(err).To(MatchError(ContainSubstring("Internal Server Error")))
						Expect(result).To(ContainSubstring("no such file or directory"))
					})
				})
			})
		})

		When("no task namespaces is explicitly requested", func() {
			var extraNs string

			BeforeEach(func() {
				extraNs = fixture.CreateExtraNamespace()
				apiConfig.DefaultWorkloadsNamespace = extraNs
				serviceNameSpace = extraNs

				request.Namespace = ""
			})

			It("creates create the task in the default namespace", func() {
				Expect(response.StatusCode).To(Equal(http.StatusAccepted))

				jobsList, err := fixture.Clientset.BatchV1().Jobs(extraNs).List(context.Background(), metav1.ListOptions{})
				Expect(err).NotTo(HaveOccurred())

				Expect(jobsList.Items).To(HaveLen(1))
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

			cloudControllerServer.AppendHandlers(
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("POST", "/"),
					ghttp.VerifyJSONRepresenting(cf.TaskCompletedRequest{
						TaskGUID:      taskGUID,
						Failed:        true,
						FailureReason: "task was cancelled",
					}),
				),
			)

			request.CompletionCallback = cloudControllerServer.URL()
		})

		JustBeforeEach(func() {
			httpRequest, err := http.NewRequest("DELETE", fmt.Sprintf("%s/tasks/%s", eiriniAPIUrl, request.GUID), nil)
			Expect(err).NotTo(HaveOccurred())

			response, err = httpClient.Do(httpRequest)
			Expect(err).NotTo(HaveOccurred())
		})

		AfterEach(func() {
			cloudControllerServer.Close()
		})

		It("returns statusNoContent", func() {
			Expect(response.StatusCode).To(Equal(http.StatusNoContent))
		})

		It("stops the job", func() {
			Eventually(func() error {
				_, err := tests.RequestServiceFn(serviceNameSpace, serviceName, servicePort, "/")()
				return err
			}).Should(MatchError(ContainSubstring("context deadline exceeded")))
		})

		It("notifies the Cloud Controller", func() {
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
		It("returns all tasks", func() {
			httpRequest, err := http.NewRequest("GET", fmt.Sprintf("%s/tasks", eiriniAPIUrl), nil)
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
