package recipe

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"

	"code.cloudfoundry.org/eirini"
	"github.com/pkg/errors"
)

type PackageInstaller struct {
	Cfclient  eirini.CfClient
	Extractor eirini.Extractor
}

func (d *PackageInstaller) Install(appID string, targetDir string) error {
	if appID == "" {
		return errors.New("empty appID provided")
	}

	if targetDir == "" {
		return errors.New("empty targetDir provided")
	}

	zipPath := filepath.Join(targetDir, appID) + ".zip"
	if err := d.download(appID, zipPath); err != nil {
		return err
	}

	return d.Extractor.Extract(zipPath, targetDir)
}

func (d *PackageInstaller) download(appID string, filepath string) error {
	file, err := os.Create(filepath)
	if err != nil {
		return err
	}
	defer file.Close()

	resp, err := d.Cfclient.GetAppBitsByAppGuid(appID)
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
