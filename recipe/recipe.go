package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"os/exec"

	"code.cloudfoundry.org/eirini"
	"github.com/JulzDiverse/cfclient"
	"github.com/pkg/errors"
	"github.com/starkandwayne/goutils/ansi"

	"code.cloudfoundry.org/bbs/models"
	"code.cloudfoundry.org/runtimeschema/cc_messages"
)

func main() {
	downloadUrl := os.Getenv(eirini.EnvDownloadUrl)
	uploadUrl := os.Getenv(eirini.EnvUploadUrl)
	appId := os.Getenv(eirini.EnvAppId)
	stagingGuid := os.Getenv(eirini.EnvStagingGuid)
	completionCallback := os.Getenv(eirini.EnvCompletionCallback)

	username := os.Getenv(eirini.EnvCfUsername)
	password := os.Getenv(eirini.EnvCfPassword)
	apiAddress := os.Getenv(eirini.EnvApiAddress)
	eiriniAddress := os.Getenv(eirini.EnvEiriniAddress)

	fmt.Println("STARTING WITH:", downloadUrl, uploadUrl, appId, stagingGuid, completionCallback)

	annotation := cc_messages.StagingTaskAnnotation{
		CompletionCallback: completionCallback,
	}

	annotationJson, err := json.Marshal(annotation)
	if err != nil {
		os.Exit(1)
	}

	cfclient, err := cfclient.NewClient(&cfclient.Config{
		SkipSslValidation: true,
		Username:          username,
		Password:          password,
		ApiAddress:        apiAddress,
	})

	respondWithFailureAndExit(err, stagingGuid, annotationJson)

	installer := PackageInstaller{cfclient, &Unzipper{}}
	uploader := Uploader{cfclient}

	workspaceDir := "/workspace"

	err = installer.Install(appId, workspaceDir)
	respondWithFailureAndExit(err, stagingGuid, annotationJson)

	err = execCmd(
		"/packs/builder", []string{
			"-buildpacksDir", "/var/lib/buildpacks",
			"-outputDroplet", "/out/droplet.tgz",
			"-outputBuildArtifactsCache", "/cache/cache.tgz",
			"-outputMetadata", "/out/result.json",
		})
	respondWithFailureAndExit(err, stagingGuid, annotationJson)

	fmt.Println("Start Upload Process.")
	err = uploader.Upload(appId, "/out/droplet.tgz")
	respondWithFailureAndExit(err, stagingGuid, annotationJson)

	fmt.Println("Upload successful!")
	result, err := readResultJson("/out/result.json")
	respondWithFailureAndExit(err, stagingGuid, annotationJson)

	cbResponse := models.TaskCallbackResponse{
		TaskGuid:   stagingGuid,
		Result:     string(result[:len(result)]),
		Failed:     false,
		Annotation: string(annotationJson[:len(annotationJson)]),
	}

	err = stagingCompleteResponse(eiriniAddress, cbResponse)
	if err != nil {
		fmt.Println("Error processsing completion callback:", err.Error())
		os.Exit(1)
	}

	fmt.Println("Staging completed")
}

func readResultJson(path string) ([]byte, error) {
	file, err := ioutil.ReadFile(path)
	if err != nil {
		return []byte{}, errors.Wrap(err, "failed to read result.json")
	}
	return file, nil
}

func stagingCompleteResponse(eiriniAddress string, callbackResponse models.TaskCallbackResponse) error {
	jsonBytes := new(bytes.Buffer)
	err := json.NewEncoder(jsonBytes).Encode(callbackResponse)

	if err != nil {
		return errors.Wrap(err, "failed to encode response")
	}

	req, err := http.NewRequest("POST", fmt.Sprintf("%s/v1/staging/%s/completed", eiriniAddress, callbackResponse.TaskGuid), jsonBytes)
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

func respondWithFailureAndExit(err error, stagingGuid string, annotationJson []byte) {
	if err != nil {
		cbResponse := models.TaskCallbackResponse{
			TaskGuid:      stagingGuid,
			Failed:        true,
			FailureReason: err.Error(),
			Annotation:    string(annotationJson[:len(annotationJson)]),
		}

		if completeErr := stagingCompleteResponse(stagingGuid, cbResponse); completeErr != nil {
			fmt.Println("Error processsing completion callback:", completeErr.Error())
			os.Exit(1)
		}

		fmt.Println(err.Error())
		os.Exit(1)
	}
}

func execCmd(cmdname string, args []string) error {
	cmd := exec.Command(cmdname, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin

	err := cmd.Run()
	if err != nil {
		return errors.Wrap(err, ansi.Sprintf("@R{Failed to run %s}", cmdname))
	}

	return nil
}
