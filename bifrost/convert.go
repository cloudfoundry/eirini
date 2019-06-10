package bifrost

import (
	"fmt"

	"code.cloudfoundry.org/eirini"
	"github.com/pkg/errors"

	"code.cloudfoundry.org/eirini/models/cf"
	"code.cloudfoundry.org/eirini/opi"
	"code.cloudfoundry.org/lager"
)

type DropletToImageConverter struct {
	logger     lager.Logger
	registryIP string
}

func NewConverter(logger lager.Logger, registryIP string) *DropletToImageConverter {
	return &DropletToImageConverter{
		logger:     logger,
		registryIP: registryIP,
	}
}

func (c *DropletToImageConverter) Convert(request cf.DesireLRPRequest) (opi.LRP, error) {
	vcapJSON := request.Environment["VCAP_APPLICATION"]
	vcap, err := parseVcapApplication(vcapJSON)
	if err != nil {
		return opi.LRP{}, errors.Wrap(err, "failed to parse vcap app")
	}

	if request.DockerImageURL == "" {
		request.DockerImageURL = c.imageURI(request.DropletGUID, request.DropletHash)
	}

	routesJSON := getRequestedRoutes(request)

	lev := eirini.SetupEnv(request.StartCommand)

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

	return opi.LRP{
		AppName:         vcap.AppName,
		SpaceName:       vcap.SpaceName,
		LRPIdentifier:   identifier,
		Image:           request.DockerImageURL,
		TargetInstances: request.NumInstances,
		Command:         append(eirini.InitProcess, eirini.Launch),
		Env:             mergeMaps(request.Environment, lev),
		Health: opi.Healtcheck{
			Type:      request.HealthCheckType,
			Endpoint:  request.HealthCheckHTTPEndpoint,
			TimeoutMs: request.HealthCheckTimeoutMs,
			Port:      int32(8080),
		},
		Ports: request.Ports,
		Metadata: map[string]string{
			cf.VcapAppName: vcap.AppName,
			cf.VcapAppID:   vcap.AppID,
			cf.VcapVersion: vcap.Version,
			cf.ProcessGUID: request.ProcessGUID,
			cf.VcapAppUris: routesJSON,
			cf.LastUpdated: request.LastUpdated,
		},
		MemoryMB:     request.MemoryMB,
		CPUWeight:    request.CPUWeight,
		VolumeMounts: volumeMounts,
		LRP:          request.LRP,
	}, nil
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
