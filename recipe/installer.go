package recipe

import (
	"fmt"
	"io"
	"net/http"
	"os"

	"code.cloudfoundry.org/eirini"
	"github.com/pkg/errors"
)

type PackageInstaller struct {
	Client    *http.Client
	Extractor eirini.Extractor
}

func (d *PackageInstaller) Install(downloadURL, zipPath, targetDir string) error {
	if downloadURL == "" {
		return errors.New("empty downloadURL provided")
	}

	if targetDir == "" {
		return errors.New("empty targetDir provided")
	}

	err := d.download(downloadURL, zipPath)
	if err != nil {
		return err
	}

	return d.Extractor.Extract(zipPath, targetDir)
}

func (d *PackageInstaller) download(downloadURL string, filepath string) error {
	file, err := os.Create(filepath)
	if err != nil {
		return err
	}
	defer file.Close()

	appBits, err := d.fetchAppBits(downloadURL)
	if err != nil {
		return err
	}

	defer appBits.Close()

	_, err = io.Copy(file, appBits)
	if err != nil {
		return errors.Wrap(err, "failed to copy content to file")
	}

	return nil
}

func (d *PackageInstaller) fetchAppBits(url string) (io.ReadCloser, error) {
	resp, err := d.Client.Get(url)
	if err != nil {
		return nil, errors.Wrap(err, "failed to perform request")
	}

	if resp.StatusCode != http.StatusOK {
		return nil, errors.New(fmt.Sprintf("Download failed. Status Code %d", resp.StatusCode))
	}

	return resp.Body, nil
}
