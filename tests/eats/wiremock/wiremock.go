package wiremock

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"time"
)

type Wiremock struct {
	httpAddress, httpsAddress string
}

func New(wiremockHost string) *Wiremock {
	return &Wiremock{
		httpAddress:  fmt.Sprintf("http://%s", wiremockHost),
		httpsAddress: fmt.Sprintf("https://%s", wiremockHost),
	}
}

func (w *Wiremock) Reset() error {
	return w.post("reset", nil)
}

type Stub struct {
	Request  RequestMatcher `json:"request"`
	Response Response       `json:"response"`
}

type RequestMatcher struct {
	Method string `json:"method"`
	URL    string `json:"url"`
}

type Response struct {
	Status int `json:"status"`
}

func (w *Wiremock) Address() string {
	return w.httpsAddress
}

func (w *Wiremock) AddStub(stub Stub) error {
	return w.post("mappings", stub)
}

type Count struct {
	Count int `json:"count"`
}

type Requests struct {
	Requests []Request `json:"requests"`
}

type Request struct {
	Body string `json:"body"`
}

func (w *Wiremock) GetRequestBody(requestMatcher RequestMatcher) (string, error) {
	resp, err := w.postWithResponse("requests/find", requestMatcher)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	responseDecoder := json.NewDecoder(resp.Body)
	reqs := &Requests{}

	err = responseDecoder.Decode(reqs)
	if err != nil {
		return "", err
	}

	if len(reqs.Requests) != 1 {
		return "", fmt.Errorf("expected one request, but instead got %d", len(reqs.Requests))
	}

	return reqs.Requests[0].Body, nil
}

func (w *Wiremock) GetCount(requestMatcher RequestMatcher) (int, error) {
	resp, err := w.postWithResponse("requests/count", requestMatcher)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	countDecoder := json.NewDecoder(resp.Body)
	count := &Count{}

	err = countDecoder.Decode(count)
	if err != nil {
		return 0, err
	}

	return count.Count, nil
}

func (w *Wiremock) GetCountFn(requestMatcher RequestMatcher) func() (int, error) {
	return func() (int, error) {
		return w.GetCount(requestMatcher)
	}
}

func (w *Wiremock) post(path string, body interface{}) error {
	resp, err := w.postWithResponse(path, body)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	return nil
}

func (w *Wiremock) postWithResponse(path string, body interface{}) (*http.Response, error) {
	bodyJSON, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}

	resp, err := retryOnTimeoutError(func() (*http.Response, error) {
		return http.Post(fmt.Sprintf("%s/__admin/%s", w.httpAddress, path), "application/json", bytes.NewReader(bodyJSON))
	}, 5, time.Second)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode >= http.StatusMultipleChoices {
		respondeBody, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, err
		}

		return nil, fmt.Errorf("request to wiremock failed: %s", respondeBody)
	}

	return resp, nil
}

func retryOnTimeoutError(fn func() (*http.Response, error), times int, wait time.Duration) (*http.Response, error) {
	resp, err := fn()
	if err == nil || times < 1 {
		return resp, err
	}

	var netErr net.Error
	if errors.As(err, &netErr) && netErr.Timeout() {
		time.Sleep(wait)

		return retryOnTimeoutError(fn, times-1, wait)
	}

	return resp, err
}
