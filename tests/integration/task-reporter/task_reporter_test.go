package task_reporter_test

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"code.cloudfoundry.org/eirini"
	"code.cloudfoundry.org/eirini/k8s/client"
	"code.cloudfoundry.org/eirini/k8s/jobs"
	"code.cloudfoundry.org/eirini/models/cf"
	"code.cloudfoundry.org/eirini/opi"
	"code.cloudfoundry.org/eirini/tests"
	"code.cloudfoundry.org/eirini/tests/integration"
	"code.cloudfoundry.org/eirini/util"
	"code.cloudfoundry.org/lager/lagertest"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"
	"github.com/onsi/gomega/ghttp"
	batchv1 "k8s.io/api/batch/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = Describe("TaskReporter", func() {
	var (
		cloudControllerServer *ghttp.Server
		handlers              []http.HandlerFunc
		configFile            string
		session               *gexec.Session
		taskDesirer           jobs.Desirer
		task                  *opi.Task
		config                *eirini.TaskReporterConfig
		ttlSeconds            int
		taskSubmittedAt       time.Time
		ctx                   context.Context
	)

	BeforeEach(func() {
		var err error
		ctx = context.Background()

		cloudControllerServer, err = integration.CreateTestServer(
			integration.PathToTestFixture("tls.crt"),
			integration.PathToTestFixture("tls.key"),
			integration.PathToTestFixture("tls.ca"),
		)
		Expect(err).ToNot(HaveOccurred())
		cloudControllerServer.HTTPTestServer.StartTLS()
		ttlSeconds = 10

		config = &eirini.TaskReporterConfig{
			KubeConfig: eirini.KubeConfig{
				ConfigPath: fixture.KubeConfigPath,
			},
			WorkloadsNamespace:           fixture.Namespace,
			CompletionCallbackRetryLimit: 3,
			TTLSeconds:                   ttlSeconds,
			LeaderElectionID:             fmt.Sprintf("test-task-reporter-%d", GinkgoParallelNode()),
			LeaderElectionNamespace:      fixture.Namespace,
		}

		taskToJobConverter := jobs.NewTaskToJobConverter("", "", false)
		taskDesirer = jobs.NewDesirer(
			lagertest.NewTestLogger("test-task-desirer"),
			taskToJobConverter,
			client.NewJob(fixture.Clientset, fixture.Namespace),
			client.NewSecret(fixture.Clientset),
		)

		taskGUID := tests.GenerateGUID()
		task = &opi.Task{
			Image:              "eirini/busybox",
			Command:            []string{"echo", "hi"},
			GUID:               taskGUID,
			CompletionCallback: fmt.Sprintf("%s/the-callback", cloudControllerServer.URL()),
			AppName:            "app",
			AppGUID:            "app-guid",
			OrgName:            "org-name",
			OrgGUID:            "org-guid",
			SpaceName:          "space-name",
			SpaceGUID:          "space-guid",
			MemoryMB:           200,
			DiskMB:             200,
			CPUWeight:          1,
		}

		handlers = []http.HandlerFunc{
			ghttp.CombineHandlers(
				ghttp.VerifyRequest("POST", "/the-callback"),
				ghttp.VerifyJSONRepresenting(cf.TaskCompletedRequest{TaskGUID: taskGUID}),
			),
		}
	})

	JustBeforeEach(func() {
		cloudControllerServer.AppendHandlers(handlers...)

		session, configFile = eiriniBins.TaskReporter.Run(config, envVarOverrides...)
		Eventually(session).Should(gbytes.Say("Starting workers"))
		Expect(taskDesirer.Desire(ctx, fixture.Namespace, task)).To(Succeed())
		taskSubmittedAt = time.Now()
	})

	AfterEach(func() {
		if session != nil {
			session.Kill()
		}
		Expect(os.Remove(configFile)).To(Succeed())
		cloudControllerServer.Close()
	})

	It("notifies the cloud controller of a task completion", func() {
		Eventually(cloudControllerServer.ReceivedRequests).Should(HaveLen(1))
		Consistently(cloudControllerServer.ReceivedRequests, "1m").Should(HaveLen(1))
	})

	Context("with a long TTL to ensure task doesn't get cleaned up", func() {
		BeforeEach(func() {
			config.TTLSeconds = 100
		})

		It("labels the job as completed", func() {
			Eventually(func() ([]batchv1.Job, error) {
				tasks, err := getCompletedTaskJobsFn(task.GUID)()
				if err != nil {
					return nil, err
				}

				return tasks, nil
			}).Should(HaveLen(1))
		})
	})

	When("the Cloud Controller is not using TLS", func() {
		BeforeEach(func() {
			config.CCTLSDisabled = true
			envVarOverrides = []string{fmt.Sprintf("%s=%s", eirini.EnvCCCertDir, "/does/not/exist")}
			cloudControllerServer.Close()
			cloudControllerServer = ghttp.NewServer()
			task.CompletionCallback = fmt.Sprintf("%s/the-callback", cloudControllerServer.URL())
		})

		It("still gets notified", func() {
			Eventually(cloudControllerServer.ReceivedRequests).Should(HaveLen(1))
		})
	})

	It("deletes the job", func() {
		Eventually(getTaskJobsFn(task.GUID)).Should(BeEmpty())
	})

	It("does not delete the job before the TTL has expired", func() {
		Eventually(getTaskJobsFn(task.GUID)).Should(BeEmpty())
		Expect(time.Now()).To(BeTemporally(">", taskSubmittedAt.Add(time.Duration(ttlSeconds)*time.Second)))
	})

	When("a task job fails", func() {
		BeforeEach(func() {
			task.Command = []string{"false"}

			handlers = []http.HandlerFunc{
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("POST", "/the-callback"),
					ghttp.VerifyJSONRepresenting(cf.TaskCompletedRequest{
						TaskGUID:      task.GUID,
						Failed:        true,
						FailureReason: "Error",
					}),
				),
			}
		})

		It("notifies the cloud controller of a task failure", func() {
			Eventually(cloudControllerServer.ReceivedRequests).Should(HaveLen(1))
			Consistently(cloudControllerServer.ReceivedRequests, "10s").Should(HaveLen(1))
		})

		It("deletes the job", func() {
			Eventually(getTaskJobsFn(task.GUID)).Should(BeEmpty())
		})
	})

	When("the completion callback fails", func() {
		BeforeEach(func() {
			cloudControllerServer.SetAllowUnhandledRequests(true)
			handlers = []http.HandlerFunc{
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("POST", "/the-callback"),
					ghttp.RespondWith(http.StatusInternalServerError, nil),
				),
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("POST", "/the-callback"),
					ghttp.RespondWith(http.StatusInternalServerError, nil),
				),
			}
			config.CompletionCallbackRetryLimit = 2
		})

		It("tries for a total of [callbackRetryLimit] times", func() {
			Eventually(cloudControllerServer.ReceivedRequests).Should(HaveLen(config.CompletionCallbackRetryLimit))
			Consistently(cloudControllerServer.ReceivedRequests, "2s").Should(HaveLen(config.CompletionCallbackRetryLimit))
		})
	})

	When("a private docker registry is used", func() {
		BeforeEach(func() {
			task.Image = "eiriniuser/notdora"
			task.PrivateRegistry = &opi.PrivateRegistry{
				Server:   util.DockerHubHost,
				Username: "eiriniuser",
				Password: tests.GetEiriniDockerHubPassword(),
			}
			task.Command = []string{"sleep", "1"}
		})

		It("deletes the docker registry secret", func() {
			registrySecretPrefix := fmt.Sprintf("%s-%s-registry-secret-", task.AppName, task.SpaceName)
			jobs, err := getTaskJobsFn(task.GUID)()
			Expect(err).NotTo(HaveOccurred())
			Expect(jobs).To(HaveLen(1))

			imagePullSecrets := jobs[0].Spec.Template.Spec.ImagePullSecrets
			var registrySecretName string
			for _, imagePullSecret := range imagePullSecrets {
				if strings.HasPrefix(imagePullSecret.Name, registrySecretPrefix) {
					registrySecretName = imagePullSecret.Name

					break
				}
			}
			Expect(registrySecretName).NotTo(BeEmpty())

			Eventually(func() error {
				_, err := fixture.Clientset.CoreV1().Secrets(fixture.Namespace).Get(context.Background(), registrySecretName, metav1.GetOptions{})

				return err
			}).Should(MatchError(ContainSubstring(`secrets "%s" not found`, registrySecretName)))
		})
	})

	When("completionCallbackRetryLimit is not set in the config", func() {
		BeforeEach(func() {
			config.CompletionCallbackRetryLimit = 0
			handlers = []http.HandlerFunc{
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("POST", "/the-callback"),
					ghttp.RespondWith(http.StatusInternalServerError, nil),
				),
			}
			cloudControllerServer.SetAllowUnhandledRequests(true)
		})

		It("uses the default value of '10'", func() {
			Eventually(cloudControllerServer.ReceivedRequests, "30s").Should(HaveLen(10))
			Consistently(cloudControllerServer.ReceivedRequests, "2s").Should(HaveLen(10))
		})
	})
})

func getTaskJobsFn(guid string) func() ([]batchv1.Job, error) {
	selector := fmt.Sprintf(
		"%s=%s, %s=%s",
		jobs.LabelSourceType, "TASK",
		jobs.LabelGUID, guid,
	)

	return func() ([]batchv1.Job, error) {
		return getTasksWithSelector(selector)
	}
}

func getCompletedTaskJobsFn(guid string) func() ([]batchv1.Job, error) {
	selector := fmt.Sprintf(
		"%s=%s, %s=%s, %s=%s",
		jobs.LabelSourceType, "TASK",
		jobs.LabelGUID, guid,
		jobs.LabelTaskCompleted, jobs.TaskCompletedTrue,
	)

	return func() ([]batchv1.Job, error) {
		return getTasksWithSelector(selector)
	}
}

func getTasksWithSelector(selector string) ([]batchv1.Job, error) {
	jobs, err := fixture.Clientset.BatchV1().Jobs(fixture.Namespace).List(context.Background(), metav1.ListOptions{
		LabelSelector: selector,
	})

	return jobs.Items, err
}
