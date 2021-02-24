package eirini_controller_test

import (
	"encoding/json"
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

func TestEiriniController(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "EiriniController Suite")
}

var (
	eiriniBins     tests.EiriniBinaries
	fixture        *tests.Fixture
	configFilePath string
	session        *gexec.Session
	config         *eirini.ControllerConfig
)

var _ = SynchronizedBeforeSuite(func() []byte {
	eiriniBins = tests.NewEiriniBinaries()
	eiriniBins.EiriniController.Build()

	data, err := json.Marshal(eiriniBins)
	Expect(err).NotTo(HaveOccurred())

	return data
}, func(data []byte) {
	err := json.Unmarshal(data, &eiriniBins)
	Expect(err).NotTo(HaveOccurred())

	fixture = tests.NewFixture(GinkgoWriter)

	SetDefaultEventuallyTimeout(2 * time.Minute)
})

var _ = SynchronizedAfterSuite(func() {
	fixture.Destroy()
}, func() {
	eiriniBins.TearDown()
})

var _ = BeforeEach(func() {
	fixture.SetUp()

	config = tests.DefaultControllerConfig(fixture.Namespace)
})

var _ = JustBeforeEach(func() {
	session, configFilePath = eiriniBins.EiriniController.Run(config)
})

var _ = AfterEach(func() {
	Expect(os.Remove(configFilePath)).To(Succeed())
	session.Kill()

	fixture.TearDown()
})
