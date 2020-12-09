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

var dockerRX = regexp.MustCompile(`([a-zA-Z0-9.-]+)(:([0-9]+))?/(\S+/\S+)`) //nolint:gochecknoglobals

type lifecycleOptions struct {
	command         []string
	env             map[string]string
	image           string
	privateRegistry *opi.PrivateRegistry
	runsAsRoot      bool
}

type OPIConverter struct {
	logger               lager.Logger
	imageMetadataFetcher ImageMetadataFetcher
	imageRefParser       ImageRefParser
	allowRunImageAsRoot  bool
}

func NewOPIConverter(logger lager.Logger, imageMetadataFetcher ImageMetadataFetcher, imageRefParser ImageRefParser, allowRunImageAsRoot bool) *OPIConverter {
	return &OPIConverter{
		logger:               logger,
		imageMetadataFetcher: imageMetadataFetcher,
		imageRefParser:       imageRefParser,
		allowRunImageAsRoot:  allowRunImageAsRoot,
	}
}

func (c *OPIConverter) ConvertLRP(request cf.DesireLRPRequest) (opi.LRP, error) {
	env := map[string]string{
		"LANG": "en_US.UTF-8",
	}

	var port int32
	if len(request.Ports) != 0 {
		port = request.Ports[0]
		env[eirini.EnvCFInstanceAddr] = fmt.Sprintf("0.0.0.0:%d", port)
		env[eirini.EnvCFInstancePort] = fmt.Sprintf("%d", port)
		env[eirini.EnvCFInstancePorts] = fmt.Sprintf(`[{"external":%d,"internal":%d}]`, port, port)
	}

	healthcheck := opi.Healtcheck{
		Type:      request.HealthCheckType,
		Endpoint:  request.HealthCheckHTTPEndpoint,
		TimeoutMs: request.HealthCheckTimeoutMs,
		Port:      port,
	}

	lrpLifecycleOptions, err := c.getLifecycleOptions(request)
	if err != nil {
		return opi.LRP{}, err
	}

	identifier := opi.LRPIdentifier{
		GUID:    request.GUID,
		Version: request.Version,
	}

	err = c.validateRequest(request)
	if err != nil {
		return opi.LRP{}, err
	}

	routes, err := getRequestedRoutes(request)
	if err != nil {
		return opi.LRP{}, err
	}

	return opi.LRP{
		AppName:                request.AppName,
		AppGUID:                request.AppGUID,
		AppURIs:                routes,
		LastUpdated:            request.LastUpdated,
		OrgName:                request.OrganizationName,
		OrgGUID:                request.OrganizationGUID,
		SpaceName:              request.SpaceName,
		SpaceGUID:              request.SpaceGUID,
		LRPIdentifier:          identifier,
		ProcessType:            request.ProcessType,
		PlacementTags:          request.PlacementTags,
		Image:                  lrpLifecycleOptions.image,
		TargetInstances:        request.NumInstances,
		Command:                lrpLifecycleOptions.command,
		Env:                    mergeMaps(request.Environment, env, lrpLifecycleOptions.env),
		Health:                 healthcheck,
		Ports:                  request.Ports,
		MemoryMB:               request.MemoryMB,
		DiskMB:                 request.DiskMB,
		CPUWeight:              request.CPUWeight,
		VolumeMounts:           convertVolumeMounts(request),
		LRP:                    request.LRP,
		UserDefinedAnnotations: request.UserDefinedAnnotations,
		PrivateRegistry:        lrpLifecycleOptions.privateRegistry,
		RunsAsRoot:             lrpLifecycleOptions.runsAsRoot,
	}, nil
}

func (c *OPIConverter) ConvertTask(taskGUID string, request cf.TaskRequest) (opi.Task, error) {
	c.logger.Debug("convert-task", lager.Data{"app-id": request.AppGUID, "task-guid": taskGUID})

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

	if request.Lifecycle.DockerLifecycle == nil {
		return opi.Task{}, errors.New("docker is the only supported lifecycle")
	}

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

	task.Env = mergeEnvs(request.Environment, env)

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

func getRequestedRoutes(request cf.DesireLRPRequest) ([]opi.Route, error) {
	jsonRoutes := request.Routes
	if jsonRoutes == nil {
		return []opi.Route{}, nil
	}

	if _, ok := jsonRoutes["cf-router"]; !ok {
		return []opi.Route{}, nil
	}

	cfRouterRoutes := jsonRoutes["cf-router"]

	var routes []opi.Route

	err := json.Unmarshal(cfRouterRoutes, &routes)
	if err != nil {
		return nil, err
	}

	return routes, nil
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

func (c *OPIConverter) getLifecycleOptions(request cf.DesireLRPRequest) (*lifecycleOptions, error) {
	options := &lifecycleOptions{}

	if request.Lifecycle.DockerLifecycle == nil {
		return nil, fmt.Errorf("missing lifecycle data")
	}

	var err error

	lifecycle := request.Lifecycle.DockerLifecycle
	options.image = lifecycle.Image
	options.command = lifecycle.Command
	options.runsAsRoot, err = c.isAllowedToRunAsRoot(lifecycle)

	if err != nil {
		return nil, errors.Wrap(err, "failed to verify if docker image needs root user")
	}

	registryUsername := lifecycle.RegistryUsername
	registryPassword := lifecycle.RegistryPassword

	if registryUsername != "" || registryPassword != "" {
		options.privateRegistry = &opi.PrivateRegistry{
			Server:   parseRegistryHost(options.image),
			Username: registryUsername,
			Password: registryPassword,
		}
	}

	return options, nil
}

func convertVolumeMounts(request cf.DesireLRPRequest) []opi.VolumeMount {
	volumeMounts := []opi.VolumeMount{}
	for _, vm := range request.VolumeMounts {
		volumeMounts = append(volumeMounts, opi.VolumeMount{
			MountPath: vm.MountDir,
			ClaimName: vm.VolumeID,
		})
	}

	return volumeMounts
}

func (c *OPIConverter) validateRequest(request cf.DesireLRPRequest) error {
	if request.DiskMB == 0 {
		return errors.New("DiskMB cannot be 0")
	}

	return nil
}
