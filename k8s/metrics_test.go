package k8s_test

import (
	"code.cloudfoundry.org/eirini/k8s"
	"code.cloudfoundry.org/eirini/k8s/k8sfakes"
	"code.cloudfoundry.org/eirini/k8s/stset"
	"code.cloudfoundry.org/eirini/metrics"
	"code.cloudfoundry.org/lager/lagertest"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/pkg/errors"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	metricsv1beta1api "k8s.io/metrics/pkg/apis/metrics/v1beta1"
)

var _ = Describe("Metrics", func() {
	const (
		podName1 = "thor-thunder-9000"
		podName2 = "loki-thunder-8000"
	)

	Describe("Collect", func() {
		var (
			podsGetter       *k8sfakes.FakePodsGetter
			podMetricsClient *k8sfakes.FakePodMetricsInterface
			collector        k8s.MetricsCollector
			diskClient       *k8sfakes.FakeDiskAPI
			logger           *lagertest.TestLogger
		)

		BeforeEach(func() {
			podsGetter = new(k8sfakes.FakePodsGetter)
			podMetricsClient = new(k8sfakes.FakePodMetricsInterface)
			diskClient = new(k8sfakes.FakeDiskAPI)
			logger = lagertest.NewTestLogger("metrics-test")
			collector = k8s.NewMetricsCollector(podMetricsClient, podsGetter, diskClient, logger)
		})

		When("all metrics are valid", func() {
			It("should return them as messages", func() {
				podMetrics := &metricsv1beta1api.PodMetricsList{
					Items: []metricsv1beta1api.PodMetrics{
						createMetrics(podName1),
						createMetrics(podName2),
					},
				}
				podMetricsClient.ListReturns(podMetrics, nil)

				podList := []v1.Pod{*createPod(podName1), *createPod(podName2)}
				podsGetter.GetAllReturns(podList, nil)

				diskClient.GetPodMetricsReturns(map[string]float64{
					podName1: 50,
					podName2: 88,
				}, nil)

				collected, err := collector.Collect(ctx)
				Expect(err).ToNot(HaveOccurred())
				Expect(collected).To(ConsistOf(
					metrics.Message{
						AppID:       podName1,
						IndexID:     "9000",
						CPU:         420.5,
						Memory:      430080,
						MemoryQuota: 800000,
						Disk:        50,
						DiskQuota:   10000000,
					},
					metrics.Message{
						AppID:       podName2,
						IndexID:     "8000",
						CPU:         420.5,
						Memory:      430080,
						MemoryQuota: 800000,
						Disk:        88,
						DiskQuota:   10000000,
					},
				))
			})
		})

		When("there are no pods", func() {
			It("should return empty list", func() {
				podsGetter.GetAllReturns([]v1.Pod{}, nil)

				podMetrics := metricsv1beta1api.PodMetricsList{
					Items: []metricsv1beta1api.PodMetrics{createMetrics(podName1)},
				}
				podMetricsClient.ListReturns(&podMetrics, nil)

				collected, err := collector.Collect(ctx)
				Expect(err).ToNot(HaveOccurred())
				Expect(collected).To(BeEmpty())
			})
		})

		When("the disk client returns an error", func() {
			BeforeEach(func() {
				podMetrics := &metricsv1beta1api.PodMetricsList{
					Items: []metricsv1beta1api.PodMetrics{
						createMetrics(podName1),
					},
				}
				podMetricsClient.ListReturns(podMetrics, nil)

				podList := []v1.Pod{*createPod(podName1)}
				podsGetter.GetAllReturns(podList, nil)

				diskClient.GetPodMetricsReturns(nil, errors.New("oopsie"))
			})

			It("should log the error", func() {
				_, err := collector.Collect(ctx)
				Expect(err).ToNot(HaveOccurred())
				Expect(logger).To(gbytes.Say("oopsie"))
			})

			It("should emmit 0 disk usage", func() {
				collected, err := collector.Collect(ctx)
				Expect(err).ToNot(HaveOccurred())
				Expect(collected).To(ConsistOf(
					metrics.Message{
						AppID:       podName1,
						IndexID:     "9000",
						CPU:         420.5,
						Memory:      430080,
						MemoryQuota: 800000,
						Disk:        0,
						DiskQuota:   10000000,
					}))
			})
		})
		When("listing pods returns an error", func() {
			It("should return an error", func() {
				podsGetter.GetAllReturns([]v1.Pod{}, errors.New("something done broke"))

				collected, err := collector.Collect(ctx)
				Expect(err).To(HaveOccurred())
				Expect(collected).To(BeEmpty())
			})
		})

		When("there are no container metrics for a pod", func() {
			It("should return only disk metrics", func() {
				diskClient.GetPodMetricsReturns(map[string]float64{
					podName1: 50,
				}, nil)

				podsGetter.GetAllReturns([]v1.Pod{*createPod(podName1)}, nil)

				podMetrics := metricsv1beta1api.PodMetricsList{
					Items: []metricsv1beta1api.PodMetrics{},
				}
				podMetricsClient.ListReturns(&podMetrics, nil)

				collected, err := collector.Collect(ctx)
				Expect(err).ToNot(HaveOccurred())
				Expect(collected).To(ConsistOf(
					metrics.Message{
						AppID:       podName1,
						IndexID:     "9000",
						CPU:         0,
						Memory:      0,
						MemoryQuota: 800000,
						Disk:        50,
						DiskQuota:   10000000,
					}))
			})
		})

		When("there are no disk metrics", func() {
			It("should return only CPU/memory metrics", func() {
				diskClient.GetPodMetricsReturns(map[string]float64{}, nil)
				podMetrics := &metricsv1beta1api.PodMetricsList{
					Items: []metricsv1beta1api.PodMetrics{
						createMetrics(podName1),
					},
				}
				podMetricsClient.ListReturns(podMetrics, nil)

				podList := []v1.Pod{*createPod(podName1)}
				podsGetter.GetAllReturns(podList, nil)

				collected, err := collector.Collect(ctx)
				Expect(err).ToNot(HaveOccurred())
				Expect(collected).To(ConsistOf(
					metrics.Message{
						AppID:       podName1,
						IndexID:     "9000",
						CPU:         420.5,
						Memory:      430080,
						MemoryQuota: 800000,
						Disk:        0,
						DiskQuota:   10000000,
					},
				))
			})
		})

		When("a pod name doesn't have an index", func() {
			It("should skip such pod", func() {
				aPodHasNoIndex := "i-dont-have-an-index"

				podsGetter.GetAllReturns([]v1.Pod{*createPod(aPodHasNoIndex), *createPod(podName2)}, nil)

				podMetrics := metricsv1beta1api.PodMetricsList{
					Items: []metricsv1beta1api.PodMetrics{createMetrics(aPodHasNoIndex), createMetrics(podName2)},
				}
				podMetricsClient.ListReturns(&podMetrics, nil)

				diskClient.GetPodMetricsReturns(map[string]float64{
					aPodHasNoIndex: 50,
					podName2:       88,
				}, nil)

				collected, err := collector.Collect(ctx)
				Expect(err).ToNot(HaveOccurred())
				Expect(collected).To(ConsistOf(metrics.Message{
					AppID:       podName2,
					IndexID:     "8000",
					CPU:         420.5,
					Memory:      430080,
					MemoryQuota: 800000,
					Disk:        88,
					DiskQuota:   10000000,
				}))
			})
		})

		When("metrics client returns an error", func() {
			BeforeEach(func() {
				podList := []v1.Pod{*createPod(podName1)}
				podsGetter.GetAllReturns(podList, nil)
				podMetricsClient.ListReturns(&metricsv1beta1api.PodMetricsList{}, errors.New("oopsie"))
				diskClient.GetPodMetricsReturns(map[string]float64{
					podName1: 50,
				}, nil)
			})

			It("should log the error in log", func() {
				_, err := collector.Collect(ctx)
				Expect(err).ToNot(HaveOccurred())
				Expect(logger).To(gbytes.Say("oopsie"))
			})

			It("should return disk metrics", func() {
				Expect(collector.Collect(ctx)).To(ConsistOf(
					metrics.Message{
						AppID:       podName1,
						IndexID:     "9000",
						CPU:         0,
						Memory:      0,
						MemoryQuota: 800000,
						Disk:        50,
						DiskQuota:   10000000,
					}))
			})
		})
	})
})

