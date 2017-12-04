package bbs

import "github.com/tedsuo/rata"

const (
	// Ping
	PingRoute = "Ping"

	// Domains
	DomainsRoute      = "Domains"
	UpsertDomainRoute = "UpsertDomain"

	// Actual LRPs
	ActualLRPGroupsRoute                     = "ActualLRPGroups"
	ActualLRPGroupsByProcessGuidRoute        = "ActualLRPGroupsByProcessGuid"
	ActualLRPGroupByProcessGuidAndIndexRoute = "ActualLRPGroupsByProcessGuidAndIndex"

	// Actual LRP Lifecycle
	ClaimActualLRPRoute  = "ClaimActualLRP"
	StartActualLRPRoute  = "StartActualLRP"
	CrashActualLRPRoute  = "CrashActualLRP"
	FailActualLRPRoute   = "FailActualLRP"
	RemoveActualLRPRoute = "RemoveActualLRP"
	RetireActualLRPRoute = "RetireActualLRP"

	// Evacuation
	RemoveEvacuatingActualLRPRoute = "RemoveEvacuatingActualLRP"
	EvacuateClaimedActualLRPRoute  = "EvacuateClaimedActualLRP"
	EvacuateCrashedActualLRPRoute  = "EvacuateCrashedActualLRP"
	EvacuateStoppedActualLRPRoute  = "EvacuateStoppedActualLRP"
	EvacuateRunningActualLRPRoute  = "EvacuateRunningActualLRP"

	// Desired LRPs
	DesiredLRPsRoute               = "DesiredLRPs_r2"
	DesiredLRPSchedulingInfosRoute = "DesiredLRPSchedulingInfos"
	DesiredLRPByProcessGuidRoute   = "DesiredLRPByProcessGuid_r2"

	DesiredLRPsRoute_r1             = "DesiredLRPs_r1" // Deprecated
	DesiredLRPByProcessGuidRoute_r1 = "DesiredLRPByProcessGuid_r1"
	DesiredLRPsRoute_r0             = "DesiredLRPs"             // Deprecated
	DesiredLRPByProcessGuidRoute_r0 = "DesiredLRPByProcessGuid" // Deprecated

	// Desire LRP Lifecycle
	DesireDesiredLRPRoute = "DesireDesiredLRP_r2"
	UpdateDesiredLRPRoute = "UpdateDesireLRP"
	RemoveDesiredLRPRoute = "RemoveDesiredLRP"

	DesireDesiredLRPRoute_r1 = "DesireDesiredLRP_r1"
	DesireDesiredLRPRoute_r0 = "DesireDesiredLRP"

	// Tasks
	TasksRoute         = "Tasks_r2"
	TaskByGuidRoute    = "TaskByGuid_r2"
	DesireTaskRoute    = "DesireTask_r2"
	StartTaskRoute     = "StartTask"
	CancelTaskRoute    = "CancelTask"
	FailTaskRoute      = "FailTask"
	CompleteTaskRoute  = "CompleteTask"
	ResolvingTaskRoute = "ResolvingTask"
	DeleteTaskRoute    = "DeleteTask"

	TasksRoute_r1      = "Tasks_r1"      // Deprecated
	TaskByGuidRoute_r1 = "TaskByGuid_r1" // Deprecated

	DesireTaskRoute_r0 = "DesireTask"    // Deprecated
	DesireTaskRoute_r1 = "DesireTask_r1" // Deprecated
	TasksRoute_r0      = "Tasks"         // Deprecated
	TaskByGuidRoute_r0 = "TaskByGuid"    // Deprecated

	// Event Streaming
	EventStreamRoute_r0     = "EventStream_r0"
	TaskEventStreamRoute_r0 = "TaskEventStream_r0"

	// Cell Presence
	CellsRoute    = "Cells_r2"
	CellsRoute_r1 = "Cells_r1"
)

