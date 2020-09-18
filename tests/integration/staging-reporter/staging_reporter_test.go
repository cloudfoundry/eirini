package staging_reporter_test

import (
	"encoding/json"
	"io/ioutil"
	"net/http"
	"os"

	"code.cloudfoundry.org/eirini"
	"code.cloudfoundry.org/eirini/k8s"
	"code.cloudfoundry.org/eirini/k8s/client"
	"code.cloudfoundry.org/eirini/models/cf"
	"code.cloudfoundry.org/eirini/opi"
	"code.cloudfoundry.org/eirini/tests"
	"code.cloudfoundry.org/lager/lagertest"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"
	"github.com/onsi/gomega/ghttp"
)

var _ = Describe("StagingReporter", func() {
	var (
		eiriniServer *ghttp.Server
		configFile   string
		certPath     string
		keyPath      string
		session      *gexec.Session
		taskDesirer  *k8s.TaskDesirer
	)

	BeforeEach(func() {
		certPath, keyPath = tests.GenerateKeyPair("opi")

		var err error
		eiriniServer, err = tests.CreateTestServer(certPath, keyPath, certPath)
		Expect(err).ToNot(HaveOccurred())
		eiriniServer.HTTPTestServer.StartTLS()

		config := &eirini.StagingReporterConfig{
			KubeConfig: eirini.KubeConfig{
				Namespace:  fixture.Namespace,
				ConfigPath: fixture.KubeConfigPath,
			},
			EiriniCertPath: certPath,
			CAPath:         certPath,
			EiriniKeyPath:  keyPath,
		}

		session, configFile = eiriniBins.StagingReporter.Run(config)

		taskDesirer = k8s.NewTaskDesirer(
			lagertest.NewTestLogger("staging-reporter-test"),
			client.NewJob(fixture.Clientset, "", true),
			nil,
			fixture.Namespace,
			nil,
			"",
			"",
			"",
			false,
		)
	})

	AfterEach(func() {
		if session != nil {
			session.Kill()
		}
		Expect(os.Remove(configFile)).To(Succeed())
		Expect(os.Remove(keyPath)).To(Succeed())
		Expect(os.Remove(certPath)).To(Succeed())
		eiriniServer.Close()
	})

	Context("When a staging job crashes", func() {
		var stagingTask *opi.StagingTask

		BeforeEach(func() {
			stagingTask = &opi.StagingTask{
				Task: &opi.Task{
					GUID: "the-staging-guid",
					Env: map[string]string{
						eirini.EnvStagingGUID: "the-staging-guid",
						"EIRINI_ADDRESS":      eiriniServer.URL(),
						"COMPLETION_CALLBACK": "the-cloud-controller-address/staging/complete",
					},
					AppName:   "test-staging-reporter-app",
					AppGUID:   "app-guid",
					OrgName:   "org-name",
					OrgGUID:   "org-guid",
					SpaceName: "space-name",
					SpaceGUID: "space-guid",
					MemoryMB:  200,
					DiskMB:    200,
					CPUWeight: 1,
				},
				DownloaderImage: "eirini/invalid-recipe-downloader",
				UploaderImage:   "eirini/recipe-uploader",
				ExecutorImage:   "eirini/recipe-executor",
			}

			eiriniServer.RouteToHandler(
				"PUT",
				"/stage/the-staging-guid/completed",
				func(w http.ResponseWriter, req *http.Request) {
					bytes, err := ioutil.ReadAll(req.Body)
					Expect(err).NotTo(HaveOccurred())

					var taskCompletedRequest cf.StagingCompletedRequest
					Expect(json.Unmarshal(bytes, &taskCompletedRequest)).To(Succeed())

					Expect(taskCompletedRequest.TaskGUID).To(Equal("the-staging-guid"))
					Expect(taskCompletedRequest.Failed).To(BeTrue())
					Expect(taskCompletedRequest.Annotation).To(ContainSubstring(`"completion_callback":"the-cloud-controller-address/staging/complete"`))
				},
			)
		})

		It("notifies eirini of a staging failure", func() {
			Expect(taskDesirer.DesireStaging(stagingTask)).To(Succeed())
			Eventually(eiriniServer.ReceivedRequests, "10s").Should(HaveLen(1))
		})
	})
})
