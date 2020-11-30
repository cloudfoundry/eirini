package opi

import (
	"fmt"
)

const (
	RunningState            = "RUNNING"
	PendingState            = "CLAIMED"
	ErrorState              = "UNCLAIMED"
	CrashedState            = "CRASHED"
	UnknownState            = "UNKNOWN"
	InsufficientMemoryError = "Insufficient resources: memory"
)

type LRPIdentifier struct {
	GUID, Version string
}

func (i *LRPIdentifier) ProcessGUID() string {
	return fmt.Sprintf("%s-%s", i.GUID, i.Version)
}

// An LRP, or long-running-process, is a stateless process
// where the scheduler should attempt to keep N copies running,
// killing and recreating as needed to maintain that guarantee.
type LRP struct {
	LRPIdentifier
	ProcessType            string
	AppName                string
	AppGUID                string
	OrgName                string
	OrgGUID                string
	SpaceName              string
	SpaceGUID              string
	Image                  string
	Command                []string
	Sidecars               []Sidecar
	PrivateRegistry        *PrivateRegistry
	Env                    map[string]string
	Health                 Healtcheck
	Ports                  []int32
	TargetInstances        int
	RunningInstances       int
	MemoryMB               int64
	DiskMB                 int64
	RunsAsRoot             bool
	CPUWeight              uint8
	VolumeMounts           []VolumeMount
	LRP                    string
	AppURIs                []Route
	LastUpdated            string
	UserDefinedAnnotations map[string]string
}

type Sidecar struct {
	Name     string
	Command  []string
	MemoryMB int64
	Env      map[string]string
}

type Route struct {
	Hostname string `json:"hostname"`
	Port     int32  `json:"port"`
}

type PrivateRegistry struct {
	Server   string
	Username string
	Password string
}

type VolumeMount struct {
	MountPath string
	ClaimName string
}

type Instance struct {
	Index          int
	Since          int64
	State          string
	PlacementError string
}

type Healtcheck struct {
	Type      string
	Port      int32
	Endpoint  string
	TimeoutMs uint
}

// A Task is a one-off process that is run exactly once and returns a
// result.
type Task struct {
	GUID               string
	Name               string
	Image              string
	CompletionCallback string
	PrivateRegistry    *PrivateRegistry
	Env                map[string]string
	Command            []string
	AppName            string
	AppGUID            string
	OrgName            string
	OrgGUID            string
	SpaceName          string
	SpaceGUID          string
	MemoryMB           int64
	DiskMB             int64
	CPUWeight          uint8
}
