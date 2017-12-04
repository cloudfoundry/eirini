package controllers

import (
	"time"

	"code.cloudfoundry.org/auctioneer"
	"code.cloudfoundry.org/bbs/db"
	"code.cloudfoundry.org/bbs/events"
	"code.cloudfoundry.org/bbs/models"
	"code.cloudfoundry.org/bbs/serviceclient"
	"code.cloudfoundry.org/bbs/taskworkpool"
	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/rep"
)

type TaskController struct {
	db                   db.TaskDB
	taskCompletionClient taskworkpool.TaskCompletionClient
	auctioneerClient     auctioneer.Client
	serviceClient        serviceclient.ServiceClient
	repClientFactory     rep.ClientFactory
	taskHub              events.Hub
}

func NewTaskController(
	db db.TaskDB,
	taskCompletionClient taskworkpool.TaskCompletionClient,
	auctioneerClient auctioneer.Client,
	serviceClient serviceclient.ServiceClient,
	repClientFactory rep.ClientFactory,
	taskHub events.Hub,
) *TaskController {
	return &TaskController{
		db:                   db,
		taskCompletionClient: taskCompletionClient,
		auctioneerClient:     auctioneerClient,
		serviceClient:        serviceClient,
		repClientFactory:     repClientFactory,
		taskHub:              taskHub,
	}
}

func (h *TaskController) Tasks(logger lager.Logger, domain, cellId string) ([]*models.Task, error) {
	logger = logger.Session("tasks")

	filter := models.TaskFilter{Domain: domain, CellID: cellId}
	return h.db.Tasks(logger, filter)
}

func (h *TaskController) TaskByGuid(logger lager.Logger, taskGuid string) (*models.Task, error) {
	logger = logger.Session("task-by-guid")

	return h.db.TaskByGuid(logger, taskGuid)
}

func (h *TaskController) DesireTask(logger lager.Logger, taskDefinition *models.TaskDefinition, taskGuid, domain string) error {
	var err error
	var task *models.Task
	logger = logger.Session("desire-task")

	logger = logger.WithData(lager.Data{"task_guid": taskGuid})

	task, err = h.db.DesireTask(logger, taskDefinition, taskGuid, domain)
	if err != nil {
		return err
	}
	go h.taskHub.Emit(models.NewTaskCreatedEvent(task))

	logger.Debug("start-task-auction-request")
	taskStartRequest := auctioneer.NewTaskStartRequestFromModel(taskGuid, domain, taskDefinition)
	err = h.auctioneerClient.RequestTaskAuctions(logger, []*auctioneer.TaskStartRequest{&taskStartRequest})
	if err != nil {
		logger.Error("failed-requesting-task-auction", err)
		// The creation succeeded, the auction request error can be dropped
	} else {
		logger.Debug("succeeded-requesting-task-auction")
	}

	return nil
}

func (h *TaskController) StartTask(logger lager.Logger, taskGuid, cellId string) (shouldStart bool, err error) {
	logger = logger.Session("start-task", lager.Data{"task_guid": taskGuid, "cell_id": cellId})
	before, after, shouldStart, err := h.db.StartTask(logger, taskGuid, cellId)
	if err == nil && shouldStart {
		go h.taskHub.Emit(models.NewTaskChangedEvent(before, after))
	}
	return shouldStart, err
}

func (h *TaskController) CancelTask(logger lager.Logger, taskGuid string) error {
	logger = logger.Session("cancel-task")

	before, after, cellID, err := h.db.CancelTask(logger, taskGuid)
	if err != nil {
		return err
	}
	go h.taskHub.Emit(models.NewTaskChangedEvent(before, after))

	if after.CompletionCallbackUrl != "" {
		logger.Info("task-client-completing-task")
		go h.taskCompletionClient.Submit(h.db, h.taskHub, after)
	}

	if cellID == "" {
		return nil
	}

	logger.Info("start-check-cell-presence", lager.Data{"cell_id": cellID})
	cellPresence, err := h.serviceClient.CellById(logger, cellID)
	if err != nil {
		logger.Error("failed-fetching-cell-presence", err)
		// don't return an error, the rep will converge later
		return nil
	}
	logger.Info("finished-check-cell-presence", lager.Data{"cell_id": cellID})

	repClient, err := h.repClientFactory.CreateClient(cellPresence.RepAddress, cellPresence.RepUrl)
	if err != nil {
		logger.Error("create-rep-client-failed", err)
		return err
	}
	logger.Info("start-rep-cancel-task", lager.Data{"task_guid": taskGuid})
	repClient.CancelTask(logger, taskGuid)
	if err != nil {
		logger.Error("failed-rep-cancel-task", err)
		// don't return an error, the rep will converge later
		return nil
	}
	logger.Info("finished-rep-cancel-task", lager.Data{"task_guid": taskGuid})
	return nil
}

