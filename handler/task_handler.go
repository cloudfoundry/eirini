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

func (t *Task) Get(resp http.ResponseWriter, req *http.Request, ps httprouter.Params) {
	taskGUID := ps.ByName("task_guid")
	logger := t.logger.Session("get-task-request", lager.Data{"task-guid": taskGUID})

	response, err := t.taskBifrost.GetTask(taskGUID)
	if err != nil {
		logger.Error("get-task-request-failed", err)
		writeErrorResponse(resp, http.StatusInternalServerError, err)

		return
	}

	if err := json.NewEncoder(resp).Encode(response); err != nil {
		logger.Error("encode-json-failed", err)
		resp.WriteHeader(http.StatusInternalServerError)
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

func (t *Task) Cancel(resp http.ResponseWriter, req *http.Request, ps httprouter.Params) {
	taskGUID := ps.ByName("task_guid")
	logger := t.logger.Session("task-cancel", lager.Data{"task-guid": taskGUID})

	if err := t.taskBifrost.CancelTask(taskGUID); err != nil {
		logger.Error("task-request-task-delete-failed", err)
		writeErrorResponse(resp, http.StatusInternalServerError, err)

		return
	}

	resp.WriteHeader(http.StatusNoContent)
}
