package handler

import (
	"encoding/json"
	"net/http"
	"strconv"

	"code.cloudfoundry.org/bbs/models"
	"code.cloudfoundry.org/eirini"
	"code.cloudfoundry.org/eirini/models/cf"
	"code.cloudfoundry.org/eirini/opi"
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

func (a *App) Desire(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	var request cf.DesireLRPRequest
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		a.logError("request-body-decoding-failed", err)
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	if err := a.bifrost.Transfer(r.Context(), request); err != nil {
		a.logError("desire-app-failed", err)
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	w.WriteHeader(http.StatusAccepted)
}

func (a *App) List(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	desiredLRPSchedulingInfos, err := a.bifrost.List(r.Context())
	if err != nil {
		a.logError("list-apps-failed", err)
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
		a.logError("encode-json-failed", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	_, err = w.Write([]byte(result))
	a.logError("Could not write response", err)
}

func (a *App) GetApp(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	identifier := opi.LRPIdentifier{
		GUID:    ps.ByName("process_guid"),
		Version: ps.ByName("version_guid"),
	}
	desiredLRP := a.bifrost.GetApp(r.Context(), identifier)
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
		a.logError("encode-json-failed", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	_, err = w.Write([]byte(result))

	a.logError("Could not write response", err)
}

func (a *App) Update(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	var request cf.UpdateDesiredLRPRequest
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		a.logError("json-decoding-failure", err)
		err = writeUpdateErrorResponse(w, err, http.StatusBadRequest)

		a.logError("Could not write response", err)

		return
	}

	if err := a.bifrost.Update(r.Context(), request); err != nil {
		a.logError("update-app-failed", err)
		err = writeUpdateErrorResponse(w, err, http.StatusInternalServerError)
		a.logError("Could not write response", err)
	}
}

func (a *App) Stop(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	identifier := opi.LRPIdentifier{
		GUID:    ps.ByName("process_guid"),
		Version: ps.ByName("version_guid"),
	}
	err := a.bifrost.Stop(r.Context(), identifier)
	if err != nil {
		a.logError("stop-app-failed", err)
		w.WriteHeader(http.StatusInternalServerError)
	}
}

func (a *App) StopInstance(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	identifier := opi.LRPIdentifier{
		GUID:    ps.ByName("process_guid"),
		Version: ps.ByName("version_guid"),
	}

	index, err := strconv.ParseUint(ps.ByName("instance"), 10, 32)
	if err != nil {
		a.logError("stop-app-instance-failed", err)
		w.WriteHeader(http.StatusBadRequest)
	}

	err = a.bifrost.StopInstance(r.Context(), identifier, uint(index))
	if err != nil {
		a.logError("stop-app-instance-failed", err)
		w.WriteHeader(http.StatusInternalServerError)
	}
}

func (a *App) GetInstances(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	identifier := opi.LRPIdentifier{
		GUID:    ps.ByName("process_guid"),
		Version: ps.ByName("version_guid"),
	}
	instances, err := a.bifrost.GetInstances(r.Context(), identifier)
	response := a.createGetInstancesResponse(identifier.ProcessGUID(), instances, err)

	err = json.NewEncoder(w).Encode(response)
	if err != nil {
		a.logError("get-instances-failed", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
}

func (a *App) createGetInstancesResponse(guid string, instances []*cf.Instance, err error) cf.GetInstancesResponse {
	if err != nil {
		return createErrorGetInstancesResponse(guid, err)
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
		Instances:   []*cf.Instance{},
	}
}

func writeUpdateErrorResponse(w http.ResponseWriter, err error, statusCode int) error {
	w.WriteHeader(statusCode)

	response := models.DesiredLRPLifecycleResponse{
		Error: &models.Error{
			Message: err.Error(),
		},
	}

	body, marshalError := json.Marshal(response)
	if marshalError != nil {
		panic(marshalError)
	}

	_, err = w.Write(body)
	return err
}

func (a *App) logError(msg string, err error) {
	if err != nil {
		a.logger.Error(msg, err)
	}
}
