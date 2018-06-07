package handler

import (
	"encoding/json"
	"net/http"

	"code.cloudfoundry.org/bbs/models"
	"code.cloudfoundry.org/eirini"
	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/runtimeschema/cc_messages"
	"github.com/julienschmidt/httprouter"
)

type DesiredLRPSchedulingInfoResponse struct {
	SchedulingInfos []models.DesiredLRPSchedulingInfo `json:"desired_lrp_scheduling_infos"`
}

func NewAppHandler(bifrost eirini.Bifrost, logger lager.Logger) *AppHandler {
	return &AppHandler{bifrost: bifrost, logger: logger}
}

type AppHandler struct {
	bifrost eirini.Bifrost
	logger  lager.Logger
}

func (a *AppHandler) Desire(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	var desiredApp cc_messages.DesireAppRequestFromCC
	if err := json.NewDecoder(r.Body).Decode(&desiredApp); err != nil {
		a.logger.Error("request-body-decoding-failed", err)
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	processGuid := ps.ByName("process_guid")
	if processGuid != desiredApp.ProcessGuid {
		a.logger.Error("process-guid-mismatch", nil, lager.Data{"desired-app-process-guid": desiredApp.ProcessGuid})
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	if err := a.bifrost.Transfer(r.Context(), []cc_messages.DesireAppRequestFromCC{desiredApp}); err != nil {
		a.logger.Error("desire-app-failed", err)
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	w.WriteHeader(http.StatusAccepted)
}

func (a *AppHandler) List(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	desiredLRPSchedulingInfos, err := a.bifrost.List(r.Context())
	if err != nil {
		a.logger.Error("list-apps-failed", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	response := DesiredLRPSchedulingInfoResponse{desiredLRPSchedulingInfos}

	w.Header().Set("Content-Type", "application/json")

	err = json.NewEncoder(w).Encode(response)
	if err != nil {
		a.logger.Error("encode-json-failed", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
}
