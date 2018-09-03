package recipe

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"os"

	"github.com/pkg/errors"
)

type DropletUploader struct {
	HTTPClient *http.Client
}

func (u *DropletUploader) Upload(path, url string) error {
	if path == "" {
		return errors.New("empty path parameter")
	}
	if url == "" {
		return errors.New("empty url parameter")
	}

	return u.uploadFile(path, url)
}

func (u *DropletUploader) uploadFile(fileLocation, url string) error {
	sourceFile, err := os.Open(fileLocation)
	if err != nil {
		return err
	}

	body := ioutil.NopCloser(sourceFile)
	request, err := http.NewRequest("POST", url, body)
	if err != nil {
		return err
	}

	contentLength, err := fileSize(sourceFile)
	if err != nil {
		return err
	}

	request.ContentLength = contentLength
	request.Header.Set("Content-Type", "application/octet-stream")
	return u.do(request)
}

func fileSize(file *os.File) (int64, error) {
	fileInfo, err := file.Stat()
	if err != nil {
		return 0, err
	}
	return fileInfo.Size(), nil
}

func (u *DropletUploader) do(req *http.Request) error {
	resp, err := u.HTTPClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return fmt.Errorf("Upload failed: Status code %d", resp.StatusCode)
	}
	return nil
}
