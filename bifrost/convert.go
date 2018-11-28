package bifrost

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"

	"code.cloudfoundry.org/eirini"
	"code.cloudfoundry.org/eirini/launcher"
	"code.cloudfoundry.org/eirini/models/cf"
	"code.cloudfoundry.org/eirini/opi"
	"code.cloudfoundry.org/lager"
)

type DropletToImageConverter struct {
	cfClient    eirini.CfClient
	client      *http.Client
	logger      lager.Logger
	registryURL string
	registryIP  string
}

func NewConverter(cfClient eirini.CfClient, client *http.Client, logger lager.Logger, registryIP, registryURL string) *DropletToImageConverter {
	return &DropletToImageConverter{
		cfClient:    cfClient,
		client:      client,
		logger:      logger,
		registryURL: registryURL,
		registryIP:  registryIP,
	}
}

func (c *DropletToImageConverter) Convert(request cf.DesireLRPRequest) (opi.LRP, error) {
	vcapJSON := request.Environment["VCAP_APPLICATION"]
	vcap, err := parseVcapApplication(vcapJSON)

	if err != nil {
		c.logger.Error("failed-to-parse-vcap-app", err, lager.Data{"vcap-json": vcapJSON})
		return opi.LRP{}, err
	}

	request.DockerImageURL, err = c.dropletToImageURI(request, vcap)
	if err != nil {
		c.logger.Error("failed-to-get-droplet-from-cloud-controller", err, lager.Data{"app-guid": vcap.AppID})
		return opi.LRP{}, err
	}

	uris, err := json.Marshal(vcap.AppUris)
	if err != nil {
		c.logger.Error("failed-to-marshal-vcap-app-uris", err, lager.Data{"app-guid": vcap.AppID})
		panic(err)
	}

	lev := launcher.SetupEnv(request.StartCommand)

	return opi.LRP{
		Name:            vcap.AppID,
		Image:           request.DockerImageURL,
		TargetInstances: request.NumInstances,
		Command:         append(launcher.InitProcess, launcher.Launch),
		Env:             mergeMaps(request.Environment, lev),
		Health: opi.Healtcheck{
			Type:      request.HealthCheckType,
			Endpoint:  request.HealthCheckHTTPEndpoint,
			TimeoutMs: request.HealthCheckTimeoutMs,
			Port:      int32(8080),
		},
		Metadata: map[string]string{
			cf.VcapAppName: vcap.AppName,
			cf.VcapAppID:   vcap.AppID,
			cf.VcapVersion: vcap.Version,
			cf.VcapAppUris: string(uris),
			cf.ProcessGUID: request.ProcessGUID,
			cf.LastUpdated: request.LastUpdated,
		},
	}, nil
}

func (c *DropletToImageConverter) dropletToImageURI(request cf.DesireLRPRequest, vcap cf.VcapApp) (string, error) {
	if request.DockerImageURL != "" {
		return request.DockerImageURL, nil
	}

	dropletBytes, err := c.cfClient.GetDropletByAppGuid(vcap.AppID)
	if err != nil {
		return "", err
	}

	if err = c.pushToRegistry(vcap, request.DropletHash, dropletBytes); err != nil {
		return "", err
	}

	return fmt.Sprintf("%s/cloudfoundry/%s:%s", c.registryIP, request.DropletGUID, request.DropletHash), nil
}

func (c *DropletToImageConverter) pushToRegistry(vcap cf.VcapApp, dropletGUID string, dropletBytes []byte) error {
	registryStageURI := fmt.Sprintf("%s/v2/%s/%s/blobs/?guid=%s", c.registryURL, vcap.SpaceName, vcap.AppName, dropletGUID)
	c.logger.Info("sending-request-to-registry", lager.Data{"request": registryStageURI})

	req, err := http.NewRequest("POST", registryStageURI, bytes.NewReader(dropletBytes))
	if err != nil {
		c.logger.Error("failed-to-create-http-request", err, nil)
		panic(err)
	}

	req.Header.Set("Content-Type", "application/gzip")

	resp, err := c.client.Do(req)
	if err != nil {
		c.logger.Error("stage-request-to-registry-failed", err, lager.Data{"request": registryStageURI})
		return err
	}

	if resp.StatusCode != http.StatusCreated {
		c.logger.Info("invalid-stage-request-to-registry-response-code", lager.Data{"response_status": resp.StatusCode})
		return fmt.Errorf("Invalid staging response: %v", resp)
	}

	return nil
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
