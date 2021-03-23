package metrics

import (
	"context"

	loggregator "code.cloudfoundry.org/go-loggregator"
)

const (
	CPUUnit    = "percentage"
	MemoryUnit = "bytes"
	DiskUnit   = "bytes"
)

//counterfeiter:generate . LoggregatorClient

type LoggregatorClient interface {
	EmitGauge(...loggregator.EmitGaugeOption)
}

type LoggregatorEmitter struct {
	client LoggregatorClient
}

type Message struct {
	AppID       string
	IndexID     string
	CPU         float64
	Memory      float64
	MemoryQuota float64
	Disk        float64
	DiskQuota   float64
}

func NewLoggregatorEmitter(client LoggregatorClient) *LoggregatorEmitter {
	return &LoggregatorEmitter{
		client: client,
	}
}

func (e *LoggregatorEmitter) Emit(ctx context.Context, m Message) {
	e.client.EmitGauge(
		loggregator.WithGaugeSourceInfo(m.AppID, m.IndexID),
		loggregator.WithGaugeValue("cpu", m.CPU, CPUUnit),
		loggregator.WithGaugeValue("memory", m.Memory, MemoryUnit),
		loggregator.WithGaugeValue("memory_quota", m.MemoryQuota, MemoryUnit),
		loggregator.WithGaugeValue("disk", m.Disk, DiskUnit),
		loggregator.WithGaugeValue("disk_quota", m.DiskQuota, DiskUnit),
	)
}
