package staging

import (
	"encoding/json"
	"fmt"
	"net/http"

	"code.cloudfoundry.org/eirini/k8s"
	"code.cloudfoundry.org/eirini/k8s/utils"
	"code.cloudfoundry.org/eirini/models/cf"
	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/runtimeschema/cc_messages"
	"github.com/pkg/errors"
	v1 "k8s.io/api/core/v1"
)

const PodInitializing = "PodInitializing"

type FailedStagingReporter struct {
	Client *http.Client
	Logger lager.Logger
}

func (r FailedStagingReporter) Report(pod *v1.Pod) {
	stagingGUID := pod.Labels[k8s.LabelStagingGUID]

	logger := r.Logger.Session("report", lager.Data{"staging-guid": stagingGUID})

	status := getFailedContainerStatusIfAny(append(
		pod.Status.ContainerStatuses, pod.Status.InitContainerStatuses...,
	))
	if status == nil {
		return
	}

	completionCallback, err := utils.GetEnvVarValue("COMPLETION_CALLBACK", pod.Spec.Containers[0].Env)
	if err != nil {
		logger.Error("failed-to-get-completion-callback", err)
		return
	}

	eiriniAddr, err := utils.GetEnvVarValue("EIRINI_ADDRESS", pod.Spec.Containers[0].Env)

	if err != nil {
		logger.Error("failed-to-get-eirini-address", err)
		return
	}

	reason := fmt.Sprintf("Container '%s' in Pod '%s' failed: %s",
		status.Name,
		pod.Name,
		status.State.Waiting.Reason,
	)
	logger.Error("staging-pod-failed", errors.New(reason))

	completionRequest, err := r.createFailureCompletionRequest(reason, stagingGUID, completionCallback)
	if err != nil {
		logger.Error("cannot-send-failed-staging-completion-request", err)
		return
	}

	uri := fmt.Sprintf("%s/stage/%s/completed", eiriniAddr, completionRequest.TaskGUID)
	if err := utils.Put(r.Client, uri, completionRequest); err != nil {
		logger.Error("cannot-send-failed-staging-response", err)
	}
}

func getFailedContainerStatusIfAny(statuses []v1.ContainerStatus) *v1.ContainerStatus {
	for _, status := range statuses {
		waiting := status.State.Waiting
		if waiting != nil && waiting.Reason != PodInitializing {
			return &status
		}
	}

	return nil
}

func (r FailedStagingReporter) createFailureCompletionRequest(failure string, stagingGUID, completionCallback string) (cf.StagingCompletedRequest, error) {
	annotation := cc_messages.StagingTaskAnnotation{
		CompletionCallback: completionCallback,
	}

	annotationJSON, err := json.Marshal(annotation)
	if err != nil {
		return cf.StagingCompletedRequest{}, errors.Wrap(err, "cannot create callback annotation")
	}

	return cf.StagingCompletedRequest{
		TaskGUID:      stagingGUID,
		Failed:        true,
		FailureReason: failure,
		Annotation:    string(annotationJSON),
	}, nil
}
