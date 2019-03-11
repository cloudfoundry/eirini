package buildpack

import (
	"crypto/md5"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"code.cloudfoundry.org/eirini/recipe"
	"github.com/pkg/errors"
)

const configFileName = "config.json"

type Manager struct {
	unzipper       recipe.Unzipper
	buildpackDir   string
	internalClient *http.Client
	defaultClient  *http.Client
}

func New(internalClient *http.Client, defaultClient *http.Client, buildpackDir string) *Manager {
	return &Manager{
		internalClient: internalClient,
		defaultClient:  defaultClient,
		buildpackDir:   buildpackDir,
	}
}

func (b *Manager) Install(buildpacks []recipe.Buildpack) error {
	for _, buildpack := range buildpacks {
		if err := b.install(buildpack); err != nil {
			return err
		}
	}

	return b.writeBuildpackJSON(buildpacks)
}

func (b *Manager) install(buildpack recipe.Buildpack) (err error) {

	var bytes []byte
	bytes, err = recipe.OpenBuildpackURL(&buildpack, b.internalClient)
	if err != nil {
		var err2 error
		bytes, err2 = recipe.OpenBuildpackURL(&buildpack, b.defaultClient)
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

func (b *Manager) writeBuildpackJSON(buildpacks []recipe.Buildpack) error {
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
