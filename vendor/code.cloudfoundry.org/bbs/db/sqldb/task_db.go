package sqldb

import (
	"database/sql"
	"strings"

	"code.cloudfoundry.org/bbs/db/sqldb/helpers"
	"code.cloudfoundry.org/bbs/models"
	"code.cloudfoundry.org/lager"
)

func (db *SQLDB) DesireTask(logger lager.Logger, taskDef *models.TaskDefinition, taskGuid, domain string) (*models.Task, error) {
	logger = logger.Session("desire-task", lager.Data{"task_guid": taskGuid})
	logger.Info("starting")
	defer logger.Info("complete")

	taskDefData, err := db.serializeModel(logger, taskDef)
	if err != nil {
		logger.Error("failed-serializing-task-definition", err)
		return nil, err
	}

	now := db.clock.Now().UnixNano()
	err = db.transact(logger, func(logger lager.Logger, tx *sql.Tx) error {
		_, err = db.insert(logger, tx, tasksTable,
			helpers.SQLAttributes{
				"guid":               taskGuid,
				"domain":             domain,
				"created_at":         now,
				"updated_at":         now,
				"first_completed_at": 0,
				"state":              models.Task_Pending,
				"task_definition":    taskDefData,
			},
		)

		return err
	})

	if err != nil {
		logger.Error("failed-inserting-task", err)
		return nil, err
	}

	return &models.Task{
		TaskDefinition:   taskDef,
		TaskGuid:         taskGuid,
		Domain:           domain,
		CreatedAt:        now,
		UpdatedAt:        now,
		FirstCompletedAt: 0,
		State:            models.Task_Pending,
	}, nil
}

func (db *SQLDB) Tasks(logger lager.Logger, filter models.TaskFilter) ([]*models.Task, error) {
	logger = logger.Session("tasks", lager.Data{"filter": filter})
	logger.Debug("starting")
	defer logger.Debug("complete")

	wheres := []string{}
	values := []interface{}{}

	if filter.Domain != "" {
		wheres = append(wheres, "domain = ?")
		values = append(values, filter.Domain)
	}

	if filter.CellID != "" {
		wheres = append(wheres, "cell_id = ?")
		values = append(values, filter.CellID)
	}

	results := []*models.Task{}

	err := db.transact(logger, func(logger lager.Logger, tx *sql.Tx) error {
		rows, err := db.all(logger, tx, tasksTable,
			taskColumns, helpers.NoLockRow,
			strings.Join(wheres, " AND "), values...,
		)
		if err != nil {
			logger.Error("failed-query", err)
			return err
		}
		defer rows.Close()

		results, _, err = db.fetchTasks(logger, rows, tx, true)
		if err != nil {
			logger.Error("failed-fetch", err)
			return err
		}

		return nil
	})

	return results, err
}

func (db *SQLDB) TaskByGuid(logger lager.Logger, taskGuid string) (*models.Task, error) {
	logger = logger.Session("task-by-guid", lager.Data{"task_guid": taskGuid})
	logger.Debug("starting")
	defer logger.Debug("complete")

	var task *models.Task

	err := db.transact(logger, func(logger lager.Logger, tx *sql.Tx) error {
		var err error
		row := db.one(logger, tx, tasksTable,
			taskColumns, helpers.NoLockRow,
			"guid = ?", taskGuid,
		)

		task, err = db.fetchTask(logger, row, tx)
		return err
	})

	return task, err
}

func (db *SQLDB) StartTask(logger lager.Logger, taskGuid, cellId string) (*models.Task, *models.Task, bool, error) {
	logger = logger.Session("start-task", lager.Data{"task_guid": taskGuid, "cell_id": cellId})

	var started bool
	var beforeTask models.Task
	var afterTask *models.Task

	err := db.transact(logger, func(logger lager.Logger, tx *sql.Tx) error {
		var err error
		afterTask, err = db.fetchTaskForUpdate(logger, taskGuid, tx)
		if err != nil {
			logger.Error("failed-locking-task", err)
			return err
		}

		beforeTask = *afterTask
		if afterTask.State == models.Task_Running && afterTask.CellId == cellId {
			logger.Debug("task-already-running-on-cell")
			return nil
		}

		if err = afterTask.ValidateTransitionTo(models.Task_Running); err != nil {
			logger.Error("failed-to-transition-task-to-running", err)
			return err
		}

		logger.Info("starting")
		defer logger.Info("complete")
		now := db.clock.Now().UnixNano()
		_, err = db.update(logger, tx, tasksTable,
			helpers.SQLAttributes{
				"state":      models.Task_Running,
				"updated_at": now,
				"cell_id":    cellId,
			},
			"guid = ?", taskGuid,
		)
		if err != nil {
			return err
		}

		afterTask.State = models.Task_Running
		afterTask.UpdatedAt = now
		afterTask.CellId = cellId

		started = true
		return nil
	})

	return &beforeTask, afterTask, started, err
}

