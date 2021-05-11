package jobs

import (
	"context"

	eiriniv1 "code.cloudfoundry.org/eirini/pkg/apis/eirini/v1"
	"code.cloudfoundry.org/lager"
	"github.com/pkg/errors"
	batchv1 "k8s.io/api/batch/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type StatusGetter struct {
	logger    lager.Logger
	jobGetter JobGetter
}

func NewStatusGetter(
	logger lager.Logger,
	jobGetter JobGetter,
) StatusGetter {
	return StatusGetter{
		logger:    logger,
		jobGetter: jobGetter,
	}
}

func (s *StatusGetter) GetStatus(ctx context.Context, taskGUID string) (eiriniv1.TaskStatus, error) {
	jobs, err := s.jobGetter.GetByGUID(ctx, taskGUID, true)
	if err != nil {
		return eiriniv1.TaskStatus{}, errors.Wrapf(err, "failed to get status for task with GUID %q", taskGUID)
	}

	job, err := getSingleJob(jobs)
	if err != nil {
		return eiriniv1.TaskStatus{}, errors.Wrapf(err, "failed to get status for task with GUID %q", taskGUID)
	}

	if job.Status.StartTime == nil {
		return eiriniv1.TaskStatus{
			ExecutionStatus: eiriniv1.TaskStarting,
		}, nil
	}

	if job.Status.Succeeded > 0 && job.Status.CompletionTime != nil {
		return eiriniv1.TaskStatus{
			ExecutionStatus: eiriniv1.TaskSucceeded,
			StartTime:       job.Status.StartTime,
			EndTime:         job.Status.CompletionTime,
		}, nil
	}

	lastFailureTimestamp := getLastFailureTimestamp(job.Status)
	if job.Status.Failed > 0 && lastFailureTimestamp != nil {
		return eiriniv1.TaskStatus{
			ExecutionStatus: eiriniv1.TaskFailed,
			StartTime:       job.Status.StartTime,
			EndTime:         lastFailureTimestamp,
		}, nil
	}

	return eiriniv1.TaskStatus{
		ExecutionStatus: eiriniv1.TaskRunning,
		StartTime:       job.Status.StartTime,
	}, nil
}

func getLastFailureTimestamp(jobStatus batchv1.JobStatus) *metav1.Time {
	var lastFailure *metav1.Time

	for _, condition := range jobStatus.Conditions {
		condition := condition
		if condition.Type != batchv1.JobFailed {
			continue
		}

		if lastFailure == nil || condition.LastTransitionTime.After(lastFailure.Time) {
			lastFailure = &condition.LastTransitionTime
		}
	}

	return lastFailure
}
