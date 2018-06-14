package bifrost

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"code.cloudfoundry.org/eirini"
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
	vcapJson := request.Environment["VCAP_APPLICATION"]
	vcap, err := parseVcapApplication(vcapJson)

	if err != nil {
		c.logger.Error("failed-to-parse-vcap-app", err, lager.Data{"vcap-json": vcapJson})
		return opi.LRP{}, err
	}

	request.DockerImageUrl, err = c.dropletToImageURI(request, vcap)
	if err != nil {
		c.logger.Error("failed-to-get-droplet-from-cloud-controller", err, lager.Data{"app-guid": vcap.AppId})
		return opi.LRP{}, err
	}

	uris, err := json.Marshal(vcap.AppUris)
	if err != nil {
		c.logger.Error("failed-to-marshal-vcap-app-uris", err, lager.Data{"app-guid": vcap.AppId})
		panic(err)
	}

	return opi.LRP{
		Name:            vcap.AppId,
		Image:           request.DockerImageUrl,
		TargetInstances: request.NumInstances,
		Command: []string{
			request.StartCommand,
		},
		Env: request.Environment,
		Metadata: map[string]string{
			cf.VcapAppName: vcap.AppName,
			cf.VcapAppId:   vcap.AppId,
			cf.VcapVersion: vcap.Version,
			cf.VcapAppUris: string(uris),
			cf.ProcessGuid: request.ProcessGuid,
			cf.LastUpdated: request.LastUpdated,
		},
	}, nil
}

func (c *DropletToImageConverter) dropletToImageURI(request cf.DesireLRPRequest, vcap cf.VcapApp) (string, error) {
	if request.DockerImageUrl != "" {
		return request.DockerImageUrl, nil
	}

	dropletBytes, err := c.cfClient.GetDropletByAppGuid(vcap.AppId)
	if err != nil {
		return "", err
	}

	if err = c.stageRequest(vcap, request.DropletHash, dropletBytes); err != nil {
		return "", err
	}

	return fmt.Sprintf("%s/cloudfoundry/app-name:%s", c.registryIP, request.DropletHash), nil
}

func (c *DropletToImageConverter) stageRequest(vcap cf.VcapApp, dropletHash string, dropletBytes []byte) error {
	registryStageUri := registryStageUri(c.registryURL, vcap.SpaceName, vcap.AppName, dropletHash)
	c.logger.Info("sending-request-to-registry", lager.Data{"request": registryStageUri})

	req, err := http.NewRequest("POST", registryStageUri, bytes.NewReader(dropletBytes))
	if err != nil {
		c.logger.Error("failed-to-create-http-request", err, nil)
		panic(err)
	}

	req.Header.Set("Content-Type", "application/gzip")

	resp, err := c.client.Do(req)
	if err != nil {
		c.logger.Error("stage-request-to-registry-failed", err, lager.Data{"request": registryStageUri})
		return err
	}

	if resp.StatusCode != http.StatusCreated {
		c.logger.Info("invalid-stage-request-to-registry-response-code", lager.Data{"response_status": resp.StatusCode})
		return errors.New(fmt.Sprintf("Invalid staging response: %s", resp))
	}

	return nil
}
