package main

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"

	"code.cloudfoundry.org/eirini"
	"code.cloudfoundry.org/eirini/recipe"
	"code.cloudfoundry.org/eirini/util"
)

func main() {
	appID := os.Getenv(eirini.EnvAppID)
	stagingGUID := os.Getenv(eirini.EnvStagingGUID)
	completionCallback := os.Getenv(eirini.EnvCompletionCallback)
	eiriniAddress := os.Getenv(eirini.EnvEiriniAddress)
	appBitsDownloadURL := os.Getenv(eirini.EnvDownloadURL)
	dropletUploadURL := os.Getenv(eirini.EnvDropletUploadURL)
	buildpacks := os.Getenv(eirini.EnvBuildpacks)

	installer := &recipe.PackageInstaller{
		Client:    createDownloadHTTPClient(),
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

	buildpacksKeyModifier := &recipe.BuildpacksKeyModifier{CCBuildpacksJSON: buildpacks}

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

	err := executor.ExecuteRecipe(recipeConf)
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

	client, err := util.CreateTLSHTTPClient([]util.CertPaths{
		{Ca: apiCA},
	})

	if err != nil {
		panic(err)
	}

	return client
}
