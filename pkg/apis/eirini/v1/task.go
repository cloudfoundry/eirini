package v1

import (
	meta_v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:subresource:status

// Task describes a short-lived job running alongside an LRP
type Task struct {
	meta_v1.TypeMeta   `json:",inline"`
	meta_v1.ObjectMeta `json:"metadata,omitempty"`

	Spec   TaskSpec   `json:"spec"`
	Status TaskStatus `json:"status"`
}

type TaskSpec struct {
	GUID               string            `json:"guid"`
	Name               string            `json:"name"`
	Image              string            `json:"image"`
	CompletionCallback string            `json:"completionCallback"`
	PrivateRegistry    *PrivateRegistry  `json:"privateRegistry,omitempty"`
	Env                map[string]string `json:"env,omitempty"`
	Command            []string          `json:"command,omitempty"`
	AppName            string            `json:"appName"`
	AppGUID            string            `json:"appGuid"`
	OrgName            string            `json:"orgName"`
	OrgGUID            string            `json:"orgGuid"`
	SpaceName          string            `json:"spaceName"`
	SpaceGUID          string            `json:"spaceGuid"`
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

type ExecutionStatus string

const (
	TaskStarting  ExecutionStatus = "starting"
	TaskRunning   ExecutionStatus = "running"
	TaskSucceeded ExecutionStatus = "succeeded"
	TaskFailed    ExecutionStatus = "failed"
)

type TaskStatus struct {
	StartTime       *meta_v1.Time   `json:"start_time"`
	EndTime         *meta_v1.Time   `json:"end_time"`
	ExecutionStatus ExecutionStatus `json:"execution_status"`
}
