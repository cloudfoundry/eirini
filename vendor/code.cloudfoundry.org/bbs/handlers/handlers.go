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
	"code.cloudfoundry.org/bbs/metrics"
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
	maxTaskPlacementRetries int,
	emitter middleware.Emitter,
	db db.DB,
	desiredHub, actualHub, actualLRPInstanceHub, taskHub events.Hub,
	taskCompletionClient taskworkpool.TaskCompletionClient,
	serviceClient serviceclient.ServiceClient,
	auctioneerClient auctioneer.Client,
	repClientFactory rep.ClientFactory,
	taskStatMetronNotifier metrics.TaskStatMetronNotifier,
	migrationsDone <-chan struct{},
	exitChan chan struct{},
) http.Handler {
	pingHandler := NewPingHandler()
	domainHandler := NewDomainHandler(db, exitChan)
	actualLRPHandler := NewActualLRPHandler(db, exitChan)
	actualLRPController := controllers.NewActualLRPLifecycleController(
		db, db, db, db,
		auctioneerClient,
		serviceClient,
		repClientFactory,
		actualHub,
		actualLRPInstanceHub,
	)
	evacuationController := controllers.NewEvacuationController(
		db, db, db, db,
		auctioneerClient,
		actualHub,
		actualLRPInstanceHub,
	)
	actualLRPLifecycleHandler := NewActualLRPLifecycleHandler(actualLRPController, exitChan)
	evacuationHandler := NewEvacuationHandler(evacuationController, exitChan)
	desiredLRPHandler := NewDesiredLRPHandler(updateWorkers, db, db, desiredHub, actualHub, actualLRPInstanceHub, auctioneerClient, repClientFactory, serviceClient, exitChan)
	taskController := controllers.NewTaskController(db, taskCompletionClient, auctioneerClient, serviceClient, repClientFactory, taskHub, taskStatMetronNotifier, maxTaskPlacementRetries)
	taskHandler := NewTaskHandler(taskController, exitChan)
	lrpGroupEventsHandler := NewLRPGroupEventsHandler(desiredHub, actualHub)
	taskEventsHandler := NewTaskEventHandler(taskHub)
	lrpInstanceEventsHandler := NewLRPInstanceEventHandler(desiredHub, actualLRPInstanceHub)
	cellsHandler := NewCellHandler(serviceClient, exitChan)

	actions := rata.Handlers{
		// Ping
		bbs.PingRoute_r0: middleware.RecordLatency(middleware.LogWrap(logger, accessLogger, pingHandler.Ping), emitter),

		// Domains
		bbs.DomainsRoute_r0:      route(middleware.RecordLatency(middleware.LogWrap(logger, accessLogger, domainHandler.Domains), emitter)),
		bbs.UpsertDomainRoute_r0: route(middleware.RecordLatency(middleware.LogWrap(logger, accessLogger, domainHandler.Upsert), emitter)),

		// Actual LRPs
		bbs.ActualLRPsRoute_r0:                          route(middleware.RecordLatency(middleware.LogWrap(logger, accessLogger, actualLRPHandler.ActualLRPs), emitter)),
		bbs.ActualLRPGroupsRoute_r0:                     route(middleware.RecordLatency(middleware.LogWrap(logger, accessLogger, actualLRPHandler.ActualLRPGroups), emitter)),
		bbs.ActualLRPGroupsByProcessGuidRoute_r0:        route(middleware.RecordLatency(middleware.LogWrap(logger, accessLogger, actualLRPHandler.ActualLRPGroupsByProcessGuid), emitter)),
		bbs.ActualLRPGroupByProcessGuidAndIndexRoute_r0: route(middleware.RecordLatency(middleware.LogWrap(logger, accessLogger, actualLRPHandler.ActualLRPGroupByProcessGuidAndIndex), emitter)),

		// Actual LRP Lifecycle
		bbs.ClaimActualLRPRoute_r0:  route(middleware.RecordLatency(middleware.LogWrap(logger, accessLogger, actualLRPLifecycleHandler.ClaimActualLRP), emitter)),
		bbs.StartActualLRPRoute_r0:  route(middleware.RecordLatency(middleware.LogWrap(logger, accessLogger, actualLRPLifecycleHandler.StartActualLRP), emitter)),
		bbs.CrashActualLRPRoute_r0:  route(middleware.RecordLatency(middleware.LogWrap(logger, accessLogger, actualLRPLifecycleHandler.CrashActualLRP), emitter)),
		bbs.RetireActualLRPRoute_r0: route(middleware.RecordLatency(middleware.LogWrap(logger, accessLogger, actualLRPLifecycleHandler.RetireActualLRP), emitter)),
		bbs.FailActualLRPRoute_r0:   route(middleware.RecordLatency(middleware.LogWrap(logger, accessLogger, actualLRPLifecycleHandler.FailActualLRP), emitter)),
		bbs.RemoveActualLRPRoute_r0: route(middleware.RecordLatency(middleware.LogWrap(logger, accessLogger, actualLRPLifecycleHandler.RemoveActualLRP), emitter)),

		// Evacuation
		bbs.RemoveEvacuatingActualLRPRoute_r0: route(middleware.RecordLatency(middleware.LogWrap(logger, accessLogger, evacuationHandler.RemoveEvacuatingActualLRP), emitter)),
		bbs.EvacuateClaimedActualLRPRoute_r0:  route(middleware.RecordLatency(middleware.LogWrap(logger, accessLogger, evacuationHandler.EvacuateClaimedActualLRP), emitter)),
		bbs.EvacuateCrashedActualLRPRoute_r0:  route(middleware.RecordLatency(middleware.LogWrap(logger, accessLogger, evacuationHandler.EvacuateCrashedActualLRP), emitter)),
		bbs.EvacuateStoppedActualLRPRoute_r0:  route(middleware.RecordLatency(middleware.LogWrap(logger, accessLogger, evacuationHandler.EvacuateStoppedActualLRP), emitter)),
		bbs.EvacuateRunningActualLRPRoute_r0:  route(middleware.RecordLatency(middleware.LogWrap(logger, accessLogger, evacuationHandler.EvacuateRunningActualLRP), emitter)),

		// Desired LRPs
		bbs.DesiredLRPsRoute_r3:               route(middleware.RecordLatency(middleware.LogWrap(logger, accessLogger, desiredLRPHandler.DesiredLRPs), emitter)),
		bbs.DesiredLRPByProcessGuidRoute_r3:   route(middleware.RecordLatency(middleware.LogWrap(logger, accessLogger, desiredLRPHandler.DesiredLRPByProcessGuid), emitter)),
		bbs.DesiredLRPsRoute_r2:               route(middleware.RecordLatency(middleware.LogWrap(logger, accessLogger, desiredLRPHandler.DesiredLRPs_r2), emitter)),             // DEPRECATED
		bbs.DesiredLRPByProcessGuidRoute_r2:   route(middleware.RecordLatency(middleware.LogWrap(logger, accessLogger, desiredLRPHandler.DesiredLRPByProcessGuid_r2), emitter)), // DEPRECATED
		bbs.DesiredLRPSchedulingInfosRoute_r0: route(middleware.RecordLatency(middleware.LogWrap(logger, accessLogger, desiredLRPHandler.DesiredLRPSchedulingInfos), emitter)),
		bbs.DesireDesiredLRPRoute_r2:          route(middleware.RecordLatency(middleware.LogWrap(logger, accessLogger, desiredLRPHandler.DesireDesiredLRP), emitter)),
		bbs.UpdateDesiredLRPRoute_r0:          route(middleware.RecordLatency(middleware.LogWrap(logger, accessLogger, desiredLRPHandler.UpdateDesiredLRP), emitter)),
		bbs.RemoveDesiredLRPRoute_r0:          route(middleware.RecordLatency(middleware.LogWrap(logger, accessLogger, desiredLRPHandler.RemoveDesiredLRP), emitter)),

		// Tasks
		bbs.TasksRoute_r2:         route(middleware.RecordLatency(middleware.LogWrap(logger, accessLogger, taskHandler.Tasks_r2), emitter)),      // DEPRECATED
		bbs.TaskByGuidRoute_r2:    route(middleware.RecordLatency(middleware.LogWrap(logger, accessLogger, taskHandler.TaskByGuid_r2), emitter)), // DEPRECATED
		bbs.TasksRoute_r3:         route(middleware.RecordLatency(middleware.LogWrap(logger, accessLogger, taskHandler.Tasks), emitter)),
		bbs.TaskByGuidRoute_r3:    route(middleware.RecordLatency(middleware.LogWrap(logger, accessLogger, taskHandler.TaskByGuid), emitter)),
		bbs.DesireTaskRoute_r2:    route(middleware.RecordLatency(middleware.LogWrap(logger, accessLogger, taskHandler.DesireTask), emitter)),
		bbs.StartTaskRoute_r0:     route(middleware.RecordLatency(middleware.LogWrap(logger, accessLogger, taskHandler.StartTask), emitter)),
		bbs.CancelTaskRoute_r0:    route(middleware.RecordLatency(middleware.LogWrap(logger, accessLogger, taskHandler.CancelTask), emitter)),
		bbs.FailTaskRoute_r0:      route(middleware.RecordLatency(middleware.LogWrap(logger, accessLogger, taskHandler.FailTask), emitter)),
		bbs.RejectTaskRoute_r0:    route(middleware.RecordLatency(middleware.LogWrap(logger, accessLogger, taskHandler.RejectTask), emitter)),
		bbs.CompleteTaskRoute_r0:  route(middleware.RecordLatency(middleware.LogWrap(logger, accessLogger, taskHandler.CompleteTask), emitter)),
		bbs.ResolvingTaskRoute_r0: route(middleware.RecordLatency(middleware.LogWrap(logger, accessLogger, taskHandler.ResolvingTask), emitter)),
		bbs.DeleteTaskRoute_r0:    route(middleware.RecordLatency(middleware.LogWrap(logger, accessLogger, taskHandler.DeleteTask), emitter)),

		// Events
		bbs.EventStreamRoute_r0:            route(middleware.LogWrap(logger, accessLogger, lrpGroupEventsHandler.Subscribe_r0)),    // DEPRECATED
		bbs.TaskEventStreamRoute_r0:        route(middleware.LogWrap(logger, accessLogger, taskEventsHandler.Subscribe_r0)),        // DEPRECATED
		bbs.LrpInstanceEventStreamRoute_r0: route(middleware.LogWrap(logger, accessLogger, lrpInstanceEventsHandler.Subscribe_r0)), // DEPRECATED
		bbs.LRPGroupEventStreamRoute_r1:    route(middleware.LogWrap(logger, accessLogger, lrpGroupEventsHandler.Subscribe_r1)),
		bbs.TaskEventStreamRoute_r1:        route(middleware.LogWrap(logger, accessLogger, taskEventsHandler.Subscribe_r1)),
		bbs.LRPInstanceEventStreamRoute_r1: route(middleware.LogWrap(logger, accessLogger, lrpInstanceEventsHandler.Subscribe_r1)),

		// Cells
		bbs.CellsRoute_r0: route(middleware.RecordLatency(middleware.LogWrap(logger, accessLogger, cellsHandler.Cells), emitter)),
	}

	handler, err := rata.NewRouter(bbs.Routes, actions)
	if err != nil {
		panic("unable to create router: " + err.Error())
	}

	return middleware.ContextCancellableRequest(
		middleware.RecordRequestCount(
			UnavailableWrap(handler,
				migrationsDone,
			),
			emitter,
		),
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
