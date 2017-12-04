package handlers

import (
	"io/ioutil"
	"net/http"

	"code.cloudfoundry.org/bbs/format"
	"code.cloudfoundry.org/bbs/models"
	"code.cloudfoundry.org/lager"
)

func (h *TaskHandler) commonTasks(logger lager.Logger, w http.ResponseWriter, req *http.Request, version format.Version) {
	var err error
	logger = logger.Session("tasks", lager.Data{"revision": 0})

	request := &models.TasksRequest{}
	response := &models.TasksResponse{}

	defer func() { exitIfUnrecoverable(logger, h.exitChan, response.Error) }()
	defer writeResponse(w, response)

	err = parseRequest(logger, req, request)
	if err != nil {
		logger.Error("failed-parsing-request", err)
		response.Error = models.ConvertError(err)
		return
	}

	filter := models.TaskFilter{Domain: request.Domain, CellID: request.CellId}
	response.Tasks, err = h.controller.Tasks(logger, filter.Domain, filter.CellID)
	if err != nil {
		response.Error = models.ConvertError(err)
		return
	}

	for i := range response.Tasks {
		task := response.Tasks[i]
		if task.TaskDefinition == nil {
			continue
		}
		response.Tasks[i] = task.VersionDownTo(version)
	}
}

func (h *TaskHandler) Tasks_r0(logger lager.Logger, w http.ResponseWriter, req *http.Request) {
	h.commonTasks(logger, w, req, format.V0)
}

func (h *TaskHandler) Tasks_r1(logger lager.Logger, w http.ResponseWriter, req *http.Request) {
	h.commonTasks(logger, w, req, format.V1)
}

func (h *TaskHandler) commonTaskByGuid(logger lager.Logger, w http.ResponseWriter, req *http.Request, version format.Version) {
	var err error
	logger = logger.Session("task-by-guid", lager.Data{"revision": 0})

	request := &models.TaskByGuidRequest{}
	response := &models.TaskResponse{}

	defer func() { exitIfUnrecoverable(logger, h.exitChan, response.Error) }()
	defer writeResponse(w, response)

	err = parseRequest(logger, req, request)
	if err != nil {
		logger.Error("failed-parsing-request", err)
		response.Error = models.ConvertError(err)
		return
	}

	response.Task, err = h.controller.TaskByGuid(logger, request.TaskGuid)
	if err != nil {
		response.Error = models.ConvertError(err)
		return
	}

	if response.Task.TaskDefinition != nil {
		response.Task = response.Task.VersionDownTo(version)
	}
}

func (h *TaskHandler) TaskByGuid_r0(logger lager.Logger, w http.ResponseWriter, req *http.Request) {
	h.commonTaskByGuid(logger, w, req, format.V0)
}

func (h *TaskHandler) TaskByGuid_r1(logger lager.Logger, w http.ResponseWriter, req *http.Request) {
	h.commonTaskByGuid(logger, w, req, format.V1)
}

// this should upconvert the deprecated VolumeMounts struct
func (h *TaskHandler) DesireTask_r1(logger lager.Logger, w http.ResponseWriter, req *http.Request) {
	var err error
	logger = logger.Session("desire-task")

	request := &models.DesireTaskRequest{}
	response := &models.TaskLifecycleResponse{}

	defer func() { exitIfUnrecoverable(logger, h.exitChan, response.Error) }()
	defer func() { writeResponse(w, response) }()

	err = parseRequest(logger, req, request)
	if err != nil {
		logger.Error("failed-parsing-request", err)
		response.Error = models.ConvertError(err)
		return
	}

	for i, mount := range request.TaskDefinition.VolumeMounts {
		request.TaskDefinition.VolumeMounts[i] = mount.VersionUpToV1()
	}

	err = h.controller.DesireTask(logger, request.TaskDefinition, request.TaskGuid, request.Domain)
	response.Error = models.ConvertError(err)
}

func (h *TaskHandler) DesireTask_r0(logger lager.Logger, w http.ResponseWriter, req *http.Request) {
	var err error
	logger = logger.Session("desire-task")

	request := &models.DesireTaskRequest{}
	response := &models.TaskLifecycleResponse{}

	defer func() { exitIfUnrecoverable(logger, h.exitChan, response.Error) }()
	defer writeResponse(w, response)

	err = parseRequestForDesireTask_r0(logger, req, request)
	if err != nil {
		logger.Error("failed-parsing-request", err)
		response.Error = models.ConvertError(err)
		return
	}

	err = h.controller.DesireTask(logger, request.TaskDefinition, request.TaskGuid, request.Domain)
	if err != nil {
		response.Error = models.ConvertError(err)
		return
	}
}

func parseRequestForDesireTask_r0(logger lager.Logger, req *http.Request, request *models.DesireTaskRequest) error {
	data, err := ioutil.ReadAll(req.Body)
	if err != nil {
		logger.Error("failed-to-read-body", err)
		return models.ErrUnknownError
	}

	err = request.Unmarshal(data)
	if err != nil {
		logger.Error("failed-to-parse-request-body", err)
		return models.ErrBadRequest
	}

	request.TaskDefinition.Action.SetTimeoutMsFromDeprecatedTimeoutNs()

	if err := request.Validate(); err != nil {
		logger.Error("invalid-request", err)
		return models.NewError(models.Error_InvalidRequest, err.Error())
	}

	return nil
}
