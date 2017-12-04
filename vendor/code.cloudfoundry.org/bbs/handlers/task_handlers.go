package handlers

import (
	"net/http"
	"time"

	"code.cloudfoundry.org/bbs/models"
	"code.cloudfoundry.org/lager"
)

//go:generate counterfeiter -o fake_controllers/fake_task_controller.go . TaskController

type TaskController interface {
	Tasks(logger lager.Logger, domain, cellId string) ([]*models.Task, error)
	TaskByGuid(logger lager.Logger, taskGuid string) (*models.Task, error)
	DesireTask(logger lager.Logger, taskDefinition *models.TaskDefinition, taskGuid, domain string) error
	StartTask(logger lager.Logger, taskGuid, cellId string) (shouldStart bool, err error)
	CancelTask(logger lager.Logger, taskGuid string) error
	FailTask(logger lager.Logger, taskGuid, failureReason string) error
	CompleteTask(logger lager.Logger, taskGuid, cellId string, failed bool, failureReason, result string) error
	ResolvingTask(logger lager.Logger, taskGuid string) error
	DeleteTask(logger lager.Logger, taskGuid string) error
	ConvergeTasks(logger lager.Logger, kickTaskDuration, expirePendingTaskDuration, expireCompletedTaskDuration time.Duration) error
}

type TaskHandler struct {
	controller TaskController
	exitChan   chan<- struct{}
}

func NewTaskHandler(
	controller TaskController,
	exitChan chan<- struct{},
) *TaskHandler {
	return &TaskHandler{
		controller: controller,
		exitChan:   exitChan,
	}
}

func (h *TaskHandler) Tasks(logger lager.Logger, w http.ResponseWriter, req *http.Request) {
	var err error
	logger = logger.Session("tasks")

	request := &models.TasksRequest{}
	response := &models.TasksResponse{}

	defer func() { exitIfUnrecoverable(logger, h.exitChan, response.Error) }()
	defer func() { writeResponse(w, response) }()

	err = parseRequest(logger, req, request)
	if err != nil {
		logger.Error("failed-parsing-request", err)
		response.Error = models.ConvertError(err)
		return
	}

	response.Tasks, err = h.controller.Tasks(logger, request.Domain, request.CellId)
	response.Error = models.ConvertError(err)
}

func (h *TaskHandler) TaskByGuid(logger lager.Logger, w http.ResponseWriter, req *http.Request) {
	var err error
	logger = logger.Session("task-by-guid")

	request := &models.TaskByGuidRequest{}
	response := &models.TaskResponse{}

	defer func() { exitIfUnrecoverable(logger, h.exitChan, response.Error) }()
	defer func() { writeResponse(w, response) }()

	err = parseRequest(logger, req, request)
	if err != nil {
		logger.Error("failed-parsing-request", err)
		response.Error = models.ConvertError(err)
		return
	}

	response.Task, err = h.controller.TaskByGuid(logger, request.TaskGuid)
	response.Error = models.ConvertError(err)
}

func (h *TaskHandler) DesireTask(logger lager.Logger, w http.ResponseWriter, req *http.Request) {
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

	err = h.controller.DesireTask(logger, request.TaskDefinition, request.TaskGuid, request.Domain)
	response.Error = models.ConvertError(err)
}

func (h *TaskHandler) StartTask(logger lager.Logger, w http.ResponseWriter, req *http.Request) {
	var err error
	logger = logger.Session("start-task")

	request := &models.StartTaskRequest{}
	response := &models.StartTaskResponse{}

	defer func() { exitIfUnrecoverable(logger, h.exitChan, response.Error) }()
	defer func() { writeResponse(w, response) }()

	err = parseRequest(logger, req, request)
	if err != nil {
		logger.Error("failed-parsing-request", err)
		response.Error = models.ConvertError(err)
		return
	}

	response.ShouldStart, err = h.controller.StartTask(logger, request.TaskGuid, request.CellId)
	response.Error = models.ConvertError(err)
}

func (h *TaskHandler) CancelTask(logger lager.Logger, w http.ResponseWriter, req *http.Request) {
	logger = logger.Session("cancel-task")

	request := &models.TaskGuidRequest{}
	response := &models.TaskLifecycleResponse{}

	defer func() { exitIfUnrecoverable(logger, h.exitChan, response.Error) }()
	defer func() { writeResponse(w, response) }()

	err := parseRequest(logger, req, request)
	if err != nil {
		logger.Error("failed-parsing-request", err)
		response.Error = models.ConvertError(err)
		return
	}

	err = h.controller.CancelTask(logger, request.TaskGuid)
	response.Error = models.ConvertError(err)
}

func (h *TaskHandler) FailTask(logger lager.Logger, w http.ResponseWriter, req *http.Request) {
	var err error
	logger = logger.Session("fail-task")

	request := &models.FailTaskRequest{}
	response := &models.TaskLifecycleResponse{}

	defer func() { exitIfUnrecoverable(logger, h.exitChan, response.Error) }()
	defer func() { writeResponse(w, response) }()

	err = parseRequest(logger, req, request)
	if err != nil {
		logger.Error("failed-parsing-request", err)
		response.Error = models.ConvertError(err)
		return
	}

	err = h.controller.FailTask(logger, request.TaskGuid, request.FailureReason)
	response.Error = models.ConvertError(err)
}

func (h *TaskHandler) CompleteTask(logger lager.Logger, w http.ResponseWriter, req *http.Request) {
	var err error
	logger = logger.Session("complete-task")

	request := &models.CompleteTaskRequest{}
	response := &models.TaskLifecycleResponse{}

	defer func() { exitIfUnrecoverable(logger, h.exitChan, response.Error) }()
	defer func() { writeResponse(w, response) }()

	err = parseRequest(logger, req, request)
	if err != nil {
		response.Error = models.ConvertError(err)
		logger.Error("failed-parsing-request", err)
		return
	}

	err = h.controller.CompleteTask(logger, request.TaskGuid, request.CellId, request.Failed, request.FailureReason, request.Result)
	response.Error = models.ConvertError(err)
}

func (h *TaskHandler) ResolvingTask(logger lager.Logger, w http.ResponseWriter, req *http.Request) {
	var err error
	logger = logger.Session("resolving-task")

	request := &models.TaskGuidRequest{}
	response := &models.TaskLifecycleResponse{}

	defer func() { exitIfUnrecoverable(logger, h.exitChan, response.Error) }()
	defer func() { writeResponse(w, response) }()

	err = parseRequest(logger, req, request)
	if err != nil {
		logger.Error("failed-parsing-request", err)
		response.Error = models.ConvertError(err)
		return
	}

	err = h.controller.ResolvingTask(logger, request.TaskGuid)
	response.Error = models.ConvertError(err)
}

func (h *TaskHandler) DeleteTask(logger lager.Logger, w http.ResponseWriter, req *http.Request) {
	var err error
	logger = logger.Session("delete-task")

	request := &models.TaskGuidRequest{}
	response := &models.TaskLifecycleResponse{}

	defer func() { exitIfUnrecoverable(logger, h.exitChan, response.Error) }()
	defer func() { writeResponse(w, response) }()

	err = parseRequest(logger, req, request)
	if err != nil {
		logger.Error("failed-parsing-request", err)
		response.Error = models.ConvertError(err)
		return
	}

	err = h.controller.DeleteTask(logger, request.TaskGuid)
	response.Error = models.ConvertError(err)
}
