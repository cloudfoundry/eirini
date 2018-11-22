package events

import (
	"code.cloudfoundry.org/eirini/route"
	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/runtimeschema/cc_messages"
)

//go:generate counterfeiter . CcClient
type CcClient interface {
	AppCrashed(proccessGuid string, crashedRequest cc_messages.AppCrashedRequest, logger lager.Logger) error
}

type CrashReport struct {
	ProcessGuid string
	cc_messages.AppCrashedRequest
}

type CrashReporter struct {
	reports   <-chan CrashReport
	scheduler route.TaskScheduler
	client    CcClient
	logger    lager.Logger
}

func NewCrashReporter(reportChan <-chan CrashReport, scheduler route.TaskScheduler, client CcClient, logger lager.Logger) *CrashReporter {
	return &CrashReporter{
		reports:   reportChan,
		scheduler: scheduler,
		client:    client,
		logger:    logger,
	}
}

func (c *CrashReporter) Run() {
	c.scheduler.Schedule(func() error {
		report := <-c.reports
		return c.client.AppCrashed(report.ProcessGuid, report.AppCrashedRequest, c.logger)
	})
}
