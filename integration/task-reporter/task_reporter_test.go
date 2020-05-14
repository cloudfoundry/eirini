package staging_reporter_test

import (
	"os"

	"code.cloudfoundry.org/eirini"
	"code.cloudfoundry.org/eirini/integration/util"
	"code.cloudfoundry.org/eirini/k8s"
	"code.cloudfoundry.org/eirini/opi"
	"code.cloudfoundry.org/lager/lagertest"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"
	"github.com/onsi/gomega/ghttp"
)

var _ = Describe("TaskReporter", func() {
	var (
		eiriniServer *ghttp.Server
		configFile   string
		certPath     string
		keyPath      string
		session      *gexec.Session
		taskDesirer  k8s.TaskDesirer
		task         *opi.Task
	)

	BeforeEach(func() {
		certPath, keyPath = util.GenerateKeyPair("opi")

		var err error
		eiriniServer, err = util.CreateTestServer(certPath, keyPath, certPath)
		Expect(err).ToNot(HaveOccurred())
		eiriniServer.Start()

		config := &eirini.ReporterConfig{
			EiriniAddress: eiriniServer.URL(),
			KubeConfig: eirini.KubeConfig{
				Namespace:  fixture.Namespace,
				ConfigPath: fixture.KubeConfigPath,
			},
			EiriniCertPath: certPath,
			CAPath:         certPath,
			EiriniKeyPath:  keyPath,
		}

		session, configFile = util.RunBinary(pathToTaskReporter, config)

		taskDesirer = k8s.TaskDesirer{
			Namespace:          fixture.Namespace,
			ServiceAccountName: "",
			JobClient:          fixture.Clientset.BatchV1().Jobs(fixture.Namespace),
			Logger:             lagertest.NewTestLogger("task-reporter-test"),
		}

		task = &opi.Task{
			Image:   "busybox",
			Command: []string{"echo", "hi"},
			GUID:    "the-task-guid",
			Env: map[string]string{
				"EIRINI_ADDRESS": eiriniServer.URL(),
			},
			AppName:   "app",
			AppGUID:   "app-guid",
			OrgName:   "org-name",
			OrgGUID:   "org-guid",
			SpaceName: "space-name",
			SpaceGUID: "space-guid",
			MemoryMB:  200,
			DiskMB:    200,
			CPUWeight: 1,
		}
	})

	AfterEach(func() {
		if session != nil {
			session.Kill()
		}
		os.Remove(configFile)
		os.Remove(keyPath)
		os.Remove(certPath)
		eiriniServer.Close()
	})

	When("a task job succeeds", func() {
		BeforeEach(func() {
			eiriniServer.Reset()
			eiriniServer.AppendHandlers(ghttp.VerifyRequest("PUT", "/tasks/the-task-guid/completed"))
		})

		It("notifies eirini of a task completion", func() {
			Expect(taskDesirer.Desire(task)).To(Succeed())
			Eventually(eiriniServer.ReceivedRequests, "10s").Should(HaveLen(1))
		})
	})

	When("a task job fails", func() {
		BeforeEach(func() {
			task.GUID = "failing-task-guid"
			task.Command = []string{"false"}

			eiriniServer.Reset()
			eiriniServer.AppendHandlers(ghttp.VerifyRequest("PUT", "/tasks/failing-task-guid/completed"))
		})

		It("notifies eirini of a task failure", func() {
			Expect(taskDesirer.Desire(task)).To(Succeed())
			Eventually(eiriniServer.ReceivedRequests, "10s").Should(HaveLen(1))
		})
	})
})
