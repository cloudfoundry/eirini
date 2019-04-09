package main

import (
	"fmt"
	"os"

	"code.cloudfoundry.org/eirini"
	"code.cloudfoundry.org/eirini/recipe"
	"github.com/pkg/errors"
)

func main() {
	stagingGUID := os.Getenv(eirini.EnvStagingGUID)
	completionCallback := os.Getenv(eirini.EnvCompletionCallback)
	eiriniAddress := os.Getenv(eirini.EnvEiriniAddress)
	buildpacksDir, ok := os.LookupEnv(eirini.EnvBuildpacksDir)
	if !ok {
		buildpacksDir = eirini.RecipeBuildPacksDir
	}

	outputDropletLocation, ok := os.LookupEnv(eirini.EnvOutputDropletLocation)
	if !ok {
		outputDropletLocation = eirini.RecipeOutputDropletLocation
	}

	outputBuildArtifactsCache, ok := os.LookupEnv(eirini.EnvOutputBuildArtifactsCache)
	if !ok {
		outputBuildArtifactsCache = eirini.RecipeOutputBuildArtifactsCache
	}

	outputMetadataLocation, ok := os.LookupEnv(eirini.EnvOutputMetadataLocation)
	if !ok {
		outputMetadataLocation = eirini.RecipeOutputMetadataLocation
	}

	packsBuilderPath, ok := os.LookupEnv(eirini.EnvPacksBuilderPath)
	if !ok {
		packsBuilderPath = eirini.RecipePacksBuilderPath
	}

	downloadDir, ok := os.LookupEnv(eirini.EnvWorkspaceDir)
	if !ok {
		downloadDir = eirini.RecipeWorkspaceDir
	}

	responder := recipe.NewResponder(stagingGUID, completionCallback, eiriniAddress)

	commander := &recipe.IOCommander{
		Stdout: os.Stdout,
		Stderr: os.Stderr,
		Stdin:  os.Stdin,
	}

	packsConf := recipe.PacksBuilderConf{
		PacksBuilderPath:          packsBuilderPath,
		BuildpacksDir:             buildpacksDir,
		OutputDropletLocation:     outputDropletLocation,
		OutputBuildArtifactsCache: outputBuildArtifactsCache,
		OutputMetadataLocation:    outputMetadataLocation,
	}

	executor := &recipe.PacksExecutor{
		Conf:        packsConf,
		Commander:   commander,
		Extractor:   &recipe.Unzipper{},
		DownloadDir: downloadDir,
	}

	err := executor.ExecuteRecipe()
	if err != nil {
		responder.RespondWithFailure(errors.Wrap(err, "failed to create droplet"))
		os.Exit(1)
	}

	fmt.Println("Recipe Execution completed")
}
