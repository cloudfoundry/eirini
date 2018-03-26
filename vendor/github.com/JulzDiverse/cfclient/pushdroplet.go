package cfclient

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"time"
)

func (c *Client) PushDroplet(path string, guid string) error {
	name := filepath.Base(path)

	droplet, size, err := c.readFile(path)
	if err != nil {
		return err
	}
	defer droplet.Close()
	return c.setDroplet(name, guid, droplet, size)
}

func (c *Client) readFile(path string) (io.ReadCloser, int64, error) {
	return c.openFile(path, os.O_RDONLY, 0)
}

func (c *Client) openFile(path string, flag int, perm os.FileMode) (*os.File, int64, error) {
	file, err := os.OpenFile(path, flag, perm)
	if err != nil {
		return nil, 0, err
	}
	fileInfo, err := file.Stat()
	if err != nil {
		return nil, 0, err
	}
	return file, fileInfo.Size(), nil
}

func (c *Client) setDroplet(filename, guid string, droplet io.Reader, size int64) error {
	fieldname := "droplet"
	extension := filepath.Ext(filename)
	name := filename[0 : len(filename)-len(extension)]

	// This is necessary because (similar to S3) CC does not accept chunked multipart MIME
	contentLength := emptyMultipartSize(fieldname, filename) + size

	readBody, writeBody := io.Pipe()
	defer readBody.Close()

	form := multipart.NewWriter(writeBody)
	errChan := make(chan error, 1)
	go func() {
		defer writeBody.Close()

		dropletPart, err := form.CreateFormFile(fieldname, filename)
		if err != nil {
			errChan <- err
			return
		}
		if _, err := io.CopyN(dropletPart, droplet, size); err != nil {
			errChan <- err
			return
		}
		errChan <- form.Close()
	}()

	if err := c.putJob(name, guid, "/droplet/upload", readBody, form.FormDataContentType(), contentLength); err != nil {
		<-errChan
		return err
	}

	return <-errChan
}

func emptyMultipartSize(fieldname, filename string) int64 {
	body := &bytes.Buffer{}
	form := multipart.NewWriter(body)
	form.CreateFormFile(fieldname, filename)
	form.Close()
	return int64(body.Len())
}

func (c *Client) putJob(name, guid, appEndpoint string, body io.Reader, contentType string, contentLength int64) error {
	response, err := c.doAppRequest(name, guid, "PUT", appEndpoint, body, contentType, contentLength, http.StatusCreated)
	if err != nil {
		return err
	}

	return c.waitForJob(response.Body)
}

func (c *Client) doAppRequest(name, guid, method, appEndpoint string, body io.Reader, contentType string, contentLength int64, desiredStatus int) (*http.Response, error) {
	endpoint := fmt.Sprintf("/v2/apps/%s", path.Join(guid, appEndpoint))
	response, err := c.doRequest(method, endpoint, body, contentType, contentLength, desiredStatus)
	if err != nil {
		return nil, err
	}
	return response, nil
}

func (c *Client) doRequest(method, endpoint string, body io.Reader, contentType string, contentLength int64, desiredStatus int) (*http.Response, error) {
	//targetURL.Path = path.Join("https://api.bosh-lite-cube.dynamic-dns.net", endpoint)
	r := c.NewRequestWithBody(method, endpoint, body)
	request, err := r.toHTTP()
	if err != nil {
		return nil, err
	}

	if contentType != "" {
		request.Header.Set("Content-Type", contentType)
	}

	if contentLength > 0 {
		request.ContentLength = contentLength
	}

	response, err := c.DoHttpRequest(request)
	if err != nil {
		return nil, err
	}
	if response.StatusCode != desiredStatus {
		response.Body.Close()
		return nil, fmt.Errorf("unexpected '%s' from: %s %s", response.Status, method, endpoint)
	}
	return response, nil
}

func (c *Client) waitForJob(body io.ReadCloser) error {
	for {
		time.Sleep(200 * time.Millisecond)
		var job struct {
			Entity struct {
				GUID   string `json:"guid"`
				Status string `json:"status"`
			} `json:"entity"`
		}
		if err := decodeJob(body, &job); err != nil {
			return err
		}

		switch job.Entity.Status {
		case "queued", "running":
			endpoint := fmt.Sprintf("/v2/jobs/%s", job.Entity.GUID)
			response, err := c.doRequest("GET", endpoint, nil, "", 0, http.StatusOK)
			if err != nil {
				return err
			}
			body = response.Body
		case "finished":
			return nil
		default:
			return errors.New("job failed")
		}
	}
}

func decodeJob(body io.ReadCloser, job interface{}) error {
	defer body.Close()
	return json.NewDecoder(body).Decode(job)
}
