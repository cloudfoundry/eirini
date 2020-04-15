package stager

import (
	"encoding/json"
	"fmt"

	"code.cloudfoundry.org/bbs/models"
	"code.cloudfoundry.org/eirini"
	"code.cloudfoundry.org/eirini/models/cf"
	"code.cloudfoundry.org/eirini/opi"
	"code.cloudfoundry.org/lager"
	"go.uber.org/multierr"
)

//go:generate counterfeiter . StagingCompleter
type StagingCompleter interface {
	CompleteStaging(*models.TaskCallbackResponse) error
}

type Stager struct {
	Desirer          opi.TaskDesirer
	StagingCompleter StagingCompleter
	Config           *eirini.StagerConfig
	Logger           lager.Logger
}

func New(desirer opi.TaskDesirer, stagingCompleter StagingCompleter, config eirini.StagerConfig, logger lager.Logger) *Stager {
	return &Stager{
		Desirer:          desirer,
		StagingCompleter: stagingCompleter,
		Config:           &config,
		Logger:           logger,
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
	if lifecycleData == nil {
		lifecycleData = request.Lifecycle.BuildpackLifecycle
	}
	buildpacksJSON, err := json.Marshal(lifecycleData.Buildpacks)
	if err != nil {
		return nil, err
	}

	eiriniEnv := map[string]string{
		eirini.EnvDownloadURL:                     lifecycleData.AppBitsDownloadURI,
		eirini.EnvDropletUploadURL:                lifecycleData.DropletUploadURI,
		eirini.EnvBuildpacks:                      string(buildpacksJSON),
		eirini.EnvAppID:                           request.AppGUID,
		eirini.EnvStagingGUID:                     stagingGUID,
		eirini.EnvCompletionCallback:              request.CompletionCallback,
		eirini.EnvEiriniAddress:                   s.Config.EiriniAddress,
		eirini.EnvBuildpackCacheDownloadURI:       lifecycleData.BuildpackCacheDownloadURI,
		eirini.EnvBuildpackCacheUploadURI:         lifecycleData.BuildpackCacheUploadURI,
		eirini.EnvBuildpackCacheChecksum:          lifecycleData.BuildpackCacheChecksum,
		eirini.EnvBuildpackCacheChecksumAlgorithm: lifecycleData.BuildpackCacheChecksumAlgorithm,
		"TMPDIR": fmt.Sprintf("%s/tmp", eirini.BuildpackCacheDir),
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
			GUID:      stagingGUID,
			AppName:   request.AppName,
			AppGUID:   request.AppGUID,
			OrgName:   request.OrgName,
			SpaceName: request.SpaceName,
			OrgGUID:   request.OrgGUID,
			SpaceGUID: request.SpaceGUID,
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
		s.StagingCompleter.CompleteStaging(task),
		s.Desirer.Delete(task.TaskGuid),
	)
}

func mergeEnvVriables(eiriniEnv map[string]string, cfEnvs []cf.EnvironmentVariable) map[string]string {
	for _, env := range cfEnvs {
		if _, present := eiriniEnv[env.Name]; !present {
			eiriniEnv[env.Name] = env.Value
		}
	}

	return eiriniEnv
}
