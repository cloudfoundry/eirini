package crclient

import (
	"context"

	eiriniv1 "code.cloudfoundry.org/eirini/pkg/apis/eirini/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type Tasks struct {
	controllerClient client.Client
}

func NewTasks(controllerClient client.Client) *Tasks {
	return &Tasks{controllerClient: controllerClient}
}

func (t *Tasks) GetTask(ctx context.Context, namespace, name string) (*eiriniv1.Task, error) {
	task := &eiriniv1.Task{}
	err := t.controllerClient.Get(ctx, types.NamespacedName{
		Namespace: namespace,
		Name:      name,
	}, task)

	return task, err
}

func (t *Tasks) UpdateTaskStatus(ctx context.Context, task *eiriniv1.Task, newStatus eiriniv1.TaskStatus) error {
	newTask := task.DeepCopy()
	newTask.Status = newStatus

	return t.controllerClient.Status().Patch(ctx, newTask, client.MergeFrom(task))
}
