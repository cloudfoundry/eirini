package utils

import (
	"code.cloudfoundry.org/eirini/opi"
	"k8s.io/api/core/v1"
)

func GetPodState(pod v1.Pod) string {
	if statusNotAvailable(&pod) || pod.Status.Phase == v1.PodUnknown {
		return opi.UnknownState
	}

	if podPending(&pod) {
		return opi.PendingState
	}

	if podCrashed(pod.Status.ContainerStatuses[0]) {
		return opi.CrashedState
	}

	if podRunning(pod.Status.ContainerStatuses[0]) {
		return opi.RunningState
	}

	return opi.UnknownState
}

func statusNotAvailable(pod *v1.Pod) bool {
	return pod.Status.ContainerStatuses == nil || len(pod.Status.ContainerStatuses) == 0
}

func podPending(pod *v1.Pod) bool {
	status := pod.Status.ContainerStatuses[0]
	return pod.Status.Phase == v1.PodPending || (status.State.Running != nil && !status.Ready)
}

func podCrashed(status v1.ContainerStatus) bool {
	return status.State.Waiting != nil || status.State.Terminated != nil
}

func podRunning(status v1.ContainerStatus) bool {
	return status.State.Running != nil && status.Ready
}
