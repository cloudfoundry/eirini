package cube

type AppInfo struct {
	AppName   string `json:"name"`
	SpaceName string `json:"space_name"`
	AppGuid   string `json:"application_id"`
}

//go:generate counterfeiter . CfClient
type CfClient interface {
	GetDropletByAppGuid(string) ([]byte, error)
}

type SyncConfig struct {
	Properties SyncProperties `yaml:"sync"`
}

type SyncProperties struct {
	KubeConfig         string `yaml:"kube_config"`
	RegistryEndpoint   string `yaml:"registry_endpoint"`
	CcApi              string `yaml:"api_endpoint"`
	Backend            string `yaml:"backend"`
	CfUsername         string `yaml:"cf_username"`
	CfPassword         string `yaml:"cf_password"`
	CcUser             string `yaml:"cc_internal_user"`
	CcPassword         string `yaml:"cc_internal_password"`
	SkipSslValidation  bool   `yaml:"skip_ssl_validation"`
	InsecureSkipVerify bool   `yaml:"insecure_skip_verify"`
}
