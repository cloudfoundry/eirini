package cf

const (
	VcapAppName   = "application_name"
	VcapVersion   = "version"
	VcapAppUris   = "application_uris"
	VcapAppId     = "application_id"
	VcapSpaceName = "space_name"

	LastUpdated = "last_updated"
	ProcessGuid = "process_guid"
)

type VcapApp struct {
	AppName   string   `json:"application_name"`
	AppId     string   `json:"application_id"`
	Version   string   `json:"version"`
	AppUris   []string `json:"application_uris"`
	SpaceName string   `json:"space_name"`
}

type DesireLRPRequest struct {
	ProcessGuid    string            `json:"process_guid"`
	DockerImageUrl string            `json:"docker_image"`
	DropletHash    string            `json:"droplet_hash"`
	StartCommand   string            `json:"start_command"`
	Environment    map[string]string `json:"environment"`
	NumInstances   int               `json:"instances"`
	LastUpdated    string            `json:"last_updated"`
}
