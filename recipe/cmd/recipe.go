package main

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"

	"code.cloudfoundry.org/eirini"
	"code.cloudfoundry.org/eirini/recipe"
	"code.cloudfoundry.org/eirini/util"
	"github.com/JulzDiverse/cfclient"
)

func main() {
	appID := os.Getenv(eirini.EnvAppID)
	stagingGUID := os.Getenv(eirini.EnvStagingGUID)
	completionCallback := os.Getenv(eirini.EnvCompletionCallback)
	eiriniAddress := os.Getenv(eirini.EnvEiriniAddress)
	dropletUploadURL := os.Getenv(eirini.EnvDropletUploadURL)
	buildpacks := os.Getenv(eirini.EnvBuildpacks)

	username := os.Getenv(eirini.EnvCfUsername)
	password := os.Getenv(eirini.EnvCfPassword)
	apiAddress := os.Getenv(eirini.EnvAPIAddress)

	cfg := &cfclient.Config{
		Username:   username,
		Password:   password,
		ApiAddress: apiAddress,
		HttpClient: createAPIHTTPClient(),
	}

	cfclient, err := cfclient.NewClient(cfg)
	if err != nil {
		fmt.Println("Failed to create cf client", err.Error())
		os.Exit(1)
	}

	installer := &recipe.PackageInstaller{Cfclient: cfclient, Extractor: &recipe.Unzipper{}}
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

func createAPIHTTPClient() *http.Client {
	apiCert := filepath.Join(eirini.CCCertsMountPath, eirini.CCAPICertName)
	apiCA := filepath.Join(eirini.CCCertsMountPath, eirini.CCInternalCACertName)
	apiKey := filepath.Join(eirini.CCCertsMountPath, eirini.CCAPIKeyName)

	uaaCert := filepath.Join(eirini.CCCertsMountPath, eirini.UAACertName)
	uaaCA := filepath.Join(eirini.CCCertsMountPath, eirini.UAAInternalCACertName)
	uaaKey := filepath.Join(eirini.CCCertsMountPath, eirini.UAAKeyName)

	client, err := util.CreateTLSHTTPClient([]util.CertPaths{
		{Crt: apiCert, Key: apiKey, Ca: apiCA},
		{Crt: uaaCert, Key: uaaKey, Ca: uaaCA},
	})
	if err != nil {
		panic(err)
	}

	return client
}
