package main

import (
	"fmt"
	"io"
	"net/http"
	"os"

	"github.com/julz/cube"
	"github.com/pkg/errors"
)

type Downloader struct {
	Cfclient cube.CfClient
}

func (d *Downloader) Download(appId string, filepath string) error {
	if appId == "" {
		return errors.New("empty appId provided")
	}

	if filepath == "" {
		return errors.New("empty filepath provided")
	}

	file, err := os.Create(filepath)
	if err != nil {
		return err
	}

	resp, err := d.Cfclient.GetAppBitsByAppGuid(appId)
	if err != nil {
		return errors.Wrap(err, "failed to perform request")
	}

	if resp.StatusCode != http.StatusOK {
		return errors.New(fmt.Sprintf("Download failed. Status Code %s", resp.StatusCode))
	}

	defer resp.Body.Close()

	_, err = io.Copy(file, resp.Body)
	if err != nil {
		return errors.Wrap(err, "failed to copy content to file")
	}

	return nil
}
