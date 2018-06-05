package handler

import (
	"encoding/json"
	"net/http"

	"code.cloudfoundry.org/eirini"
	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/runtimeschema/cc_messages"
	"github.com/julienschmidt/httprouter"
)

func NewAppHandler(converger eirini.Converger, logger lager.Logger) *AppHandler {
	return &AppHandler{converger: converger, logger: logger}
}

type AppHandler struct {
	converger eirini.Converger
	logger    lager.Logger
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
	if err := a.converger.ConvergeOnce(r.Context(), []cc_messages.DesireAppRequestFromCC{desiredApp}); err != nil {
		a.logger.Error("desire-app-failed", err)
		w.WriteHeader(http.StatusBadRequest)
		return
	}

}
