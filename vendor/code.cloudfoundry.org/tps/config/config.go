package config

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"time"

	"code.cloudfoundry.org/debugserver"
	"code.cloudfoundry.org/lager/lagerflags"
	"code.cloudfoundry.org/locket"
)

type Duration time.Duration

func (d *Duration) UnmarshalJSON(data []byte) error {
	var s string
	err := json.Unmarshal(data, &s)
	if err != nil {
		return err
	}

	dur, err := time.ParseDuration(s)
	if err != nil {
		return err
	}

	*d = Duration(dur)
	return nil
}

func (d Duration) MarshalJSON() ([]byte, error) {
	t := time.Duration(d)
	return []byte(fmt.Sprintf(`"%s"`, t.String())), nil
}

type WatcherConfig struct {
	BBSAddress                string                        `json:"bbs_api_url"`
	BBSCACert                 string                        `json:"bbs_ca_cert"`
	BBSClientCert             string                        `json:"bbs_client_cert"`
	BBSClientKey              string                        `json:"bbs_client_key"`
	BBSClientSessionCacheSize int                           `json:"bbs_client_cache_size"`
	BBSMaxIdleConnsPerHost    int                           `json:"bbs_max_idle_conns_per_host"`
	CCBaseUrl                 string                        `json:"cc_base_url"`
	ConsulCluster             string                        `json:"consul_cluster"`
	DebugServerConfig         debugserver.DebugServerConfig `json:"debug_server_config"`
	DropsondePort             int                           `json:"dropsonde_port"`
	LagerConfig               lagerflags.LagerConfig        `json:"lager_config"`
	LockRetryInterval         Duration                      `json:"lock_retry_interval"`
	LockTTL                   Duration                      `json:"lock_ttl"`
	MaxEventHandlingWorkers   int                           `json:"max_event_handling_workers"`
	CCClientCert              string                        `json:"cc_client_cert"`
	CCClientKey               string                        `json:"cc_client_key"`
	CCCACert                  string                        `json:"cc_ca_cert"`
	InstanceID                string                        `json:"instance_id"`

	SkipConsulLock bool `json:"skip_consul_lock"`

	locket.ClientLocketConfig
}

func DefaultWatcherConfig() WatcherConfig {
	return WatcherConfig{
		BBSClientSessionCacheSize: 0,
		BBSMaxIdleConnsPerHost:    0,
		DropsondePort:             3457,
		LagerConfig:               lagerflags.DefaultLagerConfig(),
		MaxEventHandlingWorkers:   500,
		LockRetryInterval:         Duration(locket.RetryInterval),
		LockTTL:                   Duration(locket.DefaultSessionTTL),
		SkipConsulLock:            false,
	}
}

func NewWatcherConfig(configPath string) (WatcherConfig, error) {
	configFile, err := ioutil.ReadFile(configPath)
	if err != nil {
		return WatcherConfig{}, err
	}

	watcherConfig := DefaultWatcherConfig()
	err = json.Unmarshal(configFile, &watcherConfig)
	if err != nil {
		return WatcherConfig{}, err
	}

	return watcherConfig, nil
}