func (h *TaskController) FailTask(logger lager.Logger, taskGuid, failureReason string) error {
	var err error
	logger = logger.Session("fail-task")

	before, after, err := h.db.FailTask(logger, taskGuid, failureReason)
	if err != nil {
		return err
	}
	go h.taskHub.Emit(models.NewTaskChangedEvent(before, after))

	if after.CompletionCallbackUrl != "" {
		logger.Info("task-client-completing-task")
		go h.taskCompletionClient.Submit(h.db, h.taskHub, after)
	}

	return nil
}

func (h *TaskController) CompleteTask(
	logger lager.Logger,
	taskGuid,
	cellId string,
	failed bool,
	failureReason,
	result string,
) error {
	var err error
	logger = logger.Session("complete-task")

	before, after, err := h.db.CompleteTask(logger, taskGuid, cellId, failed, failureReason, result)
	if err != nil {
		return err
	}
	go h.taskHub.Emit(models.NewTaskChangedEvent(before, after))

	if after.CompletionCallbackUrl != "" {
		logger.Info("task-client-completing-task")
		go h.taskCompletionClient.Submit(h.db, h.taskHub, after)
	}

	return nil
}

func (h *TaskController) ResolvingTask(logger lager.Logger, taskGuid string) error {
	logger = logger.Session("resolving-task")

	before, after, err := h.db.ResolvingTask(logger, taskGuid)
	if err != nil {
		return err
	}
	go h.taskHub.Emit(models.NewTaskChangedEvent(before, after))

	return nil
}

func (h *TaskController) DeleteTask(logger lager.Logger, taskGuid string) error {
	logger = logger.Session("delete-task")

	task, err := h.db.DeleteTask(logger, taskGuid)
	if err != nil {
		return err
	}
	go h.taskHub.Emit(models.NewTaskRemovedEvent(task))

	return nil
}

func (h *TaskController) ConvergeTasks(
	logger lager.Logger,
	kickTaskDuration,
	expirePendingTaskDuration,
	expireCompletedTaskDuration time.Duration,
) error {
	var err error
	logger = logger.Session("converge-tasks")

	logger.Debug("listing-cells")
	cellSet, err := h.serviceClient.Cells(logger)
	if err == models.ErrResourceNotFound {
		logger.Debug("no-cells-found")
		cellSet = models.CellSet{}
	} else if err != nil {
		logger.Debug("failed-listing-cells")
		return err
	}
	logger.Debug("succeeded-listing-cells")

	tasksToAuction, tasksToComplete, eventsToEmit := h.db.ConvergeTasks(
		logger,
		cellSet,
		kickTaskDuration,
		expirePendingTaskDuration,
		expireCompletedTaskDuration,
	)

	logger.Debug("emitting events from convergence", lager.Data{"num_tasks_to_complete": len(tasksToComplete)})
	for _, event := range eventsToEmit {
		go h.taskHub.Emit(event)
	}

	if len(tasksToAuction) > 0 {
		logger.Debug("requesting-task-auctions", lager.Data{"num_tasks_to_auction": len(tasksToAuction)})
		err = h.auctioneerClient.RequestTaskAuctions(logger, tasksToAuction)
		if err != nil {
			taskGuids := make([]string, len(tasksToAuction))
			for i, task := range tasksToAuction {
				taskGuids[i] = task.TaskGuid
			}
			logger.Error("failed-to-request-auctions-for-pending-tasks", err,
				lager.Data{"task_guids": taskGuids})
		}
		logger.Debug("done-requesting-task-auctions", lager.Data{"num_tasks_to_auction": len(tasksToAuction)})
	}

	logger.Debug("submitting-tasks-to-be-completed", lager.Data{"num_tasks_to_complete": len(tasksToComplete)})
	for _, task := range tasksToComplete {
		h.taskCompletionClient.Submit(h.db, h.taskHub, task)
	}
	logger.Debug("done-submitting-tasks-to-be-completed", lager.Data{"num_tasks_to_complete": len(tasksToComplete)})

	return nil
}
