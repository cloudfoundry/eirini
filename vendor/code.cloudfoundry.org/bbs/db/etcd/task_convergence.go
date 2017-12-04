package etcd

import (
	"time"

	"code.cloudfoundry.org/auctioneer"
	"code.cloudfoundry.org/bbs/models"
	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/workpool"
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
)

type compareAndSwappableTask struct {
	OldIndex uint64
	NewTask  *models.Task
}

func (db *ETCDDB) ConvergeTasks(
	logger lager.Logger,
	cellSet models.CellSet,
	kickTaskDuration, expirePendingTaskDuration, expireCompletedTaskDuration time.Duration,
) ([]*auctioneer.TaskStartRequest, []*models.Task, []models.Event) {
	logger.Info("starting-convergence")
	defer logger.Info("finished-convergence")

	db.metronClient.IncrementCounter(convergeTaskRunsCounter)

	convergeStart := db.clock.Now()

	defer func() {
		err := db.metronClient.SendDuration(convergeTaskDuration, time.Since(convergeStart))
		if err != nil {
			logger.Error("failed-to-send-converge-task-duration-metric", err)
		}
	}()

	logger.Debug("listing-tasks")
	taskState, modelErr := db.fetchRecursiveRaw(logger, TaskSchemaRoot)
	if modelErr != nil {
		logger.Debug("failed-listing-task")
		sendTaskMetrics(logger, db, -1, -1, -1, -1)
		return nil, nil, []models.Event{}
	}
	logger.Debug("succeeded-listing-task")

	logError := func(task *models.Task, message string) {
		logger.Error(message, nil, lager.Data{
			"task_guid": task.TaskGuid,
		})
	}

	tasksToComplete := []*models.Task{}
	scheduleForCompletion := func(task *models.Task) {
		if task.CompletionCallbackUrl == "" {
			return
		}
		tasksToComplete = append(tasksToComplete, task)
	}

	keysToDelete := []string{}

	tasksToCAS := []compareAndSwappableTask{}
	scheduleForCASByIndex := func(index uint64, newTask *models.Task) {
		tasksToCAS = append(tasksToCAS, compareAndSwappableTask{
			OldIndex: index,
			NewTask:  newTask,
		})
	}

	tasksToAuction := []*auctioneer.TaskStartRequest{}

	var tasksKicked uint64 = 0

	pendingCount := 0
	runningCount := 0
	completedCount := 0
	resolvingCount := 0

	logger.Debug("determining-convergence-work", lager.Data{"num_tasks": len(taskState.Nodes)})
	for _, node := range taskState.Nodes {
		task := new(models.Task)
		err := db.deserializeModel(logger, node, task)
		if err != nil || task.Validate() != nil {
			logger.Error("found-invalid-task", err, lager.Data{
				"key":   node.Key,
				"value": node.Value,
			})

			keysToDelete = append(keysToDelete, node.Key)
			continue
		}

		shouldKickTask := db.durationSinceTaskUpdated(task) >= kickTaskDuration

		switch task.State {
		case models.Task_Pending:
			pendingCount++
			shouldMarkAsFailed := db.durationSinceTaskCreated(task) >= expirePendingTaskDuration
			if shouldMarkAsFailed {
				logError(task, "failed-to-start-in-time")
				db.markTaskFailed(task, "not started within time limit")
				scheduleForCASByIndex(node.ModifiedIndex, task)
				tasksKicked++
			} else if shouldKickTask {
				logger.Info("requesting-auction-for-pending-task", lager.Data{"task_guid": task.TaskGuid})
				start := auctioneer.NewTaskStartRequestFromModel(task.TaskGuid, task.Domain, task.TaskDefinition)
				tasksToAuction = append(tasksToAuction, &start)
				tasksKicked++
			}
		case models.Task_Running:
			runningCount++
			cellIsAlive := cellSet.HasCellID(task.CellId)
			if !cellIsAlive {
				logError(task, "cell-disappeared")
				db.markTaskFailed(task, "cell disappeared before completion")
				scheduleForCASByIndex(node.ModifiedIndex, task)
				tasksKicked++
			}
		case models.Task_Completed:
			completedCount++
			shouldDeleteTask := db.durationSinceTaskFirstCompleted(task) >= expireCompletedTaskDuration
			if shouldDeleteTask {
				logError(task, "failed-to-start-resolving-in-time")
				keysToDelete = append(keysToDelete, node.Key)
			} else if shouldKickTask {
				logger.Info("kicking-completed-task", lager.Data{"task_guid": task.TaskGuid})
				scheduleForCompletion(task)
				tasksKicked++
			}
		case models.Task_Resolving:
			resolvingCount++
			shouldDeleteTask := db.durationSinceTaskFirstCompleted(task) >= expireCompletedTaskDuration
			if shouldDeleteTask {
				logError(task, "failed-to-resolve-in-time")
				keysToDelete = append(keysToDelete, node.Key)
			} else if shouldKickTask {
				logger.Info("demoting-resolving-to-completed", lager.Data{"task_guid": task.TaskGuid})
				demoted := demoteToCompleted(task)
				scheduleForCASByIndex(node.ModifiedIndex, demoted)
				scheduleForCompletion(demoted)
				tasksKicked++
			}
		}
	}
	logger.Debug("done-determining-convergence-work", lager.Data{
		"num_tasks_to_auction":  len(tasksToAuction),
		"num_tasks_to_cas":      len(tasksToCAS),
		"num_tasks_to_complete": len(tasksToComplete),
		"num_keys_to_delete":    len(keysToDelete),
	})

	sendTaskMetrics(logger, db, pendingCount, runningCount, completedCount, resolvingCount)

	db.metronClient.IncrementCounterWithDelta(tasksKickedCounter, tasksKicked)
	logger.Debug("compare-and-swapping-tasks", lager.Data{"num_tasks_to_cas": len(tasksToCAS)})
	err := db.batchCompareAndSwapTasks(tasksToCAS, logger)
	if err != nil {
		return nil, nil, []models.Event{}
	}
	logger.Debug("done-compare-and-swapping-tasks", lager.Data{"num_tasks_to_cas": len(tasksToCAS)})

	db.metronClient.IncrementCounterWithDelta(tasksPrunedCounter, uint64(len(keysToDelete)))
	logger.Debug("deleting-keys", lager.Data{"num_keys_to_delete": len(keysToDelete)})
	db.batchDeleteTasks(keysToDelete, logger)
	logger.Debug("done-deleting-keys", lager.Data{"num_keys_to_delete": len(keysToDelete)})

	return tasksToAuction, tasksToComplete, []models.Event{}
}

