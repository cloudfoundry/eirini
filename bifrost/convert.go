package bifrost

import (
	"fmt"

	"code.cloudfoundry.org/eirini/models/cf"
	"code.cloudfoundry.org/eirini/opi"
	"code.cloudfoundry.org/lager"
)

//go:generate counterfeiter . Converter
type Converter interface {
	Convert(request cf.DesireLRPRequest) (opi.LRP, error)
}

type DropletToImageConverter struct {
	logger      lager.Logger
	registryIP  string
	diskLimitMB int64
}

func NewConverter(logger lager.Logger, registryIP string, diskLimitMB int64) *DropletToImageConverter {
	return &DropletToImageConverter{
		logger:      logger,
		registryIP:  registryIP,
		diskLimitMB: diskLimitMB,
	}
}

func (c *DropletToImageConverter) Convert(request cf.DesireLRPRequest) (opi.LRP, error) {
	var command []string
	var env map[string]string
	var image string

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
		image = request.Lifecycle.DockerLifecycle.Image
		command = request.Lifecycle.DockerLifecycle.Command

	case request.Lifecycle.BuildpackLifecycle != nil:
		var buildpackEnv map[string]string
		lifecycle := request.Lifecycle.BuildpackLifecycle
		image, command, buildpackEnv = c.buildpackProperties(lifecycle.DropletGUID, lifecycle.DropletHash, lifecycle.StartCommand)
		env = mergeMaps(env, buildpackEnv)

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
		SpaceName:              request.SpaceName,
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
	}, nil
}

func (c *DropletToImageConverter) buildpackProperties(dropletGUID, dropletHash, startCommand string) (string, []string, map[string]string) {
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

func (c *DropletToImageConverter) imageURI(dropletGUID, dropletHash string) string {
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
