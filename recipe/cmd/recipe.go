package main

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"

	"code.cloudfoundry.org/eirini"
	"code.cloudfoundry.org/eirini/recipe"
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
		SkipSslValidation: true,
		Username:          username,
		Password:          password,
		ApiAddress:        apiAddress,
		HttpClient:        createHTTPClient(),
	}

	cfclient, err := cfclient.NewClient(cfg)
	if err != nil {
		fmt.Println("Failed to create cf client", err.Error())
		os.Exit(1)
	}

	installer := &recipe.PackageInstaller{Cfclient: cfclient, Extractor: &recipe.Unzipper{}}
	uploader := &recipe.DropletUploader{HTTPClient: createHTTPClient()}
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

func createHTTPClient() *http.Client {
	certLocation := filepath.Join(eirini.CCCertsMountPath, eirini.CCUploaderCertName)
	cacertLocation := filepath.Join(eirini.CCCertsMountPath, eirini.CCInternalCACertName)
	privKeyLocation := filepath.Join(eirini.CCCertsMountPath, eirini.CCUploaderKeyName)

	cert, err := tls.LoadX509KeyPair(certLocation, privKeyLocation)
	if err != nil {
		panic(err)
	}

	cacert, err := ioutil.ReadFile(cacertLocation)
	if err != nil {
		panic(err)
	}

	caCertPool := x509.NewCertPool()
	ok := caCertPool.AppendCertsFromPEM(cacert)
	if !ok {
		panic("append certs from pem failed")
	}

	tlsConf := &tls.Config{
		Certificates: []tls.Certificate{cert},
		RootCAs:      caCertPool,
	}

	return &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: tlsConf,
		},
	}
}
