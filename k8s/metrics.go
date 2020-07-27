package k8s

import (
	"context"
	"strconv"

	"code.cloudfoundry.org/eirini/metrics"
	"code.cloudfoundry.org/eirini/util"
	"code.cloudfoundry.org/lager"
	"github.com/pkg/errors"
	apiv1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	typedv1 "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/metrics/pkg/apis/metrics/v1beta1"
	metricsv1beta1 "k8s.io/metrics/pkg/client/clientset/versioned/typed/metrics/v1beta1"
)

//counterfeiter:generate . MetricsCollector
//counterfeiter:generate . DiskAPI
//counterfeiter:generate . Emitter
//counterfeiter:generate -o k8sfakes/fake_pod_interface.go ../vendor/k8s.io/client-go/kubernetes/typed/core/v1 PodInterface
//counterfeiter:generate -o k8sfakes/fake_pod_metrics_interface.go ../vendor/k8s.io/metrics/pkg/client/clientset/versioned/typed/metrics/v1beta1 PodMetricsInterface

type MetricsCollector interface {
	Collect() ([]metrics.Message, error)
}

type DiskAPI interface {
	GetPodMetrics() (map[string]float64, error)
}

type Emitter interface {
	Emit(metrics.Message)
}

func ForwardMetricsToEmitter(collector MetricsCollector, emitter Emitter) error {
	messages, err := collector.Collect()
	if err != nil {
		return err
	}

	for _, m := range messages {
		emitter.Emit(m)
	}
	return nil
}

type metricsCollector struct {
	metricsClient metricsv1beta1.PodMetricsInterface
	podClient     typedv1.PodInterface
	diskClient    DiskAPI
	logger        lager.Logger
}

func NewMetricsCollector(metricsClient metricsv1beta1.PodMetricsInterface,
	podClient typedv1.PodInterface,
	diskClient DiskAPI,
	logger lager.Logger) MetricsCollector {
	return &metricsCollector{
		metricsClient: metricsClient,
		podClient:     podClient,
		diskClient:    diskClient,
		logger:        logger,
	}
}

func (c *metricsCollector) Collect() ([]metrics.Message, error) {
	pods, err := c.podClient.List(context.Background(), metav1.ListOptions{})
	if err != nil {
		return []metrics.Message{}, errors.Wrap(err, "failed to list pods")
	}
	return c.collectMetrics(pods.Items), nil
}

func (c *metricsCollector) collectMetrics(pods []apiv1.Pod) []metrics.Message {
	diskMetrics, err := c.diskClient.GetPodMetrics()
	if err != nil {
		c.logger.Error("failed-to-get-disk-metrics", err, lager.Data{})
	}
	messages := []metrics.Message{}
	podMetrics, err := c.getPodMetrics()
	if err != nil {
		c.logger.Error("Failed to get metrics from Kubernetes", err, lager.Data{})
	}

	for _, pod := range pods {
		indexID, err := util.ParseAppIndex(pod.Name)
		if err != nil {
			continue
		}
		cpuPercentage, memoryValue := parseMetrics(podMetrics[pod.Name])

		appContainer := pod.Spec.Containers[0]
		memoryLimit := appContainer.Resources.Limits.Memory()
		diskLimit := appContainer.Resources.Limits.StorageEphemeral()

		diskUsage := diskMetrics[pod.Name]

		messages = append(messages, metrics.Message{
			AppID:       pod.Labels[LabelGUID],
			IndexID:     strconv.Itoa(indexID),
			CPU:         cpuPercentage,
			Memory:      memoryValue,
			MemoryQuota: float64(memoryLimit.Value()),
			Disk:        diskUsage,
			DiskQuota:   float64(diskLimit.Value()),
		})
	}
	return messages
}

func parseMetrics(metric v1beta1.PodMetrics) (cpu float64, memory float64) {
	if len(metric.Containers) == 0 {
		return
	}

	container := metric.Containers[0]
	usage := container.Usage
	res := usage[apiv1.ResourceCPU]
	cpu = toCPUPercentage(res.MilliValue())
	res = usage[apiv1.ResourceMemory]
	memory = float64(res.Value())
	return
}

func (c *metricsCollector) getPodMetrics() (map[string]v1beta1.PodMetrics, error) {
	metricsList, err := c.metricsClient.List(context.Background(), metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	metricsMap := make(map[string]v1beta1.PodMetrics)
	for _, m := range metricsList.Items {
		metricsMap[m.Name] = m
	}

	return metricsMap, nil
}
