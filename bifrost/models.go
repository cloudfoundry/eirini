package bifrost

import (
	"encoding/json"
	"fmt"

	"code.cloudfoundry.org/eirini/models/cf"
	"code.cloudfoundry.org/eirini/opi"
)

//go:generate counterfeiter . Converter
type Converter interface {
	Convert(request cf.DesireLRPRequest) (opi.LRP, error)
}

func dropletDownloadUri(baseUrl string, appGuid string) string {
	return fmt.Sprintf("%s/v2/apps/%s/droplet/download", baseUrl, appGuid)
}

func registryStageUri(baseUrl string, space string, appname string, guid string) string {
	return fmt.Sprintf("%s/v2/%s/%s/blobs/?guid=%s", baseUrl, space, appname, guid)
}

func parseVcapApplication(vcap string) (cf.VcapApp, error) {
	var vcapApp cf.VcapApp
	if err := json.Unmarshal([]byte(vcap), &vcapApp); err != nil {
		return vcapApp, err
	}

	return vcapApp, nil
}
