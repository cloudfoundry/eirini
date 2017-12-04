package sqldb

import (
	"fmt"
	"math"
	"time"

	"code.cloudfoundry.org/auctioneer"
	"code.cloudfoundry.org/bbs/db/sqldb/helpers"
	"code.cloudfoundry.org/bbs/models"
	"code.cloudfoundry.org/lager"
)

const (
	convergeTaskRunsCounter = "ConvergenceTaskRuns"
	convergeTaskDuration    = "ConvergenceTaskDuration"

	tasksKickedCounter = "ConvergenceTasksKicked"
	tasksPrunedCounter = "ConvergenceTasksPruned"

	pendingTasks   = "TasksPending"
	runningTasks   = "TasksRunning"
	completedTasks = "TasksCompleted"
	resolvingTasks = "TasksResolving"

	expiredFailureReason         = "not started within time limit"
	cellDisappearedFailureReason = "cell disappeared before completion"
)

func (db *SQLDB) ConvergeTasks(logger lager.Logger, cellSet models.CellSet, kickTasksDuration, expirePendingTaskDuration, expireCompletedTaskDuration time.Duration) ([]*auctioneer.TaskStartRequest, []*models.Task, []models.Event) {
	logger.Info("starting")
	defer logger.Info("completed")

	db.metronClient.IncrementCounter(convergeTaskRunsCounter)
	convergeStart := db.clock.Now()

	defer func() {
		err := db.metronClient.SendDuration(convergeTaskDuration, time.Since(convergeStart))
		if err != nil {
			logger.Error("failed-to-send-converge-task-duration-metric", err)
		}
	}()

	var tasksPruned, tasksKicked uint64

	events, failedFetches, rowsAffected := db.failExpiredPendingTasks(logger, expirePendingTaskDuration)
	tasksPruned += failedFetches
	tasksKicked += uint64(rowsAffected)

	tasksToAuction, failedFetches := db.getTaskStartRequestsForKickablePendingTasks(logger, kickTasksDuration, expirePendingTaskDuration)
	tasksPruned += failedFetches
	tasksKicked += uint64(len(tasksToAuction))

	failedEvents, failedFetches, rowsAffected := db.failTasksWithDisappearedCells(logger, cellSet)
	tasksPruned += failedFetches
	tasksKicked += uint64(rowsAffected)
	events = append(events, failedEvents...)

	// do this first so that we now have "Completed" tasks before cleaning up
	// or re-sending the completion callback
	demotedEvents, failedFetches := db.demoteKickableResolvingTasks(logger, kickTasksDuration)
	tasksPruned += failedFetches
	events = append(events, demotedEvents...)

	removedEvents, rowsAffected := db.deleteExpiredCompletedTasks(logger, expireCompletedTaskDuration)
	tasksPruned += uint64(rowsAffected)
	events = append(events, removedEvents...)

	tasksToComplete, failedFetches := db.getKickableCompleteTasksForCompletion(logger, kickTasksDuration)
	tasksPruned += failedFetches
	tasksKicked += uint64(len(tasksToComplete))

	pendingCount, runningCount, completedCount, resolvingCount := db.countTasksByState(logger.Session("count-tasks"), db.db)

	db.sendTaskMetrics(logger, pendingCount, runningCount, completedCount, resolvingCount)

	db.metronClient.IncrementCounterWithDelta(tasksKickedCounter, uint64(tasksKicked))
	db.metronClient.IncrementCounterWithDelta(tasksPrunedCounter, uint64(tasksPruned))

	return tasksToAuction, tasksToComplete, events
}

