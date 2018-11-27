package cf

import "encoding/json"

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

type DesireLRPRequest struct {
	ProcessGUID             string                      `json:"process_guid"`
	DockerImageURL          string                      `json:"docker_image"`
	DropletHash             string                      `json:"droplet_hash"`
	DropletGUID             string                      `json:"droplet_guid"`
	StartCommand            string                      `json:"start_command"`
	Environment             map[string]string           `json:"environment"`
	NumInstances            int                         `json:"instances"`
	LastUpdated             string                      `json:"last_updated"`
	HealthCheckType         string                      `json:"health_check_type"`
	HealthCheckHTTPEndpoint string                      `json:"health_check_http_endpoint"`
	HealthCheckTimeoutMs    uint                        `json:"health_check_timeout_ms"`
	Ports                   []int32                     `json:"ports"`
	Routes                  map[string]*json.RawMessage `json:"routes"`
}

type GetInstancesResponse struct {
	Error       string      `json:"error,omitempty"`
	ProcessGUID string      `json:"process_guid"`
	Instances   []*Instance `json:"instances"`
}

type Instance struct {
	Index int    `json:"index"`
	Since int64  `json:"since"`
	State string `json:"state"`
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
