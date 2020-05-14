package staging_reporter_test

import (
	"encoding/json"
	"io/ioutil"
	"net/http"
	"os"
	"os/exec"

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
		configFile   *os.File
		session      *gexec.Session
		taskDesirer  k8s.TaskDesirer
	)

	BeforeEach(func() {
		var err error

		eiriniServer, err = util.CreateTestServer(
			util.PathToTestFixture("cert"),
			util.PathToTestFixture("key"),
			util.PathToTestFixture("cert"),
		)
		Expect(err).ToNot(HaveOccurred())
		eiriniServer.Start()

		config := defaultReporterConfig()

		configFile, err = util.CreateConfigFile(config)
		Expect(err).NotTo(HaveOccurred())

		command := exec.Command(pathToStagingReporter, "-c", configFile.Name()) // #nosec G204
		session, err = gexec.Start(command, GinkgoWriter, GinkgoWriter)
		Expect(err).ToNot(HaveOccurred())

		logger := lagertest.NewTestLogger("staging-reporter-test")
		tlsconfigs := []k8s.StagingConfigTLS{
			{
				SecretName: "cc-uploader-secret",
				KeyPaths: []k8s.KeyPath{
					{
						Key:  "foo1",
						Path: "bar1",
					},
				},
			},
			{
				SecretName: "eirini-client-secret",
				KeyPaths: []k8s.KeyPath{
					{
						Key:  "foo2",
						Path: "bar2",
					},
				},
			},
			{
				SecretName: "ca-cert-secret",
				KeyPaths: []k8s.KeyPath{
					{
						Key:  "foo3",
						Path: "bar3",
					},
				},
			},
		}
		taskDesirer = k8s.TaskDesirer{
			Namespace:          fixture.Namespace,
			TLSConfig:          tlsconfigs,
			ServiceAccountName: "",
			JobClient:          fixture.Clientset.BatchV1().Jobs(fixture.Namespace),
			Logger:             logger,
		}
	})

	AfterEach(func() {
		if session != nil {
			session.Kill()
		}
		if configFile != nil {
			os.Remove(configFile.Name())
		}
		eiriniServer.Close()
	})

	Context("When a staging job crashes", func() {

		It("notifies eirini of a staging failure", func() {
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

			stagingTask := opi.StagingTask{
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

			Expect(taskDesirer.DesireStaging(&stagingTask)).To(Succeed())
			Eventually(eiriniServer.ReceivedRequests, "10s").Should(HaveLen(1))
		})
	})
})
