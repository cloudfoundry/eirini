package integration_test

import (
	"context"
	"testing"

	"code.cloudfoundry.org/eirini/tests"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	_ "k8s.io/client-go/plugin/pkg/client/auth"
)

var (
	fixture *tests.Fixture
	ctx     context.Context
)

var _ = BeforeSuite(func() {
	fixture = tests.NewFixture(GinkgoWriter)
})

var _ = BeforeEach(func() {
	fixture.SetUp()
	ctx = context.Background()
})

var _ = AfterEach(func() {
	fixture.TearDown()
})

var _ = AfterSuite(func() {
	fixture.Destroy()
})

func TestIntegration(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Integration Suite")
}
