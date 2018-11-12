package metrics

import (
	loggregator "code.cloudfoundry.org/go-loggregator"
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
		loggregator.WithGaugeValue("cpu", msg.CPU, "%"),
		loggregator.WithGaugeValue("memory", msg.Memory, msg.MemoryUnit),
		loggregator.WithGaugeValue("disk", msg.Disk, msg.DiskUnit),
		loggregator.WithGaugeValue("memory_quota", msg.MemoryQuota, msg.MemoryUnit),
		loggregator.WithGaugeValue("disk_quota", msg.DiskQuota, msg.DiskUnit),
	)
}
