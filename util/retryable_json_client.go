package util

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/hashicorp/go-retryablehttp"
)

type RetryableJSONClient struct {
	httpClient *retryablehttp.Client
}

func NewRetryableJSONClientWithConfig(httpClient *http.Client, retries int, maxDelay time.Duration) *RetryableJSONClient {
	client := NewRetryableJSONClient(httpClient)
	client.httpClient.RetryMax = retries
	client.httpClient.RetryWaitMax = maxDelay
	return client
}

func NewRetryableJSONClient(httpClient *http.Client) *RetryableJSONClient {
	retryableHTTPClient := retryablehttp.NewClient()
	retryableHTTPClient.HTTPClient = httpClient
	return &RetryableJSONClient{
		httpClient: retryableHTTPClient,
	}
}

func (c *RetryableJSONClient) Post(url string, data interface{}) error {
	jsonBody, err := json.Marshal(data)
	if err != nil {
		return err
	}

	resp, err := c.httpClient.Post(url, "application/json", jsonBody)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= http.StatusBadRequest {
		return fmt.Errorf("request failed: code %d", resp.StatusCode)
	}

	return nil
}
