package bifrost

import (
	"encoding/json"
	"fmt"

	"regexp"

	"code.cloudfoundry.org/eirini"
	"code.cloudfoundry.org/eirini/models/cf"
	"code.cloudfoundry.org/eirini/opi"
	"code.cloudfoundry.org/lager"
	"github.com/containers/image/types"
	"github.com/pkg/errors"
)

const DockerHubHost = "index.docker.io/v1/"

var dockerRX = regexp.MustCompile(`([a-zA-Z0-9.-]+)(:([0-9]+))?/(\S+/\S+)`)

type OPIConverter struct {
	logger               lager.Logger
	registryIP           string
	diskLimitMB          int64
	imageMetadataFetcher ImageMetadataFetcher
	imageRefParser       ImageRefParser
	allowRunImageAsRoot  bool
	stagerConfig         eirini.StagerConfig
}

func NewOPIConverter(logger lager.Logger, registryIP string, diskLimitMB int64, imageMetadataFetcher ImageMetadataFetcher, imageRefParser ImageRefParser, allowRunImageAsRoot bool, stagerConfig eirini.StagerConfig) *OPIConverter {
	return &OPIConverter{
		logger:               logger,
		registryIP:           registryIP,
		diskLimitMB:          diskLimitMB,
		imageMetadataFetcher: imageMetadataFetcher,
		imageRefParser:       imageRefParser,
		allowRunImageAsRoot:  allowRunImageAsRoot,
		stagerConfig:         stagerConfig,
	}
}

func (c *OPIConverter) ConvertLRP(request cf.DesireLRPRequest) (opi.LRP, error) {
	var command []string
	var env map[string]string
	var image string
	var privateRegistry *opi.PrivateRegistry
	var runsAsRoot bool

	port := request.Ports[0]
	env = map[string]string{
		"LANG":                    "en_US.UTF-8",
		eirini.EnvCFInstanceAddr:  fmt.Sprintf("0.0.0.0:%d", port),
		eirini.EnvCFInstancePort:  fmt.Sprintf("%d", port),
		eirini.EnvCFInstancePorts: fmt.Sprintf(`[{"external":%d,"internal":%d}]`, port, port),
	}

	healthcheck := opi.Healtcheck{
		Type:      request.HealthCheckType,
		Endpoint:  request.HealthCheckHTTPEndpoint,
		TimeoutMs: request.HealthCheckTimeoutMs,
		Port:      port,
	}

	switch {
	case request.Lifecycle.DockerLifecycle != nil:
		var err error
		lifecycle := request.Lifecycle.DockerLifecycle
		image = lifecycle.Image
		command = lifecycle.Command
		registryUsername := lifecycle.RegistryUsername
		registryPassword := lifecycle.RegistryPassword
		runsAsRoot, err = c.isAllowedToRunAsRoot(lifecycle)
		if err != nil {
			return opi.LRP{}, errors.Wrap(err, "failed to verify if docker image needs root user")
		}

		if registryUsername != "" || registryPassword != "" {
			privateRegistry = &opi.PrivateRegistry{
				Server:   parseRegistryHost(image),
				Username: registryUsername,
				Password: registryPassword,
			}
		}

	case request.Lifecycle.BuildpackLifecycle != nil:
		var buildpackEnv map[string]string
		lifecycle := request.Lifecycle.BuildpackLifecycle
		image, command, buildpackEnv = c.buildpackProperties(lifecycle.DropletGUID, lifecycle.DropletHash, lifecycle.StartCommand)
		env = mergeMaps(env, buildpackEnv)
		runsAsRoot = false

	default:
		return opi.LRP{}, fmt.Errorf("missing lifecycle data")
	}

	routesJSON := getRequestedRoutes(request)

	identifier := opi.LRPIdentifier{
		GUID:    request.GUID,
		Version: request.Version,
	}

	volumeMounts := []opi.VolumeMount{}

	for _, vm := range request.VolumeMounts {
		volumeMounts = append(volumeMounts, opi.VolumeMount{
			MountPath: vm.MountDir,
			ClaimName: vm.VolumeID,
		})
	}
	diskMB := c.diskLimitMB
	if request.DiskMB != 0 {
		diskMB = request.DiskMB
	}

	return opi.LRP{
		AppName:                request.AppName,
		AppGUID:                request.AppGUID,
		AppURIs:                routesJSON,
		LastUpdated:            request.LastUpdated,
		OrgName:                request.OrganizationName,
		OrgGUID:                request.OrganizationGUID,
		SpaceName:              request.SpaceName,
		SpaceGUID:              request.SpaceGUID,
		LRPIdentifier:          identifier,
		ProcessType:            request.ProcessType,
		Image:                  image,
		TargetInstances:        request.NumInstances,
		Command:                command,
		Env:                    mergeMaps(request.Environment, env),
		Health:                 healthcheck,
		Ports:                  request.Ports,
		MemoryMB:               request.MemoryMB,
		DiskMB:                 diskMB,
		CPUWeight:              request.CPUWeight,
		VolumeMounts:           volumeMounts,
		LRP:                    request.LRP,
		UserDefinedAnnotations: request.UserDefinedAnnotations,
		PrivateRegistry:        privateRegistry,
		RunsAsRoot:             runsAsRoot,
	}, nil
}

