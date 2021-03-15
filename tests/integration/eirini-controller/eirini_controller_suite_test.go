package eirini_controller_test

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"testing"
	"time"

	"code.cloudfoundry.org/eirini"
	"code.cloudfoundry.org/eirini/k8s/stset"
	"code.cloudfoundry.org/eirini/tests"
	"code.cloudfoundry.org/eirini/tests/integration"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	_ "k8s.io/client-go/plugin/pkg/client/auth"
)

func TestEiriniController(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "EiriniController Suite")
}

var (
	eiriniBins     integration.EiriniBinaries
	fixture        *tests.Fixture
	configFilePath string
	session        *gexec.Session
	config         *eirini.ControllerConfig
)

var _ = SynchronizedBeforeSuite(func() []byte {
	eiriniBins = integration.NewEiriniBinaries()
	eiriniBins.EiriniController.Build()

	data, err := json.Marshal(eiriniBins)
	Expect(err).NotTo(HaveOccurred())

	return data
}, func(data []byte) {
	err := json.Unmarshal(data, &eiriniBins)
	Expect(err).NotTo(HaveOccurred())

	fixture = tests.NewFixture(GinkgoWriter)

	SetDefaultEventuallyTimeout(30 * time.Second)
})

var _ = SynchronizedAfterSuite(func() {
	fixture.Destroy()
}, func() {
	eiriniBins.TearDown()
})

var _ = BeforeEach(func() {
	fixture.SetUp()
	config = integration.DefaultControllerConfig(fixture.Namespace)
})

var _ = JustBeforeEach(func() {
	session, configFilePath = eiriniBins.EiriniController.Run(config)
})

var _ = AfterEach(func() {
	Expect(os.Remove(configFilePath)).To(Succeed())
	session.Kill()

	fixture.TearDown()
})

func getPodReadiness(lrpGUID, lrpVersion string) bool {
	pods, err := fixture.Clientset.CoreV1().Pods(fixture.Namespace).List(context.Background(), metav1.ListOptions{
		LabelSelector: fmt.Sprintf("%s=%s,%s=%s", stset.LabelGUID, lrpGUID, stset.LabelVersion, lrpVersion),
	})
	Expect(err).NotTo(HaveOccurred())

	if len(pods.Items) != 1 {
		return false
	}

	containerStatuses := pods.Items[0].Status.ContainerStatuses
	if len(containerStatuses) == 0 {
		return false
	}

	for _, cs := range containerStatuses {
		if cs.Ready == false {
			return false
		}
	}

	return true
}
