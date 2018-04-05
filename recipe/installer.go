package main

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"

	"github.com/julz/cube"
	"github.com/pkg/errors"
)

type PackageInstaller struct {
	Cfclient  cube.CfClient
	Extractor cube.Extractor
}

func (d *PackageInstaller) Install(appId string, targetDir string) error {
	if appId == "" {
		return errors.New("empty appId provided")
	}

	if targetDir == "" {
		return errors.New("empty targetDir provided")
	}

	zipPath := filepath.Join(targetDir, appId) + ".zip"
	if err := d.download(appId, zipPath); err != nil {
		return err
	}

	return d.Extractor.Extract(zipPath, targetDir)
}

func (d *PackageInstaller) download(appId string, filepath string) error {
	file, err := os.Create(filepath)
	if err != nil {
		return err
	}
	defer file.Close()

	resp, err := d.Cfclient.GetAppBitsByAppGuid(appId)
	if err != nil {
		return errors.Wrap(err, "failed to perform request")
	}

	if resp.StatusCode != http.StatusOK {
		return errors.New(fmt.Sprintf("Download failed. Status Code %d", resp.StatusCode))
	}

	defer resp.Body.Close()

	_, err = io.Copy(file, resp.Body)
	if err != nil {
		return errors.Wrap(err, "failed to copy content to file")
	}

	return nil
}
