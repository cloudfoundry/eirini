package handler

import (
	"encoding/json"
	"net/http"

	"code.cloudfoundry.org/eirini"
	"code.cloudfoundry.org/eirini/models/cf"
	"code.cloudfoundry.org/eirini/opi"
	"code.cloudfoundry.org/lager"
	"github.com/julienschmidt/httprouter"
	"github.com/pkg/errors"
)

type Task struct {
	logger     lager.Logger
	desirer    eirini.TaskDesirer
	registryIP string
}

func NewTaskHandler(logger lager.Logger, desirer eirini.TaskDesirer, registryIP string) *Task {
	return &Task{
		logger:     logger,
		desirer:    desirer,
		registryIP: registryIP,
	}
}

func (t *Task) Run(resp http.ResponseWriter, req *http.Request, ps httprouter.Params) {
	taskGUID := ps.ByName("task_guid")
	logger := t.logger.Session("task-request", lager.Data{"task-guid": taskGUID})

	var taskRequest cf.TaskRequest
	if err := json.NewDecoder(req.Body).Decode(&taskRequest); err != nil {
		logger.Error("task-request-body-decoding-failed", err)
		writeErrorResponse(resp, http.StatusBadRequest, err)
		return
	}

	task, err := t.createTask(taskGUID, taskRequest)
	if err != nil {
		logger.Error("task-request-task-create-failed", err)
		writeErrorResponse(resp, http.StatusBadRequest, err)
		return
	}

	if err := t.desirer.Desire(task); err != nil {
		logger.Error("task-request-task-desire-failed", err)
		writeErrorResponse(resp, http.StatusInternalServerError, err)
		return
	}

	resp.WriteHeader(http.StatusAccepted)
}

func (t *Task) createTask(taskGUID string, request cf.TaskRequest) (*opi.Task, error) {
	t.logger.Debug("create-task", lager.Data{"app-id": request.AppGUID, "staging-guid": taskGUID})

	if request.Lifecycle.BuildpackLifecycle == nil {
		return nil, errors.New("unsupported lifecycle, only buildpack lifecycle is supported")
	}

	lifecycle := request.Lifecycle.BuildpackLifecycle
	buildpackEnv := map[string]string{
		"HOME":          "/home/vcap/app",
		"PATH":          "/usr/local/bin:/usr/bin:/bin",
		"USER":          "vcap",
		"TMPDIR":        "/home/vcap/tmp",
		"START_COMMAND": lifecycle.StartCommand,
	}

	task := &opi.Task{
		TaskGUID:  taskGUID,
		AppName:   request.AppName,
		AppGUID:   request.AppGUID,
		OrgName:   request.OrgName,
		SpaceName: request.SpaceName,
		OrgGUID:   request.OrgGUID,
		SpaceGUID: request.SpaceGUID,
		Env:       mergeEnvs(request.Environment, buildpackEnv),
		Image:     lifecycle.DropletURI,
	}
	return task, nil
}

func mergeEnvs(env1 []cf.EnvironmentVariable, env2 map[string]string) map[string]string {
	result := make(map[string]string)
	for _, v := range env1 {
		result[v.Name] = v.Value
	}

	for k, v := range env2 {
		result[k] = v
	}
	return result
}
