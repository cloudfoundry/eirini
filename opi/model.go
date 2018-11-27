package opi

const (
	RunningState = "RUNNING"
	PendingState = "CLAIMED"
	CrashedState = "CRASHED"
	UnknownState = "UNKNOWN"
)

// An LRP, or long-running-process, is a stateless process
// where the scheduler should attempt to keep N copies running,
// killing and recreating as needed to maintain that guarantee
type LRP struct {
	Name             string
	Image            string
	Command          []string
	Env              map[string]string
	Health           Healtcheck
	Ports            []int32
	TargetInstances  int
	RunningInstances int
	Metadata         map[string]string
}

type Instance struct {
	Index int
	Since int64
	State string
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

//go:generate counterfeiter . Desirer
type Desirer interface {
	Desire(lrp *LRP) error
	List() ([]*LRP, error)
	Get(name string) (*LRP, error)
	GetInstances(name string) ([]*Instance, error)
	Update(lrp *LRP) error
	Stop(name string) error
}

//go:generate counterfeiter . TaskDesirer
type TaskDesirer interface {
	Desire(task *Task) error
	DesireStaging(task *Task) error
	Delete(name string) error
}
