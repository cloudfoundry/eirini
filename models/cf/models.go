package cf

import (
	"encoding/json"

	"code.cloudfoundry.org/bbs/models"
)

const (
	VcapAppName   = "application_name"
	VcapVersion   = "version"
	VcapAppUris   = "application_uris"
	VcapAppID     = "application_id"
	VcapSpaceName = "space_name"

	LastUpdated = "last_updated"
	ProcessGUID = "process_guid"
)

type VcapApp struct {
	AppName   string   `json:"application_name"`
	AppID     string   `json:"application_id"`
	Version   string   `json:"version"`
	AppUris   []string `json:"application_uris"`
	SpaceName string   `json:"space_name"`
}

type VolumeMount struct {
	VolumeID string `json:"volume_id"`
	MountDir string `json:"mount_dir"`
}

type DesireLRPRequest struct {
	GUID                    string                      `json:"guid"`
	Version                 string                      `json:"version"`
	ProcessGUID             string                      `json:"process_guid"`
	ProcessType             string                      `json:"process_type"`
	AppGUID                 string                      `json:"app_guid"`
	AppName                 string                      `json:"app_name"`
	SpaceGUID               string                      `json:"space_guid"`
	SpaceName               string                      `json:"space_name"`
	OrganizationGUID        string                      `json:"organization_guid"`
	OrganizationName        string                      `json:"organization_name"`
	PlacementTags           []string                    `json:"placement_tags"`
	Ports                   []int32                     `json:"ports"`
	Routes                  map[string]*json.RawMessage `json:"routes"`
	Environment             map[string]string           `json:"environment"`
	EgressRules             []json.RawMessage           `json:"egress_rules"`
	NumInstances            int                         `json:"instances"`
	LastUpdated             string                      `json:"last_updated"`
	HealthCheckType         string                      `json:"health_check_type"`
	HealthCheckHTTPEndpoint string                      `json:"health_check_http_endpoint"`
	HealthCheckTimeoutMs    uint                        `json:"health_check_timeout_ms"`
	StartTimeoutMs          uint                        `json:"start_timeout_ms"`
	MemoryMB                int64                       `json:"memory_mb"`
	DiskMB                  int64                       `json:"disk_mb"`
	CPUWeight               uint8                       `json:"cpu_weight"`
	VolumeMounts            []VolumeMount               `json:"volume_mounts"`
	Lifecycle               Lifecycle                   `json:"lifecycle"`
	DropletHash             string                      `json:"droplet_hash"`
	DropletGUID             string                      `json:"droplet_guid"`
	StartCommand            string                      `json:"start_command"`
	LRP                     string
}

type Lifecycle struct {
	DockerLifecycle    *DockerLifecycle    `json:"docker_lifecycle"`
	BuildpackLifecycle *BuildpackLifecycle `json:"buildpack_lifecycle"`
}

type DockerLifecycle struct {
	Image   string   `json:"image"`
	Command []string `json:"command"`
}

type BuildpackLifecycle struct {
	DropletHash  string `json:"droplet_hash"`
	DropletGUID  string `json:"droplet_guid"`
	StartCommand string `json:"start_command"`
}

type StagingRequest struct {
	AppGUID            string                `json:"app_guid"`
	CompletionCallback string                `json:"completion_callback"`
	Environment        []EnvironmentVariable `json:"environment"`
	LifecycleData      LifecycleData         `json:"lifecycle_data"`
}

type LifecycleData struct {
	AppBitsDownloadURI string      `json:"app_bits_download_uri"`
	DropletUploadURI   string      `json:"droplet_upload_uri"`
	Buildpacks         []Buildpack `json:"buildpacks"`
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
	models.UpdateDesiredLRPRequest
	GUID    string `json:"guid"`
	Version string `json:"version"`
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

type StagingError struct {
	Message string `json:"message"`
}
