package v1

import (
	meta_v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +genclient
// +genclient:noStatus
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// LRP describes an Long Running Process
type LRP struct {
	meta_v1.TypeMeta   `json:",inline"`
	meta_v1.ObjectMeta `json:"metadata,omitempty"`

	Spec LRPSpec `json:"spec"`
}

type LRPSpec struct {
	GUID                   string            `json:"GUID"`
	Version                string            `json:"version"`
	ProcessType            string            `json:"processType"`
	AppName                string            `json:"appName"`
	AppGUID                string            `json:"appGUID"`
	OrgName                string            `json:"orgName"`
	OrgGUID                string            `json:"orgGUID"`
	SpaceName              string            `json:"spaceName"`
	SpaceGUID              string            `json:"spaceGUID"`
	Image                  string            `json:"image"`
	Command                []string          `json:"command,omitempty"`
	PrivateRegistry        *PrivateRegistry  `json:"privateRegistry,omitempty"`
	Env                    map[string]string `json:"env,omitempty"`
	Health                 Healtcheck        `json:"health"`
	Ports                  []int32           `json:"ports,omitempty"`
	Instances              int               `json:"instances"`
	MemoryMB               int64             `json:"memoryMB"`
	DiskMB                 int64             `json:"diskMB"`
	RunsAsRoot             bool              `json:"runsAsRoot"`
	CPUWeight              uint8             `json:"cpuWeight"`
	VolumeMounts           []VolumeMount     `json:"volumeMounts,omitempty"`
	LastUpdated            string            `json:"lastUpdated"`
	UserDefinedAnnotations map[string]string `json:"userDefinedAnnotations,omitempty"`
	AppRoutes              string            `json:"appRoutes"`
}

type PrivateRegistry struct {
	Server   string `json:"server"`
	Username string `json:"username"`
	Password string `json:"password"`
}

type VolumeMount struct {
	MountPath string `json:"mountPath"`
	ClaimName string `json:"claimName"`
}

type Healtcheck struct {
	Type      string `json:"type"`
	Port      int32  `json:"port"`
	Endpoint  string `json:"endpoint"`
	TimeoutMs uint   `json:"timeoutMs"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type LRPList struct {
	meta_v1.TypeMeta `json:",inline"`
	meta_v1.ListMeta `json:"metadata"`

	Items []LRP `json:"items"`
}
