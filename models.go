package eirini

import (
	"fmt"
	"net/http"

	"code.cloudfoundry.org/bbs/models"
	"code.cloudfoundry.org/runtimeschema/cc_messages"
	"github.com/cloudfoundry-incubator/eirini/opi"
)

//Environment Variable Names
const (
	EnvDownloadUrl        = "DOWNLOAD_URL"
	EnvUploadUrl          = "UPLOAD_URL"
	EnvAppId              = "APP_ID"
	EnvStagingGuid        = "STAGING_GUID"
	EnvCompletionCallback = "COMPLETION_CALLBACK"
	EnvCfUsername         = "CF_USERNAME"
	EnvCfPassword         = "CF_PASSWORD"
	EnvApiAddress         = "API_ADDRESS"
	EnvEiriniAddress      = "EIRINI_ADDRESS"
)

type AppInfo struct {
	AppName   string `json:"name"`
	SpaceName string `json:"space_name"`
	AppGuid   string `json:"application_id"`
}

//go:generate counterfeiter . CfClient
type CfClient interface {
	GetDropletByAppGuid(string) ([]byte, error)
	PushDroplet(string, string) error
	GetAppBitsByAppGuid(string) (*http.Response, error)
}

type SyncConfig struct {
	Properties SyncProperties `yaml:"sync"`
}

type SyncProperties struct {
	KubeConfig         string `yaml:"kube_config"`
	KubeNamespace      string `yaml:"kube_namespace"`
	KubeEndpoint       string `yaml:"kube_endpoint"`
	RegistryEndpoint   string `yaml:"registry_endpoint"`
	CcApi              string `yaml:"api_endpoint"`
	Backend            string `yaml:"backend"`
	CfUsername         string `yaml:"cf_username"`
	CfPassword         string `yaml:"cf_password"`
	CcUser             string `yaml:"cc_internal_user"`
	CcPassword         string `yaml:"cc_internal_password"`
	ExternalAddress    string `yaml:"external_eirini_address"`
	SkipSslValidation  bool   `yaml:"skip_ssl_validation"`
	InsecureSkipVerify bool   `yaml:"insecure_skip_verify"`
}

//go:generate counterfeiter . St8ger
type St8ger interface {
	Run(task opi.Task) error
}

//go:generate counterfeiter . Backend
type Backend interface {
	CreateStagingTask(string, cc_messages.StagingRequestFromCC) (opi.Task, error)
	BuildStagingResponse(*models.TaskCallbackResponse) (cc_messages.StagingResponseForCC, error)
}

type BackendConfig struct {
	CfUsername        string
	CfPassword        string
	ApiAddress        string
	EiriniAddress     string
	SkipSslValidation bool
}

//go:generate counterfeiter . Extractor
type Extractor interface {
	Extract(src, targetDir string) error
}

func GetInternalServiceName(appName string) string {
	//Prefix service as the appName could start with numerical characters, which is not allowed
	return fmt.Sprintf("cf-%s", appName)
}
