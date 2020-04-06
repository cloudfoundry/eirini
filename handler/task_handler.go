package handler

import (
	"encoding/json"
	"net/http"

	"code.cloudfoundry.org/eirini"
	"code.cloudfoundry.org/eirini/models/cf"
	"code.cloudfoundry.org/lager"
	"github.com/julienschmidt/httprouter"
)

type Task struct {
	logger  lager.Logger
	bifrost eirini.Bifrost
}

func NewTaskHandler(logger lager.Logger, bifrost eirini.Bifrost) *Task {
	return &Task{
		logger:  logger,
		bifrost: bifrost,
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

	if err := t.bifrost.TransferTask(req.Context(), taskGUID, taskRequest); err != nil {
		logger.Error("task-request-task-create-failed", err)
		writeErrorResponse(resp, http.StatusInternalServerError, err)
		return
	}

	resp.WriteHeader(http.StatusAccepted)
}
