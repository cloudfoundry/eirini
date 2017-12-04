package recipebuilder

import "encoding/json"

type DockerExecutionMetadata struct {
	Cmd          []string `json:"cmd,omitempty"`
	Entrypoint   []string `json:"entrypoint,omitempty"`
	Workdir      string   `json:"workdir,omitempty"`
	ExposedPorts []Port   `json:"ports,omitempty"`
	User         string   `json:"user,omitempty"`
}

type Port struct {
	Port     uint32
	Protocol string
}

func NewDockerExecutionMetadata(dockerExecutionMetadata string) (DockerExecutionMetadata, error) {
	metadata := DockerExecutionMetadata{}
	err := json.Unmarshal([]byte(dockerExecutionMetadata), &metadata)
	return metadata, err
}
