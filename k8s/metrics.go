package k8s

import (
	"strconv"

	"code.cloudfoundry.org/eirini/metrics"
	"code.cloudfoundry.org/eirini/route"
	"code.cloudfoundry.org/eirini/util"
	"code.cloudfoundry.org/lager"
	"golang.org/x/xerrors"
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
	scheduler     route.TaskScheduler
	logger        lager.Logger
}

func NewMetricsCollector(work chan []metrics.Message, scheduler route.TaskScheduler, metricsClient metricsv1beta1.PodMetricsInterface, podClient typedv1.PodInterface, logger lager.Logger) *MetricsCollector {
	return &MetricsCollector{
		work:          work,
		metricsClient: metricsClient,
		scheduler:     scheduler,
		podClient:     podClient,
		logger:        logger,
	}
}

func (c *MetricsCollector) Start() {
	c.scheduler.Schedule(func() error {
		metrics, err := c.metricsClient.List(metav1.ListOptions{})
		if err != nil {
			return xerrors.Errorf("%w", err)
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
	for _, metric := range podMetrics.Items {
		if len(metric.Containers) == 0 {
			c.logger.Info("pod-with-no-containers", lager.Data{"pod": metric.Name})
			continue
		}
		container := metric.Containers[0]
		_, indexID, err := util.ParseAppNameAndIndex(metric.Name)
		if err != nil {
			c.logger.Info("incorrect-pod-name", lager.Data{"pod": metric.Name})
			continue
		}
		usage := container.Usage
		res := usage[apiv1.ResourceCPU]
		cpuValue := res.Value()
		res = usage[apiv1.ResourceMemory]
		memoryValue := res.Value()

		pod, err := c.podClient.Get(metric.Name, metav1.GetOptions{})
		if err != nil {
			c.logger.Info("cannot-find-pod", lager.Data{"pod": metric.Name})
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
