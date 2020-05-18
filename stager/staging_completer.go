package stager

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"code.cloudfoundry.org/eirini/models/cf"
	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/runtimeschema/cc_messages"
)

const (
	numRetries = 10
	delay      = 2 * time.Second
)

type CallbackStagingCompleter struct {
	Logger     lager.Logger
	HTTPClient *http.Client
	Retries    int
	Delay      time.Duration
}

func NewCallbackStagingCompleter(logger lager.Logger, httpClient *http.Client) *CallbackStagingCompleter {
	return &CallbackStagingCompleter{
		Logger:     logger,
		HTTPClient: httpClient,
		Retries:    numRetries,
		Delay:      delay,
	}
}

func (s *CallbackStagingCompleter) CompleteStaging(taskCompletedRequest cf.StagingCompletedRequest) error {
	l := s.Logger.Session("complete-staging", lager.Data{"task-guid": taskCompletedRequest.TaskGUID})
	callbackBody, err := s.constructStagingResponse(taskCompletedRequest)
	if err != nil {
		l.Error("failed-to-construct-staging-response", err)
		return err
	}

	callbackURI, err := s.getCallbackURI(taskCompletedRequest)
	if err != nil {
		l.Error("failed-to-parse-callback-uri", err)
		return err
	}

	_, err = url.Parse(callbackURI)
	if err != nil {
		l.Error("failed-to-parse-callback-request", err)
	}

	makeRequest := func() *http.Request {
		request, err := http.NewRequest("POST", callbackURI, bytes.NewBuffer(callbackBody))
		if err != nil {
			panic("Should not happen: The only reason for NewRequest to error " +
				"should be a non-parsable URL, wihich is being checked for:" + err.Error())
		}
		request.Header.Set("Content-Type", "application/json")
		return request
	}

	return s.executeRequestWithRetries(makeRequest)
}

func (s *CallbackStagingCompleter) executeRequestWithRetries(makeRequest func() *http.Request) error {
	l := s.Logger.Session("execute-callback-request")
	n := 0
	var err error
	for {
		// Create a new request on each iteration to avoid race
		err = s.executeRequest(makeRequest())
		if err == nil {
			break
		}

		n++
		if n == s.Retries {
			break
		}
		l.Error("Sending delete request again", err)

		time.Sleep(s.Delay)
	}
	return err
}

func (s *CallbackStagingCompleter) executeRequest(request *http.Request) error {
	l := s.Logger.Session("execute-callback-request", lager.Data{"request-uri": request.URL})

	resp, err := s.HTTPClient.Do(request)
	if err != nil {
		l.Error("cc-staging-complete-failed", err)
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= http.StatusMultipleChoices {
		l.Error("cc-staging-complete-failed-status-code", nil, lager.Data{"status-code": resp.StatusCode})
		return fmt.Errorf("callback-response-unsuccessful, code: %d", resp.StatusCode)
	}
	return nil
}

func (s *CallbackStagingCompleter) constructStagingResponse(taskCompletedRequest cf.StagingCompletedRequest) ([]byte, error) {
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

	responseJSON, err := json.Marshal(response)
	if err != nil {
		s.Logger.Error("failed-to-marshal-response", err)
		return []byte{}, err
	}
	return responseJSON, nil
}

func (s *CallbackStagingCompleter) getCallbackURI(taskCompletedRequest cf.StagingCompletedRequest) (string, error) {
	var annotation cc_messages.StagingTaskAnnotation
	if err := json.Unmarshal([]byte(taskCompletedRequest.Annotation), &annotation); err != nil {
		s.Logger.Error("failed-to-parse-annotation", err)
		return "", err
	}

	return annotation.CompletionCallback, nil
}
