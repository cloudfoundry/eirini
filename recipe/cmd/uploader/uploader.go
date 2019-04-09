package main

import (
	"net/http"
	"os"
	"path/filepath"

	"code.cloudfoundry.org/eirini"
	"code.cloudfoundry.org/eirini/recipe"
	"code.cloudfoundry.org/eirini/util"
)

func main() {
	buildpackCfg := os.Getenv(eirini.EnvBuildpacks)
	stagingGUID := os.Getenv(eirini.EnvStagingGUID)
	completionCallback := os.Getenv(eirini.EnvCompletionCallback)
	eiriniAddress := os.Getenv(eirini.EnvEiriniAddress)
	dropletUploadURL := os.Getenv(eirini.EnvDropletUploadURL)

	certPath, ok := os.LookupEnv(eirini.EnvCertsPath)
	if !ok {
		certPath = eirini.CCCertsMountPath
	}

	dropletLocation, ok := os.LookupEnv(eirini.EnvOutputDropletLocation)
	if !ok {
		dropletLocation = eirini.RecipeOutputDropletLocation
	}

	metadataLocation, ok := os.LookupEnv(eirini.EnvOutputMetadataLocation)
	if !ok {
		metadataLocation = eirini.RecipeOutputMetadataLocation
	}

	responder := recipe.NewResponder(stagingGUID, completionCallback, eiriniAddress)

	client, err := createUploaderHTTPClient(certPath)
	if err != nil {
		responder.RespondWithFailure(err)
		os.Exit(1)
	}

	uploadClient := recipe.DropletUploader{
		Client: client,
	}

	err = uploadClient.Upload(dropletUploadURL, dropletLocation)
	if err != nil {
		responder.RespondWithFailure(err)
		os.Exit(1)
	}

	resp, err := responder.PrepareSuccessResponse(metadataLocation, buildpackCfg)
	if err != nil {
		// TODO: log error
		responder.RespondWithFailure(err)
		os.Exit(1)
	}

	err = responder.RespondWithSuccess(resp)
	if err != nil {
		// TODO: log that it didnt go through
		os.Exit(1)
	}
}

func createUploaderHTTPClient(certPath string) (*http.Client, error) {
	cacert := filepath.Join(certPath, eirini.CCInternalCACertName)
	cert := filepath.Join(certPath, eirini.CCAPICertName)
	key := filepath.Join(certPath, eirini.CCAPIKeyName)

	return util.CreateTLSHTTPClient([]util.CertPaths{
		{Crt: cert, Key: key, Ca: cacert},
	})
}
