package kubelet

type DiskMetricsClient struct {
}

func (d *DiskMetricsClient) GetPodMetrics() (map[string]float64, error) {
	return nil, nil
}
