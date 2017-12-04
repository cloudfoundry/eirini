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

	EventTypeTaskCreated = "task_created"
	EventTypeTaskChanged = "task_changed"
	EventTypeTaskRemoved = "task_removed"
)

func VersionDesiredLRPsToV0(event Event) Event {
	switch event := event.(type) {
	case *DesiredLRPCreatedEvent:
		return NewDesiredLRPCreatedEvent(event.DesiredLrp.VersionDownTo(format.V0))
	case *DesiredLRPRemovedEvent:
		return NewDesiredLRPRemovedEvent(event.DesiredLrp.VersionDownTo(format.V0))
	case *DesiredLRPChangedEvent:
		return NewDesiredLRPChangedEvent(
			event.Before.VersionDownTo(format.V0),
			event.After.VersionDownTo(format.V0),
		)
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
	actualLRP, _ := event.Before.Resolve()
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
	actualLRP, _ := event.ActualLrpGroup.Resolve()
	return actualLRP.GetInstanceGuid()
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
	actualLRP, _ := event.ActualLrpGroup.Resolve()
	return actualLRP.GetInstanceGuid()
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
