package handlers

import (
	"io/ioutil"
	"net/http"
	"strconv"

	"code.cloudfoundry.org/auctioneer"
	"code.cloudfoundry.org/bbs"
	"code.cloudfoundry.org/bbs/controllers"
	"code.cloudfoundry.org/bbs/db"
	"code.cloudfoundry.org/bbs/events"
	"code.cloudfoundry.org/bbs/handlers/middleware"
	"code.cloudfoundry.org/bbs/models"
	"code.cloudfoundry.org/bbs/serviceclient"
	"code.cloudfoundry.org/bbs/taskworkpool"
	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/rep"
	"github.com/gogo/protobuf/proto"
	"github.com/tedsuo/rata"
)

func New(
	logger,
	accessLogger lager.Logger,
	updateWorkers int,
	convergenceWorkersSize int,
	emitter middleware.Emitter,
	db db.DB,
	desiredHub, actualHub, taskHub events.Hub,
	taskCompletionClient taskworkpool.TaskCompletionClient,
	serviceClient serviceclient.ServiceClient,
	auctioneerClient auctioneer.Client,
	repClientFactory rep.ClientFactory,
	migrationsDone <-chan struct{},
	exitChan chan struct{},
) http.Handler {
	pingHandler := NewPingHandler()
	domainHandler := NewDomainHandler(db, exitChan)
	actualLRPHandler := NewActualLRPHandler(db, exitChan)
	actualLRPController := controllers.NewActualLRPLifecycleController(db, db, db, auctioneerClient, serviceClient, repClientFactory, actualHub)
	actualLRPLifecycleHandler := NewActualLRPLifecycleHandler(actualLRPController, exitChan)
	evacuationHandler := NewEvacuationHandler(db, db, db, actualHub, auctioneerClient, exitChan)
	desiredLRPHandler := NewDesiredLRPHandler(updateWorkers, db, db, desiredHub, actualHub, auctioneerClient, repClientFactory, serviceClient, exitChan)
	taskController := controllers.NewTaskController(db, taskCompletionClient, auctioneerClient, serviceClient, repClientFactory, taskHub)
	taskHandler := NewTaskHandler(taskController, exitChan)
	eventsHandler := NewEventHandler(desiredHub, actualHub)
	taskEventsHandler := NewTaskEventHandler(taskHub)
	cellsHandler := NewCellHandler(serviceClient, exitChan)

	actions := rata.Handlers{
		// Ping
		bbs.PingRoute: middleware.RecordLatency(middleware.LogWrap(logger, accessLogger, pingHandler.Ping), emitter),

		// Domains
		bbs.DomainsRoute:      route(middleware.RecordLatency(middleware.LogWrap(logger, accessLogger, domainHandler.Domains), emitter)),
		bbs.UpsertDomainRoute: route(middleware.RecordLatency(middleware.LogWrap(logger, accessLogger, domainHandler.Upsert), emitter)),

		// Actual LRPs
		bbs.ActualLRPGroupsRoute:                     route(middleware.RecordLatency(middleware.LogWrap(logger, accessLogger, actualLRPHandler.ActualLRPGroups), emitter)),
		bbs.ActualLRPGroupsByProcessGuidRoute:        route(middleware.RecordLatency(middleware.LogWrap(logger, accessLogger, actualLRPHandler.ActualLRPGroupsByProcessGuid), emitter)),
		bbs.ActualLRPGroupByProcessGuidAndIndexRoute: route(middleware.RecordLatency(middleware.LogWrap(logger, accessLogger, actualLRPHandler.ActualLRPGroupByProcessGuidAndIndex), emitter)),

		// Actual LRP Lifecycle
		bbs.ClaimActualLRPRoute:  route(middleware.RecordLatency(middleware.LogWrap(logger, accessLogger, actualLRPLifecycleHandler.ClaimActualLRP), emitter)),
		bbs.StartActualLRPRoute:  route(middleware.RecordLatency(middleware.LogWrap(logger, accessLogger, actualLRPLifecycleHandler.StartActualLRP), emitter)),
		bbs.CrashActualLRPRoute:  route(middleware.RecordLatency(middleware.LogWrap(logger, accessLogger, actualLRPLifecycleHandler.CrashActualLRP), emitter)),
		bbs.RetireActualLRPRoute: route(middleware.RecordLatency(middleware.LogWrap(logger, accessLogger, actualLRPLifecycleHandler.RetireActualLRP), emitter)),
		bbs.FailActualLRPRoute:   route(middleware.RecordLatency(middleware.LogWrap(logger, accessLogger, actualLRPLifecycleHandler.FailActualLRP), emitter)),
		bbs.RemoveActualLRPRoute: route(middleware.RecordLatency(middleware.LogWrap(logger, accessLogger, actualLRPLifecycleHandler.RemoveActualLRP), emitter)),

		// Evacuation
		bbs.RemoveEvacuatingActualLRPRoute: route(middleware.RecordLatency(middleware.LogWrap(logger, accessLogger, evacuationHandler.RemoveEvacuatingActualLRP), emitter)),
		bbs.EvacuateClaimedActualLRPRoute:  route(middleware.RecordLatency(middleware.LogWrap(logger, accessLogger, evacuationHandler.EvacuateClaimedActualLRP), emitter)),
		bbs.EvacuateCrashedActualLRPRoute:  route(middleware.RecordLatency(middleware.LogWrap(logger, accessLogger, evacuationHandler.EvacuateCrashedActualLRP), emitter)),
		bbs.EvacuateStoppedActualLRPRoute:  route(middleware.RecordLatency(middleware.LogWrap(logger, accessLogger, evacuationHandler.EvacuateStoppedActualLRP), emitter)),
		bbs.EvacuateRunningActualLRPRoute:  route(middleware.RecordLatency(middleware.LogWrap(logger, accessLogger, evacuationHandler.EvacuateRunningActualLRP), emitter)),

		// Desired LRPs
		bbs.DesiredLRPsRoute:               route(middleware.RecordLatency(middleware.LogWrap(logger, accessLogger, desiredLRPHandler.DesiredLRPs), emitter)),
		bbs.DesiredLRPByProcessGuidRoute:   route(middleware.RecordLatency(middleware.LogWrap(logger, accessLogger, desiredLRPHandler.DesiredLRPByProcessGuid), emitter)),
		bbs.DesiredLRPSchedulingInfosRoute: route(middleware.RecordLatency(middleware.LogWrap(logger, accessLogger, desiredLRPHandler.DesiredLRPSchedulingInfos), emitter)),
		bbs.DesireDesiredLRPRoute:          route(middleware.RecordLatency(middleware.LogWrap(logger, accessLogger, desiredLRPHandler.DesireDesiredLRP), emitter)),
		bbs.UpdateDesiredLRPRoute:          route(middleware.RecordLatency(middleware.LogWrap(logger, accessLogger, desiredLRPHandler.UpdateDesiredLRP), emitter)),
		bbs.RemoveDesiredLRPRoute:          route(middleware.RecordLatency(middleware.LogWrap(logger, accessLogger, desiredLRPHandler.RemoveDesiredLRP), emitter)),

		bbs.DesiredLRPsRoute_r0:             route(middleware.RecordLatency(middleware.LogWrap(logger, accessLogger, desiredLRPHandler.DesiredLRPs_r0), emitter)),
		bbs.DesiredLRPsRoute_r1:             route(middleware.RecordLatency(middleware.LogWrap(logger, accessLogger, desiredLRPHandler.DesiredLRPs_r1), emitter)),
		bbs.DesiredLRPByProcessGuidRoute_r0: route(middleware.RecordLatency(middleware.LogWrap(logger, accessLogger, desiredLRPHandler.DesiredLRPByProcessGuid_r0), emitter)),
		bbs.DesiredLRPByProcessGuidRoute_r1: route(middleware.RecordLatency(middleware.LogWrap(logger, accessLogger, desiredLRPHandler.DesiredLRPByProcessGuid_r1), emitter)),
		bbs.DesireDesiredLRPRoute_r0:        route(middleware.RecordLatency(middleware.LogWrap(logger, accessLogger, desiredLRPHandler.DesireDesiredLRP_r0), emitter)),
		bbs.DesireDesiredLRPRoute_r1:        route(middleware.RecordLatency(middleware.LogWrap(logger, accessLogger, desiredLRPHandler.DesireDesiredLRP_r1), emitter)),

		// Tasks
		bbs.TasksRoute:         route(middleware.RecordLatency(middleware.LogWrap(logger, accessLogger, taskHandler.Tasks), emitter)),
		bbs.TaskByGuidRoute:    route(middleware.RecordLatency(middleware.LogWrap(logger, accessLogger, taskHandler.TaskByGuid), emitter)),
		bbs.DesireTaskRoute:    route(middleware.RecordLatency(middleware.LogWrap(logger, accessLogger, taskHandler.DesireTask), emitter)),
		bbs.StartTaskRoute:     route(middleware.RecordLatency(middleware.LogWrap(logger, accessLogger, taskHandler.StartTask), emitter)),
		bbs.CancelTaskRoute:    route(middleware.RecordLatency(middleware.LogWrap(logger, accessLogger, taskHandler.CancelTask), emitter)),
		bbs.FailTaskRoute:      route(middleware.RecordLatency(middleware.LogWrap(logger, accessLogger, taskHandler.FailTask), emitter)),
		bbs.CompleteTaskRoute:  route(middleware.RecordLatency(middleware.LogWrap(logger, accessLogger, taskHandler.CompleteTask), emitter)),
		bbs.ResolvingTaskRoute: route(middleware.RecordLatency(middleware.LogWrap(logger, accessLogger, taskHandler.ResolvingTask), emitter)),
		bbs.DeleteTaskRoute:    route(middleware.RecordLatency(middleware.LogWrap(logger, accessLogger, taskHandler.DeleteTask), emitter)),

		bbs.TasksRoute_r1:      route(middleware.RecordLatency(middleware.LogWrap(logger, accessLogger, taskHandler.Tasks_r1), emitter)),
		bbs.TasksRoute_r0:      route(middleware.RecordLatency(middleware.LogWrap(logger, accessLogger, taskHandler.Tasks_r0), emitter)),
		bbs.TaskByGuidRoute_r1: route(middleware.RecordLatency(middleware.LogWrap(logger, accessLogger, taskHandler.TaskByGuid_r1), emitter)),
		bbs.TaskByGuidRoute_r0: route(middleware.RecordLatency(middleware.LogWrap(logger, accessLogger, taskHandler.TaskByGuid_r0), emitter)),
		bbs.DesireTaskRoute_r1: route(middleware.RecordLatency(middleware.LogWrap(logger, accessLogger, taskHandler.DesireTask_r1), emitter)),
		bbs.DesireTaskRoute_r0: route(middleware.RecordLatency(middleware.LogWrap(logger, accessLogger, taskHandler.DesireTask_r0), emitter)),

		// Events
		bbs.EventStreamRoute_r0:     route(middleware.LogWrap(logger, accessLogger, eventsHandler.Subscribe_r0)),
		bbs.TaskEventStreamRoute_r0: route(middleware.LogWrap(logger, accessLogger, taskEventsHandler.Subscribe_r0)),

		// Cells
		bbs.CellsRoute:    route(middleware.RecordLatency(middleware.LogWrap(logger, accessLogger, cellsHandler.Cells), emitter)),
		bbs.CellsRoute_r1: route(middleware.RecordLatency(middleware.LogWrap(logger, accessLogger, cellsHandler.Cells), emitter)),
	}

	handler, err := rata.NewRouter(bbs.Routes, actions)
	if err != nil {
		panic("unable to create router: " + err.Error())
	}

	return middleware.RecordRequestCount(
		UnavailableWrap(handler,
			migrationsDone,
		),
		emitter,
	)
}

