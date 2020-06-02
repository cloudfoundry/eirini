package staging_reporter_test

import (
	"fmt"
	"net/http"
	"os"

	"code.cloudfoundry.org/eirini"
	"code.cloudfoundry.org/eirini/integration/util"
	"code.cloudfoundry.org/eirini/k8s"
	"code.cloudfoundry.org/eirini/models/cf"
	"code.cloudfoundry.org/eirini/opi"
	"code.cloudfoundry.org/lager/lagertest"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
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
		certPath              string
		keyPath               string
		session               *gexec.Session
		taskDesirer           k8s.TaskDesirer
		task                  *opi.Task
	)

	BeforeEach(func() {
		certPath, keyPath = util.GenerateKeyPair("cloud_controller")

		var err error
		cloudControllerServer, err = util.CreateTestServer(certPath, keyPath, certPath)
		Expect(err).ToNot(HaveOccurred())
		cloudControllerServer.Start()

		handlers = []http.HandlerFunc{
			ghttp.VerifyRequest("POST", "/the-callback"),
			ghttp.VerifyJSONRepresenting(cf.TaskCompletedRequest{TaskGUID: "the-task-guid"}),
		}

		config := &eirini.TaskReporterConfig{
			KubeConfig: eirini.KubeConfig{
				Namespace:  fixture.Namespace,
				ConfigPath: fixture.KubeConfigPath,
			},
			CCCertPath: certPath,
			CAPath:     certPath,
			CCKeyPath:  keyPath,
		}

		session, configFile = util.RunBinary(pathToTaskReporter, config)

		taskDesirer = k8s.TaskDesirer{
			Namespace:          fixture.Namespace,
			ServiceAccountName: "",
			JobClient:          fixture.Clientset.BatchV1().Jobs(fixture.Namespace),
			Logger:             lagertest.NewTestLogger("task-reporter-test"),
		}

		task = &opi.Task{
			Image:              "busybox",
			Command:            []string{"echo", "hi"},
			GUID:               "the-task-guid",
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
	})

	JustBeforeEach(func() {
		cloudControllerServer.AppendHandlers(
			ghttp.CombineHandlers(handlers...),
		)

		Expect(taskDesirer.Desire(task)).To(Succeed())
	})

	AfterEach(func() {
		if session != nil {
			session.Kill()
		}
		os.Remove(configFile)
		os.Remove(keyPath)
		os.Remove(certPath)
		cloudControllerServer.Close()
	})

	It("notifies the cloud controller of a task completion", func() {
		Eventually(cloudControllerServer.ReceivedRequests, "10s").Should(HaveLen(1))
		Consistently(cloudControllerServer.ReceivedRequests, "10s").Should(HaveLen(1))
	})

	It("deletes the job", func() {
		Eventually(getTaskJobsFn("the-task-guid"), "20s").Should(BeEmpty())
	})

	When("a task job fails", func() {
		BeforeEach(func() {
			task.GUID = "failing-task-guid"
			task.Command = []string{"false"}

			handlers = []http.HandlerFunc{
				ghttp.VerifyRequest("POST", "/the-callback"),
				ghttp.VerifyJSONRepresenting(cf.TaskCompletedRequest{
					TaskGUID:      "failing-task-guid",
					Failed:        true,
					FailureReason: "Error",
				}),
			}
		})

		It("notifies the cloud controller of a task failure", func() {
			Eventually(cloudControllerServer.ReceivedRequests, "10s").Should(HaveLen(1))
			Consistently(cloudControllerServer.ReceivedRequests, "10s").Should(HaveLen(1))
		})

		It("deletes the job", func() {
			Eventually(getTaskJobsFn("failing-task-guid"), "20s").Should(BeEmpty())
		})
	})
})

func getTaskJobsFn(guid string) func() ([]batchv1.Job, error) {
	return func() ([]batchv1.Job, error) {
		jobs, err := fixture.Clientset.BatchV1().Jobs(fixture.Namespace).List(metav1.ListOptions{
			LabelSelector: fmt.Sprintf("%s=%s, %s=%s", k8s.LabelSourceType, "TASK", k8s.LabelGUID, guid),
		})
		return jobs.Items, err
	}
}
