package config_updater_test

import (
	"encoding/json"
	"io/ioutil"
	"os"
	"testing"
	"time"

	"code.cloudfoundry.org/eirini/integration/util"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func TestConfigUpdater(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "ConfigUpdater Suite")
}

var (
	fixture    *util.Fixture
	eiriniBins util.EiriniBinaries
	binsPath   string
)

var _ = SynchronizedBeforeSuite(func() []byte {
	var err error
	binsPath, err = ioutil.TempDir("", "bins")
	Expect(err).NotTo(HaveOccurred())

	eiriniBins = util.NewEiriniBinaries(binsPath)
	eiriniBins.TaskReporter.Build()

	data, err := json.Marshal(eiriniBins)
	Expect(err).NotTo(HaveOccurred())

	return data
}, func(data []byte) {
	err := json.Unmarshal(data, &eiriniBins)
	Expect(err).NotTo(HaveOccurred())

	fixture = util.NewFixture(GinkgoWriter)
})

var _ = SynchronizedAfterSuite(func() {}, func() {
	eiriniBins.TearDown()
	Expect(os.RemoveAll(binsPath)).To(Succeed())
})

var _ = BeforeEach(func() {
	SetDefaultEventuallyTimeout(10 * time.Second)
	fixture.SetUp()
})

var _ = AfterEach(func() {
	fixture.TearDown()
})