func (db *SQLDB) failExpiredPendingTasks(logger lager.Logger, expirePendingTaskDuration time.Duration) ([]models.Event, uint64, int64) {
	logger = logger.Session("fail-expired-pending-tasks")

	now := db.clock.Now()

	rows, err := db.all(logger, db.db, tasksTable,
		taskColumns, helpers.NoLockRow,
		"state = ? AND created_at < ?", models.Task_Pending, now.Add(-expirePendingTaskDuration).UnixNano())
	if err != nil {
		logger.Error("failed-query", err)
		return nil, 0, 0
	}
	defer rows.Close()

	tasks, invalidTasksCount, err := db.fetchTasks(logger, rows, db.db, false)

	if err != nil {
		logger.Error("failed-fetching-some-tasks", err)
	}

	result, err := db.update(logger, db.db, tasksTable,
		helpers.SQLAttributes{
			"failed":             true,
			"failure_reason":     expiredFailureReason,
			"result":             "",
			"state":              models.Task_Completed,
			"first_completed_at": now.UnixNano(),
			"updated_at":         now.UnixNano(),
		},
		"state = ? AND created_at < ?", models.Task_Pending, now.Add(-expirePendingTaskDuration).UnixNano())
	if err != nil {
		logger.Error("failed-query", err)
		return nil, uint64(invalidTasksCount), 0
	}

	var events []models.Event
	for _, task := range tasks {
		afterTask := *task
		afterTask.Failed = true
		afterTask.FailureReason = expiredFailureReason
		afterTask.Result = ""
		afterTask.State = models.Task_Completed
		afterTask.FirstCompletedAt = now.UnixNano()
		afterTask.UpdatedAt = now.UnixNano()

		events = append(events, models.NewTaskChangedEvent(task, &afterTask))
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		logger.Error("failed-rows-affected", err)
		return events, uint64(invalidTasksCount), 0
	}
	return events, uint64(invalidTasksCount), rowsAffected
}

func (db *SQLDB) getTaskStartRequestsForKickablePendingTasks(logger lager.Logger, kickTasksDuration, expirePendingTaskDuration time.Duration) ([]*auctioneer.TaskStartRequest, uint64) {
	logger = logger.Session("get-task-start-requests-for-kickable-pending-tasks")

	rows, err := db.all(logger, db.db, tasksTable,
		taskColumns, helpers.NoLockRow,
		"state = ? AND updated_at < ? AND created_at > ?",
		models.Task_Pending, db.clock.Now().Add(-kickTasksDuration).UnixNano(), db.clock.Now().Add(-expirePendingTaskDuration).UnixNano(),
	)

	if err != nil {
		logger.Error("failed-query", err)
		return []*auctioneer.TaskStartRequest{}, math.MaxUint64
	}

	defer rows.Close()

	tasksToAuction := []*auctioneer.TaskStartRequest{}
	tasks, invalidTasksCount, err := db.fetchTasks(logger, rows, db.db, false)
	for _, task := range tasks {
		taskStartRequest := auctioneer.NewTaskStartRequestFromModel(task.TaskGuid, task.Domain, task.TaskDefinition)
		tasksToAuction = append(tasksToAuction, &taskStartRequest)
	}

	if err != nil {
		logger.Error("failed-fetching-some-tasks", err)
	}

	return tasksToAuction, uint64(invalidTasksCount)
}

func (db *SQLDB) failTasksWithDisappearedCells(logger lager.Logger, cellSet models.CellSet) ([]models.Event, uint64, int64) {
	logger = logger.Session("fail-tasks-with-disappeared-cells")

	values := make([]interface{}, 0, 1+len(cellSet))
	values = append(values, models.Task_Running)

	for k := range cellSet {
		values = append(values, k)
	}

	wheres := "state = ?"
	if len(cellSet) != 0 {
		wheres += fmt.Sprintf(" AND cell_id NOT IN (%s)", helpers.QuestionMarks(len(cellSet)))
	}
	now := db.clock.Now().UnixNano()

	rows, err := db.all(logger, db.db, tasksTable, taskColumns, helpers.NoLockRow, wheres, values...)
	if err != nil {
		logger.Error("failed-query", err)
		return nil, 0, 0
	}
	defer rows.Close()

	tasks, invalidTasksCount, err := db.fetchTasks(logger, rows, db.db, false)
	if err != nil {
		logger.Error("failed-fetching-tasks", err)
	}

	result, err := db.update(logger, db.db, tasksTable,
		helpers.SQLAttributes{
			"failed":             true,
			"failure_reason":     cellDisappearedFailureReason,
			"result":             "",
			"state":              models.Task_Completed,
			"first_completed_at": now,
			"updated_at":         now,
		},
		wheres, values...,
	)
	if err != nil {
		logger.Error("failed-updating-tasks", err)
		return nil, uint64(invalidTasksCount), 0
	}

	var events []models.Event
	for _, task := range tasks {
		afterTask := *task
		afterTask.Failed = true
		afterTask.FailureReason = cellDisappearedFailureReason
		afterTask.Result = ""
		afterTask.State = models.Task_Completed
		afterTask.FirstCompletedAt = now
		afterTask.UpdatedAt = now

		events = append(events, models.NewTaskChangedEvent(task, &afterTask))
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		logger.Error("failed-rows-affected", err)
		return events, uint64(invalidTasksCount), 0
	}

	return events, uint64(invalidTasksCount), rowsAffected
}

