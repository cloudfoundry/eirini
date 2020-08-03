package eirini

import "errors"

const (
	//Environment Variable Names
	EnvEiriniNamespace                 = "EIRINI_NAMESPACE"
	EnvDownloadURL                     = "DOWNLOAD_URL"
	EnvBuildpacks                      = "BUILDPACKS"
	EnvDropletUploadURL                = "DROPLET_UPLOAD_URL"
	EnvAppID                           = "APP_ID"
	EnvStagingGUID                     = "STAGING_GUID"
	EnvCompletionCallback              = "COMPLETION_CALLBACK"
	EnvEiriniAddress                   = "EIRINI_ADDRESS"
	EnvBuildpackCacheUploadURI         = "BUILDPACK_CACHE_UPLOAD_URI"
	EnvBuildpackCacheDownloadURI       = "BUILDPACK_CACHE_DOWNLOAD_URI"
	EnvBuildpackCacheChecksum          = "BUILDPACK_CACHE_CHECKSUM"
	EnvBuildpackCacheChecksumAlgorithm = "BUILDPACK_CACHE_CHECKSUM_ALGORITHM"
	EnvBuildpackCacheArtifactsDir      = "EIRINI_BUILD_ARTIFACTS_CACHE_DIR"
	EnvBuildpackCacheOutputArtifact    = "EIRINI_OUTPUT_BUILD_ARTIFACTS_CACHE"

	EnvPodName              = "POD_NAME"
	EnvCFInstanceIP         = "CF_INSTANCE_IP"
	EnvCFInstanceGUID       = "CF_INSTANCE_GUID"
	EnvCFInstanceInternalIP = "CF_INSTANCE_INTERNAL_IP"
	EnvCFInstanceAddr       = "CF_INSTANCE_ADDR"
	EnvCFInstancePort       = "CF_INSTANCE_PORT"
	EnvCFInstancePorts      = "CF_INSTANCE_PORTS"

	RecipeBuildPacksDir    = "/var/lib/buildpacks"
	RecipeBuildPacksName   = "recipe-buildpacks"
	RecipeWorkspaceDir     = "/recipe_workspace"
	RecipeWorkspaceName    = "recipe-workspace"
	RecipeOutputName       = "staging-output"
	RecipeOutputLocation   = "/out"
	RecipePacksBuilderPath = "/packs/builder"
	BuildpackCacheDir      = "/tmp"
	BuildpackCacheName     = "buildpack-cache"

	AppMetricsEmissionIntervalInSecs = 15

	//Staging TLS:
	CertsMountPath   = "/etc/config/certs"
	CertsVolumeName  = "certs-volume"
	CACertName       = "internal-ca-cert"
	CCAPICertName    = "cc-server-crt"
	CCAPIKeyName     = "cc-server-crt-key"
	EiriniClientCert = "eirini-client-crt"
	EiriniClientKey  = "eirini-client-crt-key"

	RegistrySecretName = "default-image-pull-secret"
)

var ErrNotFound = errors.New("not found")

var ErrInvalidInstanceIndex = errors.New("invalid instance index")

type Config struct {
	Properties Properties `yaml:"opi"`
}

type KubeConfig struct {
	Namespace  string `yaml:"app_namespace"`
	ConfigPath string `yaml:"kube_config_path"`
}

type Properties struct { //nolint:maligned
	ClientCAPath   string `yaml:"client_ca_path"`
	ServerCertPath string `yaml:"server_cert_path"`
	ServerKeyPath  string `yaml:"server_key_path"`
	TLSPort        int    `yaml:"tls_port"`
	PlaintextPort  int    `yaml:"plaintext_port"`

	CCUploaderSecretName string `yaml:"cc_uploader_secret_name"`
	CCUploaderCertPath   string `yaml:"cc_uploader_cert_path"`
	CCUploaderKeyPath    string `yaml:"cc_uploader_key_path"`

	ClientCertsSecretName string `yaml:"client_certs_secret_name"`
	ClientCertPath        string `yaml:"client_cert_path"`
	ClientKeyPath         string `yaml:"client_key_path"`

	CACertSecretName string `yaml:"ca_cert_secret_name"`
	CACertPath       string `yaml:"ca_cert_path"`

	RegistryAddress                  string `yaml:"registry_address"`
	RegistrySecretName               string `yaml:"registry_secret_name"`
	EiriniAddress                    string `yaml:"eirini_address"`
	DownloaderImage                  string `yaml:"downloader_image"`
	UploaderImage                    string `yaml:"uploader_image"`
	ExecutorImage                    string `yaml:"executor_image"`
	AppMetricsEmissionIntervalInSecs int    `yaml:"app_metrics_emission_interval_in_secs"`

	CCTLSDisabled bool   `yaml:"cc_tls_disabled"`
	CCCertPath    string `yaml:"cc_cert_path"`
	CCKeyPath     string `yaml:"cc_key_path"`
	CCCAPath      string `yaml:"cc_ca_path"`

	RootfsVersion string `yaml:"rootfs_version"`
	DiskLimitMB   int64  `yaml:"disk_limit_mb"`

	KubeConfig `yaml:",inline"`

	ApplicationServiceAccount string `yaml:"application_service_account"`
	StagingServiceAccount     string `yaml:"staging_service_account"`

	AllowRunImageAsRoot                     bool `yaml:"allow_run_image_as_root"`
	UnsafeAllowAutomountServiceAccountToken bool `yaml:"unsafe_allow_automount_service_account_token"`

	EiriniInstance string `yaml:"eirini_instance,omitempty"`

	ServePlaintext bool `yaml:"serve_plaintext"`
}

type EventReporterConfig struct {
	CcInternalAPI string `yaml:"cc_internal_api"`
	CCTLSDisabled bool   `yaml:"cc_tls_disabled"`
	CCCertPath    string `yaml:"cc_cert_path"`
	CCKeyPath     string `yaml:"cc_key_path"`
	CCCAPath      string `yaml:"cc_ca_path"`

	KubeConfig `yaml:",inline"`
}

type RouteEmitterConfig struct {
	NatsPassword        string `yaml:"nats_password"`
	NatsIP              string `yaml:"nats_ip"`
	NatsPort            int    `yaml:"nats_port"`
	EmitPeriodInSeconds uint   `yaml:"emit_period_in_seconds"`

	KubeConfig `yaml:",inline"`
}

type MetricsCollectorConfig struct {
	LoggregatorAddress  string `yaml:"loggregator_address"`
	LoggregatorCertPath string `yaml:"loggregator_cert_path"`
	LoggregatorKeyPath  string `yaml:"loggregator_key_path"`
	LoggregatorCAPath   string `yaml:"loggregator_ca_path"`

	AppMetricsEmissionIntervalInSecs int `yaml:"app_metrics_emission_interval_in_secs"`

	KubeConfig `yaml:",inline"`
}

type TaskReporterConfig struct {
	CCTLSDisabled  bool   `yaml:"cc_tls_disabled"`
	CCCertPath     string `yaml:"cc_cert_path"`
	CCKeyPath      string `yaml:"cc_key_path"`
	CAPath         string `yaml:"ca_path"`
	EiriniInstance string `yaml:"eirini_instance,omitempty"`

	KubeConfig `yaml:",inline"`
}

type StagingReporterConfig struct {
	EiriniCertPath string `yaml:"eirini_cert_path"`
	EiriniKeyPath  string `yaml:"eirini_key_path"`
	CAPath         string `yaml:"ca_path"`

	KubeConfig `yaml:",inline"`
}

type StagerConfig struct {
	EiriniAddress   string
	DownloaderImage string
	UploaderImage   string
	ExecutorImage   string
}
