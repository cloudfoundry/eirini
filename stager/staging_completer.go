package stager

import (
	"context"
	"encoding/json"
	"net/url"

	"code.cloudfoundry.org/eirini/models/cf"
	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/runtimeschema/cc_messages"
	"github.com/pkg/errors"
)

//counterfeiter:generate . CallbackClient

type CallbackClient interface {
	Post(ctx context.Context, url string, data interface{}) error
}

type CallbackStagingCompleter struct {
	logger         lager.Logger
	callbackClient CallbackClient
}

func NewCallbackStagingCompleter(logger lager.Logger, callbackClient CallbackClient) *CallbackStagingCompleter {
	return &CallbackStagingCompleter{
		logger:         logger,
		callbackClient: callbackClient,
	}
}

func (s *CallbackStagingCompleter) CompleteStaging(ctx context.Context, taskCompletedRequest cf.StagingCompletedRequest) error {
	l := s.logger.Session("complete-staging", lager.Data{"task-guid": taskCompletedRequest.TaskGUID})

	callbackURI, err := s.getCallbackURI(taskCompletedRequest)
	if err != nil {
		l.Error("failed-to-parse-callback-uri", err)

		return err
	}

	_, err = url.Parse(callbackURI)
	if err != nil {
		l.Error("failed-to-parse-callback-request", err)
	}

	response := s.constructStagingResponse(taskCompletedRequest)

	return errors.Wrap(s.callbackClient.Post(ctx, callbackURI, response), "callback-response-unsuccessful")
}

func (s *CallbackStagingCompleter) constructStagingResponse(taskCompletedRequest cf.StagingCompletedRequest) cc_messages.StagingResponseForCC {
	var response cc_messages.StagingResponseForCC

	if taskCompletedRequest.Failed {
		response.Error = &cc_messages.StagingError{
			Id:      cc_messages.STAGING_ERROR,
			Message: taskCompletedRequest.FailureReason,
		}
	} else {
		result := json.RawMessage([]byte(taskCompletedRequest.Result))
		response.Result = &result
	}

	return response
}

func (s *CallbackStagingCompleter) getCallbackURI(taskCompletedRequest cf.StagingCompletedRequest) (string, error) {
	var annotation cc_messages.StagingTaskAnnotation
	if err := json.Unmarshal([]byte(taskCompletedRequest.Annotation), &annotation); err != nil {
		s.logger.Error("failed-to-parse-annotation", err)

		return "", errors.Wrap(err, "failed to parse annotation")
	}

	return annotation.CompletionCallback, nil
}
