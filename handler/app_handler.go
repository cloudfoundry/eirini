package handler

import (
	"bytes"
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

func (a *App) Desire(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	loggerSession := a.logger.Session("desire-app", lager.Data{"guid": ps.ByName("process_guid")})
	var request cf.DesireLRPRequest
	buf := new(bytes.Buffer)
	if _, err := buf.ReadFrom(r.Body); err != nil {
		loggerSession.Error("request-body-cannot-be-read", err)
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	if err := json.Unmarshal(buf.Bytes(), &request); err != nil {
		loggerSession.Error("request-body-decoding-failed", err)
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	loggerSession.Debug("requested", lager.Data{"app_guid": request.AppGUID, "version": request.Version})

	request.LRP = buf.String()

	if err := a.bifrost.Transfer(r.Context(), request); err != nil {
		loggerSession.Error("bifrost-failed", err)
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	w.WriteHeader(http.StatusAccepted)
}

func (a *App) List(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	loggerSession := a.logger.Session("list-apps")
	loggerSession.Debug("requested")
	desiredLRPSchedulingInfos, err := a.bifrost.List(r.Context())
	if err != nil {
		loggerSession.Error("bifrost-failed", err)
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
		loggerSession.Error("encode-json-failed", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	if _, err = w.Write([]byte(result)); err != nil {
		loggerSession.Error("failed-to-write-response", err)
	}
}

func (a *App) GetApp(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	loggerSession := a.logger.Session("get-app", lager.Data{"guid": ps.ByName("process_guid"), "version": ps.ByName("version_guid")})
	loggerSession.Debug("requested")
	identifier := opi.LRPIdentifier{
		GUID:    ps.ByName("process_guid"),
		Version: ps.ByName("version_guid"),
	}
	desiredLRP, err := a.bifrost.GetApp(r.Context(), identifier)
	if err != nil {
		loggerSession.Error("failed-to-get-lrp", err, lager.Data{"guid": identifier.GUID})
		w.WriteHeader(http.StatusNotFound)
		return
	}
	response := models.DesiredLRPResponse{
		DesiredLrp: desiredLRP,
	}

	marshaler := &jsonpb.Marshaler{Indent: "", OrigName: true}
	result, err := marshaler.MarshalToString(&response)
	if err != nil {
		loggerSession.Error("encode-json-failed", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	if _, err = w.Write([]byte(result)); err != nil {
		loggerSession.Error("Could not write response", err)
	}
}

func (a *App) GetInstances(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	loggerSession := a.logger.Session("get-app-instances", lager.Data{"guid": ps.ByName("process_guid"), "version": ps.ByName("version_guid")})
	loggerSession.Debug("requested")
	identifier := opi.LRPIdentifier{
		GUID:    ps.ByName("process_guid"),
		Version: ps.ByName("version_guid"),
	}
	response := cf.GetInstancesResponse{ProcessGUID: identifier.ProcessGUID()}

	instances, err := a.bifrost.GetInstances(r.Context(), identifier)
	if err != nil {
		loggerSession.Error("bifrost-failed", err)
		response.Error = err.Error()
		response.Instances = []*cf.Instance{}
	} else {
		response.Instances = instances
	}

	err = json.NewEncoder(w).Encode(response)
	if err != nil {
		loggerSession.Error("encoding-response-failed", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
}

func (a *App) Update(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	loggerSession := a.logger.Session("update-app", lager.Data{"guid": ps.ByName("process_guid")})
	var request cf.UpdateDesiredLRPRequest
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		loggerSession.Error("json-decoding-failed", err)

		writeUpdateErrorResponse(w, err, http.StatusBadRequest, loggerSession)
		return
	}

	loggerSession.Debug("requested", lager.Data{"version": request.Version})

	if err := a.bifrost.Update(r.Context(), request); err != nil {
		loggerSession.Error("bifrost-failed", err)
		writeUpdateErrorResponse(w, err, http.StatusInternalServerError, loggerSession)
	}
}

func (a *App) Stop(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	loggerSession := a.logger.Session("stop-app", lager.Data{"guid": ps.ByName("process_guid"), "version": ps.ByName("version")})
	loggerSession.Debug("requested")
	identifier := opi.LRPIdentifier{
		GUID:    ps.ByName("process_guid"),
		Version: ps.ByName("version_guid"),
	}
	err := a.bifrost.Stop(r.Context(), identifier)
	if err != nil {
		loggerSession.Error("bifrost-failed", err)
		w.WriteHeader(http.StatusInternalServerError)
	}
}

func (a *App) StopInstance(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	loggerSession := a.logger.Session("stop-app-instance", lager.Data{"guid": ps.ByName("process_guid"), "version": ps.ByName("version_guid")})
	loggerSession.Debug("requested")
	identifier := opi.LRPIdentifier{
		GUID:    ps.ByName("process_guid"),
		Version: ps.ByName("version_guid"),
	}

	index, err := strconv.ParseUint(ps.ByName("instance"), 10, 32)
	if err != nil {
		loggerSession.Error("parsing-instance-index-failed", err)
		w.WriteHeader(http.StatusBadRequest)
	}

	err = a.bifrost.StopInstance(r.Context(), identifier, uint(index))
	if err != nil {
		loggerSession.Error("bifrost-failed", err)
		w.WriteHeader(http.StatusInternalServerError)
	}
}

func writeUpdateErrorResponse(w http.ResponseWriter, err error, statusCode int, loggerSession lager.Logger) {
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

	if _, err = w.Write(body); err != nil {
		loggerSession.Error("could-not-write-response", err)
	}
}
