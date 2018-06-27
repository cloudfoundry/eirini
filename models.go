package eirini

import (
	"context"
	"fmt"
	"net/http"

	"code.cloudfoundry.org/bbs/models"
	"code.cloudfoundry.org/eirini/models/cf"
	"code.cloudfoundry.org/eirini/opi"
	"code.cloudfoundry.org/runtimeschema/cc_messages"
)

//Environment Variable Names
const (
	EnvDownloadURL        = "DOWNLOAD_URL"
	EnvUploadURL          = "UPLOAD_URL"
	EnvAppID              = "APP_ID"
	EnvStagingGUID        = "STAGING_GUID"
	EnvCompletionCallback = "COMPLETION_CALLBACK"
	EnvCfUsername         = "CF_USERNAME"
	EnvCfPassword         = "CF_PASSWORD"
	EnvAPIAddress         = "API_ADDRESS"
	EnvEiriniAddress      = "EIRINI_ADDRESS"
)

//go:generate counterfeiter . CfClient
type CfClient interface {
	GetDropletByAppGuid(string) ([]byte, error)
	PushDroplet(string, string) error
	GetAppBitsByAppGuid(string) (*http.Response, error)
}

type Config struct {
	Properties Properties `yaml:"opi"`
}

type Properties struct {
	KubeConfig         string `yaml:"kube_config"`
	KubeNamespace      string `yaml:"kube_namespace"`
	KubeEndpoint       string `yaml:"kube_endpoint"`
	NatsPassword       string `yaml:"nats_password"`
	NatsIP             string `yaml:"nats_ip"`
	RegistryEndpoint   string `yaml:"registry_endpoint"`
	CcAPI              string `yaml:"api_endpoint"`
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
	APIAddress        string
	EiriniAddress     string
	SkipSslValidation bool
}

//go:generate counterfeiter . Extractor
type Extractor interface {
	Extract(src, targetDir string) error
}

//go:generate counterfeiter . Bifrost
type Bifrost interface {
	Transfer(ctx context.Context, request cf.DesireLRPRequest) error
	List(ctx context.Context) ([]*models.DesiredLRPSchedulingInfo, error)
	Update(ctx context.Context, update models.UpdateDesiredLRPRequest) error
	Stop(ctx context.Context, guid string) error
	GetApp(ctx context.Context, guid string) *models.DesiredLRP
	GetInstances(ctx context.Context, guid string) ([]*cf.Instance, error)
}

func GetInternalServiceName(appName string) string {
	//Prefix service as the appName could start with numerical characters, which is not allowed
	return fmt.Sprintf("cf-%s", appName)
}
