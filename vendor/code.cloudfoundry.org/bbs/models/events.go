package models

import (
	"code.cloudfoundry.org/bbs/format"
	"github.com/gogo/protobuf/proto"
)

type Event interface {
	EventType() string
	Key() string
	proto.Message
}

const (
	EventTypeInvalid = ""

	EventTypeDesiredLRPCreated = "desired_lrp_created"
	EventTypeDesiredLRPChanged = "desired_lrp_changed"
	EventTypeDesiredLRPRemoved = "desired_lrp_removed"

	EventTypeActualLRPCreated = "actual_lrp_created"
	EventTypeActualLRPChanged = "actual_lrp_changed"
	EventTypeActualLRPRemoved = "actual_lrp_removed"
	EventTypeActualLRPCrashed = "actual_lrp_crashed"

	EventTypeActualLRPInstanceCreated = "actual_lrp_instance_created"
	EventTypeActualLRPInstanceChanged = "actual_lrp_instance_changed"
	EventTypeActualLRPInstanceRemoved = "actual_lrp_instance_removed"

	EventTypeTaskCreated = "task_created"
	EventTypeTaskChanged = "task_changed"
	EventTypeTaskRemoved = "task_removed"
)

// Downgrade the DesiredLRPEvent payload (i.e. DesiredLRP(s)) to the given
// target version
func VersionDesiredLRPsTo(event Event, target format.Version) Event {
	switch event := event.(type) {
	case *DesiredLRPCreatedEvent:
		return NewDesiredLRPCreatedEvent(event.DesiredLrp.VersionDownTo(target))
	case *DesiredLRPRemovedEvent:
		return NewDesiredLRPRemovedEvent(event.DesiredLrp.VersionDownTo(target))
	case *DesiredLRPChangedEvent:
		return NewDesiredLRPChangedEvent(
			event.Before.VersionDownTo(target),
			event.After.VersionDownTo(target),
		)
	default:
		return event
	}
}

// Downgrade the TaskEvent payload (i.e. Task(s)) to the given target version
func VersionTaskDefinitionsTo(event Event, target format.Version) Event {
	switch event := event.(type) {
	case *TaskCreatedEvent:
		return NewTaskCreatedEvent(event.Task.VersionDownTo(target))
	case *TaskRemovedEvent:
		return NewTaskRemovedEvent(event.Task.VersionDownTo(target))
	case *TaskChangedEvent:
		return NewTaskChangedEvent(event.Before.VersionDownTo(target), event.After.VersionDownTo(target))
	default:
		return event
	}
}

func NewDesiredLRPCreatedEvent(desiredLRP *DesiredLRP) *DesiredLRPCreatedEvent {
	return &DesiredLRPCreatedEvent{
		DesiredLrp: desiredLRP,
	}
}

func (event *DesiredLRPCreatedEvent) EventType() string {
	return EventTypeDesiredLRPCreated
}

func (event *DesiredLRPCreatedEvent) Key() string {
	return event.DesiredLrp.GetProcessGuid()
}

func NewDesiredLRPChangedEvent(before, after *DesiredLRP) *DesiredLRPChangedEvent {
	return &DesiredLRPChangedEvent{
		Before: before,
		After:  after,
	}
}

func (event *DesiredLRPChangedEvent) EventType() string {
	return EventTypeDesiredLRPChanged
}

func (event *DesiredLRPChangedEvent) Key() string {
	return event.Before.GetProcessGuid()
}

func NewDesiredLRPRemovedEvent(desiredLRP *DesiredLRP) *DesiredLRPRemovedEvent {
	return &DesiredLRPRemovedEvent{
		DesiredLrp: desiredLRP,
	}
}

func (event *DesiredLRPRemovedEvent) EventType() string {
	return EventTypeDesiredLRPRemoved
}

func (event DesiredLRPRemovedEvent) Key() string {
	return event.DesiredLrp.GetProcessGuid()
}

// FIXME: change the signature
func NewActualLRPInstanceChangedEvent(before, after *ActualLRP) *ActualLRPInstanceChangedEvent {
	return &ActualLRPInstanceChangedEvent{
		ActualLRPKey:         after.ActualLRPKey,
		ActualLRPInstanceKey: after.ActualLRPInstanceKey,
		Before:               before.ToActualLRPInfo(),
		After:                after.ToActualLRPInfo(),
	}
}

func NewActualLRPChangedEvent(before, after *ActualLRPGroup) *ActualLRPChangedEvent {
	return &ActualLRPChangedEvent{
		Before: before,
		After:  after,
	}
}

