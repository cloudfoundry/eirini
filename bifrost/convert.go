package bifrost

import (
	"fmt"

	"regexp"

	"code.cloudfoundry.org/eirini/models/cf"
	"code.cloudfoundry.org/eirini/opi"
	"code.cloudfoundry.org/eirini/stager/docker"
	"code.cloudfoundry.org/lager"
	"github.com/containers/image/types"
	"github.com/pkg/errors"
)

const DockerHubHost = "index.docker.io/v1/"

var dockerRX = regexp.MustCompile(`([a-zA-Z0-9.-]+)(:([0-9]+))?/(\S+/\S+)`)

//go:generate counterfeiter . Converter
type Converter interface {
	ConvertLRP(request cf.DesireLRPRequest) (opi.LRP, error)
	ConvertTask(taskGUID string, request cf.TaskRequest) (opi.Task, error)
}

type OPIConverter struct {
	logger               lager.Logger
	registryIP           string
	diskLimitMB          int64
	imageMetadataFetcher docker.ImageMetadataFetcher
	imageRefParser       docker.ImageRefParser
	allowRunImageAsRoot  bool
}

func NewConverter(logger lager.Logger, registryIP string, diskLimitMB int64, imageMetadataFetcher docker.ImageMetadataFetcher, imageRefParser docker.ImageRefParser, allowRunImageAsRoot bool) *OPIConverter {
	return &OPIConverter{
		logger:               logger,
		registryIP:           registryIP,
		diskLimitMB:          diskLimitMB,
		imageMetadataFetcher: imageMetadataFetcher,
		imageRefParser:       imageRefParser,
		allowRunImageAsRoot:  allowRunImageAsRoot,
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
		"LANG":              "en_US.UTF-8",
		"CF_INSTANCE_ADDR":  fmt.Sprintf("0.0.0.0:%d", port),
		"CF_INSTANCE_PORT":  fmt.Sprintf("%d", port),
		"CF_INSTANCE_PORTS": fmt.Sprintf(`[{"external":%d,"internal":%d}]`, port, port),
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
	c.logger.Debug("create-task", lager.Data{"app-id": request.AppGUID, "staging-guid": taskGUID})

	if request.Lifecycle.BuildpackLifecycle == nil {
		return opi.Task{}, errors.New("unsupported lifecycle, only buildpack lifecycle is supported")
	}

	lifecycle := request.Lifecycle.BuildpackLifecycle
	buildpackEnv := map[string]string{
		"HOME":          "/home/vcap/app",
		"PATH":          "/usr/local/bin:/usr/bin:/bin",
		"USER":          "vcap",
		"TMPDIR":        "/home/vcap/tmp",
		"START_COMMAND": lifecycle.StartCommand,
	}

	task := opi.Task{
		GUID:      taskGUID,
		AppName:   request.AppName,
		AppGUID:   request.AppGUID,
		OrgName:   request.OrgName,
		SpaceName: request.SpaceName,
		OrgGUID:   request.OrgGUID,
		SpaceGUID: request.SpaceGUID,
		Env:       mergeEnvs(request.Environment, buildpackEnv),
		Command:   []string{"/lifecycle/launch"},
		Image:     c.imageURI(lifecycle.DropletGUID, lifecycle.DropletHash),
	}
	return task, nil
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

func mergeEnvs(env1 []cf.EnvironmentVariable, env2 map[string]string) map[string]string {
	result := make(map[string]string)
	for _, v := range env1 {
		result[v.Name] = v.Value
	}

	for k, v := range env2 {
		result[k] = v
	}
	return result
}