func route(f http.HandlerFunc) http.Handler {
	return f
}

func parseRequest(logger lager.Logger, req *http.Request, request MessageValidator) error {
	data, err := ioutil.ReadAll(req.Body)
	if err != nil {
		logger.Error("failed-to-read-body", err)
		return models.ErrUnknownError
	}

	err = request.Unmarshal(data)
	if err != nil {
		logger.Error("failed-to-parse-request-body", err)
		return models.ErrBadRequest
	}

	if err := request.Validate(); err != nil {
		logger.Error("invalid-request", err)
		return models.NewError(models.Error_InvalidRequest, err.Error())
	}

	return nil
}

func exitIfUnrecoverable(logger lager.Logger, exitCh chan<- struct{}, err *models.Error) {
	if err != nil && err.Type == models.Error_Unrecoverable {
		logger.Error("unrecoverable-error", err)
		select {
		case exitCh <- struct{}{}:
		default:
		}
	}
}

func writeResponse(w http.ResponseWriter, message proto.Message) {
	responseBytes, err := proto.Marshal(message)
	if err != nil {
		panic("Unable to encode Proto: " + err.Error())
	}

	w.Header().Set("Content-Length", strconv.Itoa(len(responseBytes)))
	w.Header().Set("Content-Type", "application/x-protobuf")
	w.WriteHeader(http.StatusOK)

	w.Write(responseBytes)
}
