package stager

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"net/http"

	"code.cloudfoundry.org/bbs/models"
	"code.cloudfoundry.org/eirini"
	"code.cloudfoundry.org/eirini/opi"
	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/runtimeschema/cc_messages"
)

const StagerImage = "diegoteam/recipe:build"

type Stager struct {
	Desirer    opi.TaskDesirer
	Config     *eirini.StagerConfig
	Logger     lager.Logger
	HTTPClient *http.Client
}

func New(desirer opi.TaskDesirer, config eirini.StagerConfig) *Stager {
	return &Stager{
		Desirer: desirer,
		Config:  &config,
		Logger:  lager.NewLogger("stager"),
		HTTPClient: &http.Client{
			Transport: &http.Transport{
				TLSClientConfig: &tls.Config{
					InsecureSkipVerify: config.SkipSslValidation,
				},
			},
		},
	}
}

func (s *Stager) Stage(stagingGUID string, request cc_messages.StagingRequestFromCC) error {
	task, err := s.createStagingTask(stagingGUID, request)
	if err != nil {
		s.Logger.Error("failed-tocreate-staging-task", err)
		return err
	}

	return s.Desirer.Desire(task)
}

func (s *Stager) createStagingTask(stagingGUID string, request cc_messages.StagingRequestFromCC) (*opi.Task, error) {
	s.Logger.Debug("create-staging-task", lager.Data{"app-id": request.AppId, "staging-guid": stagingGUID})

	var lifecycleData cc_messages.BuildpackStagingData
	if err := json.Unmarshal(*request.LifecycleData, &lifecycleData); err != nil {
		s.Logger.Error("failed-parsing-lifecycle-data", err)
		return &opi.Task{}, err
	}

	stagingTask := &opi.Task{
		Image: StagerImage,
		Env: map[string]string{
			eirini.EnvDownloadURL:        lifecycleData.AppBitsDownloadUri,
			eirini.EnvUploadURL:          lifecycleData.DropletUploadUri,
			eirini.EnvAppID:              request.LogGuid,
			eirini.EnvStagingGUID:        stagingGUID,
			eirini.EnvCompletionCallback: request.CompletionCallback,
			eirini.EnvCfUsername:         s.Config.CfUsername,
			eirini.EnvCfPassword:         s.Config.CfPassword,
			eirini.EnvAPIAddress:         s.Config.APIAddress,
			eirini.EnvEiriniAddress:      s.Config.EiriniAddress,
		},
	}
	return stagingTask, nil
}

func (s *Stager) CompleteStaging(task *models.TaskCallbackResponse) error {
	l := s.Logger.Session("complete-staging", lager.Data{"task-guid": task.TaskGuid})

	callbackBody, err := s.constructStagingResponse(task)
	if err != nil {
		l.Error("failed-to-construct-staging-response", err)
		return err
	}

	callbackURI, err := s.getCallbackURI(task)
	if err != nil {
		l.Error("failed-to-parse-callback-uri", err)
		return err
	}

	request, err := http.NewRequest("PUT", callbackURI, bytes.NewBuffer(callbackBody))
	if err != nil {
		l.Error("failed-to-create-callback-request", err)
		return err
	}
	request.Header.Set("Content-Type", "application/json")

	if err := s.executeRequest(request); err != nil {
		return err
	}

	return s.Desirer.Delete(task.TaskGuid)
}

func (s *Stager) executeRequest(request *http.Request) error {
	l := s.Logger.Session("execute-callback-request", lager.Data{"request-uri": request.URL})

	resp, err := s.HTTPClient.Do(request)
	if err != nil {
		l.Error("cc-staging-complete-failed", err)
		return err
	}
	if resp.StatusCode >= 300 {
		l.Error("cc-staging-complete-failed-status-code", nil, lager.Data{"status-code": resp.StatusCode})
		return fmt.Errorf("callback-response-unsuccessful, code: %d", resp.StatusCode)
	}
	return nil
}

func (s *Stager) constructStagingResponse(task *models.TaskCallbackResponse) ([]byte, error) {
	var response cc_messages.StagingResponseForCC

	if task.Failed {
		response.Error = &cc_messages.StagingError{
			Id:      cc_messages.STAGING_ERROR,
			Message: task.FailureReason,
		}
	} else {
		result := json.RawMessage([]byte(task.Result))
		response.Result = &result
	}

	responseJSON, err := json.Marshal(response)
	if err != nil {
		s.Logger.Error("failed-to-marshal-response", err)
		return []byte{}, err
	}
	return responseJSON, nil
}

func (s *Stager) getCallbackURI(task *models.TaskCallbackResponse) (string, error) {
	var annotation cc_messages.StagingTaskAnnotation
	if err := json.Unmarshal([]byte(task.Annotation), &annotation); err != nil {
		s.Logger.Error("failed-to-parse-annotation", err)
		return "", err
	}

	return annotation.CompletionCallback, nil
}
