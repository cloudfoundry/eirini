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
	eventsClient k8s.EventsClient
}

func NewDefaultCrashEventGenerator(eventsClient k8s.EventsClient) DefaultCrashEventGenerator {
	return DefaultCrashEventGenerator{
		eventsClient: eventsClient,
	}
}

func (g DefaultCrashEventGenerator) Generate(pod *v1.Pod, logger lager.Logger) (events.CrashEvent, bool) {
	logger = logger.Session("generate-crash-event",
		lager.Data{
			"pod-name": pod.Name,
			"guid":     pod.Annotations[k8s.AnnotationProcessGUID],
			"version":  pod.Annotations[k8s.AnnotationVersion],
		})

	statuses := pod.Status.ContainerStatuses
	if len(statuses) == 0 {
		return events.CrashEvent{}, false
	}

	if pod.Labels[k8s.LabelSourceType] != k8s.AppSourceType {
		logger.Debug("skipping-non-eirini-pod")

		return events.CrashEvent{}, false
	}

	appStatus := getOPIContainerStatus(pod.Status.ContainerStatuses)
	if appStatus == nil {
		logger.Debug("eirini-pod-has-no-opi-container-statuses")

		return events.CrashEvent{}, false
	}

	if appStatus.State.Terminated != nil {
		return g.generateReportForTerminatedPod(pod, appStatus, logger)
	}

	if appStatus.State.Waiting != nil && appStatus.LastTerminationState.Terminated != nil {
		exitStatus := int(appStatus.LastTerminationState.Terminated.ExitCode)
		exitDescription := appStatus.LastTerminationState.Terminated.Reason
		crashTimestamp := appStatus.LastTerminationState.Terminated.StartedAt.Unix()

		return generateReport(pod, appStatus.State.Waiting.Reason, exitStatus, exitDescription, crashTimestamp, calculateCrashCount(appStatus.RestartCount)), true
	}

	return events.CrashEvent{}, false
}

func (g DefaultCrashEventGenerator) generateReportForTerminatedPod(pod *v1.Pod, status *v1.ContainerStatus, logger lager.Logger) (events.CrashEvent, bool) {
	podEvents, err := g.eventsClient.GetByPod(*pod)
	if err != nil {
		logger.Error("failed-to-get-k8s-events", err)

		return events.CrashEvent{}, false
	}

	if k8s.IsStopped(podEvents) {
		return events.CrashEvent{}, false
	}

	terminated := status.State.Terminated

	return generateReport(pod, terminated.Reason, int(terminated.ExitCode), terminated.Reason, terminated.StartedAt.Unix(), calculateCrashCount(status.RestartCount)), true
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

func getOPIContainerStatus(statuses []v1.ContainerStatus) *v1.ContainerStatus {
	for _, status := range statuses {
		if status.Name == k8s.OPIContainerName {
			return &status
		}
	}

	return nil
}

func calculateCrashCount(restartCount int32) int {
	// if this is the first time that an app has crashed,
	// this means that the restart count will be 0. Currently
	// the RestartCount is limited to 5 by K8s Garbage Collection
	return int(restartCount + 1)
}
