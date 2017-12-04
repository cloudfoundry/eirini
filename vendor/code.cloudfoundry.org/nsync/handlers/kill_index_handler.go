package handlers

import (
	"errors"
	"net/http"
	"strconv"

	"code.cloudfoundry.org/bbs"
	"code.cloudfoundry.org/bbs/models"
	"code.cloudfoundry.org/lager"
)

var (
	missingParameterErr = errors.New("missing from request")
	invalidNumberErr    = errors.New("not a number")
)

type KillIndexHandler struct {
	bbsClient bbs.Client
	logger    lager.Logger
}

func NewKillIndexHandler(logger lager.Logger, bbsClient bbs.Client) KillIndexHandler {
	return KillIndexHandler{
		bbsClient: bbsClient,
		logger:    logger,
	}
}

func (h *KillIndexHandler) KillIndex(resp http.ResponseWriter, req *http.Request) {
	processGuid := req.FormValue(":process_guid")
	indexString := req.FormValue(":index")

	logger := h.logger.Session("kill-index", lager.Data{
		"process_guid": processGuid,
		"index":        indexString,
		"method":       req.Method,
		"request":      req.URL.String(),
	})

	logger.Info("serving")
	defer logger.Info("complete")

	if processGuid == "" {
		logger.Error("missing-process-guid", missingParameterErr)
		resp.WriteHeader(http.StatusBadRequest)
		return
	}

	if indexString == "" {
		logger.Error("missing-index", missingParameterErr)
		resp.WriteHeader(http.StatusBadRequest)
		return
	}

	index, err := strconv.Atoi(indexString)
	if err != nil {
		logger.Error("invalid-index", invalidNumberErr)
		resp.WriteHeader(http.StatusBadRequest)
		return
	}

	err = h.killActualLRPByProcessGuidAndIndex(logger, processGuid, index)
	if err != nil {
		status := http.StatusServiceUnavailable
		bbsError := models.ConvertError(err)
		if bbsError.Type == models.Error_ResourceNotFound {
			status = http.StatusNotFound
		}
		resp.WriteHeader(status)
		return
	}

	resp.WriteHeader(http.StatusAccepted)
}

func (h *KillIndexHandler) killActualLRPByProcessGuidAndIndex(logger lager.Logger, processGuid string, index int) error {
	logger.Debug("fetching-actual-lrp-group")
	actualLRPGroup, err := h.bbsClient.ActualLRPGroupByProcessGuidAndIndex(logger, processGuid, index)
	if err != nil {
		logger.Error("failed-fetching-actual-lrp-group", err)
		return err
	}
	logger.Debug("fetched-actual-lrp-group")

	actualLRP, _ := actualLRPGroup.Resolve()

	logger.Debug("retiring-actual-lrp")
	err = h.bbsClient.RetireActualLRP(logger, &actualLRP.ActualLRPKey)
	if err != nil {
		logger.Error("failed-to-retire-actual-lrp", err)
		return err
	}
	logger.Debug("retired-actual-lrp")

	return nil
}