func (db *SQLDB) CancelTask(logger lager.Logger, taskGuid string) (*models.Task, *models.Task, string, error) {
	logger = logger.Session("cancel-task", lager.Data{"task_guid": taskGuid})
	logger.Info("starting")
	defer logger.Info("complete")

	var beforeTask models.Task
	var afterTask *models.Task
	var cellID string

	err := db.transact(logger, func(logger lager.Logger, tx *sql.Tx) error {
		var err error
		afterTask, err = db.fetchTaskForUpdate(logger, taskGuid, tx)
		if err != nil {
			logger.Error("failed-locking-task", err)
			return err
		}

		beforeTask = *afterTask
		cellID = afterTask.CellId

		if err = afterTask.ValidateTransitionTo(models.Task_Completed); err != nil {
			if afterTask.State != models.Task_Pending {
				logger.Error("failed-to-transition-task-to-completed", err)
				return err
			}
		}
		err = db.completeTask(logger, afterTask, true, "task was cancelled", "", tx)
		if err != nil {
			return err
		}

		return nil
	})

	return &beforeTask, afterTask, cellID, err
}

func (db *SQLDB) CompleteTask(logger lager.Logger, taskGuid, cellID string, failed bool, failureReason, taskResult string) (*models.Task, *models.Task, error) {
	logger = logger.Session("complete-task", lager.Data{"task_guid": taskGuid, "cell_id": cellID})
	logger.Info("starting")
	defer logger.Info("complete")

	var beforeTask models.Task
	var afterTask *models.Task

	err := db.transact(logger, func(logger lager.Logger, tx *sql.Tx) error {
		var err error
		afterTask, err = db.fetchTaskForUpdate(logger, taskGuid, tx)
		if err != nil {
			logger.Error("failed-locking-task", err)
			return err
		}
		beforeTask = *afterTask

		if afterTask.CellId != cellID && afterTask.State == models.Task_Running {
			logger.Error("failed-task-already-running-on-different-cell", err)
			return models.NewRunningOnDifferentCellError(cellID, afterTask.CellId)
		}

		if err = afterTask.ValidateTransitionTo(models.Task_Completed); err != nil {
			logger.Error("failed-to-transition-task-to-completed", err)
			return err
		}

		err = db.completeTask(logger, afterTask, failed, failureReason, taskResult, tx)
		if err != nil {
			return err
		}

		return nil
	})

	return &beforeTask, afterTask, err
}

func (db *SQLDB) FailTask(logger lager.Logger, taskGuid, failureReason string) (*models.Task, *models.Task, error) {
	logger = logger.Session("fail-task", lager.Data{"task_guid": taskGuid})
	logger.Info("starting")
	defer logger.Info("complete")

	var beforeTask models.Task
	var afterTask *models.Task

	err := db.transact(logger, func(logger lager.Logger, tx *sql.Tx) error {
		var err error
		afterTask, err = db.fetchTaskForUpdate(logger, taskGuid, tx)
		if err != nil {
			logger.Error("failed-locking-task", err)
			return err
		}

		beforeTask = *afterTask

		if err = afterTask.ValidateTransitionTo(models.Task_Completed); err != nil {
			if afterTask.State != models.Task_Pending {
				logger.Error("failed-to-transition-task-to-completed", err)
				return err
			}
		}

		err = db.completeTask(logger, afterTask, true, failureReason, "", tx)
		if err != nil {
			return err
		}

		afterTask.State = models.Task_Completed
		afterTask.Failed = true
		afterTask.FailureReason = failureReason
		return nil
	})

	return &beforeTask, afterTask, err
}

// The stager calls this when it wants to claim a completed task.  This ensures that only one
// stager ever attempts to handle a completed task
func (db *SQLDB) ResolvingTask(logger lager.Logger, taskGuid string) (*models.Task, *models.Task, error) {
	logger = logger.WithData(lager.Data{"task_guid": taskGuid})
	logger.Info("starting")
	defer logger.Info("complete")

	var beforeTask models.Task
	var afterTask *models.Task

	err := db.transact(logger, func(logger lager.Logger, tx *sql.Tx) error {
		var err error
		afterTask, err = db.fetchTaskForUpdate(logger, taskGuid, tx)
		if err != nil {
			logger.Error("failed-locking-task", err)
			return err
		}

		beforeTask = *afterTask

		if err = afterTask.ValidateTransitionTo(models.Task_Resolving); err != nil {
			logger.Error("invalid-state-transition", err)
			return err
		}

		now := db.clock.Now().UnixNano()
		_, err = db.update(logger, tx, tasksTable,
			helpers.SQLAttributes{
				"state":      models.Task_Resolving,
				"updated_at": now,
			},
			"guid = ?", taskGuid,
		)
		if err != nil {
			logger.Error("failed-updating-tasks", err)
			return err
		}

		afterTask.State = models.Task_Resolving
		afterTask.UpdatedAt = now

		return nil
	})

	return &beforeTask, afterTask, err
}

