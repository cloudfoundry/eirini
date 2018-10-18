package bifrost

import (
	"encoding/json"

	"code.cloudfoundry.org/eirini/models/cf"
	"code.cloudfoundry.org/eirini/opi"
)

//go:generate counterfeiter . Converter
type Converter interface {
	Convert(request cf.DesireLRPRequest) (opi.LRP, error)
}

func parseVcapApplication(vcap string) (cf.VcapApp, error) {
	var vcapApp cf.VcapApp
	if err := json.Unmarshal([]byte(vcap), &vcapApp); err != nil {
		return vcapApp, err
	}

	return vcapApp, nil
}
