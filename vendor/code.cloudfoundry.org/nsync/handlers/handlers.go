package handlers

import (
	"net/http"

	"code.cloudfoundry.org/bbs"
	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/nsync"
	"code.cloudfoundry.org/nsync/recipebuilder"
	"github.com/tedsuo/rata"
)

func New(logger lager.Logger, bbsClient bbs.Client, recipebuilders map[string]recipebuilder.RecipeBuilder) http.Handler {
	desireAppHandler := NewDesireAppHandler(logger, bbsClient, recipebuilders)
	stopAppHandler := NewStopAppHandler(logger, bbsClient)
	killIndexHandler := NewKillIndexHandler(logger, bbsClient)
	taskHandler := NewTaskHandler(logger, bbsClient, recipebuilders)
	cancelTaskHandler := NewCancelTaskHandler(logger, bbsClient)

	actions := rata.Handlers{
		nsync.DesireAppRoute:  http.HandlerFunc(desireAppHandler.DesireApp),
		nsync.StopAppRoute:    http.HandlerFunc(stopAppHandler.StopApp),
		nsync.KillIndexRoute:  http.HandlerFunc(killIndexHandler.KillIndex),
		nsync.TasksRoute:      http.HandlerFunc(taskHandler.DesireTask),
		nsync.CancelTaskRoute: http.HandlerFunc(cancelTaskHandler.CancelTask),
	}

	handler, err := rata.NewRouter(nsync.Routes, actions)
	if err != nil {
		panic("unable to create router: " + err.Error())
	}

	return handler
}
