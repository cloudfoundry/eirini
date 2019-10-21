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
// killing and recreating as needed to maintain that guarantee
type LRP struct {
	LRPIdentifier
	ProcessType      string
	AppName          string
	AppGUID          string
	SpaceName        string
	Image            string
	Command          []string
	Env              map[string]string
	Health           Healtcheck
	Ports            []int32
	TargetInstances  int
	RunningInstances int
	Metadata         map[string]string
	MemoryMB         int64
	DiskMB           int64
	CPUWeight        uint8
	VolumeMounts     []VolumeMount
	LRP              string
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
// result
type Task struct {
	Image   string
	Command []string
	Env     map[string]string
}

type StagingTask struct {
	*Task
	DownloaderImage string
	UploaderImage   string
	ExecutorImage   string
}

//go:generate counterfeiter . Desirer
type Desirer interface {
	Desire(lrp *LRP) error
	List() ([]*LRP, error)
	Get(identifier LRPIdentifier) (*LRP, error)
	GetInstances(identifier LRPIdentifier) ([]*Instance, error)
	Update(lrp *LRP) error
	Stop(identifier LRPIdentifier) error
	StopInstance(identifier LRPIdentifier, index uint) error
}

//go:generate counterfeiter . TaskDesirer
type TaskDesirer interface {
	Desire(task *Task) error
	DesireStaging(task *StagingTask) error
	Delete(name string) error
}
