package task

import (
	"net/http"

	"code.cloudfoundry.org/eirini/k8s"
	"code.cloudfoundry.org/eirini/k8s/utils"
	"code.cloudfoundry.org/eirini/models/cf"
	"code.cloudfoundry.org/lager"
	corev1 "k8s.io/api/core/v1"
)

//counterfeiter:generate . Deleter

type Deleter interface {
	Delete(guid string) (string, error)
}

type StateReporter struct {
	Client      *http.Client
	Logger      lager.Logger
	TaskDeleter Deleter
}

func (r StateReporter) Report(pod *corev1.Pod) error {
	taskGUID := pod.Annotations[k8s.AnnotationGUID]
	uri := pod.Annotations[k8s.AnnotationCompletionCallback]

	logger := r.Logger.Session("report", lager.Data{"task-guid": taskGUID})

	if !r.taskContainerHasTerminated(logger, pod) {
		return nil
	}

	logger.Debug("sending completion notification")
	req := r.generateTaskCompletedRequest(logger, taskGUID, pod)

	if err := utils.Post(r.Client, uri, req); err != nil {
		logger.Error("cannot-send-task-status-response", err)

		return err
	}

	if _, err := r.TaskDeleter.Delete(taskGUID); err != nil {
		logger.Error("cannot-delete-job", err)

		return err
	}

	return nil
}

func (r StateReporter) taskContainerHasTerminated(logger lager.Logger, pod *corev1.Pod) bool {
	status, ok := getTaskContainerStatus(pod)
	if !ok {
		logger.Info("pod-has-no-task-container-status")

		return false
	}

	return isTerminatedStatus(status)
}

func isTerminatedStatus(status corev1.ContainerStatus) bool {
	return status.State.Terminated != nil
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
	taskContainerName := pod.Annotations[k8s.AnnotationOpiTaskContainerName]
	for _, status := range pod.Status.ContainerStatuses {
		if status.Name == taskContainerName {
			return status, true
		}
	}

	return corev1.ContainerStatus{}, false
}
