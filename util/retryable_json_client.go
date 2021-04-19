package util

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"

	"github.com/hashicorp/go-retryablehttp"
	"github.com/pkg/errors"
)

type RetryableJSONClient struct {
	httpClient *retryablehttp.Client
}

func NewRetryableJSONClientWithConfig(httpClient *http.Client, retries int, maxDelay time.Duration, logWriter io.Writer) *RetryableJSONClient {
	client := NewRetryableJSONClient(httpClient)
	client.httpClient.RetryMax = retries
	client.httpClient.RetryWaitMax = maxDelay
	client.httpClient.Logger = log.New(logWriter, "", log.LstdFlags)

	return client
}

func NewRetryableJSONClient(httpClient *http.Client) *RetryableJSONClient {
	retryableHTTPClient := retryablehttp.NewClient()
	retryableHTTPClient.HTTPClient = httpClient

	return &RetryableJSONClient{
		httpClient: retryableHTTPClient,
	}
}

func (c *RetryableJSONClient) Post(ctx context.Context, url string, data interface{}) error {
	jsonBody, err := json.Marshal(data)
	if err != nil {
		return errors.Wrap(err, "failed to marshal json body")
	}

	req, err := retryablehttp.NewRequest(http.MethodPost, url, jsonBody)
	if err != nil {
		return fmt.Errorf("creating request failed: %w", err)
	}

	req = req.WithContext(ctx)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}

	defer resp.Body.Close()

	if resp.StatusCode >= http.StatusBadRequest {
		return fmt.Errorf("request failed: code %d", resp.StatusCode)
	}

	return nil
}
