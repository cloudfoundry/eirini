package crash_test

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"os/exec"

	"code.cloudfoundry.org/eirini/integration/util"
	"code.cloudfoundry.org/eirini/k8s"
	"code.cloudfoundry.org/eirini/models/cf"
	"code.cloudfoundry.org/eirini/opi"
	"code.cloudfoundry.org/lager/lagertest"
	"code.cloudfoundry.org/tlsconfig"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"
	"github.com/onsi/gomega/ghttp"
)

var _ = Describe("Crashes", func() {

	var (
		capiServer *ghttp.Server
		configFile *os.File
		session    *gexec.Session

		desirer     opi.Desirer
		crashingLRP *opi.LRP
	)

	BeforeEach(func() {
		capiServer = createTestServer(
			util.PathToTestFixture("cert"),
			util.PathToTestFixture("key"),
			util.PathToTestFixture("cert"),
		)
		capiServer.Start()

		config := defaultEventReporterConfig()
		config.CcInternalAPI = capiServer.URL()

		var err error
		configFile, err = createEventReporterConfigFile(config)
		Expect(err).NotTo(HaveOccurred())

		command := exec.Command(pathToCrashEmitter, "-c", configFile.Name()) // #nosec G204
		session, err = gexec.Start(command, GinkgoWriter, GinkgoWriter)
		Expect(err).ToNot(HaveOccurred())

		logger := lagertest.NewTestLogger("crash-event-logger-test")
		desirer = k8s.NewStatefulSetDesirer(
			clientset,
			namespace,
			"registry-secret",
			"rootfsversion",
			"default",
			"default",
			logger,
		)
	})

	AfterEach(func() {
		if session != nil {
			session.Kill()
		}
		if configFile != nil {
			os.Remove(configFile.Name())
		}
		capiServer.Close()
	})

	Context("When an app crashes", func() {

		BeforeEach(func() {
			crashingLRP = createCrashingLRP("ödin")
			Expect(desirer.Desire(crashingLRP)).To(Succeed())
		})

		It("generates crash report when the app goes into CrashLoopBackOff", func() {
			capiServer.RouteToHandler(
				"POST",
				fmt.Sprintf("/internal/v4/apps/%s/crashed", crashingLRP.ProcessGUID()),
				func(w http.ResponseWriter, req *http.Request) {
					bytes, err := ioutil.ReadAll(req.Body)
					Expect(err).NotTo(HaveOccurred())
					Expect(string(bytes)).To(ContainSubstring(crashingLRP.GUID))
				},
			)
			Eventually(capiServer.ReceivedRequests, "10s").Should(HaveLen(2))
		})
	})
})

func createCrashingLRP(name string) *opi.LRP {
	guid := util.RandomString()
	routes, err := json.Marshal([]cf.Route{{Hostname: "foo.example.com", Port: 8080}})
	Expect(err).ToNot(HaveOccurred())
	return &opi.LRP{
		Command: []string{
			"/bin/sh",
			"-c",
			"exit 1",
		},
		AppName:         name,
		SpaceName:       "space-foo",
		TargetInstances: 1,
		Image:           "alpine",
		AppURIs:         string(routes),
		LRPIdentifier:   opi.LRPIdentifier{GUID: guid, Version: "version_" + guid},
		LRP:             "metadata",
		DiskMB:          2047,
	}
}

func createTestServer(certPath, keyPath, caCertPath string) *ghttp.Server {
	tlsConf, tlsErr := tlsconfig.Build(
		tlsconfig.WithInternalServiceDefaults(),
		tlsconfig.WithIdentityFromFile(certPath, keyPath),
	).Server(
		tlsconfig.WithClientAuthenticationFromFile(caCertPath),
	)
	Expect(tlsErr).NotTo(HaveOccurred())

	testServer := ghttp.NewUnstartedServer()
	testServer.HTTPTestServer.TLS = tlsConf

	return testServer
}
