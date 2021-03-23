package kubelet

import (
	"context"

	"code.cloudfoundry.org/lager"
	"github.com/pkg/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type DiskMetricsClient struct {
	nodeClient    NodeAPI
	kubeletClient API
	logger        lager.Logger
}

func NewDiskMetricsClient(nodeClient NodeAPI, kubeletClient API, logger lager.Logger) DiskMetricsClient {
	return DiskMetricsClient{
		nodeClient:    nodeClient,
		kubeletClient: kubeletClient,
		logger:        logger,
	}
}

func (d DiskMetricsClient) GetPodMetrics(ctx context.Context) (map[string]float64, error) {
	metrics := map[string]float64{}
	pods := []PodStats{}

	nodes, err := d.nodeClient.List(ctx, metav1.ListOptions{})
	if err != nil {
		return metrics, errors.Wrap(err, "failed to list nodes")
	}

	for _, n := range nodes.Items {
		statsSummary, err := d.kubeletClient.StatsSummary(n.Name)
		if err != nil {
			d.logger.Error("failed-to-get-stats-summary", err, lager.Data{"node-name": n.Name})
		}

		pods = append(pods, statsSummary.Pods...)
	}

	for _, p := range pods {
		if len(p.Containers) != 0 {
			logsBytes := getUsedBytes(p.Containers[0].Logs)
			rootfsBytes := getUsedBytes(p.Containers[0].Rootfs)
			metrics[p.PodRef.Name] = logsBytes + rootfsBytes
		}
	}

	return metrics, nil
}

func getUsedBytes(stats *FsStats) float64 {
	if stats == nil || stats.UsedBytes == nil {
		return 0
	}

	return float64(*stats.UsedBytes)
}
