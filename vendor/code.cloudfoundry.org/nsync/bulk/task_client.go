package bulk

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"

	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/runtimeschema/cc_messages"
)

//go:generate counterfeiter -o fakes/fake_task_client.go . TaskClient

type TaskClient interface {
	FailTask(logger lager.Logger, taskState *cc_messages.CCTaskState, httpClient *http.Client) error
}

type CCTaskClient struct {
}

func (tc *CCTaskClient) FailTask(logger lager.Logger, taskState *cc_messages.CCTaskState, httpClient *http.Client) error {
	taskGuid := taskState.TaskGuid
	payload, err := json.Marshal(cc_messages.TaskFailResponseForCC{
		TaskGuid:      taskGuid,
		Failed:        true,
		FailureReason: "Unable to determine completion status",
	})
	if err != nil {
		logger.Error("failed-to-marshal", err, lager.Data{"task_guid": taskGuid})
		return err
	}

	req, err := http.NewRequest("POST", taskState.CompletionCallbackUrl, bytes.NewReader(payload))
	if err != nil {
		return err
	}

	req.Header.Set("Content-Type", "application/json")
	resp, err := httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		logger.Error("bad-response-from-cc", err, lager.Data{"task_guid": taskGuid})
		return errors.New("received bad response from CC ")
	}

	return nil
}
