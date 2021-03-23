package kubelet_test

import (
	"errors"

	"code.cloudfoundry.org/eirini/k8s/kubelet"
	"code.cloudfoundry.org/eirini/k8s/kubelet/kubeletfakes"
	"code.cloudfoundry.org/lager/lagertest"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = Describe("Stats", func() {
	var (
		diskMetricsClient kubelet.DiskMetricsClient
		nodeClient        *kubeletfakes.FakeNodeAPI
		kubeletClient     *kubeletfakes.FakeAPI
		logger            *lagertest.TestLogger
	)

	createStatsSummary := func(podName string, namespace string, rootfsBytes, logsBytes uint64) kubelet.StatsSummary {
		return kubelet.StatsSummary{
			Pods: []kubelet.PodStats{
				{
					PodRef: kubelet.PodReference{
						Name:      podName,
						Namespace: namespace,
					},
					Containers: []kubelet.ContainerStats{
						{
							Rootfs: &kubelet.FsStats{
								UsedBytes: &rootfsBytes,
							},
							Logs: &kubelet.FsStats{
								UsedBytes: &logsBytes,
							},
						},
					},
				},
			},
		}
	}

	BeforeEach(func() {
		nodeClient = new(kubeletfakes.FakeNodeAPI)
		kubeletClient = new(kubeletfakes.FakeAPI)
		logger = lagertest.NewTestLogger("statstest")

		diskMetricsClient = kubelet.NewDiskMetricsClient(nodeClient, kubeletClient, logger)
	})

	It("should return the disk metrics for all pods on all nodes", func() {
		nodeClient.ListReturns(&corev1.NodeList{
			Items: []corev1.Node{
				{ObjectMeta: metav1.ObjectMeta{
					Name: "node1",
				}},
				{ObjectMeta: metav1.ObjectMeta{
					Name: "node2",
				}},
			},
		}, nil)
		kubeletClient.StatsSummaryReturnsOnCall(0, createStatsSummary("pod-1", "ns-1", 300, 700), nil)
		kubeletClient.StatsSummaryReturnsOnCall(1, createStatsSummary("pod-2", "ns-2", 200, 256), nil)

		metrics, err := diskMetricsClient.GetPodMetrics(ctx)
		Expect(err).ToNot(HaveOccurred())
		Expect(nodeClient.ListCallCount()).To(Equal(1))
		Expect(kubeletClient.StatsSummaryCallCount()).To(Equal(2))
		Expect(kubeletClient.StatsSummaryArgsForCall(0)).To(Equal("node1"))
		Expect(kubeletClient.StatsSummaryArgsForCall(1)).To(Equal("node2"))
		Expect(metrics).To(HaveKeyWithValue("pod-1", float64(1000)))
		Expect(metrics).To(HaveKeyWithValue("pod-2", float64(456)))
	})

	When("the node client return an error", func() {
		It("should return an error", func() {
			nodeClient.ListReturns(&corev1.NodeList{}, errors.New("oopsie"))
			_, err := diskMetricsClient.GetPodMetrics(ctx)
			Expect(err).To(HaveOccurred())
			Expect(err).To(MatchError(ContainSubstring("oopsie")))
		})
	})

	When("there are no containers in the pod stats", func() {
		It("the pod should be ignored", func() {
			nodeClient.ListReturns(&corev1.NodeList{
				Items: []corev1.Node{
					{ObjectMeta: metav1.ObjectMeta{
						Name: "node1",
					}},
					{ObjectMeta: metav1.ObjectMeta{
						Name: "node2",
					}},
				},
			}, nil)
			stats := createStatsSummary("pod-1", "ns-1", 300, 700)
			stats.Pods[0].Containers = nil
			kubeletClient.StatsSummaryReturnsOnCall(0, stats, nil)
			kubeletClient.StatsSummaryReturnsOnCall(1, createStatsSummary("pod-2", "ns-1", 200, 256), nil)

			metrics, _ := diskMetricsClient.GetPodMetrics(ctx)
			Expect(metrics).To(HaveLen(1))
			Expect(metrics).To(HaveKeyWithValue("pod-2", float64(456)))
		})
	})

	When("the kubeletClient returns an error for a node", func() {
		It("should ignore that node", func() {
			nodeClient.ListReturns(&corev1.NodeList{
				Items: []corev1.Node{
					{ObjectMeta: metav1.ObjectMeta{
						Name: "node1",
					}},
				},
			}, nil)
			kubeletClient.StatsSummaryReturnsOnCall(0, kubelet.StatsSummary{}, errors.New("oopsie"))

			metrics, _ := diskMetricsClient.GetPodMetrics(ctx)
			Expect(metrics).To(BeEmpty())
			logs := logger.Logs()
			Expect(logs).To(HaveLen(1))
			Expect(logs[0].Data).To(HaveKeyWithValue("node-name", "node1"))
			Expect(logs[0].Data).To(HaveKeyWithValue("error", "oopsie"))
		})
	})

	When("the disk metrics for a pod are missing", func() {
		It("should report the used bytes as zero", func() {
			nodeClient.ListReturns(&corev1.NodeList{
				Items: []corev1.Node{
					{ObjectMeta: metav1.ObjectMeta{
						Name: "node1",
					}},
				},
			}, nil)

			stats := createStatsSummary("pod-1", "ns-1", 300, 700)
			stats.Pods[0].Containers[0].Rootfs = nil
			stats.Pods[0].Containers[0].Logs.UsedBytes = nil
			kubeletClient.StatsSummaryReturnsOnCall(0, stats, nil)

			metrics, _ := diskMetricsClient.GetPodMetrics(ctx)
			Expect(metrics).To(HaveLen(1))
			Expect(metrics).To(HaveKeyWithValue("pod-1", float64(0)))
		})
	})
})
