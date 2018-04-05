package route

type RegistryMessage struct {
	Host                 string            `json:"host"`
	Port                 uint32            `json:"port"`
	TlsPort              uint32            `json:"tls_port,omitempty"`
	URIs                 []string          `json:"uris"`
	App                  string            `json:"app,omitempty"`
	RouteServiceUrl      string            `json:"route_service_url,omitempty"`
	PrivateInstanceId    string            `json:"private_instance_id,omitempty"`
	PrivateInstanceIndex string            `json:"private_instance_index,omitempty"`
	ServerCertDomainSAN  string            `json:"server_cert_domain_san,omitempty"`
	IsolationSegment     string            `json:"isolation_segment,omitempty"`
	EndpointUpdatedAtNs  int64             `json:"endpoint_updated_at_ns,omitempty"`
	Tags                 map[string]string `json:"tags,omitempty"`
}
