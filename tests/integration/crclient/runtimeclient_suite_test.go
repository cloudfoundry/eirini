package integration_test

import (
	"context"
	"testing"
	"time"

	eiriniv1 "code.cloudfoundry.org/eirini/pkg/apis/eirini/v1"
	"code.cloudfoundry.org/eirini/tests"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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

func TestRuntimeclient(t *testing.T) {
	SetDefaultEventuallyTimeout(4 * time.Minute)
	RegisterFailHandler(Fail)
	RunSpecs(t, "Runtime Client")
}

func createTask(ns, name string) *eiriniv1.Task {
	task := &eiriniv1.Task{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		Spec: eiriniv1.TaskSpec{
			Command: []string{"echo"},
		},
	}
	task, err := fixture.EiriniClientset.EiriniV1().Tasks(ns).Create(context.Background(), task, metav1.CreateOptions{})

	Expect(err).NotTo(HaveOccurred())

	return task
}
