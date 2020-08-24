package eats_test

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"

	"code.cloudfoundry.org/eirini"
	"code.cloudfoundry.org/eirini/models/cf"
	"code.cloudfoundry.org/eirini/tests"
	"code.cloudfoundry.org/runtimeschema/cc_messages"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"
	"github.com/onsi/gomega/ghttp"
	"gopkg.in/yaml.v2"
)

var _ = Describe("Events", func() {
	var (
		eventsConfigFile string
		eventsSession    *gexec.Session

		capiServer *ghttp.Server
		certPath   string
		keyPath    string
	)

	restartWithConfig := func(updateConfig func(cfg eirini.EventReporterConfig) eirini.EventReporterConfig) string {
		configBytes, err := ioutil.ReadFile(eventsConfigFile)
		Expect(err).NotTo(HaveOccurred())
		var eventsConfig eirini.EventReporterConfig
		Expect(yaml.Unmarshal(configBytes, &eventsConfig)).To(Succeed())

		newConfig := updateConfig(eventsConfig)

		configBytes, err = yaml.Marshal(newConfig)
		Expect(err).NotTo(HaveOccurred())
		newConfigFile, err := ioutil.TempFile("", "")
		Expect(err).NotTo(HaveOccurred())
		Expect(ioutil.WriteFile(newConfigFile.Name(), configBytes, 0o600)).To(Succeed())

		eventsSession = eiriniBins.EventsReporter.Restart(newConfigFile.Name(), eventsSession)

		return newConfigFile.Name()
	}

	BeforeEach(func() {
		var err error
		certPath, keyPath = tests.GenerateKeyPair("capi")
		capiServer, err = tests.CreateTestServer(
			certPath, keyPath, certPath,
		)
		Expect(err).NotTo(HaveOccurred())
		capiServer.HTTPTestServer.StartTLS()

		config := &eirini.EventReporterConfig{
			KubeConfig: eirini.KubeConfig{
				Namespace:  fixture.Namespace,
				ConfigPath: fixture.KubeConfigPath,
			},
			CcInternalAPI: capiServer.URL(),
			CCCertPath:    certPath,
			CCKeyPath:     keyPath,
			CCCAPath:      certPath,
		}
		eventsSession, eventsConfigFile = eiriniBins.EventsReporter.Run(config)
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
				Namespace:    fixture.Namespace,
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

		When("CC TLS is disabled in the config", func() {
			var configPath string

			BeforeEach(func() {
				capiServer.Close()
				capiServer = ghttp.NewServer()

				configPath = restartWithConfig(func(cfg eirini.EventReporterConfig) eirini.EventReporterConfig {
					cfg.CCTLSDisabled = true
					cfg.CCCertPath = ""
					cfg.CCKeyPath = ""
					cfg.CCCAPath = ""
					cfg.CcInternalAPI = capiServer.URL()

					return cfg
				})
			})
			AfterEach(func() {
				os.RemoveAll(configPath)
			})

			It("should generate and send a crash event", func() {
				Eventually(capiServer.ReceivedRequests).Should(HaveLen(1))
			})
		})
	})
})
