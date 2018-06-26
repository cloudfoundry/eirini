package handler

import (
	"encoding/json"
	"errors"
	"net/http"

	"code.cloudfoundry.org/bbs/models"
	"code.cloudfoundry.org/eirini"
	"code.cloudfoundry.org/eirini/models/cf"
	"code.cloudfoundry.org/lager"
	"github.com/gogo/protobuf/jsonpb"
	"github.com/julienschmidt/httprouter"
)

func NewAppHandler(bifrost eirini.Bifrost, logger lager.Logger) *AppHandler {
	return &AppHandler{bifrost: bifrost, logger: logger}
}

type AppHandler struct {
	bifrost eirini.Bifrost
	logger  lager.Logger
}

func (a *AppHandler) Desire(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	var request cf.DesireLRPRequest
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		a.logger.Error("request-body-decoding-failed", err)
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	processGuid := ps.ByName("process_guid")
	if processGuid != request.ProcessGuid {
		a.logger.Error("process-guid-mismatch", nil, lager.Data{"desired-app-process-guid": request.ProcessGuid})
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	if err := a.bifrost.Transfer(r.Context(), request); err != nil {
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

	response := models.DesiredLRPSchedulingInfosResponse{
		DesiredLrpSchedulingInfos: desiredLRPSchedulingInfos,
	}

	w.Header().Set("Content-Type", "application/json")

	marshaler := &jsonpb.Marshaler{Indent: "", OrigName: true}
	result, err := marshaler.MarshalToString(&response)
	if err != nil {
		a.logger.Error("encode-json-failed", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	_, err = w.Write([]byte(result))
	a.logError("Could not write response", err)
}

func (a *AppHandler) Get(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	processGuid := ps.ByName("process_guid")
	desiredLRP := a.bifrost.Get(r.Context(), processGuid)
	if desiredLRP == nil {
		w.WriteHeader(http.StatusNotFound)
		return
	}
	response := models.DesiredLRPResponse{
		DesiredLrp: desiredLRP,
	}

	marshaler := &jsonpb.Marshaler{Indent: "", OrigName: true}
	result, err := marshaler.MarshalToString(&response)
	if err != nil {
		a.logger.Error("encode-json-failed", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	_, err = w.Write([]byte(result))

	a.logError("Could not write response", err)
}

func (a *AppHandler) Update(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	var request models.UpdateDesiredLRPRequest
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		a.logger.Error("json-decoding-failure", err)
		err = writeUpdateErrorResponse(w, err, http.StatusBadRequest)

		a.logError("Could not write response", err)

		return
	}

	guid := ps.ByName("process_guid")
	if guid != request.ProcessGuid {
		a.logger.Error("process-guid-mismatch", nil, lager.Data{"update-app-process-guid": request.ProcessGuid})
		err := writeUpdateErrorResponse(w, errors.New("Process guid missmatch"), http.StatusBadRequest)
		a.logError("Could not write response", err)

		return
	}

	if err := a.bifrost.Update(r.Context(), request); err != nil {
		a.logger.Error("update-app-failed", err)
		err = writeUpdateErrorResponse(w, err, http.StatusInternalServerError)
		a.logError("Could not write response", err)
	}
}

func writeUpdateErrorResponse(w http.ResponseWriter, err error, statusCode int) error {
	w.WriteHeader(statusCode)

	response := models.DesiredLRPLifecycleResponse{
		Error: &models.Error{
			Message: err.Error(),
		},
	}

	body, _ := json.Marshal(response)

	_, err = w.Write(body)
	return err
}

func (a *AppHandler) logError(msg string, err error) {
	if err != nil {
		a.logger.Error(msg, err)
	}
}
