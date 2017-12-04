package cc_messages

import (
	"encoding/json"

	"code.cloudfoundry.org/bbs/models"
)

type StagingErrorID string

const (
	STAGING_ERROR            StagingErrorID = "StagingError"
	INSUFFICIENT_RESOURCES   StagingErrorID = "InsufficientResources"
	NO_COMPATIBLE_CELL       StagingErrorID = "NoCompatibleCell"
	CELL_COMMUNICATION_ERROR StagingErrorID = "CellCommunicationError"
	BUILDPACK_DETECT_FAILED  StagingErrorID = "NoAppDetectedError"
	BUILDPACK_COMPILE_FAILED StagingErrorID = "BuildpackCompileFailed"
	BUILDPACK_RELEASE_FAILED StagingErrorID = "BuildpackReleaseFailed"
)

type StagingError struct {
	Id      StagingErrorID `json:"id"`
	Message string         `json:"message"`
}

type StagingRequestFromCC struct {
	AppId              string                        `json:"app_id"`
	FileDescriptors    int                           `json:"file_descriptors"`
	MemoryMB           int                           `json:"memory_mb"`
	DiskMB             int                           `json:"disk_mb"`
	Environment        []*models.EnvironmentVariable `json:"environment"`
	EgressRules        []*models.SecurityGroupRule   `json:"egress_rules,omitempty"`
	Timeout            int                           `json:"timeout"`
	LogGuid            string                        `json:"log_guid"`
	Lifecycle          string                        `json:"lifecycle"`
	LifecycleData      *json.RawMessage              `json:"lifecycle_data,omitempty"`
	CompletionCallback string                        `json:"completion_callback"`
	IsolationSegment   string                        `json:"isolation_segment"`
}

type BuildpackStagingData struct {
	AppBitsDownloadUri             string      `json:"app_bits_download_uri"`
	BuildArtifactsCacheDownloadUri string      `json:"build_artifacts_cache_download_uri,omitempty"`
	BuildArtifactsCacheUploadUri   string      `json:"build_artifacts_cache_upload_uri"`
	Buildpacks                     []Buildpack `json:"buildpacks"`
	DropletUploadUri               string      `json:"droplet_upload_uri"`
	Stack                          string      `json:"stack"`
}

type DockerStagingData struct {
	DockerImageUrl    string `json:"docker_image"`
	DockerLoginServer string `json:"docker_login_server,omitempty"`
	DockerUser        string `json:"docker_user,omitempty"`
	DockerPassword    string `json:"docker_password,omitempty"`
	DockerEmail       string `json:"docker_email,omitempty"`
}

const CUSTOM_BUILDPACK = "custom"

type Buildpack struct {
	Name       string `json:"name"`
	Key        string `json:"key"`
	Url        string `json:"url"`
	SkipDetect bool   `json:"skip_detect"`
}

type StagingResponseForCC struct {
	Error  *StagingError    `json:"error,omitempty"`
	Result *json.RawMessage `json:"result,omitempty"`
}

type StagingTaskAnnotation struct {
	Lifecycle          string `json:"lifecycle"`
	CompletionCallback string `json:"completion_callback"`
}
