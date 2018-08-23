package route

type RegistryMessage struct {
	NatsMessage
	App                  string            `json:"app,omitempty"`
	RouteServiceURL      string            `json:"route_service_url,omitempty"`
	PrivateInstanceID    string            `json:"private_instance_id,omitempty"`
	PrivateInstanceIndex string            `json:"private_instance_index,omitempty"`
	ServerCertDomainSAN  string            `json:"server_cert_domain_san,omitempty"`
	IsolationSegment     string            `json:"isolation_segment,omitempty"`
	EndpointUpdatedAtNs  int64             `json:"endpoint_updated_at_ns,omitempty"`
	Tags                 map[string]string `json:"tags,omitempty"`
}

type NatsMessage struct {
	Host    string   `json:"host"`
	Port    uint32   `json:"port"`
	TLSPort uint32   `json:"tls_port,omitempty"`
	URIs    []string `json:"uris"`
}
