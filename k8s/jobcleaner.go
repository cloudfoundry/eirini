package k8s

import (
	"github.com/pkg/errors"
	batchv1 "k8s.io/api/batch/v1"
	meta "k8s.io/apimachinery/pkg/apis/meta/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

//go:generate counterfeiter . JobDeleterClient
type JobDeleterClient interface {
	List(opts metav1.ListOptions) (*batchv1.JobList, error)
	Delete(name string, options *metav1.DeleteOptions) error
}

//go:generate counterfeiter . Cleaner
type Cleaner interface {
	Clean(id string) error
}

type JobCleaner struct {
	Jobs JobDeleterClient
}

func (c JobCleaner) Clean(selector string) error {
	options := metav1.ListOptions{LabelSelector: selector}
	jobs, err := c.Jobs.List(options)
	if err != nil {
		return errors.Wrap(err, "failed to list jobs")
	}

	backgroundPropagation := metav1.DeletePropagationBackground
	for _, job := range jobs.Items {
		err = c.Jobs.Delete(job.Name, &meta.DeleteOptions{PropagationPolicy: &backgroundPropagation})
		if err != nil {
			return errors.Wrap(err, "failed to delete job")
		}
	}
	return nil
}
