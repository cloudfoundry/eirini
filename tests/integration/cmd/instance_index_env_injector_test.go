package cmd_test

import (
	"context"
	"fmt"
	"net"
	"os"

	"code.cloudfoundry.org/eirini"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = Describe("InstanceIndexEnvInjector", func() {
	var (
		config         *eirini.InstanceIndexEnvInjectorConfig
		configFilePath string
		session        *gexec.Session
	)

	BeforeEach(func() {
		config = &eirini.InstanceIndexEnvInjectorConfig{
			KubeConfig: eirini.KubeConfig{
				ConfigPath: fixture.KubeConfigPath,
			},
			ServiceName:                "foo",
			ServiceNamespace:           "default",
			ServicePort:                8080,
			EiriniXOperatorFingerprint: "cmd-test",
		}
	})

	JustBeforeEach(func() {
		session, configFilePath = eiriniBins.InstanceIndexEnvInjector.Run(config)
	})

	AfterEach(func() {
		Expect(fixture.Clientset.AdmissionregistrationV1().MutatingWebhookConfigurations().
			DeleteCollection(context.Background(), metav1.DeleteOptions{}, metav1.ListOptions{
				FieldSelector: "metadata.name=cmd-test-mutating-hook",
			}),
		).To(Succeed())

		if configFilePath != "" {
			Expect(os.Remove(configFilePath)).To(Succeed())
		}

		if session != nil {
			Eventually(session.Kill()).Should(gexec.Exit())
		}
	})

	It("starts properly with valid config", func() {
		var hook *admissionregistrationv1.MutatingWebhookConfiguration

		Eventually(func() error {
			var err error
			hook, err = fixture.Clientset.AdmissionregistrationV1().MutatingWebhookConfigurations().
				Get(context.Background(), "cmd-test-mutating-hook", metav1.GetOptions{})

			return err
		}).Should(Succeed())

		Expect(hook.Webhooks).To(HaveLen(1))

		Eventually(func() error {
			_, err := net.Dial("tcp", fmt.Sprintf(":%d", config.ServicePort))

			return err
		}).Should(Succeed())
	})

	When("the config file doesn't exist", func() {
		It("exits reporting missing config file", func() {
			session = eiriniBins.InstanceIndexEnvInjector.Restart("/does/not/exist", session)
			Eventually(session).Should(gexec.Exit())
			Expect(session.ExitCode).ToNot(BeZero())
			Expect(session.Err).To(gbytes.Say("failed to read file"))
		})
	})

	When("service namespace is missing in the config", func() {
		BeforeEach(func() {
			config.ServiceNamespace = ""
		})

		It("fails", func() {
			Eventually(session, "10s").Should(gexec.Exit())
			Expect(session.ExitCode()).NotTo(BeZero())
			Expect(session.Err).To(gbytes.Say("setting up the webhook server certificate: an empty namespace may not be set when a resource name is provided"))
		})
	})
})
