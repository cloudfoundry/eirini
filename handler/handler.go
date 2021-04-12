package handler

import (
	"context"
	"net/http"

	"code.cloudfoundry.org/eirini/api"
	"code.cloudfoundry.org/eirini/models/cf"
	"code.cloudfoundry.org/lager"
	"github.com/julienschmidt/httprouter"
)

//counterfeiter:generate . LRPBifrost
//counterfeiter:generate . StagingBifrost
//counterfeiter:generate . TaskBifrost

type LRPBifrost interface {
	Transfer(ctx context.Context, request cf.DesireLRPRequest) error
	List(ctx context.Context) ([]cf.DesiredLRPSchedulingInfo, error)
	Update(ctx context.Context, update cf.UpdateDesiredLRPRequest) error
	Stop(ctx context.Context, identifier api.LRPIdentifier) error
	StopInstance(ctx context.Context, identifier api.LRPIdentifier, index uint) error
	GetApp(ctx context.Context, identifier api.LRPIdentifier) (cf.DesiredLRP, error)
	GetInstances(ctx context.Context, identifier api.LRPIdentifier) ([]*cf.Instance, error)
}

type TaskBifrost interface {
	GetTask(ctx context.Context, taskGUID string) (cf.TaskResponse, error)
	ListTasks(ctx context.Context) (cf.TasksResponse, error)
	TransferTask(ctx context.Context, taskGUID string, request cf.TaskRequest) error
	CancelTask(ctx context.Context, taskGUID string) error
}

type StagingBifrost interface {
	TransferStaging(ctx context.Context, stagingGUID string, request cf.StagingRequest) error
	CompleteStaging(ctx context.Context, request cf.StagingCompletedRequest) error
}

func New(lrpBifrost LRPBifrost,
	dockerStagingBifrost StagingBifrost,
	taskBifrost TaskBifrost,
	lager lager.Logger) http.Handler {
	handler := httprouter.New()

	appHandler := NewAppHandler(lrpBifrost, lager)
	stageHandler := NewStageHandler(dockerStagingBifrost, lager)
	taskHandler := NewTaskHandler(lager, taskBifrost)

	registerAppsEndpoints(handler, appHandler)
	registerStageEndpoint(handler, stageHandler)
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
	handler.GET("/apps/:process_guid/:version_guid", appHandler.Get)
}

func registerStageEndpoint(handler *httprouter.Router, stageHandler *Stage) {
	handler.POST("/stage/:staging_guid", stageHandler.Run)
}

func registerTaskEndpoints(handler *httprouter.Router, taskHandler *Task) {
	handler.GET("/tasks", taskHandler.List)
	handler.GET("/tasks/:task_guid", taskHandler.Get)
	handler.POST("/tasks/:task_guid", taskHandler.Run)
	handler.DELETE("/tasks/:task_guid", taskHandler.Cancel)
}
