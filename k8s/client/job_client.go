package client

import (
	"context"
	"fmt"

	"code.cloudfoundry.org/eirini/k8s/jobs"
	"code.cloudfoundry.org/eirini/k8s/patching"
	"github.com/pkg/errors"
	batchv1 "k8s.io/api/batch/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

type Job struct {
	clientSet          kubernetes.Interface
	workloadsNamespace string
	jobType            string
	guidLabel          string
}

func NewJob(clientSet kubernetes.Interface, workloadsNamespace string) *Job {
	return &Job{
		clientSet:          clientSet,
		workloadsNamespace: workloadsNamespace,
		jobType:            "TASK",
		guidLabel:          jobs.LabelGUID,
	}
}

func (c *Job) Create(ctx context.Context, namespace string, job *batchv1.Job) (*batchv1.Job, error) {
	ctx, cancel := context.WithTimeout(ctx, k8sTimeout)
	defer cancel()

	return c.clientSet.BatchV1().Jobs(namespace).Create(ctx, job, metav1.CreateOptions{})
}

func (c *Job) Delete(ctx context.Context, namespace string, name string) error {
	ctx, cancel := context.WithTimeout(ctx, k8sTimeout)
	defer cancel()

	backgroundPropagation := metav1.DeletePropagationBackground
	deleteOpts := metav1.DeleteOptions{
		PropagationPolicy: &backgroundPropagation,
	}

	return c.clientSet.BatchV1().Jobs(namespace).Delete(ctx, name, deleteOpts)
}

func (c *Job) GetByGUID(ctx context.Context, guid string, includeCompleted bool) ([]batchv1.Job, error) {
	ctx, cancel := context.WithTimeout(ctx, k8sTimeout)
	defer cancel()

	labelSelector := fmt.Sprintf("%s=%s", jobs.LabelGUID, guid)

	if !includeCompleted {
		labelSelector += fmt.Sprintf(",%s!=%s", jobs.LabelTaskCompleted, jobs.TaskCompletedTrue)
	}

	listOpts := metav1.ListOptions{LabelSelector: labelSelector}
	jobs, err := c.clientSet.BatchV1().Jobs(c.workloadsNamespace).List(ctx, listOpts)

	return jobs.Items, errors.Wrap(err, "failed to list jobs by guid")
}

func (c *Job) List(ctx context.Context, includeCompleted bool) ([]batchv1.Job, error) {
	ctx, cancel := context.WithTimeout(ctx, k8sTimeout)
	defer cancel()

	labelSelector := fmt.Sprintf("%s=%s", jobs.LabelSourceType, c.jobType)

	if !includeCompleted {
		labelSelector += fmt.Sprintf(",%s!=%s", jobs.LabelTaskCompleted, jobs.TaskCompletedTrue)
	}

	listOpts := metav1.ListOptions{LabelSelector: labelSelector}
	jobs, err := c.clientSet.BatchV1().Jobs(c.workloadsNamespace).List(ctx, listOpts)

	return jobs.Items, errors.Wrap(err, "failed to list jobs")
}

func (c *Job) SetLabel(ctx context.Context, job *batchv1.Job, label, value string) (*batchv1.Job, error) {
	ctx, cancel := context.WithTimeout(ctx, k8sTimeout)
	defer cancel()

	labelPatch := patching.NewLabel(label, value)

	return c.clientSet.BatchV1().Jobs(job.Namespace).Patch(
		ctx,
		job.Name,
		labelPatch.Type(),
		labelPatch.GetPatchBytes(),
		metav1.PatchOptions{})
}
