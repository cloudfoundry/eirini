package cmd_test

import (
	"context"
	"fmt"
	"net"
	"os"
	"syscall"

	"code.cloudfoundry.org/eirini"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = Describe("InstanceIndexEnvInjector", func() {
	var (
		config         *eirini.InstanceIndexEnvInjectorConfig
		configFilePath string
		session        *gexec.Session
	)

	JustBeforeEach(func() {
		session, configFilePath = eiriniBins.InstanceIndexEnvInjector.Run(config)
	})

	AfterEach(func() {
		if configFilePath != "" {
			Expect(os.Remove(configFilePath)).To(Succeed())
		}
		if session != nil {
			Eventually(session.Kill()).Should(gexec.Exit())
		}

		Expect(fixture.Clientset.AdmissionregistrationV1().MutatingWebhookConfigurations().Delete(context.Background(), "cmd-test-mutating-hook", metav1.DeleteOptions{})).To(Succeed())
	})

	Context("When the webhook is executed with a valid config", func() {
		BeforeEach(func() {
			config = defaultInstanceIndexEnvInjectorConfig()
		})

		It("should be able to start properly", func() {
			Expect(session.Command.Process.Signal(syscall.Signal(0))).To(Succeed())
			Eventually(func() error {
				_, err := net.Dial("tcp", fmt.Sprintf(":%d", config.ServicePort))
				return err
			}).Should(Succeed())
		})
	})
})
