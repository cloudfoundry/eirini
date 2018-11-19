package metrics

import (
	loggregator "code.cloudfoundry.org/go-loggregator"
)

const (
	cpuUnit    = "percentage"
	memoryUnit = "bytes"
	diskUnit   = "bytes"
)

type LoggregatorForwarder struct {
	client *loggregator.IngressClient
}

func NewLoggregatorForwarder(client *loggregator.IngressClient) *LoggregatorForwarder {
	return &LoggregatorForwarder{
		client: client,
	}
}

func (l *LoggregatorForwarder) Forward(msg Message) {
	l.client.EmitGauge(
		loggregator.WithGaugeSourceInfo(msg.AppID, msg.IndexID),
		loggregator.WithGaugeValue("cpu", msg.CPU, cpuUnit),
		loggregator.WithGaugeValue("memory", msg.Memory, memoryUnit),
		loggregator.WithGaugeValue("disk", msg.Disk, diskUnit),
		loggregator.WithGaugeValue("memory_quota", msg.MemoryQuota, memoryUnit),
		loggregator.WithGaugeValue("disk_quota", msg.DiskQuota, diskUnit),
	)
}
