package eirini

import "errors"

const (
	// Environment Variable Names
	EnvEiriniNamespace            = "EIRINI_NAMESPACE"
	EnvDownloadURL                = "DOWNLOAD_URL"
	EnvDropletUploadURL           = "DROPLET_UPLOAD_URL"
	EnvAppID                      = "APP_ID"
	EnvCompletionCallback         = "COMPLETION_CALLBACK"
	EnvEiriniAddress              = "EIRINI_ADDRESS"
	EnvInstanceEnvInjectorCertDir = "INSTANCE_ENV_INJECTOR_CERTS_DIR"
	EnvLoggregatorCertDir         = "LOGGREGATOR_CERTS_DIR"
	EnvCCCertDir                  = "CC_CERTS_DIR"
	EnvServerCertDir              = "SERVER_CERTS_DIR"

	EnvPodName              = "POD_NAME"
	EnvCFInstanceIP         = "CF_INSTANCE_IP"
	EnvCFInstanceIndex      = "CF_INSTANCE_INDEX"
	EnvCFInstanceGUID       = "CF_INSTANCE_GUID"
	EnvCFInstanceInternalIP = "CF_INSTANCE_INTERNAL_IP"
	EnvCFInstanceAddr       = "CF_INSTANCE_ADDR"
	EnvCFInstancePort       = "CF_INSTANCE_PORT"
	EnvCFInstancePorts      = "CF_INSTANCE_PORTS"

	AppMetricsEmissionIntervalInSecs = 15

	RegistrySecretName = "default-image-pull-secret"

	// Certs
	TLSSecretKey  = "tls.key"
	TLSSecretCert = "tls.crt"
	TLSSecretCA   = "tls.ca"

	EiriniCrtDir       = "/etc/eirini/certs/"
	CCCrtDir           = "/etc/cf-api/certs/"
	LoggregatorCertDir = "/etc/loggregator/certs/"

	CCUploaderSecretName   = "cc-uploader-certs"   //#nosec G101
	EiriniClientSecretName = "eirini-client-certs" //#nosec G101

	InstanceEnvInjectorCertDir = "/etc/eirini/certs"
)

var ErrNotFound = errors.New("not found")

var ErrInvalidInstanceIndex = errors.New("invalid instance index")

type Config struct {
	Properties              Properties `yaml:"opi"`
	WorkloadsNamespace      string
	LeaderElectionID        string
	LeaderElectionNamespace string
}

type KubeConfig struct {
	ConfigPath string `yaml:"kube_config_path"`
}

type Properties struct { //nolint:maligned
	DefaultWorkloadsNamespace string `yaml:"app_namespace"`
	TLSPort                   int    `yaml:"tls_port"`
	PlaintextPort             int    `yaml:"plaintext_port"`

	RegistrySecretName               string `yaml:"registry_secret_name"`
	AppMetricsEmissionIntervalInSecs int    `yaml:"app_metrics_emission_interval_in_secs"`

	CCTLSDisabled bool `yaml:"cc_tls_disabled"`

	KubeConfig `yaml:",inline"`

	ApplicationServiceAccount string `yaml:"application_service_account"`

	AllowRunImageAsRoot                     bool `yaml:"allow_run_image_as_root"`
	UnsafeAllowAutomountServiceAccountToken bool `yaml:"unsafe_allow_automount_service_account_token"`

	DefaultMinAvailableInstances string `yaml:"default_min_available_instances"`

	ServePlaintext bool `yaml:"serve_plaintext"`

	PrometheusPort int `yaml:"prometheus_port"`
}

type EventReporterConfig struct {
	CcInternalAPI string `yaml:"cc_internal_api"`
	CCTLSDisabled bool   `yaml:"cc_tls_disabled"`

	WorkloadsNamespace      string
	LeaderElectionID        string
	LeaderElectionNamespace string

	KubeConfig `yaml:",inline"`
}

type RouteEmitterConfig struct {
	NatsPassword        string `yaml:"nats_password"`
	NatsIP              string `yaml:"nats_ip"`
	NatsPort            int    `yaml:"nats_port"`
	EmitPeriodInSeconds uint   `yaml:"emit_period_in_seconds"`
	WorkloadsNamespace  string

	KubeConfig `yaml:",inline"`
}

type MetricsCollectorConfig struct {
	LoggregatorAddress string `yaml:"loggregator_address"`

	WorkloadsNamespace string

	AppMetricsEmissionIntervalInSecs int `yaml:"app_metrics_emission_interval_in_secs"`

	KubeConfig `yaml:",inline"`
}

type TaskReporterConfig struct {
	CCTLSDisabled                bool `yaml:"cc_tls_disabled"`
	LeaderElectionID             string
	LeaderElectionNamespace      string
	CompletionCallbackRetryLimit int `yaml:"completion_callback_retry_limit"`
	TTLSeconds                   int `yaml:"ttl_seconds"`

	WorkloadsNamespace string

	KubeConfig `yaml:",inline"`
}

type InstanceIndexEnvInjectorConfig struct {
	Port       int32 `yaml:"service_port"`
	KubeConfig `yaml:",inline"`
}
