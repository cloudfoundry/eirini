package handlers

import (
	"net/http"

	"code.cloudfoundry.org/bbs"
	"code.cloudfoundry.org/bbs/models"
	"code.cloudfoundry.org/lager"
)

type CancelTaskHandler struct {
	logger    lager.Logger
	bbsClient bbs.Client
}

func NewCancelTaskHandler(
	logger lager.Logger,
	bbsClient bbs.Client,
) CancelTaskHandler {
	return CancelTaskHandler{
		logger:    logger,
		bbsClient: bbsClient,
	}
}

func (h *CancelTaskHandler) CancelTask(resp http.ResponseWriter, req *http.Request) {
	logger := h.logger.Session("cancel-task", lager.Data{
		"method":  req.Method,
		"request": req.URL.String(),
	})

	logger.Info("serving")
	defer logger.Info("complete")

	taskGuid := req.FormValue(":task_guid")

	logger.Info("canceling-task", lager.Data{"task-guid": taskGuid})
	err := h.bbsClient.CancelTask(logger, taskGuid)
	if err != nil {
		logger.Error("cancel-task-failed", err)
		if err == models.ErrResourceNotFound {
			resp.WriteHeader(http.StatusNotFound)
			return
		}

		resp.WriteHeader(http.StatusInternalServerError)
		return
	}

	resp.WriteHeader(http.StatusAccepted)
}
