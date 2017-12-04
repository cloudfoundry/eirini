package cc_messages

import "time"
import "code.cloudfoundry.org/bbs/models"

type LRPInstanceState string

const (
	LRPInstanceStateStarting LRPInstanceState = "STARTING"
	LRPInstanceStateRunning  LRPInstanceState = "RUNNING"
	LRPInstanceStateCrashed  LRPInstanceState = "CRASHED"
	LRPInstanceStateDown     LRPInstanceState = "DOWN"
	LRPInstanceStateUnknown  LRPInstanceState = "UNKNOWN"
)

type LRPInstance struct {
	ProcessGuid  string                  `json:"process_guid"`
	InstanceGuid string                  `json:"instance_guid"`
	Index        uint                    `json:"index"`
	State        LRPInstanceState        `json:"state"`
	Details      string                  `json:"details,omitempty"`
	Host         string                  `json:"host,omitempty"`
	Port         uint16                  `json:"port,omitempty"`
	NetInfo      models.ActualLRPNetInfo `json:"net_info"`
	Uptime       int64                   `json:"uptime"`
	Since        int64                   `json:"since"`
	Stats        *LRPInstanceStats       `json:"stats,omitempty"`
}

type LRPInstanceStats struct {
	Time          time.Time `json:"time"`
	CpuPercentage float64   `json:"cpu"`
	MemoryBytes   uint64    `json:"mem"`
	DiskBytes     uint64    `json:"disk"`
}
