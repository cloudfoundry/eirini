package route

import (
	"io/ioutil"
	"os"
	"path/filepath"

	"code.cloudfoundry.org/eirini"
	"github.com/pkg/errors"
	"gopkg.in/yaml.v2"
)

func ReadConfig(path string) (*eirini.RouteEmitterConfig, error) {
	cfg, err := readRouteEmitterConfigFromFile(path)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to read config from %s", path)
	}

	envNATSPassword := os.Getenv("NATS_PASSWORD")
	if envNATSPassword != "" {
		cfg.NatsPassword = envNATSPassword
	}

	return cfg, nil
}

func readRouteEmitterConfigFromFile(path string) (*eirini.RouteEmitterConfig, error) {
	fileBytes, err := ioutil.ReadFile(filepath.Clean(path))
	if err != nil {
		return nil, errors.Wrap(err, "failed to read file")
	}

	var conf eirini.RouteEmitterConfig
	err = yaml.Unmarshal(fileBytes, &conf)

	return &conf, errors.Wrap(err, "failed to unmarshal yaml")
}
