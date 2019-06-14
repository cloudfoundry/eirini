package event

import (
	"code.cloudfoundry.org/eirini/events"
	"code.cloudfoundry.org/eirini/k8s"
	"code.cloudfoundry.org/eirini/models/cf"
	"code.cloudfoundry.org/eirini/util"
	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/runtimeschema/cc_messages"
	v1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"
)

type DefaultCrashReportGenerator struct{}

func (DefaultCrashReportGenerator) Generate(pod *v1.Pod, clientset kubernetes.Interface, logger lager.Logger) (events.CrashReport, bool) {
	statuses := pod.Status.ContainerStatuses
	if len(statuses) == 0 {
		return events.CrashReport{}, false
	}

	_, err := util.ParseAppIndex(pod.Name)
	if err != nil {
		logger.Error("failed-to-parse-app-index", err, lager.Data{"pod-name": pod.Name, "guid": pod.Annotations[cf.ProcessGUID]})
		return events.CrashReport{}, false
	}

	terminated := pod.Status.ContainerStatuses[0].State.Terminated
	if terminated != nil && terminated.ExitCode != 0 {
		return generateReportForTerminatedPod(pod, clientset, logger)
	}

	waiting := pod.Status.ContainerStatuses[0].State.Waiting
	if waiting != nil && waiting.Reason == CrashLoopBackOff {
		container := pod.Status.ContainerStatuses[0]
		exitStatus := int(container.LastTerminationState.Terminated.ExitCode)
		exitDescription := container.LastTerminationState.Terminated.Reason
		crashTimestamp := int64(container.LastTerminationState.Terminated.StartedAt.Second())
		return generateReport(pod, waiting.Reason, exitStatus, exitDescription, crashTimestamp)
	}
	return events.CrashReport{}, false
}

func generateReportForTerminatedPod(pod *v1.Pod, clientset kubernetes.Interface, logger lager.Logger) (events.CrashReport, bool) {
	podEvents, err := k8s.GetEvents(clientset, *pod)
	if err != nil {
		logger.Error("failed-to-get-k8s-events", err, lager.Data{"guid": pod.Annotations[cf.ProcessGUID]})
		return events.CrashReport{}, false
	}
	if k8s.IsStopped(podEvents) {
		return events.CrashReport{}, false
	}

	terminated := pod.Status.ContainerStatuses[0].State.Terminated
	return generateReport(pod, terminated.Reason, int(terminated.ExitCode), terminated.Reason, int64(terminated.StartedAt.Second()))
}

func generateReport(
	pod *v1.Pod,
	reason string,
	exitStatus int,
	exitDescription string,
	crashTimestamp int64,
) (events.CrashReport, bool) {
	index, _ := util.ParseAppIndex(pod.Name)
	container := pod.Status.ContainerStatuses[0]

	return events.CrashReport{
		ProcessGUID: pod.Annotations[cf.ProcessGUID],
		AppCrashedRequest: cc_messages.AppCrashedRequest{
			Reason:          reason,
			Instance:        pod.Name,
			Index:           index,
			ExitStatus:      exitStatus,
			ExitDescription: exitDescription,
			CrashTimestamp:  crashTimestamp,
			CrashCount:      int(container.RestartCount),
		},
	}, true
}
