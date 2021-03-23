package bifrost

import (
	"context"

	"code.cloudfoundry.org/eirini/k8s/shared"
	"code.cloudfoundry.org/eirini/models/cf"
	"code.cloudfoundry.org/eirini/opi"
	"github.com/pkg/errors"
)

//counterfeiter:generate . TaskConverter
//counterfeiter:generate . TaskClient
//counterfeiter:generate . JSONClient
//counterfeiter:generate . TaskNamespacer

type TaskConverter interface {
	ConvertTask(taskGUID string, request cf.TaskRequest) (opi.Task, error)
}

type TaskClient interface {
	Desire(ctx context.Context, namespace string, task *opi.Task, opts ...shared.Option) error
	Get(ctx context.Context, guid string) (*opi.Task, error)
	List(ctx context.Context) ([]*opi.Task, error)
	Delete(ctx context.Context, guid string) (string, error)
}

type JSONClient interface {
	Post(ctx context.Context, url string, data interface{}) error
}

type TaskNamespacer interface {
	GetNamespace(requestedNamespace string) string
}

type Task struct {
	Namespacer TaskNamespacer
	Converter  TaskConverter
	TaskClient TaskClient
	JSONClient JSONClient
}

func (t *Task) GetTask(ctx context.Context, taskGUID string) (cf.TaskResponse, error) {
	task, err := t.TaskClient.Get(ctx, taskGUID)
	if err != nil {
		return cf.TaskResponse{}, errors.Wrap(err, "failed to get task")
	}

	return cf.TaskResponse{GUID: task.GUID}, nil
}

func (t *Task) ListTasks(ctx context.Context) (cf.TasksResponse, error) {
	tasks, err := t.TaskClient.List(ctx)
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

	return errors.Wrap(t.TaskClient.Desire(ctx, namespace, &desiredTask), "failed to desire")
}

func (t *Task) CancelTask(ctx context.Context, taskGUID string) error {
	callbackURL, err := t.TaskClient.Delete(ctx, taskGUID)
	if err != nil {
		return errors.Wrapf(err, "failed to delete task %s", taskGUID)
	}

	if len(callbackURL) == 0 {
		return nil
	}

	go func() {
		_ = t.JSONClient.Post(ctx, callbackURL, cf.TaskCompletedRequest{
			TaskGUID:      taskGUID,
			Failed:        true,
			FailureReason: "task was cancelled",
		})
	}()

	return nil
}