func (db *ETCDDB) durationSinceTaskCreated(task *models.Task) time.Duration {
	return db.clock.Now().Sub(time.Unix(0, task.CreatedAt))
}

func (db *ETCDDB) durationSinceTaskUpdated(task *models.Task) time.Duration {
	return db.clock.Now().Sub(time.Unix(0, task.UpdatedAt))
}

func (db *ETCDDB) durationSinceTaskFirstCompleted(task *models.Task) time.Duration {
	if task.FirstCompletedAt == 0 {
		return 0
	}
	return db.clock.Now().Sub(time.Unix(0, task.FirstCompletedAt))
}

func (db *ETCDDB) markTaskFailed(task *models.Task, reason string) {
	db.markTaskCompleted(task, true, reason, "")
}

func demoteToCompleted(task *models.Task) *models.Task {
	task.State = models.Task_Completed
	return task
}

func (db *ETCDDB) batchCompareAndSwapTasks(tasksToCAS []compareAndSwappableTask, logger lager.Logger) error {
	if len(tasksToCAS) == 0 {
		return nil
	}

	works := []func(){}

	for _, taskToCAS := range tasksToCAS {
		task := taskToCAS.NewTask
		task.UpdatedAt = db.clock.Now().UnixNano()
		value, err := db.serializeModel(logger, task)
		if err != nil {
			logger.Error("failed-to-marshal", err, lager.Data{
				"task_guid": task.TaskGuid,
			})
			continue
		}

		index := taskToCAS.OldIndex
		works = append(works, func() {
			_, err := db.client.CompareAndSwap(TaskSchemaPathByGuid(task.TaskGuid), value, NO_TTL, index)
			if err != nil {
				logger.Error("failed-to-compare-and-swap", err, lager.Data{
					"task_guid": task.TaskGuid,
				})
			}
		})
	}

	throttler, err := workpool.NewThrottler(db.convergenceWorkersSize, works)
	if err != nil {
		return err
	}

	throttler.Work()
	return nil
}

func (db *ETCDDB) batchDeleteTasks(taskGuids []string, logger lager.Logger) {
	if len(taskGuids) == 0 {
		return
	}

	works := []func(){}

	for _, taskGuid := range taskGuids {
		taskGuid := taskGuid
		works = append(works, func() {
			_, err := db.client.Delete(taskGuid, true)
			if err != nil {
				logger.Error("failed-to-delete", err, lager.Data{
					"task_guid": taskGuid,
				})
			}
		})
	}

	throttler, err := workpool.NewThrottler(db.convergenceWorkersSize, works)
	if err != nil {
		logger.Error("failed-to-create-throttler", err)
	}

	throttler.Work()
	return
}

func sendTaskMetrics(logger lager.Logger, db *ETCDDB, pendingCount, runningCount, completedCount, resolvingCount int) {
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
