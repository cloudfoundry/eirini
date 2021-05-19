package reconciler

import (
	"context"
	"fmt"

	"code.cloudfoundry.org/eirini/api"
	"code.cloudfoundry.org/eirini/k8s/shared"
	eiriniv1 "code.cloudfoundry.org/eirini/pkg/apis/eirini/v1"
	"code.cloudfoundry.org/eirini/util"
	"code.cloudfoundry.org/lager"
	exterrors "github.com/pkg/errors"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

type Task struct {
	taskCrClient   TasksCrClient
	workloadClient TaskWorkloadClient
	scheme         *runtime.Scheme
	logger         lager.Logger
}

//counterfeiter:generate . TasksCrClient

type TasksCrClient interface {
	UpdateTaskStatus(context.Context, *eiriniv1.Task, eiriniv1.TaskStatus) error
	GetTask(context.Context, string, string) (*eiriniv1.Task, error)
}

//counterfeiter:generate . TaskWorkloadClient

type TaskWorkloadClient interface {
	Desire(ctx context.Context, namespace string, task *api.Task, opts ...shared.Option) error
	GetStatus(ctx context.Context, taskGUID string) (eiriniv1.TaskStatus, error)
}

func NewTask(logger lager.Logger, taskCrClient TasksCrClient, workloadClient TaskWorkloadClient, scheme *runtime.Scheme) *Task {
	return &Task{
		taskCrClient:   taskCrClient,
		workloadClient: workloadClient,
		scheme:         scheme,
		logger:         logger,
	}
}

func (t *Task) Reconcile(ctx context.Context, request reconcile.Request) (reconcile.Result, error) {
	logger := t.logger.Session("reconcile-task", lager.Data{"request": request})
	logger.Debug("start")

	task, err := t.taskCrClient.GetTask(ctx, request.NamespacedName.Namespace, request.NamespacedName.Name)
	if errors.IsNotFound(err) {
		logger.Debug("task-not-found")

		return reconcile.Result{}, nil
	}

	if err != nil {
		logger.Error("task-get-failed", err)

		return reconcile.Result{}, fmt.Errorf("could not fetch task: %w", err)
	}

	if taskHasCompleted(task) {
		return reconcile.Result{}, nil
	}

	err = t.workloadClient.Desire(ctx, task.Namespace, toAPITask(task), t.setOwnerFn(task))
	if err != nil && !errors.IsAlreadyExists(err) {
		logger.Error("desire-task-failed", err)

		return reconcile.Result{}, exterrors.Wrap(err, "failed to desire task")
	}

	status, err := t.workloadClient.GetStatus(ctx, task.Spec.GUID)
	if err != nil {
		logger.Error("failed-to-get-task-status", err)

		return reconcile.Result{}, exterrors.Wrap(err, "failed to get task status")
	}

	if err := t.taskCrClient.UpdateTaskStatus(ctx, task, status); err != nil {
		return reconcile.Result{}, exterrors.Wrap(err, "failed to update task status")
	}

	return reconcile.Result{}, nil
}

func (t *Task) setOwnerFn(task *eiriniv1.Task) func(interface{}) error {
	return func(resource interface{}) error {
		obj, ok := resource.(metav1.Object)
		if !ok {
			return fmt.Errorf("could not cast %v to metav1.Object", resource)
		}

		if err := ctrl.SetControllerReference(task, obj, t.scheme); err != nil {
			return exterrors.Wrap(err, "failed to set controller reference")
		}

		return nil
	}
}

func taskHasCompleted(task *eiriniv1.Task) bool {
	return task.Status.EndTime != nil &&
		(task.Status.ExecutionStatus == eiriniv1.TaskFailed ||
			task.Status.ExecutionStatus == eiriniv1.TaskSucceeded)
}

func toAPITask(task *eiriniv1.Task) *api.Task {
	apiTask := &api.Task{
		GUID:      task.Spec.GUID,
		Name:      task.Spec.Name,
		Image:     task.Spec.Image,
		Env:       task.Spec.Env,
		Command:   task.Spec.Command,
		AppName:   task.Spec.AppName,
		AppGUID:   task.Spec.AppGUID,
		OrgName:   task.Spec.OrgName,
		OrgGUID:   task.Spec.OrgGUID,
		SpaceName: task.Spec.SpaceName,
		SpaceGUID: task.Spec.SpaceGUID,
		MemoryMB:  task.Spec.MemoryMB,
		DiskMB:    task.Spec.DiskMB,
		CPUWeight: task.Spec.CPUWeight,
	}

	if task.Spec.PrivateRegistry != nil {
		apiTask.PrivateRegistry = &api.PrivateRegistry{
			Username: task.Spec.PrivateRegistry.Username,
			Password: task.Spec.PrivateRegistry.Password,
			Server:   util.ParseImageRegistryHost(task.Spec.Image),
		}
	}

	return apiTask
}
