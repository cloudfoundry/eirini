package handler

import (
	"net/http"

	"code.cloudfoundry.org/eirini"
	"code.cloudfoundry.org/lager"
	"github.com/julienschmidt/httprouter"
)

func New(bifrost eirini.Bifrost, stager eirini.Stager, lager lager.Logger) http.Handler {
	handler := httprouter.New()

	appHandler := NewAppHandler(bifrost, lager)
	stageHandler := NewStageHandler(stager, lager)

	registerAppsEndpoints(handler, appHandler)
	registerStageEndpoints(handler, stageHandler)

	return handler
}

func registerAppsEndpoints(handler *httprouter.Router, appHandler *App) {
	handler.GET("/apps", appHandler.List)
	handler.PUT("/apps/:process_guid", appHandler.Desire)
	handler.POST("/apps/:process_guid", appHandler.Update)
	handler.PUT("/apps/:process_guid/stop", appHandler.Stop)
	handler.GET("/apps/:process_guid", appHandler.GetApp)
	handler.GET("/apps/:process_guid/instances", appHandler.GetInstances)
}

func registerStageEndpoints(handler *httprouter.Router, stageHandler *Stage) {
	handler.PUT("/stage/:staging_guid", stageHandler.Stage)
	handler.POST("/stage/:staging_guid/completed", stageHandler.StagingComplete)
}
