package recipe

import (
	"crypto/md5"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/pkg/errors"
)

type Buildpack struct {
	Name string `json:"name"`
	Key  string `json:"key"`
	URL  string `json:"url"`
}

type BuildpackManager struct {
	unzipper       Unzipper
	buildpackDir   string
	buildpacksJSON string
	internalClient *http.Client
	defaultClient  *http.Client
}

const configFileName = "config.json"

func OpenBuildpackURL(buildpack *Buildpack, client *http.Client) ([]byte, error) {
	resp, err := client.Get(buildpack.URL)
	if err != nil {
		return nil, errors.Wrap(err, "failed to request buildpack")
	}

	if resp.StatusCode != http.StatusOK {
		return nil, errors.New(fmt.Sprintf("downloading buildpack failed with status code %d", resp.StatusCode))
	}

	bytes, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	return bytes, nil
}

func NewBuildpackManager(internalClient *http.Client, defaultClient *http.Client, buildpackDir, buildpacksJSON string) Installer {
	return &BuildpackManager{
		internalClient: internalClient,
		defaultClient:  defaultClient,
		buildpackDir:   buildpackDir,
		buildpacksJSON: buildpacksJSON,
	}
}

func (b *BuildpackManager) Install() error {
	var buildpacks []Buildpack

	err := json.Unmarshal([]byte(b.buildpacksJSON), &buildpacks)
	if err != nil {
		fmt.Println(fmt.Sprintf("Error unmarshaling environment variable %s: %s", b.buildpacksJSON, err.Error()))
		return err
	}

	for _, buildpack := range buildpacks {
		if err := b.install(buildpack); err != nil {
			return err
		}
	}

	return b.writeBuildpackJSON(buildpacks)
}

func (b *BuildpackManager) install(buildpack Buildpack) (err error) {
	var bytes []byte
	bytes, err = OpenBuildpackURL(&buildpack, b.internalClient)
	if err != nil {
		var err2 error
		bytes, err2 = OpenBuildpackURL(&buildpack, b.defaultClient)
		if err2 != nil {
			return errors.Wrap(err, fmt.Sprintf("default client also failed: %s", err2.Error()))
		}
	}

	var tempDirName string
	tempDirName, err = ioutil.TempDir("", "buildpacks")
	if err != nil {
		return err
	}

	fileName := filepath.Join(tempDirName, fmt.Sprintf("buildback-%d-.zip", time.Now().Nanosecond()))
	defer func() {
		err = os.Remove(fileName)
	}()

	err = ioutil.WriteFile(fileName, bytes, 0777)
	if err != nil {
		return err
	}

	buildpackName := fmt.Sprintf("%x", md5.Sum([]byte(buildpack.Name)))
	buildpackPath := filepath.Join(b.buildpackDir, buildpackName)
	err = os.MkdirAll(buildpackPath, 0777)
	if err != nil {
		return err
	}

	err = b.unzipper.Extract(fileName, buildpackPath)
	if err != nil {
		return err
	}

	return err
}

func (b *BuildpackManager) writeBuildpackJSON(buildpacks []Buildpack) error {
	bytes, err := json.Marshal(buildpacks)
	if err != nil {
		return err
	}

	err = ioutil.WriteFile(filepath.Join(b.buildpackDir, configFileName), bytes, 0644)
	if err != nil {
		return err
	}

	return nil
}
