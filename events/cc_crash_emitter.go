package events

import (
	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/runtimeschema/cc_messages"
)

//go:generate counterfeiter . CcClient
type CcClient interface {
	AppCrashed(proccessGUID string, crashedRequest cc_messages.AppCrashedRequest, logger lager.Logger) error
}

type CrashEvent struct {
	ProcessGUID string
	cc_messages.AppCrashedRequest
}

type CcCrashEmitter struct {
	logger lager.Logger
	client CcClient
}

func NewCcCrashEmitter(logger lager.Logger, client CcClient) *CcCrashEmitter {
	return &CcCrashEmitter{
		logger: logger,
		client: client,
	}
}

func (c *CcCrashEmitter) Emit(event CrashEvent) error {
	return c.client.AppCrashed(event.ProcessGUID, event.AppCrashedRequest, c.logger)
}
