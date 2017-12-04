package bulk

import (
	"net/http"
	"os"
	"time"

	"code.cloudfoundry.org/bbs"
	"code.cloudfoundry.org/bbs/models"
	"code.cloudfoundry.org/clock"
	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/runtimeschema/cc_messages"
	"code.cloudfoundry.org/workpool"
)

type TaskProcessor struct {
	bbsClient          bbs.Client
	taskClient         TaskClient
	pollingInterval    time.Duration
	domainTTL          time.Duration
	failTaskPoolSize   int
	cancelTaskPoolSize int
	httpClient         *http.Client
	logger             lager.Logger
	fetcher            Fetcher
	clock              clock.Clock
}

func NewTaskProcessor(
	logger lager.Logger,
	bbsClient bbs.Client,
	taskClient TaskClient,
	pollingInterval time.Duration,
	domainTTL time.Duration,
	failTaskPoolSize int,
	cancelTaskPoolSize int,
	skipCertVerify bool,
	fetcher Fetcher,
	clock clock.Clock) *TaskProcessor {
	return &TaskProcessor{
		bbsClient:          bbsClient,
		taskClient:         taskClient,
		pollingInterval:    pollingInterval,
		domainTTL:          domainTTL,
		failTaskPoolSize:   failTaskPoolSize,
		cancelTaskPoolSize: cancelTaskPoolSize,
		httpClient:         initializeHttpClient(skipCertVerify),
		logger:             logger,
		fetcher:            fetcher,
		clock:              clock,
	}
}

func (t *TaskProcessor) Run(signals <-chan os.Signal, ready chan<- struct{}) error {
	close(ready)

	timer := t.clock.NewTimer(t.pollingInterval)
	stop := t.sync(signals)

	for {
		if stop {
			return nil
		}

		select {
		case <-signals:
			return nil
		case <-timer.C():
			stop = t.sync(signals)
			timer.Reset(t.pollingInterval)
		}
	}
}

func (t *TaskProcessor) sync(signals <-chan os.Signal) bool {
	logger := t.logger.Session("sync")
	logger.Info("starting")

	existingTasks, err := t.existingTasksMap()
	if err != nil {
		return false
	}

	cancelCh := make(chan struct{})

	taskStateCh, taskStateErrorCh := t.fetcher.FetchTaskStates(
		logger,
		cancelCh,
		t.httpClient,
	)

	taskDiffer := NewTaskDiffer(existingTasks)
	taskDiffer.Diff(logger, taskStateCh, cancelCh)

	failTaskErrorCh := t.failTasks(logger, taskDiffer.TasksToFail())
	cancelTaskErrorCh := t.cancelTasks(logger, taskDiffer.TasksToCancel())

	taskStateErrorCh, taskStateErrorCount := countErrors(taskStateErrorCh)

	errors := mergeErrors(
		taskStateErrorCh,
		failTaskErrorCh,
		cancelTaskErrorCh,
	)

	bumpFreshness := true
	logger.Info("processing-updates-and-creates")
process_loop:
	for {
		select {
		case err, open := <-errors:
			if err != nil {
				bumpFreshness = false
				logger.Error("not-bumping-freshness-because-of", err)
			}
			if !open {
				break process_loop
			}
		case sig := <-signals:
			logger.Info("exiting", lager.Data{"received-signal": sig})
			close(cancelCh)
			return true
		}
	}
	logger.Info("done-processing-updates-and-creates")

	if <-taskStateErrorCount != 0 {
		logger.Error("failed-to-fetch-all-cc-task-states", nil)
	}

	if bumpFreshness {
		t.bbsClient.UpsertDomain(logger, cc_messages.RunningTaskDomain, t.domainTTL)
		logger.Info("bumpin-freshness")
	}

	return false
}

func (t *TaskProcessor) existingTasksMap() (map[string]*models.Task, error) {
	logger := t.logger.Session("exiting-task-map")
	existingTasks, err := t.bbsClient.TasksByDomain(logger, cc_messages.RunningTaskDomain)
	if err != nil {
		return nil, err
	}

	existingTasksMap := make(map[string]*models.Task, len(existingTasks))

	for _, existingTask := range existingTasks {
		existingTasksMap[existingTask.TaskGuid] = existingTask
	}

	return existingTasksMap, nil
}

func (t *TaskProcessor) failTasks(
	logger lager.Logger,
	tasksCh <-chan []cc_messages.CCTaskState,
) <-chan error {

	logger = logger.Session("fail-mismatched-tasks")
	errc := make(chan error, 1)

	go func() {
		defer close(errc)

		for {
			var tasksToFail []cc_messages.CCTaskState

			select {
			case selected, open := <-tasksCh:
				if !open {
					return
				}

				tasksToFail = selected
			}

			works := make([]func(), len(tasksToFail))

			for i, taskState := range tasksToFail {
				taskState := taskState

				works[i] = func() {
					err := t.taskClient.FailTask(logger, &taskState, t.httpClient)
					if err != nil {
						logger.Error("failed-failing-mismatched-task", err)
						errc <- err
					} else {
						logger.Debug("succeeded-failing-mismatched-task", lager.Data{"task_guid": taskState.TaskGuid})
					}
				}
			}

			throttler, err := workpool.NewThrottler(t.failTaskPoolSize, works)
			if err != nil {
				errc <- err
				return
			}

			logger.Info("processing-batch", lager.Data{"size": len(tasksToFail)})
			throttler.Work()
			logger.Info("done-processing-batch", lager.Data{"size": len(tasksToFail)})
		}
	}()
	return errc
}

func (t *TaskProcessor) cancelTasks(logger lager.Logger, tasksCh <-chan []string) <-chan error {
	logger = logger.Session("cancel-mismatched-tasks")
	errc := make(chan error, 1)

	go func() {
		defer close(errc)

		for {
			var tasksToCancel []string

			select {
			case selected, open := <-tasksCh:
				if !open {
					return
				}

				tasksToCancel = selected
			}

			works := make([]func(), len(tasksToCancel))

			for i, taskGuid := range tasksToCancel {
				taskGuid := taskGuid

				works[i] = func() {
					err := t.bbsClient.CancelTask(logger, taskGuid)
					if err != nil {
						logger.Error("failed-canceling-mismatched-task", err)
						errc <- err
					} else {
						logger.Debug("succeeded-canceling-mismatched-task", lager.Data{"task_guid": taskGuid})
					}
				}
			}

			throttler, err := workpool.NewThrottler(t.cancelTaskPoolSize, works)
			if err != nil {
				errc <- err
				return
			}

			logger.Info("processing-batch", lager.Data{"size": len(tasksToCancel)})
			throttler.Work()
			logger.Info("done-processing-batch", lager.Data{"size": len(tasksToCancel)})
		}
	}()
	return errc
}
