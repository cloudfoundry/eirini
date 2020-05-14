package cf

import (
	"encoding/json"
)

type VolumeMount struct {
	VolumeID string `json:"volume_id"`
	MountDir string `json:"mount_dir"`
}

type DesiredLRP struct {
	ProcessGUID string                     `json:"process_guid"`
	Instances   int32                      `json:"instances"`
	Routes      map[string]json.RawMessage `json:"routes,omitempty"`
	Annotation  string                     `json:"annotation"`
}

type DesiredLRPResponse struct {
	DesiredLRP DesiredLRP `json:"desired_lrp"`
}

type DesireLRPRequest struct {
	GUID                    string                     `json:"guid"`
	Version                 string                     `json:"version"`
	ProcessGUID             string                     `json:"process_guid"`
	ProcessType             string                     `json:"process_type"`
	AppGUID                 string                     `json:"app_guid"`
	AppName                 string                     `json:"app_name"`
	SpaceGUID               string                     `json:"space_guid"`
	SpaceName               string                     `json:"space_name"`
	OrganizationGUID        string                     `json:"organization_guid"`
	OrganizationName        string                     `json:"organization_name"`
	PlacementTags           []string                   `json:"placement_tags"`
	Ports                   []int32                    `json:"ports"`
	Routes                  map[string]json.RawMessage `json:"routes"`
	Environment             map[string]string          `json:"environment"`
	EgressRules             []json.RawMessage          `json:"egress_rules"`
	NumInstances            int                        `json:"instances"`
	LastUpdated             string                     `json:"last_updated"`
	HealthCheckType         string                     `json:"health_check_type"`
	HealthCheckHTTPEndpoint string                     `json:"health_check_http_endpoint"`
	HealthCheckTimeoutMs    uint                       `json:"health_check_timeout_ms"`
	StartTimeoutMs          uint                       `json:"start_timeout_ms"`
	MemoryMB                int64                      `json:"memory_mb"`
	DiskMB                  int64                      `json:"disk_mb"`
	CPUWeight               uint8                      `json:"cpu_weight"`
	VolumeMounts            []VolumeMount              `json:"volume_mounts"`
	Lifecycle               Lifecycle                  `json:"lifecycle"`
	DropletHash             string                     `json:"droplet_hash"`
	DropletGUID             string                     `json:"droplet_guid"`
	StartCommand            string                     `json:"start_command"`
	UserDefinedAnnotations  map[string]string          `json:"user_defined_annotations"`
	LRP                     string
}

type DesiredLRPSchedulingInfo struct {
	DesiredLRPKey `json:"desired_lrp_key"`
	Annotation    string `json:"annotation"`
}

type DesiredLRPKey struct {
	ProcessGUID string `json:"process_guid"`
}

type DesiredLRPSchedulingInfosResponse struct {
	DesiredLrpSchedulingInfos []DesiredLRPSchedulingInfo `json:"desired_lrp_scheduling_infos"`
}

type DesiredLRPLifecycleResponse struct {
	Error Error `json:"error,omitempty"`
}

type Lifecycle struct {
	DockerLifecycle    *DockerLifecycle    `json:"docker_lifecycle"`
	BuildpackLifecycle *BuildpackLifecycle `json:"buildpack_lifecycle"`
}

type DockerLifecycle struct {
	Image            string   `json:"image"`
	Command          []string `json:"command"`
	RegistryUsername string   `json:"registry_username"`
	RegistryPassword string   `json:"registry_password"`
}

type BuildpackLifecycle struct {
	DropletHash  string `json:"droplet_hash"`
	DropletGUID  string `json:"droplet_guid"`
	StartCommand string `json:"start_command"`
}

type TaskRequest struct {
	GUID               string                `json:"guid"`
	Name               string                `json:"name"`
	AppGUID            string                `json:"app_guid"`
	AppName            string                `json:"app_name"`
	OrgName            string                `json:"org_name"`
	OrgGUID            string                `json:"org_guid"`
	SpaceName          string                `json:"space_name"`
	SpaceGUID          string                `json:"space_guid"`
	CompletionCallback string                `json:"completion_callback"`
	Environment        []EnvironmentVariable `json:"environment"`
	Lifecycle          Lifecycle             `json:"lifecycle"`
}

