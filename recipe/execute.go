package recipe

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"

	"code.cloudfoundry.org/bbs/models"
	bap "code.cloudfoundry.org/buildpackapplifecycle"
	"code.cloudfoundry.org/runtimeschema/cc_messages"
	"github.com/pkg/errors"
)

const workspaceDir = "/workspace"

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
	BuildpacksDir             string
	OutputDropletLocation     string
	OutputBuildArtifactsCache string
	OutputMetadataLocation    string
}

type Config struct {
	AppID              string
	StagingGUID        string
	CompletionCallback string
	EiriniAddr         string
	DropletUploadURL   string
	PackageDownloadURL string
}

type PacksExecutor struct {
	Conf           PacksBuilderConf
	Installer      Installer
	Uploader       Uploader
	Commander      Commander
	ResultModifier StagingResultModifier
}

func (e *PacksExecutor) ExecuteRecipe(recipeConf Config) error {
	zipPath, err := ioutil.TempFile("", "app.zip")
	if err != nil {
		respondWithFailure(err, recipeConf)
		return err
	}

	err = e.Installer.Install(recipeConf.PackageDownloadURL, zipPath.Name(), workspaceDir)
	if err != nil {
		respondWithFailure(err, recipeConf)
		return err
	}

	args := []string{
		"-buildpacksDir", e.Conf.BuildpacksDir,
		"-outputDroplet", e.Conf.OutputDropletLocation,
		"-outputBuildArtifactsCache", e.Conf.OutputBuildArtifactsCache,
		"-outputMetadata", e.Conf.OutputMetadataLocation,
	}

	err = e.Commander.Exec("/packs/builder", args...)
	if err != nil {
		respondWithFailure(err, recipeConf)
		return err
	}

	err = e.Uploader.Upload(e.Conf.OutputDropletLocation, recipeConf.DropletUploadURL)
	if err != nil {
		respondWithFailure(err, recipeConf)
		return err
	}

	cbResponse, err := e.createSuccessResponse(recipeConf)
	if err != nil {
		return err
	}

	return sendCompleteResponse(recipeConf.EiriniAddr, cbResponse)
}

func (e *PacksExecutor) createSuccessResponse(recipeConf Config) (*models.TaskCallbackResponse, error) {
	stagingResult, err := getStagingResult(e.Conf.OutputMetadataLocation)
	if err != nil {
		return nil, err
	}
	stagingResult, err = e.ResultModifier.Modify(stagingResult)
	if err != nil {
		return nil, err
	}

	result, err := json.Marshal(stagingResult)
	if err != nil {
		panic(err)
	}

	annotation := cc_messages.StagingTaskAnnotation{
		CompletionCallback: recipeConf.CompletionCallback,
	}

	annotationJSON, err := json.Marshal(annotation)
	if err != nil {
		panic(err)
	}

	return &models.TaskCallbackResponse{
		TaskGuid:   recipeConf.StagingGUID,
		Result:     string(result),
		Failed:     false,
		Annotation: string(annotationJSON),
	}, nil
}

func createFailureResponse(failure error, stagingGUID, completionCallback string) *models.TaskCallbackResponse {
	annotation := cc_messages.StagingTaskAnnotation{
		CompletionCallback: completionCallback,
	}

	annotationJSON, err := json.Marshal(annotation)
	if err != nil {
		panic(err)
	}

	return &models.TaskCallbackResponse{
		TaskGuid:      stagingGUID,
		Failed:        true,
		FailureReason: failure.Error(),
		Annotation:    string(annotationJSON),
	}
}

func respondWithFailure(failure error, recipeConf Config) {
	cbResponse := createFailureResponse(failure, recipeConf.StagingGUID, recipeConf.CompletionCallback)

	if completeErr := sendCompleteResponse(recipeConf.EiriniAddr, cbResponse); completeErr != nil {
		fmt.Println("Error processsing completion callback:", completeErr.Error())
	}
}

func getStagingResult(path string) (bap.StagingResult, error) {
	contents, err := ioutil.ReadFile(filepath.Clean(path))
	if err != nil {
		return bap.StagingResult{}, errors.Wrap(err, "failed to read result.json")
	}
	var stagingResult bap.StagingResult
	err = json.Unmarshal(contents, &stagingResult)
	if err != nil {
		return bap.StagingResult{}, err
	}
	return stagingResult, nil
}

func sendCompleteResponse(eiriniAddress string, response *models.TaskCallbackResponse) error {
	responseJSON, err := json.Marshal(response)
	if err != nil {
		panic(err)
	}

	uri := fmt.Sprintf("%s/stage/%s/completed", eiriniAddress, response.TaskGuid)
	req, err := http.NewRequest("PUT", uri, bytes.NewBuffer(responseJSON))
	if err != nil {
		return errors.Wrap(err, "failed to create request")
	}
	req.Header.Set("Content-Type", "application/json")

	client := http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return errors.Wrap(err, "request failed")
	}

	if resp.StatusCode >= 400 {
		return errors.New("Request not successful")
	}

	return nil
}