func (c *OPIConverter) ConvertTask(taskGUID string, request cf.TaskRequest) (opi.Task, error) {
	c.logger.Debug("convert-task", lager.Data{"app-id": request.AppGUID, "staging-guid": taskGUID})

	env := map[string]string{
		"HOME":   "/home/vcap/app",
		"PATH":   "/usr/local/bin:/usr/bin:/bin",
		"USER":   "vcap",
		"TMPDIR": "/home/vcap/tmp",
	}

	task := opi.Task{
		GUID:               taskGUID,
		Name:               request.Name,
		CompletionCallback: request.CompletionCallback,
		AppName:            request.AppName,
		AppGUID:            request.AppGUID,
		OrgName:            request.OrgName,
		SpaceName:          request.SpaceName,
		OrgGUID:            request.OrgGUID,
		SpaceGUID:          request.SpaceGUID,
	}

	if request.Lifecycle.BuildpackLifecycle != nil {
		lifecycle := request.Lifecycle.BuildpackLifecycle
		task.Command = []string{"/lifecycle/launch"}
		task.Image = c.imageURI(lifecycle.DropletGUID, lifecycle.DropletHash)
		env["START_COMMAND"] = lifecycle.StartCommand
	}

	if request.Lifecycle.DockerLifecycle != nil {
		lifecycle := request.Lifecycle.DockerLifecycle
		task.Command = lifecycle.Command
		task.Image = lifecycle.Image
		if lifecycle.RegistryUsername != "" || lifecycle.RegistryPassword != "" {
			task.PrivateRegistry = &opi.PrivateRegistry{
				Server:   parseRegistryHost(lifecycle.Image),
				Username: lifecycle.RegistryUsername,
				Password: lifecycle.RegistryPassword,
			}
		}
	}

	task.Env = mergeEnvs(request.Environment, env)

	return task, nil
}

func (c *OPIConverter) ConvertStaging(stagingGUID string, request cf.StagingRequest) (opi.StagingTask, error) {
	c.logger.Debug("convert-staging", lager.Data{"app-id": request.AppGUID, "staging-guid": stagingGUID})

	lifecycleData := request.LifecycleData
	if lifecycleData == nil {
		lifecycleData = request.Lifecycle.BuildpackLifecycle
	}
	buildpacksJSON, err := json.Marshal(lifecycleData.Buildpacks)
	if err != nil {
		return opi.StagingTask{}, err
	}

	eiriniEnv := map[string]string{
		eirini.EnvDownloadURL:                     lifecycleData.AppBitsDownloadURI,
		eirini.EnvDropletUploadURL:                lifecycleData.DropletUploadURI,
		eirini.EnvBuildpacks:                      string(buildpacksJSON),
		eirini.EnvAppID:                           request.AppGUID,
		eirini.EnvStagingGUID:                     stagingGUID,
		eirini.EnvCompletionCallback:              request.CompletionCallback,
		eirini.EnvEiriniAddress:                   c.stagerConfig.EiriniAddress,
		eirini.EnvBuildpackCacheDownloadURI:       lifecycleData.BuildpackCacheDownloadURI,
		eirini.EnvBuildpackCacheUploadURI:         lifecycleData.BuildpackCacheUploadURI,
		eirini.EnvBuildpackCacheChecksum:          lifecycleData.BuildpackCacheChecksum,
		eirini.EnvBuildpackCacheChecksumAlgorithm: lifecycleData.BuildpackCacheChecksumAlgorithm,
		eirini.EnvBuildpackCacheArtifactsDir:      fmt.Sprintf("%s/buildpack-cache", eirini.BuildpackCacheDir),
		eirini.EnvBuildpackCacheOutputArtifact:    fmt.Sprintf("%s/cache.tgz", eirini.BuildpackCacheDir),
	}

	stagingEnv := mergeEnvs(request.Environment, eiriniEnv)

	stagingTask := opi.StagingTask{
		DownloaderImage: c.stagerConfig.DownloaderImage,
		UploaderImage:   c.stagerConfig.UploaderImage,
		ExecutorImage:   c.stagerConfig.ExecutorImage,
		Task: &opi.Task{
			GUID:      stagingGUID,
			AppName:   request.AppName,
			AppGUID:   request.AppGUID,
			OrgName:   request.OrgName,
			SpaceName: request.SpaceName,
			OrgGUID:   request.OrgGUID,
			SpaceGUID: request.SpaceGUID,
			Env:       stagingEnv,
			MemoryMB:  request.MemoryMB,
			DiskMB:    request.DiskMB,
			CPUWeight: request.CPUWeight,
		},
	}
	return stagingTask, nil
}

