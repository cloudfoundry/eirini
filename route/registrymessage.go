package route

type RegistryMessage struct {
	Host              string   `json:"host"`
	Port              uint32   `json:"port"`
	TLSPort           uint32   `json:"tls_port,omitempty"`
	URIs              []string `json:"uris"`
	App               string   `json:"app,omitempty"`
	PrivateInstanceID string   `json:"private_instance_id"`
}
