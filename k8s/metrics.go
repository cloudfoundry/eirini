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

type MetricsCollector struct {
	work          chan<- []metrics.Message
	metricsClient metricsv1beta1.PodMetricsInterface
	podClient     typedv1.PodInterface
	scheduler     util.TaskScheduler
}

func NewMetricsCollector(work chan []metrics.Message, scheduler util.TaskScheduler, metricsClient metricsv1beta1.PodMetricsInterface, podClient typedv1.PodInterface) *MetricsCollector {
	return &MetricsCollector{
		work:          work,
		metricsClient: metricsClient,
		scheduler:     scheduler,
		podClient:     podClient,
	}
}

func (c *MetricsCollector) Start() {
	c.scheduler.Schedule(func() error {
		metrics, err := c.metricsClient.List(metav1.ListOptions{})
		if err != nil {
			return err
		}
		messages := c.convertMetricsList(metrics)

		if len(messages) > 0 {
			c.work <- messages
		}

		return nil
	})
}

func (c *MetricsCollector) convertMetricsList(podMetrics *metricsv1beta1api.PodMetricsList) []metrics.Message {
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
		cpuValue := res.Value()
		res = usage[apiv1.ResourceMemory]
		memoryValue := res.Value()

		pod, ok := pods[metric.Name]
		if !ok {
			continue
		}

		messages = append(messages, metrics.Message{
			AppID:       pod.Labels["guid"],
			IndexID:     strconv.Itoa(indexID),
			CPU:         float64(cpuValue),
			Memory:      float64(memoryValue),
			MemoryQuota: 10,
			Disk:        42000000,
			DiskQuota:   10,
		})
	}
	return messages
}

func (c *MetricsCollector) getPods() (map[string]apiv1.Pod, error) {
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