func (db *SQLDB) DeleteTask(logger lager.Logger, taskGuid string) (*models.Task, error) {
	logger = logger.Session("delete-task", lager.Data{"task_guid": taskGuid})
	logger.Info("starting")
	defer logger.Info("complete")

	var task *models.Task

	err := db.transact(logger, func(logger lager.Logger, tx *sql.Tx) error {
		var err error
		task, err = db.fetchTaskForUpdate(logger, taskGuid, tx)
		if err != nil {
			logger.Error("failed-locking-task", err)
			return err
		}

		if task.State != models.Task_Resolving {
			err = models.NewTaskTransitionError(task.State, models.Task_Resolving)
			logger.Error("invalid-state-transition", err)
			return err
		}

		_, err = db.delete(logger, tx, tasksTable, "guid = ?", taskGuid)
		if err != nil {
			logger.Error("failed-deleting-task", err)
			return err
		}

		return nil
	})
	return task, err
}

func (db *SQLDB) completeTask(logger lager.Logger, task *models.Task, failed bool, failureReason, result string, tx *sql.Tx) error {
	now := db.clock.Now().UnixNano()
	_, err := db.update(logger, tx, tasksTable,
		helpers.SQLAttributes{
			"failed":             failed,
			"failure_reason":     failureReason,
			"result":             result,
			"state":              models.Task_Completed,
			"first_completed_at": now,
			"updated_at":         now,
			"cell_id":            "",
		},
		"guid = ?", task.TaskGuid,
	)
	if err != nil {
		logger.Error("failed-updating-tasks", err)
		return err
	}

	task.State = models.Task_Completed
	task.UpdatedAt = now
	task.FirstCompletedAt = now
	task.Failed = failed
	task.FailureReason = failureReason
	task.Result = result
	task.CellId = ""

	return nil
}

func (db *SQLDB) fetchTaskForUpdate(logger lager.Logger, taskGuid string, queryable Queryable) (*models.Task, error) {
	row := db.one(logger, queryable, tasksTable,
		taskColumns, helpers.LockRow,
		"guid = ?", taskGuid,
	)
	return db.fetchTask(logger, row, queryable)
}

func (db *SQLDB) fetchTasks(logger lager.Logger, rows *sql.Rows, queryable Queryable, abortOnError bool) ([]*models.Task, int, error) {
	tasks := []*models.Task{}
	invalidGuids := []string{}
	var err error
	for rows.Next() {
		var task *models.Task
		var guid string

		task, guid, err = db.fetchTaskInternal(logger, rows)
		if err == models.ErrDeserialize {
			invalidGuids = append(invalidGuids, guid)
			if abortOnError {
				break
			}
			continue
		}
		tasks = append(tasks, task)
	}

	if err == nil {
		err = rows.Err()
	}

	rows.Close()

	if len(invalidGuids) > 0 {
		db.deleteInvalidTasks(logger, queryable, invalidGuids...)
	}

	return tasks, len(invalidGuids), err
}

func (db *SQLDB) fetchTask(logger lager.Logger, scanner RowScanner, queryable Queryable) (*models.Task, error) {
	task, guid, err := db.fetchTaskInternal(logger, scanner)
	if err == models.ErrDeserialize {
		db.deleteInvalidTasks(logger, queryable, guid)
	}
	return task, err
}

func (db *SQLDB) fetchTaskInternal(logger lager.Logger, scanner RowScanner) (*models.Task, string, error) {
	var guid, domain, cellID, failureReason string
	var result sql.NullString
	var createdAt, updatedAt, firstCompletedAt int64
	var state int32
	var failed bool
	var taskDefData []byte

	err := scanner.Scan(
		&guid,
		&domain,
		&updatedAt,
		&createdAt,
		&firstCompletedAt,
		&state,
		&cellID,
		&result,
		&failed,
		&failureReason,
		&taskDefData,
	)

	if err == sql.ErrNoRows {
		return nil, "", err
	}

	if err != nil {
		logger.Error("failed-scanning-row", err)
		return nil, "", err
	}

	var taskDef models.TaskDefinition
	err = db.deserializeModel(logger, taskDefData, &taskDef)
	if err != nil {
		return nil, guid, models.ErrDeserialize
	}

	task := &models.Task{
		TaskGuid:         guid,
		Domain:           domain,
		CreatedAt:        createdAt,
		UpdatedAt:        updatedAt,
		FirstCompletedAt: firstCompletedAt,
		State:            models.Task_State(state),
		CellId:           cellID,
		Result:           result.String,
		Failed:           failed,
		FailureReason:    failureReason,
		TaskDefinition:   &taskDef,
	}
	return task, guid, nil
}

func (db *SQLDB) deleteInvalidTasks(logger lager.Logger, queryable Queryable, guids ...string) error {
	for _, guid := range guids {
		logger.Info("deleting-invalid-task-from-db", lager.Data{"guid": guid})
		_, err := db.delete(logger, queryable, tasksTable, "guid = ?", guid)
		if err != nil {
			logger.Error("failed-deleting-task", err)
			return err
		}
	}
	return nil
}
