package event

import (
	"code.cloudfoundry.org/eirini/events"
	"code.cloudfoundry.org/eirini/k8s"
	"code.cloudfoundry.org/eirini/util"
	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/runtimeschema/cc_messages"
	v1 "k8s.io/api/core/v1"
)

type DefaultCrashEventGenerator struct {
	eventLister k8s.EventLister
}

func NewDefaultCrashEventGenerator(eventLister k8s.EventLister) DefaultCrashEventGenerator {
	return DefaultCrashEventGenerator{
		eventLister: eventLister,
	}
}

func (g DefaultCrashEventGenerator) Generate(pod *v1.Pod, logger lager.Logger) (events.CrashEvent, bool) {
	statuses := pod.Status.ContainerStatuses
	if len(statuses) == 0 {
		return events.CrashEvent{}, false
	}

	_, err := util.ParseAppIndex(pod.Name)
	if err != nil {
		logger.Error("failed-to-parse-app-index", err, lager.Data{"pod-name": pod.Name, "guid": pod.Annotations[k8s.AnnotationProcessGUID]})

		return events.CrashEvent{}, false
	}

	if status := getTerminatedContainerStatusIfAny(pod.Status.ContainerStatuses); status != nil {
		return g.generateReportForTerminatedPod(pod, status, logger)
	}

	if container := getMisconfiguredContainerStatusIfAny(pod.Status.ContainerStatuses); container != nil {
		exitDescription := container.State.Waiting.Message

		return generateReport(pod, container.State.Waiting.Reason, 0, exitDescription, 0, int(container.RestartCount)), true
	}

	if container := getCrashedContainerStatusIfAny(pod.Status.ContainerStatuses); container != nil {
		exitStatus := int(container.LastTerminationState.Terminated.ExitCode)
		exitDescription := container.LastTerminationState.Terminated.Reason
		crashTimestamp := container.LastTerminationState.Terminated.StartedAt.Unix()
		return generateReport(pod, container.State.Waiting.Reason, exitStatus, exitDescription, crashTimestamp, int(container.RestartCount)), true
	}

	return events.CrashEvent{}, false
}

func (g DefaultCrashEventGenerator) generateReportForTerminatedPod(pod *v1.Pod, status *v1.ContainerStatus, logger lager.Logger) (events.CrashEvent, bool) {
	podEvents, err := k8s.GetEvents(g.eventLister, *pod)
	if err != nil {
		logger.Error("failed-to-get-k8s-events", err, lager.Data{"guid": pod.Annotations[k8s.AnnotationProcessGUID]})

		return events.CrashEvent{}, false
	}

	if k8s.IsStopped(podEvents) {
		return events.CrashEvent{}, false
	}

	terminated := status.State.Terminated

	return generateReport(pod, terminated.Reason, int(terminated.ExitCode), terminated.Reason, terminated.StartedAt.Unix(), int(status.RestartCount)), true
}

func generateReport(
	pod *v1.Pod,
	reason string,
	exitStatus int,
	exitDescription string,
	crashTimestamp int64,
	restartCount int,
) events.CrashEvent {
	index, _ := util.ParseAppIndex(pod.Name)

	return events.CrashEvent{
		ProcessGUID: pod.Annotations[k8s.AnnotationProcessGUID],
		AppCrashedRequest: cc_messages.AppCrashedRequest{
			Reason:          reason,
			Instance:        pod.Name,
			Index:           index,
			ExitStatus:      exitStatus,
			ExitDescription: exitDescription,
			CrashTimestamp:  crashTimestamp,
			CrashCount:      restartCount,
		},
	}
}

func getTerminatedContainerStatusIfAny(statuses []v1.ContainerStatus) *v1.ContainerStatus {
	for _, status := range statuses {
		terminated := status.State.Terminated
		if terminated != nil && terminated.ExitCode != 0 {
			return &status
		}
	}

	return nil
}

func getMisconfiguredContainerStatusIfAny(statuses []v1.ContainerStatus) *v1.ContainerStatus {
	for _, status := range statuses {
		waiting := status.State.Waiting
		if waiting != nil && waiting.Reason == CreateContainerConfigError {
			return &status
		}
	}

	return nil
}

func getCrashedContainerStatusIfAny(statuses []v1.ContainerStatus) *v1.ContainerStatus {
	for _, status := range statuses {
		waiting := status.State.Waiting
		if waiting != nil && waiting.Reason == CrashLoopBackOff {
			return &status
		}
	}

	return nil
}
