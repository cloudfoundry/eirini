package task

import (
	"net/http"

	"code.cloudfoundry.org/eirini/k8s"
	"code.cloudfoundry.org/eirini/k8s/utils"
	"code.cloudfoundry.org/eirini/models/cf"
	"code.cloudfoundry.org/lager"
	batchv1 "k8s.io/api/batch/v1"
)

//counterfeiter:generate . Deleter

type Deleter interface {
	Delete(guid string) error
}

type StateReporter struct {
	Client      *http.Client
	Logger      lager.Logger
	TaskDeleter Deleter
}

func (r StateReporter) Report(job *batchv1.Job) {
	if len(job.Status.Conditions) != 0 {
		taskGUID := job.Labels[k8s.LabelGUID]

		uri := job.Annotations[k8s.AnnotationCompletionCallback]
		req := r.generateTaskComletedRequest(taskGUID, job.Status.Conditions)

		if err := utils.Put(r.Client, uri, req); err != nil {
			r.Logger.Error("cannot send task status response", err, lager.Data{"taskGuid": taskGUID})
		}

		if err := r.TaskDeleter.Delete(taskGUID); err != nil {
			r.Logger.Error("cannot delete job", err, lager.Data{"taskGuid": taskGUID})
		}
	}
}

func (r StateReporter) generateTaskComletedRequest(guid string, conditions []batchv1.JobCondition) cf.TaskCompletedRequest {
	res := cf.TaskCompletedRequest{
		TaskGUID: guid,
	}

	lastCondition := conditions[len(conditions)-1]
	if lastCondition.Type == batchv1.JobFailed {
		res.Failed = true
		res.FailureReason = lastCondition.Reason
		r.Logger.Error("job failed", nil, lager.Data{
			"taskGuid":       guid,
			"failureReason":  lastCondition.Reason,
			"failureMessage": lastCondition.Message,
		})
	}

	return res
}
