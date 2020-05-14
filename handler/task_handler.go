package handler

import (
	"encoding/json"
	"net/http"

	"code.cloudfoundry.org/eirini/models/cf"
	"code.cloudfoundry.org/lager"
	"github.com/julienschmidt/httprouter"
)

type Task struct {
	logger      lager.Logger
	taskBifrost TaskBifrost
}

func NewTaskHandler(logger lager.Logger, taskBifrost TaskBifrost) *Task {
	return &Task{
		logger:      logger,
		taskBifrost: taskBifrost,
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

	if err := t.taskBifrost.TransferTask(req.Context(), taskGUID, taskRequest); err != nil {
		logger.Error("task-request-task-create-failed", err)
		writeErrorResponse(resp, http.StatusInternalServerError, err)
		return
	}

	resp.WriteHeader(http.StatusAccepted)
}

func (t *Task) CompleteTask(resp http.ResponseWriter, req *http.Request, ps httprouter.Params) {
	taskGUID := ps.ByName("task_guid")
	logger := t.logger.Session("task-delete", lager.Data{"task-guid": taskGUID})

	if err := t.taskBifrost.CompleteTask(taskGUID); err != nil {
		logger.Error("task-request-task-delete-failed", err)
		writeErrorResponse(resp, http.StatusInternalServerError, err)
		return
	}

	resp.WriteHeader(http.StatusOK)
}
