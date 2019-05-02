package k8s_test

import (
	"fmt"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"

	. "code.cloudfoundry.org/eirini/k8s"
	"code.cloudfoundry.org/eirini/metrics"
	"code.cloudfoundry.org/eirini/route/routefakes"
	"code.cloudfoundry.org/lager/lagertest"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"
	core "k8s.io/client-go/kubernetes/typed/core/v1"
	testcore "k8s.io/client-go/testing"
	metricsv1beta1api "k8s.io/metrics/pkg/apis/metrics/v1beta1"
	metricsfake "k8s.io/metrics/pkg/client/clientset/versioned/fake"
	metricsv1typed "k8s.io/metrics/pkg/client/clientset/versioned/typed/metrics/v1beta1"
)

var podClient core.PodInterface

var _ = Describe("Metrics", func() {

	const podName = "thor-thunder-9000"

	var (
		collector        *MetricsCollector
		work             chan []metrics.Message
		metricsClient    *metricsfake.Clientset
		podMetricsClient metricsv1typed.PodMetricsInterface
		scheduler        *routefakes.FakeTaskScheduler
		expectedMetrics  metricsv1beta1api.PodMetricsList
		logger           *lagertest.TestLogger
		validMetrics     metricsv1beta1api.PodMetrics
		brokenMetrics    metricsv1beta1api.PodMetrics
		wrongNameMetrics metricsv1beta1api.PodMetrics
		podlessMetrics   metricsv1beta1api.PodMetrics
	)

	BeforeEach(func() {
		logger = lagertest.NewTestLogger("test-logger")
		metricsClient = &metricsfake.Clientset{}
		podMetricsClient = metricsClient.MetricsV1beta1().PodMetricses("opi")

		client := fake.NewSimpleClientset()
		podClient = client.CoreV1().Pods("opi")
		validMetrics = createPodForMetrics(podName)
		wrongNameMetrics = createPodForMetrics("iamstagingtask")
		podlessMetrics = createPodForMetrics("pod-less-0")
		Expect(podClient.Delete("pod-less-0", &metav1.DeleteOptions{})).To(Succeed())
		brokenMetrics = createPodForMetrics("broken-pod-metrics-0")
		brokenMetrics.Containers = []metricsv1beta1api.ContainerMetrics{}
	})

	JustBeforeEach(func() {
		scheduler = new(routefakes.FakeTaskScheduler)
		work = make(chan []metrics.Message, 1)
		collector = NewMetricsCollector(work, scheduler, podMetricsClient, podClient, logger)
	})

	Context("When collecting metrics", func() {
		var err error

		BeforeEach(func() {
			expectedMetrics = metricsv1beta1api.PodMetricsList{
				Items: []metricsv1beta1api.PodMetrics{validMetrics},
			}

			metricsClient.AddReactor("list", "pods", func(action testcore.Action) (handled bool, ret runtime.Object, err error) {
				return true, &expectedMetrics, nil
			})
		})

		JustBeforeEach(func() {
			collector.Start()
			task := scheduler.ScheduleArgsForCall(0)
			err = task()
		})

		It("should not return an error", func() {
			Expect(err).ToNot(HaveOccurred())
		})

		It("should send the received metrics", func() {
			Eventually(work).Should(Receive(Equal([]metrics.Message{
				{
					AppID:       "app-guid",
					IndexID:     "9000",
					CPU:         420,
					Memory:      430080,
					MemoryQuota: 10,
					Disk:        42000000,
					DiskQuota:   10,
				},
			})))
		})

		Context("there are no items", func() {
			BeforeEach(func() {
				expectedMetrics = metricsv1beta1api.PodMetricsList{}
			})

			It("should not return an error", func() {
				Expect(err).ToNot(HaveOccurred())
			})

			It("should not send anything", func() {
				Consistently(work).ShouldNot(Receive())
			})
		})

		Context("there are no containers data", func() {
			BeforeEach(func() {
				expectedMetrics = metricsv1beta1api.PodMetricsList{
					Items: []metricsv1beta1api.PodMetrics{brokenMetrics},
				}
			})

			It("should not return an error", func() {
				Expect(err).ToNot(HaveOccurred())
			})

			It("should not send anything", func() {
				Consistently(work).ShouldNot(Receive())
			})

			It("should log that situation", func() {
				Eventually(logger.Buffer()).Should(gbytes.Say(`"message":"test-logger.pod-with-no-containers"`))
				Eventually(logger.Buffer()).Should(gbytes.Say(`"pod":"broken-pod-metrics-0"`))
			})
		})

		Context("pod name doesn't have an index (eg staging tasks)", func() {
			BeforeEach(func() {
				expectedMetrics = metricsv1beta1api.PodMetricsList{
					Items: []metricsv1beta1api.PodMetrics{wrongNameMetrics},
				}
			})

			It("should not return an error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should not send a message", func() {
				Expect(work).ShouldNot(Receive())
			})

			It("should log that situation", func() {
				Eventually(logger.Buffer()).Should(gbytes.Say(`"message":"test-logger.incorrect-pod-name"`))
				Eventually(logger.Buffer()).Should(gbytes.Say(`"pod":"iamstagingtask"`))
			})
		})

		Context("metrics source responds with an error", func() {

			BeforeEach(func() {
				metricsClient.PrependReactor("list", "pods", func(action testcore.Action) (handled bool, ret runtime.Object, err error) {
					return true, nil, fmt.Errorf("Better luck next time")
				})
			})

			It("should return an error", func() {
				Expect(err).To(MatchError(ContainSubstring("Better luck next time")))
			})
		})

		Context("when pod client fails to get the pod", func() {
			BeforeEach(func() {
				expectedMetrics = metricsv1beta1api.PodMetricsList{
					Items: []metricsv1beta1api.PodMetrics{podlessMetrics},
				}
			})

			It("executes successfully", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should not send metrics", func() {
				Consistently(work).ShouldNot(Receive())
			})

			It("should log that situation", func() {
				Eventually(logger.Buffer()).Should(gbytes.Say(`"message":"test-logger.cannot-find-pod"`))
				Eventually(logger.Buffer()).Should(gbytes.Say(`"pod":"pod-less-0"`))
			})
		})

		Context("when there is a mix of broken metricsand valid metrics", func() {
			BeforeEach(func() {
				expectedMetrics = metricsv1beta1api.PodMetricsList{
					Items: []metricsv1beta1api.PodMetrics{
						brokenMetrics,
						podlessMetrics,
						validMetrics,
						wrongNameMetrics,
					},
				}

			})

			It("executes successfully", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should send metrics", func() {
				Eventually(work).Should(Receive(Equal([]metrics.Message{
					{
						AppID:       "app-guid",
						IndexID:     "9000",
						CPU:         420,
						Memory:      430080,
						MemoryQuota: 10,
						Disk:        42000000,
						DiskQuota:   10,
					},
				})))
			})
		})
	})
})

func createPodForMetrics(podName string) metricsv1beta1api.PodMetrics {
	_, createErr := podClient.Create(&v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name: podName,
			Labels: map[string]string{
				"guid": "app-guid",
			},
		},
	})
	Expect(createErr).ToNot(HaveOccurred())
	return metricsv1beta1api.PodMetrics{
		ObjectMeta: metav1.ObjectMeta{Name: podName, Namespace: "opi", ResourceVersion: "10", Labels: map[string]string{"key": "value"}},
		Containers: []metricsv1beta1api.ContainerMetrics{
			{
				Usage: v1.ResourceList{
					v1.ResourceCPU:    resource.MustParse("420000m"),
					v1.ResourceMemory: resource.MustParse("420Ki"),
				},
			},
		},
	}
}
