package instance_index_injector_test

import (
	"context"
	"net"
	"os"

	"code.cloudfoundry.org/eirini"
	"code.cloudfoundry.org/eirini/k8s"
	"code.cloudfoundry.org/eirini/tests"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = Describe("InstanceIndexInjector", func() {
	var (
		config              *eirini.InstanceIndexEnvInjectorConfig
		configFilePath      string
		hookSession         *gexec.Session
		telepresenceSession *tests.TelepresenceRunner
		pod                 *corev1.Pod
	)

	BeforeEach(func() {
		serviceName := "instance-index-env-injector"

		config = &eirini.InstanceIndexEnvInjectorConfig{
			ServiceName:      serviceName,
			ServicePort:      8443,
			ServiceNamespace: fixture.Namespace,
		}
		hookSession, configFilePath = eiriniBins.InstanceIndexEnvInjector.Run(config)
		Eventually(func() error {
			_, err := net.Dial("tcp", ":8443")

			return err
		}).Should(Succeed())

		var err error
		telepresenceSession, err = tests.StartTelepresence(fixture.Namespace, serviceName, "8443:443")
		Expect(err).NotTo(HaveOccurred())
	})

	AfterEach(func() {
		if configFilePath != "" {
			Expect(os.Remove(configFilePath)).To(Succeed())
		}
		if hookSession != nil {
			Eventually(hookSession.Kill()).Should(gexec.Exit())
		}
		if telepresenceSession != nil {
			telepresenceSession.Stop()
		}
	})

	JustBeforeEach(func() {
		var err error
		pod, err = fixture.Clientset.CoreV1().Pods(fixture.Namespace).Create(context.Background(), pod, metav1.CreateOptions{})
		Expect(err).NotTo(HaveOccurred())
	})

	getCFInstanceIndex := func(pod *corev1.Pod, containerName string) string {
		for _, container := range pod.Spec.Containers {
			if container.Name != containerName {
				continue
			}

			for _, e := range container.Env {
				if e.Name != eirini.EnvCFInstanceIndex {
					continue
				}

				return e.Value
			}
		}

		return ""
	}

	When("an eirini LRP pod is created", func() {
		BeforeEach(func() {
			pod = &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name: "app-name-0",
					Labels: map[string]string{
						k8s.LabelSourceType: "APP",
					},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  k8s.OPIContainerName,
							Image: "eirini/dorini",
						},
						{
							Name:  "not-opi",
							Image: "eirini/dorini",
						},
					},
				},
			}
		})

		It("sets CF_INSTANCE_INDEX in the opi container environment", func() {
			Expect(getCFInstanceIndex(pod, k8s.OPIContainerName)).To(Equal("0"))
		})

		It("does not set CF_INSTANCE_INDEX on the nont-opi container", func() {
			Expect(getCFInstanceIndex(pod, "not-opi")).To(Equal(""))
		})
	})
})
