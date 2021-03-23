package task

import (
	"context"
	"net/http"

	"code.cloudfoundry.org/eirini/k8s/jobs"
	"code.cloudfoundry.org/eirini/k8s/utils"
	"code.cloudfoundry.org/eirini/models/cf"
	"code.cloudfoundry.org/lager"
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
)

type StateReporter struct {
	Client *http.Client
	Logger lager.Logger
}

func (r StateReporter) Report(ctx context.Context, pod *corev1.Pod) error {
	taskGUID := pod.Annotations[jobs.AnnotationGUID]
	uri := pod.Annotations[jobs.AnnotationCompletionCallback]

	logger := r.Logger.Session("report", lager.Data{"task-guid": taskGUID})

	logger.Debug("sending completion notification")
	req := r.generateTaskCompletedRequest(logger, taskGUID, pod)

	if err := utils.Post(ctx, r.Client, uri, req); err != nil {
		logger.Error("cannot-send-task-status-response", err)

		return errors.Wrap(err, "failed to complete task")
	}

	return nil
}

func (r StateReporter) generateTaskCompletedRequest(logger lager.Logger, guid string, pod *corev1.Pod) cf.TaskCompletedRequest {
	res := cf.TaskCompletedRequest{
		TaskGUID: guid,
	}
	taskContainerStatus, _ := getTaskContainerStatus(pod)
	terminated := taskContainerStatus.State.Terminated

	if terminated.ExitCode != 0 {
		res.Failed = true
		res.FailureReason = terminated.Reason

		logger.Error("job-failed", nil, lager.Data{
			"failure-reason":  terminated.Reason,
			"failure-message": terminated.Message,
		})
	}

	return res
}

func getTaskContainerStatus(pod *corev1.Pod) (corev1.ContainerStatus, bool) {
	taskContainerName := pod.Annotations[jobs.AnnotationOpiTaskContainerName]
	for _, status := range pod.Status.ContainerStatuses {
		if status.Name == taskContainerName {
			return status, true
		}
	}

	return corev1.ContainerStatus{}, false
}
