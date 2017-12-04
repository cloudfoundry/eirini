package db

import (
	"time"

	"code.cloudfoundry.org/auctioneer"
	"code.cloudfoundry.org/bbs/models"
	"code.cloudfoundry.org/lager"
)

type CompleteTaskWork func(logger lager.Logger, taskDB TaskDB, task *models.Task) func()

//go:generate counterfeiter . TaskDB
type TaskDB interface {
	Tasks(logger lager.Logger, filter models.TaskFilter) ([]*models.Task, error)
	TaskByGuid(logger lager.Logger, taskGuid string) (*models.Task, error)

	DesireTask(logger lager.Logger, taskDefinition *models.TaskDefinition, taskGuid, domain string) (*models.Task, error)
	StartTask(logger lager.Logger, taskGuid, cellId string) (before *models.Task, after *models.Task, shouldStart bool, rr error)
	CancelTask(logger lager.Logger, taskGuid string) (before *models.Task, after *models.Task, cellID string, err error)
	FailTask(logger lager.Logger, taskGuid, failureReason string) (before *models.Task, after *models.Task, err error)
	CompleteTask(logger lager.Logger, taskGuid, cellId string, failed bool, failureReason, result string) (before *models.Task, after *models.Task, err error)
	ResolvingTask(logger lager.Logger, taskGuid string) (before *models.Task, after *models.Task, err error)
	DeleteTask(logger lager.Logger, taskGuid string) (task *models.Task, err error)

	ConvergeTasks(
		logger lager.Logger,
		cellSet models.CellSet,
		kickTaskDuration, expirePendingTaskDuration, expireCompletedTaskDuration time.Duration,
	) (tasksToAuction []*auctioneer.TaskStartRequest, tasksToComplete []*models.Task, taskEvents []models.Event)
}