type TaskCompletedRequest struct {
	TaskGUID      string `json:"task_guid"`
	Failed        bool   `json:"failed"`
	FailureReason string `json:"failure_reason"`
	Result        string `json:"result"`
	Annotation    string `json:"annotation,omitempty"`
}

type StagingRequest struct {
	AppGUID            string                     `json:"app_guid"`
	AppName            string                     `json:"app_name"`
	OrgName            string                     `json:"org_name"`
	OrgGUID            string                     `json:"org_guid"`
	SpaceName          string                     `json:"space_name"`
	SpaceGUID          string                     `json:"space_guid"`
	CompletionCallback string                     `json:"completion_callback"`
	Environment        []EnvironmentVariable      `json:"environment"`
	LifecycleData      *StagingBuildpackLifecycle `json:"lifecycle_data"`
	Lifecycle          StagingLifecycle           `json:"lifecycle"`
	MemoryMB           int64                      `json:"memory_mb"`
	DiskMB             int64                      `json:"disk_mb"`
	CPUWeight          uint8                      `json:"cpu_weight"`
}

type StagingLifecycle struct {
	DockerLifecycle    *StagingDockerLifecycle    `json:"docker_lifecycle"`
	BuildpackLifecycle *StagingBuildpackLifecycle `json:"buildpack_lifecycle"`
}

type StagingDockerLifecycle struct {
	Image            string `json:"image"`
	RegistryUsername string `json:"registry_username"`
	RegistryPassword string `json:"registry_password"`
}

type StagingBuildpackLifecycle struct {
	AppBitsDownloadURI              string      `json:"app_bits_download_uri"`
	DropletUploadURI                string      `json:"droplet_upload_uri"`
	Buildpacks                      []Buildpack `json:"buildpacks"`
	BuildpackCacheDownloadURI       string      `json:"buildpack_cache_download_uri"`
	BuildpackCacheUploadURI         string      `json:"buildpack_cache_upload_uri"`
	BuildpackCacheChecksum          string      `json:"buildpack_cache_checksum"`
	BuildpackCacheChecksumAlgorithm string      `json:"buildpack_cache_checksum_algorithm"`
}

type Buildpack struct {
	Name       string `json:"name"`
	Key        string `json:"key"`
	URL        string `json:"url"`
	SkipDetect bool   `json:"skip_detect"`
}

type EnvironmentVariable struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

type UpdateDesiredLRPRequest struct {
	GUID    string           `json:"guid"`
	Version string           `json:"version"`
	Update  DesiredLRPUpdate `json:"update,omitempty"`
}

type DesiredLRPUpdate struct {
	Instances  int                        `json:"instances"`
	Routes     map[string]json.RawMessage `json:"routes"`
	Annotation string                     `json:"annotation"`
}

type GetInstancesResponse struct {
	Error       string      `json:"error,omitempty"`
	ProcessGUID string      `json:"process_guid"`
	Instances   []*Instance `json:"instances"`
}

type Instance struct {
	Index          int    `json:"index"`
	Since          int64  `json:"since"`
	State          string `json:"state"`
	PlacementError string `json:"placement_error,omitempty"`
}

type Route struct {
	Hostname string `json:"hostname"`
	Port     int32  `json:"port"`
}

type AppCrashedRequest struct {
	Instance        string `json:"instance"`
	Index           int    `json:"index"`
	Reason          string `json:"reason"`
	ExitStatus      int    `json:"exit_status,omitempty"`
	ExitDescription string `json:"exit_description,omitempty"`
	CrashCount      int    `json:"crash_count"`
	CrashTimestamp  int64  `json:"crash_timestamp"`
}

type Error struct {
	Message string `json:"message"`
}
