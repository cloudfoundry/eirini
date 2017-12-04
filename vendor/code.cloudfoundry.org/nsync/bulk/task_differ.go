package bulk

import (
	"code.cloudfoundry.org/bbs/models"
	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/runtimeschema/cc_messages"
)

type TaskDiffer interface {
	Diff(lager.Logger, <-chan []cc_messages.CCTaskState, <-chan struct{})
	TasksToFail() <-chan []cc_messages.CCTaskState
	TasksToCancel() <-chan []string
}

type taskDiffer struct {
	bbsTasks      map[string]*models.Task
	tasksToFail   chan []cc_messages.CCTaskState
	tasksToCancel chan []string
}

func NewTaskDiffer(bbsTasks map[string]*models.Task) TaskDiffer {
	return &taskDiffer{
		bbsTasks:      bbsTasks,
		tasksToFail:   make(chan []cc_messages.CCTaskState, 1),
		tasksToCancel: make(chan []string, 1),
	}
}

func (t *taskDiffer) Diff(logger lager.Logger, ccTasks <-chan []cc_messages.CCTaskState, cancelCh <-chan struct{}) {
	logger = logger.Session("task_diff")

	tasksToCancel := cloneBbsTasks(t.bbsTasks)

	go func() {
		defer func() {
			close(t.tasksToFail)
			close(t.tasksToCancel)
		}()

		for {
			select {
			case <-cancelCh:
				return

			case batchCCTasks, open := <-ccTasks:
				if !open {
					guids := filterTasksToCancel(logger, tasksToCancel)
					if len(guids) > 0 {
						t.tasksToCancel <- guids
					}

					return
				}

				batchTasksToFail := []cc_messages.CCTaskState{}
				for _, ccTask := range batchCCTasks {

					_, exists := t.bbsTasks[ccTask.TaskGuid]

					if exists {
						if ccTask.State != cc_messages.TaskStateCanceling {
							delete(tasksToCancel, ccTask.TaskGuid)
						}
					} else {
						if ccTask.State == cc_messages.TaskStateRunning || ccTask.State == cc_messages.TaskStateCanceling {
							batchTasksToFail = append(batchTasksToFail, ccTask)

							logger.Info("found-task-to-fail", lager.Data{
								"guid": ccTask.TaskGuid,
							})
						}
					}
				}

				if len(batchTasksToFail) > 0 {
					t.tasksToFail <- batchTasksToFail
				}
			}
		}
	}()
}

func (t *taskDiffer) TasksToFail() <-chan []cc_messages.CCTaskState {
	return t.tasksToFail
}

func (t *taskDiffer) TasksToCancel() <-chan []string {
	return t.tasksToCancel
}

func cloneBbsTasks(bbsTasks map[string]*models.Task) map[string]*models.Task {
	clone := map[string]*models.Task{}
	for k, v := range bbsTasks {
		clone[k] = v
	}
	return clone
}

func filterTasksToCancel(logger lager.Logger, tasksToCancel map[string]*models.Task) []string {
	guids := make([]string, 0, len(tasksToCancel))
	for _, bbsTask := range tasksToCancel {
		if bbsTask.State == models.Task_Completed || bbsTask.State == models.Task_Resolving {
			continue
		}
		guids = append(guids, bbsTask.TaskGuid)

		logger.Info("found-task-to-cancel", lager.Data{
			"guid": bbsTask.TaskGuid,
		})
	}
	return guids
}
