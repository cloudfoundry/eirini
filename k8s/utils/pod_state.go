package utils

import (
	"code.cloudfoundry.org/eirini/opi"
	corev1 "k8s.io/api/core/v1"
)

func GetPodState(pod corev1.Pod) string {
	if len(pod.Status.ContainerStatuses) == 0 || pod.Status.Phase == corev1.PodUnknown {
		return opi.UnknownState
	}

	if podPending(&pod) {
		if brokenImage(&pod) {
			return opi.CrashedState
		}
		return opi.PendingState
	}

	if podNotReady(&pod) {
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

func brokenImage(pod *corev1.Pod) bool {
	status := pod.Status.ContainerStatuses[0]
	return status.State.Waiting.Reason == "ErrImagePull" || status.State.Waiting.Reason == "ImagePullBackOff"
}

func podPending(pod *corev1.Pod) bool {
	return pod.Status.Phase == corev1.PodPending
}
func podNotReady(pod *corev1.Pod) bool {
	status := pod.Status.ContainerStatuses[0]
	return (status.State.Running != nil && !status.Ready)
}

func podCrashed(status corev1.ContainerStatus) bool {
	return status.State.Waiting != nil || status.State.Terminated != nil
}

func podRunning(status corev1.ContainerStatus) bool {
	return status.State.Running != nil && status.Ready
}
