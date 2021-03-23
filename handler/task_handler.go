package handler

import (
	"encoding/json"
	"errors"
	"net/http"

	"code.cloudfoundry.org/eirini"
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

	ctx := req.Context()

	response, err := t.taskBifrost.GetTask(ctx, taskGUID)
	if err != nil {
		if errors.Is(err, eirini.ErrNotFound) {
			logger.Info("task-not-found")
			writeErrorResponse(logger, resp, http.StatusNotFound, err)

			return
		}

		logger.Error("get-task-request-failed", err)
		writeErrorResponse(logger, resp, http.StatusInternalServerError, err)

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
		writeErrorResponse(logger, resp, http.StatusBadRequest, err)

		return
	}

	if err := t.taskBifrost.TransferTask(req.Context(), taskGUID, taskRequest); err != nil {
		logger.Error("task-request-task-create-failed", err)
		writeErrorResponse(logger, resp, http.StatusInternalServerError, err)

		return
	}

	resp.WriteHeader(http.StatusAccepted)
}

func (t *Task) Cancel(resp http.ResponseWriter, req *http.Request, ps httprouter.Params) {
	taskGUID := ps.ByName("task_guid")
	logger := t.logger.Session("task-cancel", lager.Data{"task-guid": taskGUID})

	ctx := req.Context()

	if err := t.taskBifrost.CancelTask(ctx, taskGUID); err != nil {
		logger.Error("task-request-task-delete-failed", err)
		writeErrorResponse(logger, resp, http.StatusInternalServerError, err)

		return
	}

	resp.WriteHeader(http.StatusNoContent)
}

func (t *Task) List(resp http.ResponseWriter, req *http.Request, ps httprouter.Params) {
	logger := t.logger.Session("list-tasks")
	ctx := req.Context()

	tasks, err := t.taskBifrost.ListTasks(ctx)
	if err != nil {
		logger.Error("list-tasks-request-failed", err)
		resp.WriteHeader(http.StatusInternalServerError)

		return
	}

	if err = json.NewEncoder(resp).Encode(tasks); err != nil {
		logger.Error("encode-json-failed", err)
		resp.WriteHeader(http.StatusInternalServerError)
	}
}
