package handlers

import (
	"net/http"

	"code.cloudfoundry.org/bbs"
	"code.cloudfoundry.org/bbs/models"
	"code.cloudfoundry.org/lager"
)

type StopAppHandler struct {
	bbsClient bbs.Client
	logger    lager.Logger
}

func NewStopAppHandler(logger lager.Logger, bbsClient bbs.Client) *StopAppHandler {
	return &StopAppHandler{
		logger:    logger,
		bbsClient: bbsClient,
	}
}

func (h *StopAppHandler) StopApp(resp http.ResponseWriter, req *http.Request) {
	processGuid := req.FormValue(":process_guid")

	logger := h.logger.Session("stop-app", lager.Data{
		"process-guid": processGuid,
		"method":       req.Method,
		"request":      req.URL.String(),
	})

	if processGuid == "" {
		logger.Error("missing-process-guid", missingParameterErr)
		resp.WriteHeader(http.StatusBadRequest)
		return
	}

	logger.Info("serving")
	defer logger.Info("complete")

	logger.Debug("removing-desired-lrp")
	err := h.bbsClient.RemoveDesiredLRP(logger, processGuid)
	if err != nil {
		logger.Error("failed-to-remove-desired-lrp", err)

		bbsError := models.ConvertError(err)
		if bbsError.Type == models.Error_ResourceNotFound {
			resp.WriteHeader(http.StatusNotFound)
			return
		}

		resp.WriteHeader(http.StatusServiceUnavailable)
		return
	}
	logger.Debug("removed-desired-lrp")

	resp.WriteHeader(http.StatusAccepted)
}
