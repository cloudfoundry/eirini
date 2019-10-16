package metrics

import (
	loggregator "code.cloudfoundry.org/go-loggregator"
)

const (
	CPUUnit    = "percentage"
	MemoryUnit = "bytes"
	DiskUnit   = "bytes"
)

//go:generate counterfeiter . LoggregatorClient
type LoggregatorClient interface {
	EmitGauge(...loggregator.EmitGaugeOption)
}

type LoggregatorForwarder struct {
	client LoggregatorClient
}

func NewLoggregatorForwarder(client LoggregatorClient) *LoggregatorForwarder {
	return &LoggregatorForwarder{
		client: client,
	}
}

func (l *LoggregatorForwarder) Forward(msg Message) {
	l.client.EmitGauge(
		loggregator.WithGaugeSourceInfo(msg.AppID, msg.IndexID),
		loggregator.WithGaugeValue("cpu", msg.CPU, CPUUnit),
		loggregator.WithGaugeValue("memory", msg.Memory, MemoryUnit),
		loggregator.WithGaugeValue("memory_quota", msg.MemoryQuota, MemoryUnit),
		loggregator.WithGaugeValue("disk", msg.Disk, DiskUnit),
		loggregator.WithGaugeValue("disk_quota", msg.DiskQuota, DiskUnit),
	)
}
