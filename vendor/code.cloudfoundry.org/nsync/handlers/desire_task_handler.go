package handlers

import (
	"encoding/json"
	"errors"
	"net/http"

	"code.cloudfoundry.org/bbs"
	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/nsync/recipebuilder"
	"code.cloudfoundry.org/runtimeschema/cc_messages"
)

type TaskHandler struct {
	logger         lager.Logger
	recipeBuilders map[string]recipebuilder.RecipeBuilder
	bbsClient      bbs.Client
}

func NewTaskHandler(
	logger lager.Logger,
	bbsClient bbs.Client,
	recipeBuilders map[string]recipebuilder.RecipeBuilder,
) TaskHandler {
	return TaskHandler{
		logger:         logger,
		recipeBuilders: recipeBuilders,
		bbsClient:      bbsClient,
	}
}

func (h *TaskHandler) DesireTask(resp http.ResponseWriter, req *http.Request) {
	logger := h.logger.Session("create-task", lager.Data{
		"method":  req.Method,
		"request": req.URL.String(),
	})

	logger.Info("serving")
	defer logger.Info("complete")

	task := cc_messages.TaskRequestFromCC{}
	err := json.NewDecoder(req.Body).Decode(&task)
	if err != nil {
		logger.Error("parse-task-request-failed", err)
		resp.WriteHeader(http.StatusBadRequest)
		return
	}

	builder, ok := h.recipeBuilders[task.Lifecycle]
	if !ok {
		logger.Error("builder-not-found", errors.New("no-builder"), lager.Data{"lifecycle": task.Lifecycle})
		resp.WriteHeader(http.StatusBadRequest)
		return
	}

	desiredTask, err := builder.BuildTask(&task)
	if err != nil {
		logger.Error("building-task-failed", err)
		resp.WriteHeader(http.StatusBadRequest)
		return
	}

	logger.Info("desiring-task", lager.Data{"task-guid": task.TaskGuid})
	err = h.bbsClient.DesireTask(logger, task.TaskGuid, cc_messages.RunningTaskDomain, desiredTask)
	if err != nil {
		logger.Error("desire-task-failed", err)
		resp.WriteHeader(http.StatusBadRequest)
		return
	}

	resp.WriteHeader(http.StatusAccepted)
}
