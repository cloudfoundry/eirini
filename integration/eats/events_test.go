package eats_test

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"

	"code.cloudfoundry.org/eirini"
	"code.cloudfoundry.org/eirini/integration/util"
	"code.cloudfoundry.org/eirini/models/cf"
	"code.cloudfoundry.org/runtimeschema/cc_messages"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"
	"github.com/onsi/gomega/ghttp"
)

var _ = Describe("Events", func() {
	var (
		eventsConfigFile string
		eventsSession    *gexec.Session

		capiServer *ghttp.Server
		certPath   string
		keyPath    string
	)

	BeforeEach(func() {
		var err error
		certPath, keyPath = util.GenerateKeyPair("capi")
		capiServer, err = util.CreateTestServer(
			certPath, keyPath, certPath,
		)
		Expect(err).NotTo(HaveOccurred())
		capiServer.Start()

		config := &eirini.EventReporterConfig{
			KubeConfig: eirini.KubeConfig{
				Namespace:  fixture.DefaultNamespace,
				ConfigPath: fixture.KubeConfigPath,
			},
			CcInternalAPI: capiServer.URL(),
			CCCertPath:    certPath,
			CCKeyPath:     keyPath,
			CCCAPath:      certPath,
		}
		eventsSession, eventsConfigFile = util.RunBinary(binPaths.EventsReporter, config)
	})

	AfterEach(func() {
		if eventsSession != nil {
			eventsSession.Kill()
		}
		Expect(os.Remove(eventsConfigFile)).To(Succeed())
		Expect(os.Remove(certPath)).To(Succeed())
		Expect(os.Remove(keyPath)).To(Succeed())
		capiServer.Close()
	})

	Context("When an app crashes", func() {

		var lrp cf.DesireLRPRequest

		BeforeEach(func() {
			lrp = cf.DesireLRPRequest{
				GUID:         "the-app-guid",
				Version:      "the-version",
				NumInstances: 1,
				Ports:        []int32{8080},
				Lifecycle: cf.Lifecycle{
					DockerLifecycle: &cf.DockerLifecycle{
						Image: "alpine",
						Command: []string{
							"/bin/sh",
							"-c",
							"exit 1",
						},
					},
				},
				MemoryMB: 200,
				DiskMB:   300,
			}
		})

		JustBeforeEach(func() {
			processGUID := fmt.Sprintf("%s-%s", lrp.GUID, lrp.Version)
			capiServer.RouteToHandler(
				"POST",
				fmt.Sprintf("/internal/v4/apps/%s/crashed", processGUID),
				func(w http.ResponseWriter, req *http.Request) {
					bytes, err := ioutil.ReadAll(req.Body)
					Expect(err).NotTo(HaveOccurred())
					request := &cc_messages.AppCrashedRequest{}
					Expect(json.Unmarshal(bytes, request)).To(Succeed())

					Expect(request.Instance).To(ContainSubstring(lrp.GUID))
				},
			)

			Expect(desireLRP(lrp).StatusCode).To(Equal(http.StatusAccepted))
		})

		It("should generate and send a crash event", func() {
			Eventually(capiServer.ReceivedRequests).Should(HaveLen(1))
		})
	})
})