func (c *OPIConverter) isAllowedToRunAsRoot(lifecycle *cf.DockerLifecycle) (bool, error) {
	if !c.allowRunImageAsRoot {
		return false, nil
	}

	user, err := c.getImageUser(lifecycle)
	if err != nil {
		return false, errors.Wrap(err, "failed to get the user of the image")
	}

	return user == "" || user == "root" || user == "0", nil
}

func (c *OPIConverter) getImageUser(lifecycle *cf.DockerLifecycle) (string, error) {
	dockerRef, err := c.imageRefParser.Parse(lifecycle.Image)
	if err != nil {
		return "", errors.Wrap(err, "failed to parse image ref")
	}
	imgMetadata, err := c.imageMetadataFetcher.Fetch(dockerRef, types.SystemContext{
		DockerAuthConfig: &types.DockerAuthConfig{
			Username: lifecycle.RegistryUsername,
			Password: lifecycle.RegistryPassword,
		},
	})
	if err != nil {
		return "", errors.Wrap(err, "failed to fetch image metadata")
	}

	return imgMetadata.User, nil
}

func (c *OPIConverter) buildpackProperties(dropletGUID, dropletHash, startCommand string) (string, []string, map[string]string) {
	image := c.imageURI(dropletGUID, dropletHash)
	command := []string{"dumb-init", "--", "/lifecycle/launch"}
	buildpackEnv := map[string]string{
		"HOME":          "/home/vcap/app",
		"PATH":          "/usr/local/bin:/usr/bin:/bin",
		"USER":          "vcap",
		"PWD":           "/home/vcap/app",
		"TMPDIR":        "/home/vcap/tmp",
		"START_COMMAND": startCommand,
	}

	return image, command, buildpackEnv
}

func getRequestedRoutes(request cf.DesireLRPRequest) string {
	routes := request.Routes
	if routes == nil {
		return ""
	}
	if _, ok := routes["cf-router"]; !ok {
		return ""
	}

	cfRouterRoutes := routes["cf-router"]
	data, err := cfRouterRoutes.MarshalJSON()
	if err != nil {
		panic("This should never happen!")
	}

	return string(data)
}

func (c *OPIConverter) imageURI(dropletGUID, dropletHash string) string {
	return fmt.Sprintf("%s/cloudfoundry/%s:%s", c.registryIP, dropletGUID, dropletHash)
}

func mergeMaps(maps ...map[string]string) map[string]string {
	result := make(map[string]string)
	for _, m := range maps {
		for k, v := range m {
			result[k] = v
		}
	}
	return result
}

func parseRegistryHost(imageURL string) string {
	if !dockerRX.MatchString(imageURL) {
		return DockerHubHost
	}

	matches := dockerRX.FindStringSubmatch(imageURL)
	return matches[1]
}

func mergeEnvs(requestEnv []cf.EnvironmentVariable, appliedEnv map[string]string) map[string]string {
	result := make(map[string]string)
	for _, v := range requestEnv {
		result[v.Name] = v.Value
	}

	for k, v := range appliedEnv {
		result[k] = v
	}

	return result
}
