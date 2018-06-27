package handler

import (
	"net/http"

	"code.cloudfoundry.org/eirini"
	"code.cloudfoundry.org/lager"
	"github.com/julienschmidt/httprouter"
)

func New(converger eirini.Bifrost, lager lager.Logger) http.Handler {
	handler := httprouter.New()

	appHandler := NewAppHandler(converger, lager)

	handler.GET("/apps", appHandler.List)
	handler.PUT("/apps/:process_guid", appHandler.Desire)
	handler.POST("/apps/:process_guid", appHandler.Update)
	handler.PUT("/apps/:process_guid/stop", appHandler.Stop)
	handler.GET("/apps/:process_guid", appHandler.GetApp)
	handler.GET("/apps/:process_guid/instances", appHandler.GetInstances)

	return handler
}
