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
	"code.cloudfoundry.org/eirini/tests"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"
	_ "k8s.io/client-go/plugin/pkg/client/auth"
)

func TestOpi(t *testing.T) {
	SetDefaultEventuallyTimeout(time.Minute)
	RegisterFailHandler(Fail)
	RunSpecs(t, "Opi Suite")
}

const secretName = "certs-secret"

var (
	eiriniBins           tests.EiriniBinaries
	binsPath             string
	fixture              *tests.Fixture
	httpClient           *http.Client
	eiriniConfigFilePath string
	session              *gexec.Session
	url                  string
	certPath             string
	keyPath              string
	eiriniConfig         *eirini.Config
)

var _ = SynchronizedBeforeSuite(func() []byte {
	var err error
	binsPath, err = ioutil.TempDir("", "bins")
	Expect(err).NotTo(HaveOccurred())

	eiriniBins = tests.NewEiriniBinaries(binsPath)
	eiriniBins.OPI.Build()

	data, err := json.Marshal(eiriniBins)
	Expect(err).NotTo(HaveOccurred())

	return data
}, func(data []byte) {
	err := json.Unmarshal(data, &eiriniBins)
	Expect(err).NotTo(HaveOccurred())

	fixture = tests.NewFixture(GinkgoWriter)
	certPath, keyPath = tests.GenerateKeyPair("capi")
})

var _ = SynchronizedAfterSuite(func() {
	fixture.Destroy()
}, func() {
	eiriniBins.TearDown()
	Expect(os.RemoveAll(binsPath)).To(Succeed())
})

var _ = BeforeEach(func() {
	fixture.SetUp()

	Expect(tests.CreateEmptySecret(fixture.Namespace, secretName, fixture.Clientset)).To(Succeed())

	var err error
	httpClient, err = tests.MakeTestHTTPClient()
	Expect(err).ToNot(HaveOccurred())

	eiriniConfig = tests.DefaultEiriniConfig(fixture.Namespace, fixture.NextAvailablePort())
	eiriniConfig.Properties.CCCertPath = certPath
	eiriniConfig.Properties.CCKeyPath = keyPath
	eiriniConfig.Properties.CCCAPath = certPath
})

var _ = JustBeforeEach(func() {
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
