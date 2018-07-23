package st8ger

import (
	"encoding/json"

	"code.cloudfoundry.org/bbs/models"
	"code.cloudfoundry.org/eirini"
	"code.cloudfoundry.org/eirini/opi"
	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/runtimeschema/cc_messages"
)

type backend struct {
	config eirini.BackendConfig
	logger lager.Logger
}

func NewBackend(config eirini.BackendConfig, logger lager.Logger) eirini.Backend {
	return &backend{
		config: config,
		logger: logger.Session("kubernetes"),
	}
}

func (b backend) CreateStagingTask(stagingGUID string, request cc_messages.StagingRequestFromCC) (opi.Task, error) {
	logger := b.logger.Session("create-staging-task", lager.Data{"app-id": request.AppId, "staging-guid": stagingGUID})
	logger.Info("staging-request")

	var lifecycleData cc_messages.BuildpackStagingData
	err := json.Unmarshal(*request.LifecycleData, &lifecycleData)
	if err != nil {
		return opi.Task{}, err
	}

	stagingTask := opi.Task{
		Image: "diegoteam/recipe:build",
		Env: map[string]string{
			eirini.EnvDownloadURL:        lifecycleData.AppBitsDownloadUri,
			eirini.EnvUploadURL:          lifecycleData.DropletUploadUri,
			eirini.EnvAppID:              request.LogGuid,
			eirini.EnvStagingGUID:        stagingGUID,
			eirini.EnvCompletionCallback: request.CompletionCallback,
			eirini.EnvCfUsername:         b.config.CfUsername,
			eirini.EnvCfPassword:         b.config.CfPassword,
			eirini.EnvAPIAddress:         b.config.APIAddress,
			eirini.EnvEiriniAddress:      b.config.EiriniAddress,
		},
	}
	return stagingTask, nil
}

func (b backend) BuildStagingResponse(task *models.TaskCallbackResponse) (cc_messages.StagingResponseForCC, error) {
	var response cc_messages.StagingResponseForCC

	result := json.RawMessage([]byte(task.Result))
	response.Result = &result

	return response, nil
}
