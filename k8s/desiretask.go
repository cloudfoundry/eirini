package k8s

import (
	"context"

	"github.com/julz/cube"
	"github.com/julz/cube/opi"
	"k8s.io/api/core/v1"
	av1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	batch "k8s.io/api/batch/v1"
	"k8s.io/client-go/kubernetes"
)

type JobConfig struct {
	ActiveDeadlineSeconds int64
	RestartPolicy         string
	Namespace             string
}

type TaskDesirer struct {
	Config JobConfig
	Client *kubernetes.Clientset
}

func (d TaskDesirer) Desire(ctx context.Context, tasks []opi.Task) error {
	namespace := d.Config.Namespace
	jobs, err := d.Client.BatchV1().Jobs(namespace).List(av1.ListOptions{})
	if err != nil {
		return err
	}

	dByName := make(map[string]struct{})
	for _, d := range jobs.Items {
		dByName[d.Name] = struct{}{}
	}

	for _, task := range tasks {
		if _, err := d.Client.BatchV1().Jobs(namespace).Create(toJob(task)); err != nil {
			// fixme: this should be a multi-error and deferred
			return err
		}
	}

	return nil
}

func (d *TaskDesirer) DeleteJob(job string) error {
	namespace := d.Config.Namespace
	return d.Client.BatchV1().Jobs(namespace).Delete(job, nil)
}

func toJob(task opi.Task) *batch.Job {
	job := &batch.Job{
		Spec: batch.JobSpec{
			ActiveDeadlineSeconds: int64ptr(600),
			Template: v1.PodTemplateSpec{
				Spec: v1.PodSpec{
					Containers: []v1.Container{{
						Name:  "opi-task",
						Image: task.Image,
						Env:   mapToEnvVar(task.Env),
						//ImagePullPolicy: "Always",
					}},
					RestartPolicy: "Never",
				},
			},
		},
	}

	job.Name = task.Env[cube.EnvAppId]

	job.Spec.Template.Labels = map[string]string{
		"name": task.Env[cube.EnvAppId],
	}

	job.Labels = map[string]string{
		"cube": "cube",
		"name": task.Env[cube.EnvAppId],
	}
	return job
}

func mapToEnvVar(env map[string]string) []v1.EnvVar {
	envVars := []v1.EnvVar{}
	for k, v := range env {
		envVar := v1.EnvVar{Name: k, Value: v}
		envVars = append(envVars, envVar)
	}
	return envVars
}
