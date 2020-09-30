package events_test

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"

	"code.cloudfoundry.org/eirini"
	"code.cloudfoundry.org/eirini/k8s"
	"code.cloudfoundry.org/eirini/k8s/client"
	"code.cloudfoundry.org/eirini/opi"
	"code.cloudfoundry.org/eirini/tests"
	"code.cloudfoundry.org/lager/lagertest"
	"code.cloudfoundry.org/runtimeschema/cc_messages"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"
	"github.com/onsi/gomega/ghttp"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = Describe("Events", func() {
	var (
		eventsConfigFile string
		eventsSession    *gexec.Session

		capiServer *ghttp.Server
		certPath   string
		keyPath    string
		logger     *lagertest.TestLogger
		config     *eirini.EventReporterConfig
	)

	BeforeEach(func() {
		var err error
		logger = lagertest.NewTestLogger("events")

		certPath, keyPath = tests.GenerateKeyPair("capi")
		capiServer, err = tests.CreateTestServer(
			certPath, keyPath, certPath,
		)
		Expect(err).NotTo(HaveOccurred())
		capiServer.HTTPTestServer.StartTLS()

		config = &eirini.EventReporterConfig{
			KubeConfig: eirini.KubeConfig{
				Namespace:                   fixture.Namespace,
				EnableMultiNamespaceSupport: false,
				ConfigPath:                  fixture.KubeConfigPath,
			},
			CcInternalAPI: capiServer.URL(),
			CCCertPath:    certPath,
			CCKeyPath:     keyPath,
			CCCAPath:      certPath,
		}
	})

	AfterEach(func() {
		if eventsSession != nil {
			eventsSession.Kill()
		}
		Expect(os.Remove(eventsConfigFile)).To(Succeed())
		Expect(os.Remove(certPath)).To(Succeed())
		Expect(os.Remove(keyPath)).To(Succeed())
		capiServer.Close()
	})

	JustBeforeEach(func() {
		eventsSession, eventsConfigFile = eiriniBins.EventsReporter.Run(config)
		Eventually(eventsSession).Should(gbytes.Say("Starting workers"))
	})

	Describe("LRP events", func() {
		var (
			lrpDesirer *k8s.StatefulSetDesirer
			lrp        opi.LRP
			lrpCommand []string
		)

		BeforeEach(func() {
			lrpDesirer = &k8s.StatefulSetDesirer{
				Pods:                      client.NewPod(fixture.Clientset, "", true),
				Secrets:                   client.NewSecret(fixture.Clientset),
				StatefulSets:              client.NewStatefulSet(fixture.Clientset, "", true),
				PodDisruptionBudgets:      client.NewPodDisruptionBudget(fixture.Clientset),
				EventsClient:              client.NewEvent(fixture.Clientset, "", true),
				StatefulSetToLRPMapper:    k8s.StatefulSetToLRP,
				RegistrySecretName:        "registry-secret",
				LivenessProbeCreator:      k8s.CreateLivenessProbe,
				ReadinessProbeCreator:     k8s.CreateReadinessProbe,
				Logger:                    logger,
				ApplicationServiceAccount: tests.GetApplicationServiceAccount(),
			}
		})

		JustBeforeEach(func() {
			lrp = opi.LRP{
				Command:         lrpCommand,
				TargetInstances: 1,
				Image:           "busybox",
				LRPIdentifier:   opi.LRPIdentifier{GUID: tests.GenerateGUID(), Version: tests.GenerateGUID()},
			}
			Expect(lrpDesirer.Desire(fixture.Namespace, &lrp)).To(Succeed())
		})

		When("the LRP crashes", func() {
			BeforeEach(func() {
				lrpCommand = []string{"exit", "1"}
			})

			JustBeforeEach(func() {
				processGUID := fmt.Sprintf("%s-%s", lrp.GUID, lrp.Version)
				capiServer.RouteToHandler(
					"POST",
					fmt.Sprintf("/internal/v4/apps/%s/crashed", processGUID),
					func(w http.ResponseWriter, req *http.Request) {
						bytes, err := ioutil.ReadAll(req.Body)
						Expect(err).NotTo(HaveOccurred())
						request := &cc_messages.AppCrashedRequest{}
						Expect(json.Unmarshal(bytes, request)).To(Succeed())

						Expect(request.Instance).To(ContainSubstring(lrp.GUID))
					},
				)
			})

			It("should generate and send a crash event", func() {
				Eventually(capiServer.ReceivedRequests).Should(HaveLen(1))
			})

			Context("when TLS to Cloud Controller is disabled", func() {
				var noTLSCapiServer *ghttp.Server

				BeforeEach(func() {
					noTLSCapiServer = ghttp.NewServer()
					noTLSCapiServer.AllowUnhandledRequests = true

					config = &eirini.EventReporterConfig{
						KubeConfig: eirini.KubeConfig{
							Namespace:  fixture.Namespace,
							ConfigPath: fixture.KubeConfigPath,
						},
						CcInternalAPI: noTLSCapiServer.URL(),
						CCTLSDisabled: true,
					}
				})

				AfterEach(func() {
					noTLSCapiServer.Close()
				})

				It("should generate and send a crash event", func() {
					Eventually(noTLSCapiServer.ReceivedRequests).Should(HaveLen(1))
				})
			})
		})

		When("the LRP does not crash", func() {
			BeforeEach(func() {
				lrpCommand = []string{"sleep", "100"}
			})

			It("should not send crash events", func() {
				Consistently(capiServer.ReceivedRequests).Should(HaveLen(0))
			})
		})
	})

	Describe("Task events", func() {
		var taskDesirer *k8s.TaskDesirer

		BeforeEach(func() {
			taskDesirer = k8s.NewTaskDesirer(
				logger,
				client.NewJob(fixture.Clientset, "", true),
				nil,
				fixture.Namespace,
				nil,
				tests.GetApplicationServiceAccount(),
				"",
				"",
				false,
			)
		})

		JustBeforeEach(func() {
			task := opi.Task{
				Command: []string{"exit", "1"},
				Image:   "busybox",
				GUID:    tests.GenerateGUID(),
			}
			Expect(taskDesirer.Desire(fixture.Namespace, &task)).To(Succeed())
		})

		It("should not send crash events", func() {
			Consistently(capiServer.ReceivedRequests).Should(HaveLen(0))
		})
	})

	Describe("Events from non-eirini pods", func() {
		BeforeEach(func() {
			pod := &v1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name: "app-0",
				},
				Spec: v1.PodSpec{
					Containers: []v1.Container{
						{
							Name:    "potato",
							Image:   "busybox",
							Command: []string{"exit", "1"},
						},
					},
				},
			}
			_, err := fixture.Clientset.CoreV1().Pods(fixture.Namespace).Create(
				context.Background(),
				pod,
				metav1.CreateOptions{})
			Expect(err).NotTo(HaveOccurred())
		})

		It("should not send crash events", func() {
			Consistently(capiServer.ReceivedRequests).Should(HaveLen(0))
		})
	})
})
