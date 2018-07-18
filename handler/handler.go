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

	handler.PUT("/apps/:process_guid", appHandler.Desire)
	handler.GET("/apps", appHandler.List)
	handler.POST("/apps/:process_guid", appHandler.Update)
	handler.GET("/app/:process_guid", appHandler.Get)
	handler.PUT("/apps/:process_guid/stop", appHandler.Stop)

	return handler
}