func (event *ActualLRPChangedEvent) EventType() string {
	return EventTypeActualLRPChanged
}

func (event *ActualLRPChangedEvent) Key() string {
	actualLRP, _, resolveError := event.Before.Resolve()
	if resolveError != nil {
		return ""
	}
	return actualLRP.GetInstanceGuid()
}

func NewActualLRPCrashedEvent(before, after *ActualLRP) *ActualLRPCrashedEvent {
	return &ActualLRPCrashedEvent{
		ActualLRPKey:         after.ActualLRPKey,
		ActualLRPInstanceKey: before.ActualLRPInstanceKey,
		CrashCount:           after.CrashCount,
		CrashReason:          after.CrashReason,
		Since:                after.Since,
	}
}

func (event *ActualLRPCrashedEvent) EventType() string {
	return EventTypeActualLRPCrashed
}

func (event *ActualLRPCrashedEvent) Key() string {
	return event.ActualLRPInstanceKey.InstanceGuid
}

func NewActualLRPRemovedEvent(actualLRPGroup *ActualLRPGroup) *ActualLRPRemovedEvent {
	return &ActualLRPRemovedEvent{
		ActualLrpGroup: actualLRPGroup,
	}
}

func (event *ActualLRPRemovedEvent) EventType() string {
	return EventTypeActualLRPRemoved
}

func (event *ActualLRPRemovedEvent) Key() string {
	actualLRP, _, resolveError := event.ActualLrpGroup.Resolve()
	if resolveError != nil {
		return ""
	}
	return actualLRP.GetInstanceGuid()
}

func NewActualLRPInstanceRemovedEvent(actualLrp *ActualLRP) *ActualLRPInstanceRemovedEvent {
	return &ActualLRPInstanceRemovedEvent{
		ActualLrp: actualLrp,
	}
}

func (event *ActualLRPInstanceRemovedEvent) EventType() string {
	return EventTypeActualLRPInstanceRemoved
}

func (event *ActualLRPInstanceRemovedEvent) Key() string {
	return event.ActualLrp.GetInstanceGuid()
}

func NewActualLRPCreatedEvent(actualLRPGroup *ActualLRPGroup) *ActualLRPCreatedEvent {
	return &ActualLRPCreatedEvent{
		ActualLrpGroup: actualLRPGroup,
	}
}

func (event *ActualLRPCreatedEvent) EventType() string {
	return EventTypeActualLRPCreated
}

func (event *ActualLRPCreatedEvent) Key() string {
	actualLRP, _, resolveError := event.ActualLrpGroup.Resolve()
	if resolveError != nil {
		return ""
	}
	return actualLRP.GetInstanceGuid()
}

func NewActualLRPInstanceCreatedEvent(actualLrp *ActualLRP) *ActualLRPInstanceCreatedEvent {
	return &ActualLRPInstanceCreatedEvent{
		ActualLrp: actualLrp,
	}
}

func (event *ActualLRPInstanceCreatedEvent) EventType() string {
	return EventTypeActualLRPInstanceCreated
}

func (event *ActualLRPInstanceCreatedEvent) Key() string {
	return event.ActualLrp.GetInstanceGuid()
}

func (request *EventsByCellId) Validate() error {
	return nil
}

func NewTaskCreatedEvent(task *Task) *TaskCreatedEvent {
	return &TaskCreatedEvent{
		Task: task,
	}
}

func (event *TaskCreatedEvent) EventType() string {
	return EventTypeTaskCreated
}

func (event *TaskCreatedEvent) Key() string {
	return event.Task.GetTaskGuid()
}

func NewTaskChangedEvent(before, after *Task) *TaskChangedEvent {
	return &TaskChangedEvent{
		Before: before,
		After:  after,
	}
}

func (event *TaskChangedEvent) EventType() string {
	return EventTypeTaskChanged
}

func (event *TaskChangedEvent) Key() string {
	return event.Before.GetTaskGuid()
}

func NewTaskRemovedEvent(task *Task) *TaskRemovedEvent {
	return &TaskRemovedEvent{
		Task: task,
	}
}

func (event *TaskRemovedEvent) EventType() string {
	return EventTypeTaskRemoved
}

func (event TaskRemovedEvent) Key() string {
	return event.Task.GetTaskGuid()
}

func (event *ActualLRPInstanceChangedEvent) EventType() string {
	return EventTypeActualLRPInstanceChanged
}

func (event *ActualLRPInstanceChangedEvent) Key() string {
	return event.GetInstanceGuid()
}
