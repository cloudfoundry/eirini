package handler

import (
	"context"
	"net/http"

	"code.cloudfoundry.org/eirini/models/cf"
	"code.cloudfoundry.org/eirini/opi"
	"code.cloudfoundry.org/lager"
	"github.com/julienschmidt/httprouter"
)

//counterfeiter:generate . LRPBifrost
type LRPBifrost interface {
	Transfer(ctx context.Context, request cf.DesireLRPRequest) error
	List(ctx context.Context) ([]cf.DesiredLRPSchedulingInfo, error)
	Update(ctx context.Context, update cf.UpdateDesiredLRPRequest) error
	Stop(ctx context.Context, identifier opi.LRPIdentifier) error
	StopInstance(ctx context.Context, identifier opi.LRPIdentifier, index uint) error
	GetApp(ctx context.Context, identifier opi.LRPIdentifier) (cf.DesiredLRP, error)
	GetInstances(ctx context.Context, identifier opi.LRPIdentifier) ([]*cf.Instance, error)
}

//counterfeiter:generate . TaskBifrost
type TaskBifrost interface {
	TransferTask(ctx context.Context, taskGUID string, request cf.TaskRequest) error
	CompleteTask(taskGUID string) error
}

//counterfeiter:generate . StagingBifrost
type StagingBifrost interface {
	TransferStaging(ctx context.Context, stagingGUID string, request cf.StagingRequest) error
	CompleteStaging(cf.TaskCompletedRequest) error
}

func New(lrpBifrost LRPBifrost,
	buildpackStagingBifrost StagingBifrost,
	dockerStagingBifrost StagingBifrost,
	taskBifrost TaskBifrost,
	lager lager.Logger) http.Handler {
	handler := httprouter.New()

	appHandler := NewAppHandler(lrpBifrost, lager)
	stageHandler := NewStageHandler(buildpackStagingBifrost, dockerStagingBifrost, lager)
	taskHandler := NewTaskHandler(lager, taskBifrost)

	registerAppsEndpoints(handler, appHandler)
	registerStageEndpoints(handler, stageHandler)
	registerTaskEndpoints(handler, taskHandler)

	return handler
}

func registerAppsEndpoints(handler *httprouter.Router, appHandler *App) {
	handler.GET("/apps", appHandler.List)
	handler.PUT("/apps/:process_guid", appHandler.Desire)
	handler.POST("/apps/:process_guid", appHandler.Update)
	handler.PUT("/apps/:process_guid/:version_guid/stop", appHandler.Stop)
	handler.PUT("/apps/:process_guid/:version_guid/stop/:instance", appHandler.StopInstance)
	handler.GET("/apps/:process_guid/:version_guid/instances", appHandler.GetInstances)
	handler.GET("/apps/:process_guid/:version_guid", appHandler.GetApp)
}

func registerStageEndpoints(handler *httprouter.Router, stageHandler *Stage) {
	handler.POST("/stage/:staging_guid", stageHandler.Stage)
	handler.PUT("/stage/:staging_guid/completed", stageHandler.CompleteStaging)
}

func registerTaskEndpoints(handler *httprouter.Router, taskHandler *Task) {
	handler.POST("/tasks/:task_guid", taskHandler.Run)
	handler.PUT("/tasks/:task_guid/completed", taskHandler.CompleteTask)
}
