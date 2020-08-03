package opi_test

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"testing"
	"time"

	"code.cloudfoundry.org/eirini"
	"code.cloudfoundry.org/eirini/integration/util"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"
	"gopkg.in/yaml.v2"
	_ "k8s.io/client-go/plugin/pkg/client/auth"
)

func TestOpi(t *testing.T) {
	SetDefaultEventuallyTimeout(time.Minute)
	RegisterFailHandler(Fail)
	RunSpecs(t, "Opi Suite")
}

const secretName = "certs-secret"

var (
	eiriniBins           util.EiriniBinaries
	binsPath             string
	fixture              *util.Fixture
	httpClient           *http.Client
	eiriniConfigFilePath string
	session              *gexec.Session
	url                  string
	certPath             string
	keyPath              string
)

var _ = SynchronizedBeforeSuite(func() []byte {
	var err error
	binsPath, err = ioutil.TempDir("", "bins")
	Expect(err).NotTo(HaveOccurred())

	eiriniBins = util.NewEiriniBinaries(binsPath)
	eiriniBins.OPI.Build()

	data, err := json.Marshal(eiriniBins)
	Expect(err).NotTo(HaveOccurred())

	return data
}, func(data []byte) {
	err := json.Unmarshal(data, &eiriniBins)
	Expect(err).NotTo(HaveOccurred())

	fixture = util.NewFixture(GinkgoWriter)
	certPath, keyPath = util.GenerateKeyPair("capi")
})

var _ = SynchronizedAfterSuite(func() {}, func() {
	eiriniBins.TearDown()
	Expect(os.RemoveAll(binsPath)).To(Succeed())
})

var _ = BeforeEach(func() {
	fixture.SetUp()

	Expect(util.CreateEmptySecret(fixture.Namespace, secretName, fixture.Clientset)).To(Succeed())

	var err error
	httpClient, err = util.MakeTestHTTPClient()
	Expect(err).ToNot(HaveOccurred())

	eiriniConfig := util.DefaultEiriniConfig(fixture.Namespace, fixture.NextAvailablePort())
	eiriniConfig.Properties.CCCertPath = certPath
	eiriniConfig.Properties.CCKeyPath = keyPath
	eiriniConfig.Properties.CCCAPath = certPath

	session, eiriniConfigFilePath = eiriniBins.OPI.Run(eiriniConfig)

	url = fmt.Sprintf("https://localhost:%d/", eiriniConfig.Properties.TLSPort)
	Eventually(func() error {
		_, getErr := httpClient.Get(url)
		return getErr
	}).Should(Succeed())
})

var _ = AfterEach(func() {
	Expect(os.Remove(eiriniConfigFilePath)).To(Succeed())
	session.Kill()

	fixture.TearDown()
})

func restartWithConfig(updateConfig func(cfg eirini.Config) eirini.Config) string {
	configBytes, err := ioutil.ReadFile(eiriniConfigFilePath)
	Expect(err).NotTo(HaveOccurred())

	var eiriniConfig eirini.Config

	Expect(yaml.Unmarshal(configBytes, &eiriniConfig)).To(Succeed())

	newConfig := updateConfig(eiriniConfig)

	configBytes, err = yaml.Marshal(newConfig)
	Expect(err).NotTo(HaveOccurred())
	newConfigFile, err := ioutil.TempFile("", "")
	Expect(err).NotTo(HaveOccurred())
	Expect(ioutil.WriteFile(newConfigFile.Name(), configBytes, 0600)).To(Succeed())

	session = eiriniBins.OPI.Restart(newConfigFile.Name(), session)

	Eventually(func() error {
		resp, getErr := httpClient.Get(url)
		if getErr == nil {
			defer resp.Body.Close()
		}
		return getErr
	}).Should(Succeed())

	return newConfigFile.Name()
}
