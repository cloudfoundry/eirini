package utils

import (
	"code.cloudfoundry.org/eirini/api"
	corev1 "k8s.io/api/core/v1"
)

func GetPodState(pod corev1.Pod) string {
	if len(pod.Status.ContainerStatuses) == 0 || pod.Status.Phase == corev1.PodUnknown {
		return api.UnknownState
	}

	if podPending(&pod) {
		if containersHaveBrokenImage(pod.Status.ContainerStatuses) {
			return api.CrashedState
		}

		return api.PendingState
	}

	if podFailed(&pod) {
		return api.CrashedState
	}

	if podRunning(&pod) {
		if containersReady(pod.Status.ContainerStatuses) {
			return api.RunningState
		}

		if containersRunning(pod.Status.ContainerStatuses) {
			return api.PendingState
		}
	}

	if containersFailed(pod.Status.ContainerStatuses) {
		return api.CrashedState
	}

	return api.UnknownState
}

func podPending(pod *corev1.Pod) bool {
	return pod.Status.Phase == corev1.PodPending
}

func podFailed(pod *corev1.Pod) bool {
	return pod.Status.Phase == corev1.PodFailed
}

func podRunning(pod *corev1.Pod) bool {
	return pod.Status.Phase == corev1.PodRunning
}

func containersHaveBrokenImage(statuses []corev1.ContainerStatus) bool {
	for _, status := range statuses {
		if status.State.Waiting == nil {
			continue
		}

		if status.State.Waiting.Reason == "ErrImagePull" || status.State.Waiting.Reason == "ImagePullBackOff" {
			return true
		}
	}

	return false
}

func containersFailed(statuses []corev1.ContainerStatus) bool {
	for _, status := range statuses {
		if status.State.Waiting != nil || status.State.Terminated != nil {
			return true
		}
	}

	return false
}

func containersReady(statuses []corev1.ContainerStatus) bool {
	for _, status := range statuses {
		if status.State.Running == nil || !status.Ready {
			return false
		}
	}

	return true
}

func containersRunning(statuses []corev1.ContainerStatus) bool {
	for _, status := range statuses {
		if status.State.Running == nil {
			return false
		}
	}

	return true
}
