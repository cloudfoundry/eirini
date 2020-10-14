package instance_index_injector_test

import (
	"encoding/json"
	"io/ioutil"
	"os"
	"testing"

	"code.cloudfoundry.org/eirini/tests"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	_ "k8s.io/client-go/plugin/pkg/client/auth"
)

func TestInstanceIndexInjector(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "InstanceIndexInjector Suite")
}

var (
	fixture    *tests.Fixture
	eiriniBins tests.EiriniBinaries
	binsPath   string
)

var _ = SynchronizedBeforeSuite(func() []byte {
	var err error

	binsPath, err = ioutil.TempDir("", "bins")
	Expect(err).NotTo(HaveOccurred())

	eiriniBins = tests.NewEiriniBinaries(binsPath)

	data, err := json.Marshal(eiriniBins)
	Expect(err).NotTo(HaveOccurred())

	return data
}, func(data []byte) {
	err := json.Unmarshal(data, &eiriniBins)
	Expect(err).NotTo(HaveOccurred())

	fixture = tests.NewFixture(GinkgoWriter)
})

var _ = SynchronizedAfterSuite(func() {
	fixture.Destroy()
}, func() {
	eiriniBins.TearDown()
	Expect(os.RemoveAll(binsPath)).To(Succeed())
})

var _ = BeforeEach(func() {
	fixture.SetUp()
})

var _ = AfterEach(func() {
	fixture.TearDown()
})
