package recipe

import (
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"

	"code.cloudfoundry.org/eirini"
)

type IOCommander struct {
	Stdout *os.File
	Stderr *os.File
	Stdin  *os.File
}

func (c *IOCommander) Exec(cmd string, args ...string) error {
	command := exec.Command(cmd, args...) //#nosec
	command.Stdout = c.Stdout
	command.Stderr = c.Stderr
	command.Stdin = c.Stdin

	return command.Run()
}

type PacksBuilderConf struct {
	PacksBuilderPath          string
	BuildpacksDir             string
	OutputDropletLocation     string
	OutputBuildArtifactsCache string
	OutputMetadataLocation    string
}

type PacksExecutor struct {
	Conf        PacksBuilderConf
	Commander   Commander
	Extractor   eirini.Extractor
	DownloadDir string
}

func (e *PacksExecutor) ExecuteRecipe() error {
	buildDir, err := e.extract()
	if err != nil {
		return err
	}

	args := []string{
		"-buildDir", buildDir,
		"-buildpacksDir", e.Conf.BuildpacksDir,
		"-outputDroplet", e.Conf.OutputDropletLocation,
		"-outputBuildArtifactsCache", e.Conf.OutputBuildArtifactsCache,
		"-outputMetadata", e.Conf.OutputMetadataLocation,
	}

	err = e.Commander.Exec(e.Conf.PacksBuilderPath, args...)
	if err != nil {
		return err
	}

	err = os.RemoveAll(buildDir)
	if err != nil {
		return err
	}

	return nil
}

func (e *PacksExecutor) extract() (string, error) {
	buildDir, err := ioutil.TempDir("", "app-bits")
	if err != nil {
		return "", err
	}

	err = e.Extractor.Extract(filepath.Join(e.DownloadDir, eirini.AppBits), buildDir)
	if err != nil {
		return "", err
	}

	return buildDir, err
}
