package reconciler

import (
	"context"
	"fmt"

	"code.cloudfoundry.org/eirini/k8s/shared"
	"code.cloudfoundry.org/eirini/opi"
	eiriniv1 "code.cloudfoundry.org/eirini/pkg/apis/eirini/v1"
	"code.cloudfoundry.org/eirini/util"
	"code.cloudfoundry.org/lager"
	exterrors "github.com/pkg/errors"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

//counterfeiter:generate . TaskDesirer

type Task struct {
	client      client.Client
	taskDesirer TaskDesirer
	scheme      *runtime.Scheme
	logger      lager.Logger
}

func NewTask(logger lager.Logger, client client.Client, taskDesirer TaskDesirer, scheme *runtime.Scheme) *Task {
	return &Task{
		client:      client,
		taskDesirer: taskDesirer,
		scheme:      scheme,
		logger:      logger,
	}
}

type TaskDesirer interface {
	Desire(ctx context.Context, namespace string, task *opi.Task, opts ...shared.Option) error
}

func (t *Task) Reconcile(ctx context.Context, request reconcile.Request) (reconcile.Result, error) {
	task := &eiriniv1.Task{}
	logger := t.logger.Session("reconcile-task", lager.Data{"request": request})
	logger.Debug("start")

	err := t.client.Get(ctx, request.NamespacedName, task)
	if errors.IsNotFound(err) {
		logger.Error("no-such-task", err)

		return reconcile.Result{}, nil
	}

	if err != nil {
		logger.Error("task-get-failed", err)

		return reconcile.Result{}, fmt.Errorf("could not fetch task: %w", err)
	}

	err = t.taskDesirer.Desire(ctx, task.Namespace, toOpiTask(task), t.setOwnerFn(task))
	if errors.IsAlreadyExists(err) {
		logger.Info("task-already-exists")

		return reconcile.Result{}, nil
	}

	if err != nil {
		logger.Error("desire-task-failed", err)

		return reconcile.Result{}, exterrors.Wrap(err, "failed to desire task")
	}

	logger.Debug("task-desired-successfully")

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

func toOpiTask(task *eiriniv1.Task) *opi.Task {
	opiTask := &opi.Task{
		GUID:               task.Spec.GUID,
		Name:               task.Spec.Name,
		Image:              task.Spec.Image,
		CompletionCallback: task.Spec.CompletionCallback,
		Env:                task.Spec.Env,
		Command:            task.Spec.Command,
		AppName:            task.Spec.AppName,
		AppGUID:            task.Spec.AppGUID,
		OrgName:            task.Spec.OrgName,
		OrgGUID:            task.Spec.OrgGUID,
		SpaceName:          task.Spec.SpaceName,
		SpaceGUID:          task.Spec.SpaceGUID,
		MemoryMB:           task.Spec.MemoryMB,
		DiskMB:             task.Spec.DiskMB,
		CPUWeight:          task.Spec.CPUWeight,
	}

	if task.Spec.PrivateRegistry != nil {
		opiTask.PrivateRegistry = &opi.PrivateRegistry{
			Username: task.Spec.PrivateRegistry.Username,
			Password: task.Spec.PrivateRegistry.Password,
			Server:   util.ParseImageRegistryHost(task.Spec.Image),
		}
	}

	return opiTask
}
