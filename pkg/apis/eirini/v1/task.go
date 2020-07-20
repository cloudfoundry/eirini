package v1

import (
	meta_v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +genclient
// +genclient:noStatus
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// Task describes a short-lived job running alongside an LRP
type Task struct {
	meta_v1.TypeMeta   `json:",inline"`
	meta_v1.ObjectMeta `json:"metadata,omitempty"`

	Spec TaskSpec `json:"spec"`
}

type TaskSpec struct {
	GUID               string            `json:"GUID"`
	Name               string            `json:"name"`
	Image              string            `json:"image"`
	CompletionCallback string            `json:"completion_callback"`
	PrivateRegistry    *PrivateRegistry  `json:"privateRegistry,omitempty"`
	Env                map[string]string `json:"env,omitempty"`
	Command            []string          `json:"command,omitempty"`
	AppName            string            `json:"appName"`
	AppGUID            string            `json:"appGUID"`
	OrgName            string            `json:"orgName"`
	OrgGUID            string            `json:"orgGUID"`
	SpaceName          string            `json:"spaceName"`
	SpaceGUID          string            `json:"spaceGUID"`
	MemoryMB           int64             `json:"memoryMB"`
	DiskMB             int64             `json:"diskMB"`
	CPUWeight          uint8             `json:"cpuWeight"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type TaskList struct {
	meta_v1.TypeMeta `json:",inline"`
	meta_v1.ListMeta `json:"metadata"`

	Items []Task `json:"items"`
}
