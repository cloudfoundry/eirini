package bulk

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/runtimeschema/cc_messages"
)

//go:generate counterfeiter -o fakes/fake_fetcher.go . Fetcher

type Fetcher interface {
	FetchFingerprints(
		logger lager.Logger,
		cancel <-chan struct{},
		httpClient *http.Client,
	) (<-chan []cc_messages.CCDesiredAppFingerprint, <-chan error)

	FetchTaskStates(
		logger lager.Logger,
		cancel <-chan struct{},
		httpClient *http.Client,
	) (<-chan []cc_messages.CCTaskState, <-chan error)

	FetchDesiredApps(
		logger lager.Logger,
		cancel <-chan struct{},
		httpClient *http.Client,
		fingerprints <-chan []cc_messages.CCDesiredAppFingerprint,
	) (<-chan []cc_messages.DesireAppRequestFromCC, <-chan error)
}

type CCFetcher struct {
	BaseURI   string
	BatchSize int
	Username  string
	Password  string
}

const initialBulkToken = "{}"

func (fetcher *CCFetcher) FetchFingerprints(
	logger lager.Logger,
	cancel <-chan struct{},
	httpClient *http.Client,
) (<-chan []cc_messages.CCDesiredAppFingerprint, <-chan error) {
	results := make(chan []cc_messages.CCDesiredAppFingerprint)
	errc := make(chan error, 1)

	logger = logger.Session("fetch-fingerprints-from-cc")

	go func() {
		defer close(results)
		defer close(errc)
		defer logger.Info("done-fetching-desired")

		token := initialBulkToken
		for {
			logger.Info("fetching-desired")

			req, err := http.NewRequest("GET", fetcher.fingerprintURL(token), nil)
			if err != nil {
				errc <- err
				return
			}

			response := cc_messages.CCDesiredStateFingerprintResponse{}

			err = fetcher.doRequest(logger, httpClient, req, &response)
			if err != nil {
				errc <- err
				return
			}

			select {
			case results <- response.Fingerprints:
			case <-cancel:
				return
			}

			if len(response.Fingerprints) < fetcher.BatchSize {
				return
			}

			if response.CCBulkToken == nil {
				errc <- errors.New("token not included in response")
				return
			}

			token = string(*response.CCBulkToken)
		}
	}()

	return results, errc
}

func (fetcher *CCFetcher) FetchDesiredApps(
	logger lager.Logger,
	cancel <-chan struct{},
	httpClient *http.Client,
	fingerprintCh <-chan []cc_messages.CCDesiredAppFingerprint,
) (<-chan []cc_messages.DesireAppRequestFromCC, <-chan error) {
	results := make(chan []cc_messages.DesireAppRequestFromCC)
	errc := make(chan error, 1)

	go func() {
		defer close(results)
		defer close(errc)

		for {
			var fingerprints []cc_messages.CCDesiredAppFingerprint

			select {
			case <-cancel:
				return
			case selected, ok := <-fingerprintCh:
				if !ok {
					return
				}
				fingerprints = selected
			}

			if len(fingerprints) == 0 {
				continue
			}

			processGuids := make([]string, len(fingerprints))
			for i, fingerprint := range fingerprints {
				processGuids[i] = fingerprint.ProcessGuid
			}

			payload, err := json.Marshal(processGuids)
			if err != nil {
				logger.Error("failed-to-marshal", err, lager.Data{"guids": processGuids})
				errc <- err
				return
			}

			logger.Info("fetching-desired", lager.Data{"fingerprints-length": len(fingerprints)})

			req, err := http.NewRequest("POST", fetcher.desiredURL(), bytes.NewReader(payload))
			if err != nil {
				logger.Error("failed-to-create-request", err)
				errc <- err
				continue
			}

			response := []cc_messages.DesireAppRequestFromCC{}

			err = fetcher.doRequest(logger, httpClient, req, &response)
			if err != nil {
				errc <- err
				continue
			}

			select {
			case results <- response:
			case <-cancel:
				return
			}
		}
	}()

	return results, errc
}

func (fetcher *CCFetcher) FetchTaskStates(
	logger lager.Logger,
	cancel <-chan struct{},
	httpClient *http.Client,
) (<-chan []cc_messages.CCTaskState, <-chan error) {
	results := make(chan []cc_messages.CCTaskState)
	errc := make(chan error, 1)

	logger = logger.Session("fetch-task-states-from-cc")

	go func() {
		defer close(results)
		defer close(errc)
		defer logger.Info("done-fetching-task-states")

		token := initialBulkToken
		for {
			logger.Info("fetching-task-states")

			req, err := http.NewRequest("GET", fetcher.taskStatesURL(token), nil)
			if err != nil {
				errc <- err
				return
			}

			response := cc_messages.CCTaskStatesResponse{}

			err = fetcher.doRequest(logger, httpClient, req, &response)
			if err != nil {
				errc <- err
				return
			}

			select {
			case results <- response.TaskStates:
			case <-cancel:
				return
			}

			if len(response.TaskStates) < fetcher.BatchSize {
				return
			}

			if response.CCBulkToken == nil {
				errc <- errors.New("token not included in response")
				return
			}

			token = string(*response.CCBulkToken)
		}
	}()

	return results, errc
}

func (fetcher *CCFetcher) doRequest(
	logger lager.Logger,
	httpClient *http.Client,
	req *http.Request,
	value interface{},
) error {
	req.Header.Set("Content-Type", "application/json")
	req.SetBasicAuth(fetcher.Username, fetcher.Password)

	resp, err := httpClient.Do(req)
	if err != nil {
		return err
	}

	defer resp.Body.Close()

	logger.Info("fetching-desired-complete", lager.Data{
		"StatusCode": resp.StatusCode,
	})

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("invalid response code %d", resp.StatusCode)
	}

	err = json.NewDecoder(resp.Body).Decode(value)
	if err != nil {
		logger.Error("decode-body", err)
		return err
	}

	return nil
}

func (fetcher *CCFetcher) fingerprintURL(bulkToken string) string {
	return fmt.Sprintf("%s/internal/bulk/apps?batch_size=%d&format=fingerprint&token=%s", fetcher.BaseURI, fetcher.BatchSize, bulkToken)
}

func (fetcher *CCFetcher) desiredURL() string {
	return fmt.Sprintf("%s/internal/bulk/apps", fetcher.BaseURI)
}

func (fetcher *CCFetcher) taskStatesURL(bulkToken string) string {
	return fmt.Sprintf("%s/internal/v3/bulk/task_states?batch_size=%d&token=%s", fetcher.BaseURI, fetcher.BatchSize, bulkToken)
}
