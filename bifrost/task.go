package bifrost

import (
	"context"

	"code.cloudfoundry.org/eirini/k8s"
	"code.cloudfoundry.org/eirini/models/cf"
	"code.cloudfoundry.org/eirini/opi"
	"github.com/pkg/errors"
)

//counterfeiter:generate . TaskConverter
//counterfeiter:generate . TaskDesirer
//counterfeiter:generate . TaskDeleter
//counterfeiter:generate . JSONClient
//counterfeiter:generate . TaskNamespacer

type TaskConverter interface {
	ConvertTask(taskGUID string, request cf.TaskRequest) (opi.Task, error)
}

type TaskDesirer interface {
	Desire(namespace string, task *opi.Task, opts ...k8s.DesireOption) error
	Get(guid string) (*opi.Task, error)
	List() ([]*opi.Task, error)
}

type TaskDeleter interface {
	Delete(guid string) (string, error)
}

type JSONClient interface {
	Post(url string, data interface{}) error
}

type TaskNamespacer interface {
	GetNamespace(requestedNamespace string) string
}

type Task struct {
	Namespacer  TaskNamespacer
	Converter   TaskConverter
	TaskDesirer TaskDesirer
	TaskDeleter TaskDeleter
	JSONClient  JSONClient
}

func (t *Task) GetTask(taskGUID string) (cf.TaskResponse, error) {
	task, err := t.TaskDesirer.Get(taskGUID)
	if err != nil {
		return cf.TaskResponse{}, errors.Wrap(err, "failed to get task")
	}

	return cf.TaskResponse{GUID: task.GUID}, nil
}

func (t *Task) ListTasks() (cf.TasksResponse, error) {
	tasks, err := t.TaskDesirer.List()
	if err != nil {
		return nil, errors.Wrap(err, "failed to list tasks")
	}

	tasksResp := cf.TasksResponse{}
	for _, task := range tasks {
		tasksResp = append(tasksResp, cf.TaskResponse{GUID: task.GUID})
	}

	return tasksResp, nil
}

func (t *Task) TransferTask(ctx context.Context, taskGUID string, taskRequest cf.TaskRequest) error {
	desiredTask, err := t.Converter.ConvertTask(taskGUID, taskRequest)
	if err != nil {
		return errors.Wrap(err, "failed to convert task")
	}

	namespace := t.Namespacer.GetNamespace(taskRequest.Namespace)

	return errors.Wrap(t.TaskDesirer.Desire(namespace, &desiredTask), "failed to desire")
}

func (t *Task) CancelTask(taskGUID string) error {
	callbackURL, err := t.TaskDeleter.Delete(taskGUID)
	if err != nil {
		return errors.Wrapf(err, "failed to delete task %s", taskGUID)
	}

	if len(callbackURL) == 0 {
		return nil
	}

	go func() {
		_ = t.JSONClient.Post(callbackURL, cf.TaskCompletedRequest{
			TaskGUID:      taskGUID,
			Failed:        true,
			FailureReason: "task was cancelled",
		})
	}()

	return nil
}