func (db *SQLDB) demoteKickableResolvingTasks(logger lager.Logger, kickTasksDuration time.Duration) ([]models.Event, uint64) {
	logger = logger.Session("demote-kickable-resolving-tasks")

	rows, err := db.all(logger, db.db, tasksTable,
		taskColumns, helpers.NoLockRow,
		"state = ? AND updated_at < ?", models.Task_Resolving, db.clock.Now().Add(-kickTasksDuration).UnixNano(),
	)
	if err != nil {
		logger.Error("failed-query", err)
		return nil, 0
	}
	defer rows.Close()

	tasks, invalidTasksCount, err := db.fetchTasks(logger, rows, db.db, false)
	if err != nil {
		logger.Error("failed-fetching-tasks", err)
	}

	_, err = db.update(logger, db.db, tasksTable,
		helpers.SQLAttributes{"state": models.Task_Completed},
		"state = ? AND updated_at < ?",
		models.Task_Resolving, db.clock.Now().Add(-kickTasksDuration).UnixNano(),
	)
	if err != nil {
		logger.Error("failed-updating-tasks", err)
	}

	var events []models.Event
	for _, task := range tasks {
		afterTask := *task
		afterTask.State = models.Task_Completed
		events = append(events, models.NewTaskChangedEvent(task, &afterTask))
	}

	return events, uint64(invalidTasksCount)
}

func (db *SQLDB) deleteExpiredCompletedTasks(logger lager.Logger, expireCompletedTaskDuration time.Duration) ([]models.Event, int64) {
	logger = logger.Session("delete-expired-completed-tasks")
	wheres := "state = ? AND first_completed_at < ?"
	values := []interface{}{models.Task_Completed, db.clock.Now().Add(-expireCompletedTaskDuration).UnixNano()}

	rows, err := db.all(logger, db.db, tasksTable,
		taskColumns, helpers.NoLockRow,
		wheres, values...,
	)
	if err != nil {
		logger.Error("failed-query", err)
		return nil, 0
	}
	defer rows.Close()

	tasks, invalidTasksCount, err := db.fetchTasks(logger, rows, db.db, false)
	if err != nil {
		logger.Error("failed-fetching-tasks", err)
		return nil, int64(invalidTasksCount)
	}

	result, err := db.delete(logger, db.db, tasksTable, wheres, values...)
	if err != nil {
		logger.Error("failed-query", err)
		return nil, int64(invalidTasksCount)
	}

	var events []models.Event
	for _, task := range tasks {
		events = append(events, models.NewTaskRemovedEvent(task))
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		logger.Error("failed-rows-affected", err)
		return events, int64(invalidTasksCount)
	}
	rowsAffected += int64(invalidTasksCount)

	return events, rowsAffected
}

func (db *SQLDB) getKickableCompleteTasksForCompletion(logger lager.Logger, kickTasksDuration time.Duration) ([]*models.Task, uint64) {
	logger = logger.Session("get-kickable-complete-tasks-for-completion")

	rows, err := db.all(logger, db.db, tasksTable,
		taskColumns, helpers.NoLockRow,
		"state = ? AND updated_at < ?",
		models.Task_Completed, db.clock.Now().Add(-kickTasksDuration).UnixNano(),
	)

	if err != nil {
		logger.Error("failed-query", err)
		return []*models.Task{}, math.MaxUint64
	}

	defer rows.Close()

	tasksToComplete, failedFetches, err := db.fetchTasks(logger, rows, db.db, false)

	if err != nil {
		logger.Error("failed-fetching-some-tasks", err)
	}

	return tasksToComplete, uint64(failedFetches)
}

func (db *SQLDB) sendTaskMetrics(logger lager.Logger, pendingCount, runningCount, completedCount, resolvingCount int) {
	err := db.metronClient.SendMetric(pendingTasks, pendingCount)
	if err != nil {
		logger.Error("failed-to-send-pending-tasks-metric", err)
	}

	err = db.metronClient.SendMetric(runningTasks, runningCount)
	if err != nil {
		logger.Error("failed-to-send-running-tasks-metric", err)
	}

	err = db.metronClient.SendMetric(completedTasks, completedCount)
	if err != nil {
		logger.Error("failed-to-send-completed-tasks-metric", err)
	}

	err = db.metronClient.SendMetric(resolvingTasks, resolvingCount)
	if err != nil {
		logger.Error("failed-to-send-resolving-tasks-metric", err)
	}
}
