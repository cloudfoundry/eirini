package utils

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/pkg/errors"
)

func Post(ctx context.Context, client *http.Client, uri string, body interface{}) error {
	return do(ctx, client, http.MethodPost, uri, body)
}

func do(ctx context.Context, client *http.Client, method string, uri string, body interface{}) error {
	bodyJSON, err := json.Marshal(body)
	if err != nil {
		return errors.Wrap(err, "cannot marshal body")
	}

	req, err := http.NewRequestWithContext(ctx, method, uri, bytes.NewBuffer(bodyJSON))
	if err != nil {
		return errors.Wrap(err, "failed to create request")
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return errors.Wrap(err, "request failed")
	}
	defer resp.Body.Close()

	if resp.StatusCode >= http.StatusBadRequest {
		body, err := io.ReadAll(resp.Body)

		var message string

		if err == nil {
			message = string(body)
		}

		return fmt.Errorf("request not successful: status=%d %s", resp.StatusCode, message)
	}

	return nil
}