var Routes = rata.Routes{
	// Ping
	{Path: "/v1/ping", Method: "POST", Name: PingRoute},

	// Domains
	{Path: "/v1/domains/list", Method: "POST", Name: DomainsRoute},
	{Path: "/v1/domains/upsert", Method: "POST", Name: UpsertDomainRoute},

	// Actual LRPs
	{Path: "/v1/actual_lrp_groups/list", Method: "POST", Name: ActualLRPGroupsRoute},
	{Path: "/v1/actual_lrp_groups/list_by_process_guid", Method: "POST", Name: ActualLRPGroupsByProcessGuidRoute},
	{Path: "/v1/actual_lrp_groups/get_by_process_guid_and_index", Method: "POST", Name: ActualLRPGroupByProcessGuidAndIndexRoute},

	// Actual LRP Lifecycle
	{Path: "/v1/actual_lrps/claim", Method: "POST", Name: ClaimActualLRPRoute},
	{Path: "/v1/actual_lrps/start", Method: "POST", Name: StartActualLRPRoute},
	{Path: "/v1/actual_lrps/crash", Method: "POST", Name: CrashActualLRPRoute},
	{Path: "/v1/actual_lrps/fail", Method: "POST", Name: FailActualLRPRoute},
	{Path: "/v1/actual_lrps/remove", Method: "POST", Name: RemoveActualLRPRoute},
	{Path: "/v1/actual_lrps/retire", Method: "POST", Name: RetireActualLRPRoute},

	// Evacuation
	{Path: "/v1/actual_lrps/remove_evacuating", Method: "POST", Name: RemoveEvacuatingActualLRPRoute},
	{Path: "/v1/actual_lrps/evacuate_claimed", Method: "POST", Name: EvacuateClaimedActualLRPRoute},
	{Path: "/v1/actual_lrps/evacuate_crashed", Method: "POST", Name: EvacuateCrashedActualLRPRoute},
	{Path: "/v1/actual_lrps/evacuate_stopped", Method: "POST", Name: EvacuateStoppedActualLRPRoute},
	{Path: "/v1/actual_lrps/evacuate_running", Method: "POST", Name: EvacuateRunningActualLRPRoute},

	// Desired LRPs
	{Path: "/v1/desired_lrp_scheduling_infos/list", Method: "POST", Name: DesiredLRPSchedulingInfosRoute},

	{Path: "/v1/desired_lrps/list.r2", Method: "POST", Name: DesiredLRPsRoute},
	{Path: "/v1/desired_lrps/get_by_process_guid.r2", Method: "POST", Name: DesiredLRPByProcessGuidRoute},

	{Path: "/v1/desired_lrps/list.r1", Method: "POST", Name: DesiredLRPsRoute_r1},                            // Deprecated
	{Path: "/v1/desired_lrps/get_by_process_guid.r1", Method: "POST", Name: DesiredLRPByProcessGuidRoute_r1}, // Deprecated
	{Path: "/v1/desired_lrps/list", Method: "POST", Name: DesiredLRPsRoute_r0},                               // Deprecated
	{Path: "/v1/desired_lrps/get_by_process_guid", Method: "POST", Name: DesiredLRPByProcessGuidRoute_r0},    // Deprecated

	// Desire LPR Lifecycle
	{Path: "/v1/desired_lrp/desire.r2", Method: "POST", Name: DesireDesiredLRPRoute},
	{Path: "/v1/desired_lrp/desire.r1", Method: "POST", Name: DesireDesiredLRPRoute_r1}, // Deprecated
	{Path: "/v1/desired_lrp/update", Method: "POST", Name: UpdateDesiredLRPRoute},
	{Path: "/v1/desired_lrp/remove", Method: "POST", Name: RemoveDesiredLRPRoute},
	{Path: "/v1/desired_lrp/desire", Method: "POST", Name: DesireDesiredLRPRoute_r0}, // Deprecated

	// Tasks
	{Path: "/v1/tasks/list.r2", Method: "POST", Name: TasksRoute},
	{Path: "/v1/tasks/get_by_task_guid.r2", Method: "POST", Name: TaskByGuidRoute},

	{Path: "/v1/tasks/list.r1", Method: "POST", Name: TasksRoute_r1},                  // Deprecated
	{Path: "/v1/tasks/get_by_task_guid.r1", Method: "POST", Name: TaskByGuidRoute_r1}, // Deprecated
	{Path: "/v1/tasks/list", Method: "POST", Name: TasksRoute_r0},                     // Deprecated
	{Path: "/v1/tasks/get_by_task_guid", Method: "GET", Name: TaskByGuidRoute_r0},     // Deprecated

	// Task Lifecycle
	{Path: "/v1/tasks/desire.r2", Method: "POST", Name: DesireTaskRoute},
	{Path: "/v1/tasks/desire.r1", Method: "POST", Name: DesireTaskRoute_r1}, // Deprecated
	{Path: "/v1/tasks/start", Method: "POST", Name: StartTaskRoute},
	{Path: "/v1/tasks/cancel", Method: "POST", Name: CancelTaskRoute},
	{Path: "/v1/tasks/fail", Method: "POST", Name: FailTaskRoute},
	{Path: "/v1/tasks/complete", Method: "POST", Name: CompleteTaskRoute},
	{Path: "/v1/tasks/resolving", Method: "POST", Name: ResolvingTaskRoute},
	{Path: "/v1/tasks/delete", Method: "POST", Name: DeleteTaskRoute},

	{Path: "/v1/tasks/desire", Method: "POST", Name: DesireTaskRoute_r0}, // Deprecated

	// Event Streaming
	{Path: "/v1/events", Method: "GET", Name: EventStreamRoute_r0},
	{Path: "/v1/events/tasks", Method: "POST", Name: TaskEventStreamRoute_r0},

	// Cells
	{Path: "/v1/cells/list.r1", Method: "POST", Name: CellsRoute},
	{Path: "/v1/cells/list.r1", Method: "GET", Name: CellsRoute_r1}, // Deprecated
}
