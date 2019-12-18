package stager

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"code.cloudfoundry.org/bbs/models"
	"code.cloudfoundry.org/eirini"
	"code.cloudfoundry.org/eirini/models/cf"
	"code.cloudfoundry.org/eirini/opi"
	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/runtimeschema/cc_messages"
	"go.uber.org/multierr"
)

type Stager struct {
	Desirer    opi.TaskDesirer
	Config     *eirini.StagerConfig
	Logger     lager.Logger
	HTTPClient *http.Client
	Retries    int
	Delay      time.Duration
}

func New(desirer opi.TaskDesirer, httpClient *http.Client, config eirini.StagerConfig, logger lager.Logger) *Stager {
	return &Stager{
		Desirer:    desirer,
		Config:     &config,
		Logger:     logger,
		HTTPClient: httpClient,
		Retries:    10,
		Delay:      2 * time.Second,
	}
}

func (s *Stager) Stage(stagingGUID string, request cf.StagingRequest) error {
	task, err := s.createStagingTask(stagingGUID, request)
	if err != nil {
		s.Logger.Error("failed-tocreate-staging-task", err)
		return err
	}

	return s.Desirer.DesireStaging(task)
}

func (s *Stager) createStagingTask(stagingGUID string, request cf.StagingRequest) (*opi.StagingTask, error) {
	s.Logger.Debug("create-staging-task", lager.Data{"app-id": request.AppGUID, "staging-guid": stagingGUID})

	lifecycleData := request.LifecycleData
	buildpacksJSON, err := json.Marshal(lifecycleData.Buildpacks)
	if err != nil {
		return nil, err
	}

	eiriniEnv := map[string]string{
		eirini.EnvDownloadURL:        lifecycleData.AppBitsDownloadURI,
		eirini.EnvDropletUploadURL:   lifecycleData.DropletUploadURI,
		eirini.EnvBuildpacks:         string(buildpacksJSON),
		eirini.EnvAppID:              request.AppGUID,
		eirini.EnvStagingGUID:        stagingGUID,
		eirini.EnvCompletionCallback: request.CompletionCallback,
		eirini.EnvEiriniAddress:      s.Config.EiriniAddress,
	}

	stagingEnv := mergeEnvVriables(eiriniEnv, request.Environment)

	memMB := request.MemoryMB
	if memMB == 0 {
		memMB = 200
	}

	diskMB := request.DiskMB
	if diskMB == 0 {
		diskMB = 500
	}

	cpuWeight := request.CPUWeight
	if cpuWeight == 0 {
		cpuWeight = 50
	}

	stagingTask := &opi.StagingTask{
		DownloaderImage: s.Config.DownloaderImage,
		UploaderImage:   s.Config.UploaderImage,
		ExecutorImage:   s.Config.ExecutorImage,
		Task: &opi.Task{
			Env:       stagingEnv,
			MemoryMB:  memMB,
			DiskMB:    diskMB,
			CPUWeight: cpuWeight,
		},
	}
	return stagingTask, nil
}

func (s *Stager) CompleteStaging(task *models.TaskCallbackResponse) error {
	l := s.Logger.Session("complete-staging", lager.Data{"task-guid": task.TaskGuid})
	l.Debug("Complete staging")
	return multierr.Combine(
		s.sendCompletionCallback(task),
		s.Desirer.Delete(task.TaskGuid),
	)
}

func (s *Stager) sendCompletionCallback(task *models.TaskCallbackResponse) error {
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

func (s *Stager) executeRequestWithRetries(makeRequest func() *http.Request) error {
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

func (s *Stager) executeRequest(request *http.Request) error {
	l := s.Logger.Session("execute-callback-request", lager.Data{"request-uri": request.URL})

	resp, err := s.HTTPClient.Do(request)
	if err != nil {
		l.Error("cc-staging-complete-failed", err)
		return err
	}
	defer resp.Body.Close()
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

func mergeEnvVriables(eiriniEnv map[string]string, cfEnvs []cf.EnvironmentVariable) map[string]string {
	for _, env := range cfEnvs {
		if _, present := eiriniEnv[env.Name]; !present {
			eiriniEnv[env.Name] = env.Value
		}
	}

	return eiriniEnv
}