var _ = Describe("ForwardMetricsToEmitter", func() {
	It("should forward the messages when collector returns them", func() {
		emitter := new(k8sfakes.FakeEmitter)
		collector := new(k8sfakes.FakeMetricsCollector)
		collector.CollectReturns([]metrics.Message{{AppID: "metric"}}, nil)

		Expect(k8s.ForwardMetricsToEmitter(ctx, collector, emitter)).To(Succeed())
	})

	It("should return error if collector returns error", func() {
		emitter := new(k8sfakes.FakeEmitter)
		collector := new(k8sfakes.FakeMetricsCollector)
		collector.CollectReturns(nil, errors.New("oopsie"))

		Expect(k8s.ForwardMetricsToEmitter(ctx, collector, emitter)).To(MatchError(ContainSubstring("oopsie")))
	})
})

func createMetrics(podName string) metricsv1beta1api.PodMetrics {
	return metricsv1beta1api.PodMetrics{
		ObjectMeta: metav1.ObjectMeta{Name: podName, Namespace: "opi", ResourceVersion: "10", Labels: map[string]string{"key": "value"}},
		Containers: []metricsv1beta1api.ContainerMetrics{
			{
				Usage: v1.ResourceList{
					v1.ResourceCPU:    resource.MustParse("4205m"),
					v1.ResourceMemory: resource.MustParse("420Ki"),
				},
			},
		},
	}
}

func createPod(podName string) *v1.Pod {
	return &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name: podName,
			Labels: map[string]string{
				stset.LabelGUID: podName,
			},
			UID: types.UID(podName + "-uid"),
		},
		Spec: v1.PodSpec{
			Containers: []v1.Container{
				{
					Resources: v1.ResourceRequirements{
						Limits: v1.ResourceList{
							v1.ResourceMemory:           *resource.NewScaledQuantity(800, resource.Kilo),
							v1.ResourceEphemeralStorage: *resource.NewScaledQuantity(10, resource.Mega),
						},
					},
				},
			},
		},
	}
}
