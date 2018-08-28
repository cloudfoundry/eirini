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

func NewAppHandler(bifrost eirini.Bifrost, logger lager.Logger) *App {
	return &App{bifrost: bifrost, logger: logger}
}

type App struct {
	bifrost eirini.Bifrost
	logger  lager.Logger
}

func (a *App) Desire(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	var request cf.DesireLRPRequest
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		a.logger.Error("request-body-decoding-failed", err)
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	processGUID := ps.ByName("process_guid")
	if processGUID != request.ProcessGUID {
		a.logger.Error("process-guid-mismatch", nil, lager.Data{"desired-app-process-guid": request.ProcessGUID})
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

func (a *App) List(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
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

func (a *App) GetApp(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	processGUID := ps.ByName("process_guid")
	desiredLRP := a.bifrost.GetApp(r.Context(), processGUID)
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

func (a *App) Update(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
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

func (a *App) Stop(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	processGUID := ps.ByName("process_guid")
	if len(processGUID) > 36 {
		processGUID = processGUID[:36]
	}

	err := a.bifrost.Stop(r.Context(), processGUID)
	if err != nil {
		a.logError("stop-app-failed", err)
		w.WriteHeader(http.StatusInternalServerError)
	}
}

func (a *App) GetInstances(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	guid := ps.ByName("process_guid")
	instances, err := a.bifrost.GetInstances(r.Context(), guid)
	response := a.createGetInstancesResponse(guid, instances, err)

	err = json.NewEncoder(w).Encode(response)
	if err != nil {
		a.logger.Error("encode-json-failed", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
}

func (a *App) createGetInstancesResponse(guid string, instances []*cf.Instance, err error) cf.GetInstancesResponse {
	if err != nil {
		return createErrorGetInstancesResponse(guid, err)
	}

	if len(instances) == 0 {
		return createErrorGetInstancesResponse(guid, errors.New("no-running-instances"))
	}

	return cf.GetInstancesResponse{
		ProcessGUID: guid,
		Instances:   instances,
	}
}

func createErrorGetInstancesResponse(guid string, err error) cf.GetInstancesResponse {
	return cf.GetInstancesResponse{
		ProcessGUID: guid,
		Error:       err.Error(),
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

func (a *App) logError(msg string, err error) {
	if err != nil {
		a.logger.Error(msg, err)
	}
}
