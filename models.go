package cube

type AppInfo struct {
	AppName   string `json:"name"`
	SpaceName string `json:"space_name"`
	AppGuid   string `json:"application_id"`
}

//go:generate counterfeiter . CfClient
type CfClient interface {
	GetDropletByAppGuid(string) ([]byte, error)
}
