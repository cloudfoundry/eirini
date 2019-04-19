package bifrost

import (
	"fmt"

	"code.cloudfoundry.org/eirini"

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
		c.logger.Error("failed-to-parse-vcap-app", err, lager.Data{"vcap-json": vcapJSON})
		return opi.LRP{}, err
	}

	if request.DockerImageURL == "" {
		request.DockerImageURL = c.imageURI(request.DropletGUID, request.DropletHash)
	}

	routesJSON, err := getRequestedRoutes(request)
	if err != nil {
		c.logger.Error("failed-to-marshal-vcap-app-uris", err, lager.Data{"app-guid": vcap.AppID})
		panic(err)
	}

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
	}, nil
}

func getRequestedRoutes(request cf.DesireLRPRequest) (string, error) {
	routes := request.Routes
	if routes == nil {
		return "", nil
	}
	if _, ok := routes["cf-router"]; !ok {
		return "", nil
	}

	cfRouterRoutes := routes["cf-router"]
	data, err := cfRouterRoutes.MarshalJSON()
	if err != nil {
		return "", err
	}

	return string(data), nil
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
