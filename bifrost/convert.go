package bifrost

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"

	"code.cloudfoundry.org/bbs/models"
	"code.cloudfoundry.org/eirini"
	"code.cloudfoundry.org/eirini/models/cf"
	"code.cloudfoundry.org/eirini/opi"
	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/runtimeschema/cc_messages"
)

func Convert(
	msg cc_messages.DesireAppRequestFromCC,
	registryUrl string,
	registryIP string,
	cfClient eirini.CfClient,
	client *http.Client,
	log lager.Logger,
) opi.LRP {
	envMap := envVarsToMap(msg.Environment)
	vcap := parseVcapApplication(envMap["VCAP_APPLICATION"])

	if msg.DockerImageUrl == "" {
		msg.DockerImageUrl = dropletToImageURI(msg, vcap, cfClient, client, registryUrl, registryIP, log)
	}

	uris, err := json.Marshal(vcap.AppUris)
	if err != nil {
		log.Error("failed-to-marshal-vcap-app-uris", err, lager.Data{"app-guid": vcap.AppId})
		uris = []byte{}
	}

	return opi.LRP{
		Name:            vcap.AppId,
		Image:           msg.DockerImageUrl,
		TargetInstances: msg.NumInstances,
		Command: []string{
			msg.StartCommand,
		},
		Env: envMap,
		Metadata: map[string]string{
			cf.VcapAppName: vcap.AppName,
			cf.VcapAppId:   vcap.AppId,
			cf.VcapVersion: vcap.Version,
			cf.VcapAppUris: string(uris),
			cf.ProcessGuid: msg.ProcessGuid,
		},
	}
}

func parseVcapApplication(vcap string) cf.VcapApp {
	var vcapApp cf.VcapApp
	if err := json.Unmarshal([]byte(vcap), &vcapApp); err != nil {
		panic(err)
	}

	return vcapApp
}

func envVarsToMap(envs []*models.EnvironmentVariable) map[string]string {
	envMap := map[string]string{}
	for _, v := range envs {
		envMap[v.Name] = v.Value
	}
	return envMap
}

func dropletToImageURI(
	msg cc_messages.DesireAppRequestFromCC,
	vcap cf.VcapApp,
	cfClient eirini.CfClient,
	client *http.Client,
	registryUrl string,
	registryIP string,
	log lager.Logger,
) string {
	dropletBytes, err := cfClient.GetDropletByAppGuid(vcap.AppId)
	if err != nil {
		log.Error("failed-to-get-droplet-from-cloud-controller", err, lager.Data{"app-guid": vcap.AppId})
		panic(err)
	}

	stageRequest(client, registryUrl, vcap, msg.DropletHash, dropletBytes, log)

	return fmt.Sprintf("%s/cloudfoundry/app-name:%s", registryIP, msg.DropletHash)
}

func stageRequest(
	client *http.Client,
	registryUrl string,
	vcap cf.VcapApp,
	dropletHash string,
	dropletBytes []byte,
	log lager.Logger,
) string {
	registryStageUri := registryStageUri(registryUrl, vcap.SpaceName, vcap.AppName, dropletHash)

	log.Info("sending-request-to-registry", lager.Data{"request": registryStageUri})

	req, err := http.NewRequest("POST", registryStageUri, bytes.NewReader(dropletBytes))
	if err != nil {
		log.Error("failed-to-create-http-request", err, nil)
		panic(err)
	}

	req.Header.Set("Content-Type", "application/gzip")

	resp, err := client.Do(req)
	if err != nil {
		log.Error("stage-request-to-registry-failed", err, lager.Data{"request": registryStageUri})
		return ""
	}

	log.Info("request-successful", lager.Data{"response_status": resp.StatusCode})

	digest, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Error("read-response-failed", err)
		return ""
	}

	return string(digest)

}
