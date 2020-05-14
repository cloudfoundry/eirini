package staging_reporter_test

import (
	"encoding/json"
	"io/ioutil"
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
)

var _ = Describe("StagingReporter", func() {
	var (
		eiriniServer *ghttp.Server
		configFile   string
		certPath     string
		keyPath      string
		session      *gexec.Session
		taskDesirer  k8s.TaskDesirer
	)

	BeforeEach(func() {
		certPath, keyPath = util.GenerateKeyPair("opi")

		var err error
		eiriniServer, err = util.CreateTestServer(certPath, keyPath, certPath)
		Expect(err).ToNot(HaveOccurred())
		eiriniServer.Start()

		config := &eirini.ReporterConfig{
			KubeConfig: eirini.KubeConfig{
				Namespace:  fixture.Namespace,
				ConfigPath: fixture.KubeConfigPath,
			},
			EiriniCertPath: certPath,
			CAPath:         certPath,
			EiriniKeyPath:  keyPath,
		}

		session, configFile = util.RunBinary(pathToStagingReporter, config)

		taskDesirer = k8s.TaskDesirer{
			Namespace:          fixture.Namespace,
			ServiceAccountName: "",
			JobClient:          fixture.Clientset.BatchV1().Jobs(fixture.Namespace),
			Logger:             lagertest.NewTestLogger("staging-reporter-test"),
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

					var taskCompletedRequest cf.TaskCompletedRequest
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
