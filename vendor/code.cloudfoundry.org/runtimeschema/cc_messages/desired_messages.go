package cc_messages

import (
	"encoding/json"

	"code.cloudfoundry.org/bbs/models"
)

type HealthCheckType string

const UnspecifiedHealthCheckType HealthCheckType = "" // backwards-compatibility
const HTTPHealthCheckType HealthCheckType = "http"
const PortHealthCheckType HealthCheckType = "port"
const NoneHealthCheckType HealthCheckType = "none"

const CC_HTTP_ROUTES = "http_routes"

const CC_TCP_ROUTES = "tcp_routes"

const (
	TaskStatePending   = "PENDING"
	TaskStateRunning   = "RUNNING"
	TaskStateCanceling = "CANCELING"
	TaskStateSucceeded = "SUCCEEDED"
)

type DesireAppRequestFromCC struct {
	ProcessGuid                 string                        `json:"process_guid"`
	DropletUri                  string                        `json:"droplet_uri"`
	DropletHash                 string                        `json:"droplet_hash"`
	DockerImageUrl              string                        `json:"docker_image"`
	DockerLoginServer           string                        `json:"docker_login_server,omitempty"`
	DockerUser                  string                        `json:"docker_user,omitempty"`
	DockerPassword              string                        `json:"docker_password,omitempty"`
	DockerEmail                 string                        `json:"docker_email,omitempty"`
	Stack                       string                        `json:"stack"`
	StartCommand                string                        `json:"start_command"`
	ExecutionMetadata           string                        `json:"execution_metadata"`
	Environment                 []*models.EnvironmentVariable `json:"environment"`
	MemoryMB                    int                           `json:"memory_mb"`
	DiskMB                      int                           `json:"disk_mb"`
	FileDescriptors             uint64                        `json:"file_descriptors"`
	NumInstances                int                           `json:"num_instances"`
	RoutingInfo                 CCRouteInfo                   `json:"routing_info"`
	AllowSSH                    bool                          `json:"allow_ssh"`
	LogGuid                     string                        `json:"log_guid"`
	HealthCheckType             HealthCheckType               `json:"health_check_type"`
	HealthCheckHTTPEndpoint     string                        `json:"health_check_http_endpoint"`
	HealthCheckTimeoutInSeconds uint                          `json:"health_check_timeout_in_seconds"`
	EgressRules                 []*models.SecurityGroupRule   `json:"egress_rules,omitempty"`
	ETag                        string                        `json:"etag"`
	Ports                       []uint32                      `json:"ports,omitempty"`
	LogSource                   string                        `json:"log_source,omitempty"`
	Network                     *models.Network               `json:"network,omitempty"`
	VolumeMounts                []*VolumeMount                `json:"volume_mounts"`
	IsolationSegment            string                        `json:"isolation_segment"`
}

type CCRouteInfo map[string]*json.RawMessage

type CCHTTPRoutes []CCHTTPRoute

type VolumeMount struct {
	Driver       string       `json:"driver"`
	ContainerDir string       `json:"container_dir"`
	Mode         string       `json:"mode"`
	DeviceType   string       `json:"device_type"`
	Device       SharedDevice `json:"device"`
}

type SharedDevice struct {
	VolumeId    string                 `json:"volume_id"`
	MountConfig map[string]interface{} `json:"mount_config,omitempty"`
}

func (r CCHTTPRoutes) CCRouteInfo() (CCRouteInfo, error) {
	routesJson, err := json.Marshal(r)
	if err != nil {
		return nil, err
	}

	routesPayload := json.RawMessage(routesJson)
	routingInfo := make(map[string]*json.RawMessage)
	routingInfo[CC_HTTP_ROUTES] = &routesPayload
	return routingInfo, nil
}

type CCHTTPRoute struct {
	Hostname        string `json:"hostname"`
	RouteServiceUrl string `json:"route_service_url,omitempty"`
	Port            uint32 `json:"port,omitempty"`
}

type CCTCPRoutes []CCTCPRoute

type CCTCPRoute struct {
	RouterGroupGuid string `json:"router_group_guid"`
	ExternalPort    uint32 `json:"external_port,omitempty"`
	ContainerPort   uint32 `json:"container_port,omitempty"`
}

func (r CCTCPRoutes) CCRouteInfo() (CCRouteInfo, error) {
	routesJson, err := json.Marshal(r)
	if err != nil {
		return nil, err
	}

	routesPayload := json.RawMessage(routesJson)
	routingInfo := make(map[string]*json.RawMessage)
	routingInfo[CC_TCP_ROUTES] = &routesPayload
	return routingInfo, nil
}

type CCDesiredStateServerResponse struct {
	Apps        []DesireAppRequestFromCC `json:"apps"`
	CCBulkToken *json.RawMessage         `json:"token"`
}

type CCDesiredAppFingerprint struct {
	ProcessGuid string `json:"process_guid"`
	ETag        string `json:"etag"`
}

type CCTaskState struct {
	TaskGuid              string `json:"task_guid"`
	State                 string `json:"state"`
	CompletionCallbackUrl string `json:"completion_callback"`
}

type CCDesiredStateFingerprintResponse struct {
	Fingerprints []CCDesiredAppFingerprint `json:"fingerprints"`
	CCBulkToken  *json.RawMessage          `json:"token"`
}

type CCTaskStatesResponse struct {
	TaskStates  []CCTaskState    `json:"task_states"`
	CCBulkToken *json.RawMessage `json:"token"`
}

type CCBulkToken struct {
	Id int `json:"id"`
}

type TaskErrorID string

type TaskRequestFromCC struct {
	TaskGuid              string                        `json:"task_guid"`
	LogGuid               string                        `json:"log_guid"`
	MemoryMb              int                           `json:"memory_mb"`
	DiskMb                int                           `json:"disk_mb"`
	Lifecycle             string                        `json:"lifecycle"`
	EnvironmentVariables  []*models.EnvironmentVariable `json:"environment"`
	EgressRules           []*models.SecurityGroupRule   `json:"egress_rules,omitempty"`
	DropletUri            string                        `json:"droplet_uri"`
	DropletHash           string                        `json:"droplet_hash"`
	DockerPath            string                        `json:"docker_path"`
	DockerUser            string                        `json:"docker_user,omitempty"`
	DockerPassword        string                        `json:"docker_password,omitempty"`
	RootFs                string                        `json:"rootfs"`
	CompletionCallbackUrl string                        `json:"completion_callback"`
	Command               string                        `json:"command"`
	LogSource             string                        `json:"log_source,omit_empty"`
	VolumeMounts          []*VolumeMount                `json:"volume_mounts"`
	IsolationSegment      string                        `json:"isolation_segment"`
}

type TaskFailResponseForCC struct {
	TaskGuid      string `json:"task_guid"`
	Failed        bool   `json:"failed"`
	FailureReason string `json:"failure_reason"`
}

type TaskError struct {
	Id      TaskErrorID `json:"id"`
	Message string      `json:"message"`
}
