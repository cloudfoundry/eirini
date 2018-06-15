package cf

const (
	VcapAppName   = "application_name"
	VcapVersion   = "version"
	VcapAppUris   = "application_uris"
	VcapAppId     = "application_id"
	VcapSpaceName = "space_name"

	ProcessGuid = "process_guid"
)

type VcapApp struct {
	AppName   string   `json:"application_name"`
	AppId     string   `json:"application_id"`
	Version   string   `json:"version"`
	AppUris   []string `json:"application_uris"`
	SpaceName string   `json:"space_name"`
}
