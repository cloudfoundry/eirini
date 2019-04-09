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

	stagingGUID := os.Getenv(eirini.EnvStagingGUID)
	completionCallback := os.Getenv(eirini.EnvCompletionCallback)
	eiriniAddress := os.Getenv(eirini.EnvEiriniAddress)
	appBitsDownloadURL := os.Getenv(eirini.EnvDownloadURL)
	buildpacksJSON := os.Getenv(eirini.EnvBuildpacks)

	buildpacksDir, ok := os.LookupEnv(eirini.EnvBuildpacksDir)
	if !ok {
		buildpacksDir = eirini.RecipeBuildPacksDir
	}

	certPath, ok := os.LookupEnv(eirini.EnvCertsPath)
	if !ok {
		certPath = eirini.CCCertsMountPath
	}

	workspaceDir, ok := os.LookupEnv(eirini.EnvWorkspaceDir)
	if !ok {
		workspaceDir = eirini.RecipeWorkspaceDir
	}

	responder := recipe.NewResponder(stagingGUID, completionCallback, eiriniAddress)

	downloadClient, err := createDownloadHTTPClient(certPath)
	if err != nil {
		fmt.Println(fmt.Sprintf("error creating http client: %s", err))
		responder.RespondWithFailure(err)
		os.Exit(1)
	}

	buildpackManager := recipe.NewBuildpackManager(downloadClient, http.DefaultClient, buildpacksDir, buildpacksJSON)
	packageInstaller := recipe.NewPackageManager(downloadClient, appBitsDownloadURL, workspaceDir)

	for _, installer := range []recipe.Installer{
		buildpackManager,
		packageInstaller,
	} {
		if err = installer.Install(); err != nil {
			responder.RespondWithFailure(err)
			os.Exit(1)
		}
	}

	fmt.Println("Downloading completed")
}

func createDownloadHTTPClient(certPath string) (*http.Client, error) {
	cacert := filepath.Join(certPath, eirini.CCInternalCACertName)
	cert := filepath.Join(certPath, eirini.CCAPICertName)
	key := filepath.Join(certPath, eirini.CCAPIKeyName)

	return util.CreateTLSHTTPClient([]util.CertPaths{
		{Crt: cert, Key: key, Ca: cacert},
	})
}
