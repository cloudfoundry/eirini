package k8s

import (
	"context"
	"strconv"

	"code.cloudfoundry.org/eirini/k8s/stset"
	"code.cloudfoundry.org/eirini/metrics"
	"code.cloudfoundry.org/eirini/util"
	"code.cloudfoundry.org/lager"
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/metrics/pkg/apis/metrics/v1beta1"
	metricsv1beta1 "k8s.io/metrics/pkg/client/clientset/versioned/typed/metrics/v1beta1"
)

//counterfeiter:generate . PodsGetter
//counterfeiter:generate . MetricsCollector
//counterfeiter:generate . DiskAPI
//counterfeiter:generate . Emitter
//counterfeiter:generate -o k8sfakes/fake_pod_metrics_interface.go k8s.io/metrics/pkg/client/clientset/versioned/typed/metrics/v1beta1.PodMetricsInterface

type PodsGetter interface {
	GetAll(ctx context.Context) ([]corev1.Pod, error)
}

type MetricsCollector interface {
	Collect(ctx context.Context) ([]metrics.Message, error)
}

type DiskAPI interface {
	GetPodMetrics(ctx context.Context) (map[string]float64, error)
}

type Emitter interface {
	Emit(context.Context, metrics.Message)
}

func ForwardMetricsToEmitter(ctx context.Context, collector MetricsCollector, emitter Emitter) error {
	messages, err := collector.Collect(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to collect metrics")
	}

	for _, m := range messages {
		emitter.Emit(ctx, m)
	}

	return nil
}

type metricsCollector struct {
	metricsClient metricsv1beta1.PodMetricsInterface
	podsGetter    PodsGetter
	diskClient    DiskAPI
	logger        lager.Logger
}

func NewMetricsCollector(metricsClient metricsv1beta1.PodMetricsInterface,
	podsGetter PodsGetter,
	diskClient DiskAPI,
	logger lager.Logger) MetricsCollector {
	return &metricsCollector{
		metricsClient: metricsClient,
		podsGetter:    podsGetter,
		diskClient:    diskClient,
		logger:        logger,
	}
}

func (c *metricsCollector) Collect(ctx context.Context) ([]metrics.Message, error) {
	pods, err := c.podsGetter.GetAll(ctx)
	if err != nil {
		return []metrics.Message{}, errors.Wrap(err, "failed to list pods")
	}

	return c.collectMetrics(ctx, pods), nil
}

func (c *metricsCollector) collectMetrics(ctx context.Context, pods []corev1.Pod) []metrics.Message {
	logger := c.logger.Session("collect")

	diskMetrics, err := c.diskClient.GetPodMetrics(ctx)
	if err != nil {
		logger.Error("failed-to-get-disk-metrics", err, lager.Data{})
	}

	messages := []metrics.Message{}

	podMetrics, err := c.getPodMetrics()
	if err != nil {
		logger.Error("failed-to-get-metrics-from-kubernetes", err, lager.Data{})
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
			AppID:       pod.Labels[stset.LabelGUID],
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
	res := usage[corev1.ResourceCPU]
	cpu = toCPUPercentage(res.MilliValue())
	res = usage[corev1.ResourceMemory]
	memory = float64(res.Value())

	return
}

func (c *metricsCollector) getPodMetrics() (map[string]v1beta1.PodMetrics, error) {
	metricsList, err := c.metricsClient.List(context.Background(), metav1.ListOptions{})
	if err != nil {
		return nil, errors.Wrap(err, "failed to list metrics")
	}

	metricsMap := make(map[string]v1beta1.PodMetrics)

	for _, m := range metricsList.Items {
		metricsMap[m.Name] = m
	}

	return metricsMap, nil
}

func toCPUPercentage(cpuMillicores int64) float64 {
	return float64(cpuMillicores) / 10 //nolint:gomnd
}
