package cmd_test

import (
	"context"
	"fmt"
	"net"
	"os"

	"code.cloudfoundry.org/eirini"
	"code.cloudfoundry.org/eirini/tests"
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
		fingerprint    string
		hook           *admissionregistrationv1.MutatingWebhookConfiguration
	)

	BeforeEach(func() {
		fingerprint = tests.GenerateGUID()
		config = &eirini.InstanceIndexEnvInjectorConfig{
			KubeConfig: eirini.KubeConfig{
				ConfigPath: fixture.KubeConfigPath,
			},
			ServiceName:                "foo",
			ServiceNamespace:           "default",
			ServicePort:                int32(8080 + GinkgoParallelNode()),
			EiriniXOperatorFingerprint: fingerprint,
		}
	})

	AfterEach(func() {
		if configFilePath != "" {
			Expect(os.Remove(configFilePath)).To(Succeed())
		}

		if session != nil {
			Eventually(session.Kill()).Should(gexec.Exit())
		}
	})

	getHook := func() error {
		var err error
		hook, err = fixture.Clientset.AdmissionregistrationV1().MutatingWebhookConfigurations().
			Get(context.Background(), fingerprint+"-mutating-hook", metav1.GetOptions{})

		return err
	}

	hookIsRegistered := func() {
		Eventually(getHook, "20s").Should(Succeed())
		Expect(hook.Webhooks).To(HaveLen(1))
	}

	hookIsNotRegistered := func() {
		ConsistentlyWithOffset(1, getHook, "2s").ShouldNot(Succeed())
	}

	dialService := func() error {
		_, err := net.Dial("tcp", fmt.Sprintf(":%d", config.ServicePort))

		return err
	}

	serviceIsRunning := func() {
		Eventually(dialService).Should(Succeed())
	}

	serviceIsNotRunning := func() {
		Consistently(dialService).ShouldNot(Succeed(), "2s")
	}

	Describe("register and execute (default options)", func() {
		JustBeforeEach(func() {
			eiriniBins.InstanceIndexEnvInjector.ExtraArgs = []string{}
			session, configFilePath = eiriniBins.InstanceIndexEnvInjector.Run(config)
		})

		It("runs the webhook service and registers it", func() {
			hookIsRegistered()
			serviceIsRunning()

			Consistently(session).ShouldNot(gexec.Exit())
		})

		When("the config file doesn't exist", func() {
			It("exits reporting missing config file", func() {
				session = eiriniBins.InstanceIndexEnvInjector.Restart("/does/not/exist", session)
				Eventually(session).Should(gexec.Exit())
				Expect(session.ExitCode).ToNot(BeZero())
				Expect(session.Err).To(gbytes.Say("Failed to read config file: failed to read file"))
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

	Describe("Hook Registration", func() {
		JustBeforeEach(func() {
			eiriniBins.InstanceIndexEnvInjector.ExtraArgs = []string{"--register-only"}
			session, configFilePath = eiriniBins.InstanceIndexEnvInjector.Run(config)
		})

		AfterEach(func() {
			Expect(fixture.Clientset.AdmissionregistrationV1().MutatingWebhookConfigurations().
				DeleteCollection(context.Background(), metav1.DeleteOptions{}, metav1.ListOptions{
					FieldSelector: fmt.Sprintf("metadata.name=%s-mutating-hook", fingerprint),
				}),
			).To(Succeed())
		})

		It("registers webhook but doesn't run service", func() {
			hookIsRegistered()
			serviceIsNotRunning()
			Eventually(session).Should(gexec.Exit(0))
		})
	})

	Describe("Hook service execution", func() {
		JustBeforeEach(func() {
			eiriniBins.InstanceIndexEnvInjector.ExtraArgs = []string{"--execute-only"}
			session, configFilePath = eiriniBins.InstanceIndexEnvInjector.Run(config)
		})

		It("runs the webhook service but does not register it", func() {
			hookIsNotRegistered()
			serviceIsRunning()

			Consistently(session).ShouldNot(gexec.Exit())
		})
	})
})
