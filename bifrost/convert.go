package bifrost

import (
	"encoding/json"
	"fmt"

	"code.cloudfoundry.org/eirini"
	"code.cloudfoundry.org/eirini/api"
	"code.cloudfoundry.org/eirini/models/cf"
	"code.cloudfoundry.org/eirini/util"
	"code.cloudfoundry.org/lager"
	"github.com/pkg/errors"
)

type lifecycleOptions struct {
	command         []string
	env             map[string]string
	image           string
	privateRegistry *api.PrivateRegistry
}

type APIConverter struct {
	logger lager.Logger
}

func NewAPIConverter(logger lager.Logger) *APIConverter {
	return &APIConverter{
		logger: logger,
	}
}

func (c *APIConverter) ConvertLRP(request cf.DesireLRPRequest) (api.LRP, error) {
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

	healthcheck := api.Healthcheck{
		Type:      request.HealthCheckType,
		Endpoint:  request.HealthCheckHTTPEndpoint,
		TimeoutMs: request.HealthCheckTimeoutMs,
		Port:      port,
	}

	lrpLifecycleOptions, err := c.getLifecycleOptions(request)
	if err != nil {
		return api.LRP{}, err
	}

	identifier := api.LRPIdentifier{
		GUID:    request.GUID,
		Version: request.Version,
	}

	err = c.validateRequest(request)
	if err != nil {
		return api.LRP{}, err
	}

	routes, err := getRequestedRoutes(request)
	if err != nil {
		return api.LRP{}, err
	}

	return api.LRP{
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
	}, nil
}

func (c *APIConverter) ConvertTask(taskGUID string, request cf.TaskRequest) (api.Task, error) {
	c.logger.Debug("convert-task", lager.Data{"app-id": request.AppGUID, "task-guid": taskGUID})

	env := map[string]string{
		"HOME":   "/home/vcap/app",
		"PATH":   "/usr/local/bin:/usr/bin:/bin",
		"USER":   "vcap",
		"TMPDIR": "/home/vcap/tmp",
	}

	task := api.Task{
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
		return api.Task{}, errors.New("docker is the only supported lifecycle")
	}

	lifecycle := request.Lifecycle.DockerLifecycle
	task.Command = lifecycle.Command
	task.Image = lifecycle.Image

	if lifecycle.RegistryUsername != "" || lifecycle.RegistryPassword != "" {
		task.PrivateRegistry = &api.PrivateRegistry{
			Server:   util.ParseImageRegistryHost(lifecycle.Image),
			Username: lifecycle.RegistryUsername,
			Password: lifecycle.RegistryPassword,
		}
	}

	task.Env = mergeEnvs(request.Environment, env)

	return task, nil
}

func getRequestedRoutes(request cf.DesireLRPRequest) ([]api.Route, error) {
	jsonRoutes := request.Routes
	if jsonRoutes == nil {
		return []api.Route{}, nil
	}

	if _, ok := jsonRoutes["cf-router"]; !ok {
		return []api.Route{}, nil
	}

	cfRouterRoutes := jsonRoutes["cf-router"]

	var routes []api.Route

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

func (c *APIConverter) getLifecycleOptions(request cf.DesireLRPRequest) (*lifecycleOptions, error) {
	options := &lifecycleOptions{}

	if request.Lifecycle.DockerLifecycle == nil {
		return nil, fmt.Errorf("missing lifecycle data")
	}

	var err error

	lifecycle := request.Lifecycle.DockerLifecycle
	options.image = lifecycle.Image
	options.command = lifecycle.Command

	if err != nil {
		return nil, errors.Wrap(err, "failed to verify if docker image needs root user")
	}

	registryUsername := lifecycle.RegistryUsername
	registryPassword := lifecycle.RegistryPassword

	if registryUsername != "" || registryPassword != "" {
		options.privateRegistry = &api.PrivateRegistry{
			Server:   util.ParseImageRegistryHost(options.image),
			Username: registryUsername,
			Password: registryPassword,
		}
	}

	return options, nil
}

func convertVolumeMounts(request cf.DesireLRPRequest) []api.VolumeMount {
	volumeMounts := []api.VolumeMount{}
	for _, vm := range request.VolumeMounts {
		volumeMounts = append(volumeMounts, api.VolumeMount{
			MountPath: vm.MountDir,
			ClaimName: vm.VolumeID,
		})
	}

	return volumeMounts
}

func (c *APIConverter) validateRequest(request cf.DesireLRPRequest) error {
	if request.DiskMB == 0 {
		return errors.New("DiskMB cannot be 0")
	}

	return nil
}
