package utils

import (
	"code.cloudfoundry.org/eirini/opi"
	corev1 "k8s.io/api/core/v1"
)

func GetPodState(pod corev1.Pod) string {
	if statusNotAvailable(&pod) || pod.Status.Phase == corev1.PodUnknown {
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

func statusNotAvailable(pod *corev1.Pod) bool {
	return pod.Status.ContainerStatuses == nil || len(pod.Status.ContainerStatuses) == 0
}

func podPending(pod *corev1.Pod) bool {
	status := pod.Status.ContainerStatuses[0]
	return pod.Status.Phase == corev1.PodPending || (status.State.Running != nil && !status.Ready)
}

func podCrashed(status corev1.ContainerStatus) bool {
	return status.State.Waiting != nil || status.State.Terminated != nil
}

func podRunning(status corev1.ContainerStatus) bool {
	return status.State.Running != nil && status.Ready
}
