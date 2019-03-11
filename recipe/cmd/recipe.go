package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"

	"code.cloudfoundry.org/eirini"
	"code.cloudfoundry.org/eirini/recipe"
	"code.cloudfoundry.org/eirini/recipe/cmd/buildpack"
	"code.cloudfoundry.org/eirini/util"
)

const buildPacksDir = "/var/lib/buildpacks"

func main() {
	appID := os.Getenv(eirini.EnvAppID)
	stagingGUID := os.Getenv(eirini.EnvStagingGUID)
	completionCallback := os.Getenv(eirini.EnvCompletionCallback)
	eiriniAddress := os.Getenv(eirini.EnvEiriniAddress)
	appBitsDownloadURL := os.Getenv(eirini.EnvDownloadURL)
	dropletUploadURL := os.Getenv(eirini.EnvDropletUploadURL)
	buildpacksJSON := os.Getenv(eirini.EnvBuildpacks)

	downloadClient := createDownloadHTTPClient()
	buildPackManager := buildpack.New(downloadClient, http.DefaultClient, buildPacksDir)

	installer := &recipe.PackageInstaller{
		Client:    downloadClient,
		Extractor: &recipe.Unzipper{},
	}

	uploader := &recipe.DropletUploader{
		HTTPClient: createUploaderHTTPClient(),
	}

	commander := &recipe.IOCommander{
		Stdout: os.Stdout,
		Stderr: os.Stderr,
		Stdin:  os.Stdin,
	}

	packsConf := recipe.PacksBuilderConf{
		BuildpacksDir:             "/var/lib/buildpacks",
		OutputDropletLocation:     "/out/droplet.tgz",
		OutputBuildArtifactsCache: "/cache/cache.tgz",
		OutputMetadataLocation:    "/out/result.json",
	}

	buildpacksKeyModifier := &recipe.BuildpacksKeyModifier{CCBuildpacksJSON: buildpacksJSON}

	executor := &recipe.PacksExecutor{
		Conf:           packsConf,
		Installer:      installer,
		Uploader:       uploader,
		Commander:      commander,
		ResultModifier: buildpacksKeyModifier,
	}

	recipeConf := recipe.Config{
		AppID:              appID,
		StagingGUID:        stagingGUID,
		CompletionCallback: completionCallback,
		EiriniAddr:         eiriniAddress,
		DropletUploadURL:   dropletUploadURL,
		PackageDownloadURL: appBitsDownloadURL,
	}

	var buildpacks []recipe.Buildpack
	err := json.Unmarshal([]byte(buildpacksJSON), &buildpacks)
	if err != nil {
		fmt.Println(fmt.Sprintf("Error unmarshaling environment variable %s: %s", eirini.EnvBuildpacks, err.Error()))
		os.Exit(1)
	}

	if err = buildPackManager.Install(buildpacks); err != nil {
		fmt.Println("Error while installing buildpacks:", err.Error())
		os.Exit(1)
	}

	err = executor.ExecuteRecipe(recipeConf)
	if err != nil {
		fmt.Println("Error while executing staging recipe:", err.Error())
		os.Exit(1)
	}

	fmt.Println("Staging completed")
}

func createUploaderHTTPClient() *http.Client {
	cert := filepath.Join(eirini.CCCertsMountPath, eirini.CCUploaderCertName)
	cacert := filepath.Join(eirini.CCCertsMountPath, eirini.CCInternalCACertName)
	key := filepath.Join(eirini.CCCertsMountPath, eirini.CCUploaderKeyName)

	client, err := util.CreateTLSHTTPClient([]util.CertPaths{
		{Crt: cert, Key: key, Ca: cacert},
	})
	if err != nil {
		panic(err)
	}

	return client
}

func createDownloadHTTPClient() *http.Client {
	apiCA := filepath.Join(eirini.CCCertsMountPath, eirini.CCInternalCACertName)
	cert := filepath.Join(eirini.CCCertsMountPath, eirini.CCAPICertName)
	key := filepath.Join(eirini.CCCertsMountPath, eirini.CCAPIKeyName)

	client, err := util.CreateTLSHTTPClient([]util.CertPaths{
		{Crt: cert, Key: key, Ca: apiCA},
	})

	if err != nil {
		panic(err)
	}

	return client
}
