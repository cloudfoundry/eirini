package k8s

import (
	"strconv"

	"code.cloudfoundry.org/eirini/metrics"
	"code.cloudfoundry.org/eirini/util"
	"github.com/pkg/errors"
	apiv1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	typedv1 "k8s.io/client-go/kubernetes/typed/core/v1"
	metricsv1beta1api "k8s.io/metrics/pkg/apis/metrics/v1beta1"
	metricsv1beta1 "k8s.io/metrics/pkg/client/clientset/versioned/typed/metrics/v1beta1"
)

//go:generate counterfeiter . MetricsCollector
type MetricsCollector interface {
	Collect() ([]metrics.Message, error)
}

func ForwardMetricsToChannel(collector MetricsCollector, work chan<- []metrics.Message) error {
	messages, err := collector.Collect()
	if err != nil {
		return err
	}
	work <- messages
	return nil
}

//go:generate counterfeiter -o k8sfakes/fake_pod_interface.go ../vendor/k8s.io/client-go/kubernetes/typed/core/v1 PodInterface
//go:generate counterfeiter -o k8sfakes/fake_pod_metrics_interface.go ../vendor/k8s.io/metrics/pkg/client/clientset/versioned/typed/metrics/v1beta1 PodMetricsInterface
type metricsCollector struct {
	metricsClient metricsv1beta1.PodMetricsInterface
	podClient     typedv1.PodInterface
}

func NewMetricsCollector(metricsClient metricsv1beta1.PodMetricsInterface, podClient typedv1.PodInterface) MetricsCollector {
	return &metricsCollector{
		metricsClient: metricsClient,
		podClient:     podClient,
	}
}

func (c *metricsCollector) Collect() ([]metrics.Message, error) {
	metrics, err := c.metricsClient.List(metav1.ListOptions{})
	if err != nil {
		return nil, errors.Wrap(err, "failed to list metrics")
	}
	return c.convertMetricsList(metrics), nil
}

func (c *metricsCollector) convertMetricsList(podMetrics *metricsv1beta1api.PodMetricsList) []metrics.Message {
	messages := []metrics.Message{}
	pods, err := c.getPods()
	if err != nil {
		return messages
	}

	for _, metric := range podMetrics.Items {
		if len(metric.Containers) == 0 {
			continue
		}
		container := metric.Containers[0]
		indexID, err := util.ParseAppIndex(metric.Name)
		if err != nil {
			continue
		}
		usage := container.Usage
		res := usage[apiv1.ResourceCPU]
		cpuMillicores := res.MilliValue()
		cpuPercentage := float64(cpuMillicores) / 10
		res = usage[apiv1.ResourceMemory]
		memoryValue := res.Value()

		pod, ok := pods[metric.Name]
		if !ok {
			continue
		}
		appContainer := pod.Spec.Containers[0]
		memoryLimit := appContainer.Resources.Limits.Memory()

		messages = append(messages, metrics.Message{
			AppID:       pod.Labels["guid"],
			IndexID:     strconv.Itoa(indexID),
			CPU:         cpuPercentage,
			Memory:      float64(memoryValue),
			MemoryQuota: float64(memoryLimit.Value()),
			Disk:        42000000,
			DiskQuota:   10,
		})
	}
	return messages
}

func (c *metricsCollector) getPods() (map[string]apiv1.Pod, error) {
	podsList, err := c.podClient.List(metav1.ListOptions{})
	if err != nil {
		return nil, errors.Wrap(err, "failed to list pods")
	}
	podsMap := make(map[string]apiv1.Pod)
	for _, s := range podsList.Items {
		podsMap[s.Name] = s
	}

	return podsMap, nil
}
