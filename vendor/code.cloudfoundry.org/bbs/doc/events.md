# Events

The BBS emits events when a DesiredLRP, ActualLRP or Task is created,
updated, or deleted. The following sections provide details on how to subscribe
to those events as well as the type of events supported by the BBS.

## Subscribing to LRP Events

You can use the `SubscribeToEvents(logger lager.Logger) (events.EventSource,
error)` client method to subscribe to lrp events. For example:

``` go
client := bbs.NewClient(url)
eventSource, err := client.SubscribeToEvents(logger)
if err != nil {
    log.Printf("failed to subscribe to lrp events: " + err.Error())
}
```

Alternatively you can use the `SubscribeToEventsByCellID` client method to subscribe to events that are relevant to the given cell. For example:

``` go
client := bbs.NewClient(url)
eventSource, err := client.SubscribeToEventsByCellID(logger, "some-cell-id")
if err != nil {
    log.Printf("failed to subscribe to lrp events: " + err.Error())
}
```

Events relevant to the cell are defined as:

1. `ActualLRPCreatedEvent` that is running on that cell
2. `ActualLRPRemovedEvent` that used to run on that cell
3. `ActualLRPChangedEvent` that used to/started running on that cell
4. `ActualLRPCrashedEvent` that used to run on that cell

**Note** Passing an empty string `cellID` argument to `SubscribeToEventsByCellID` is equivalent to calling `SubscribeToEvents`

**Note** `SubscribeToEventsByCellID` and `SubscribeToEvents` do not have events related to Tasks.

## Subscribing to Task Events

You can use the `SubscribeToTaskEvents(logger lager.Logger) (events.EventSource,
error)` client method to subscribe to task events. For example:

``` go
client := bbs.NewClient(url)
eventSource, err := client.SubscribeToTaskEvents(logger)
if err != nil {
    log.Printf("failed to subscribe to task events: " + err.Error())
}
```

## Using the event source

Once an `EventSource` is created, you can then loop through the events by calling
[Next](https://godoc.org/code.cloudfoundry.org/bbs/events#EventSource) in a
loop, for example:

``` go
event, err := eventSource.Next()
if err != nil {
	switch err {
	case events.ErrUnrecognizedEventType:
                //log and skip unrecognized events
		logger.Error("failed-getting-next-event", err)
	case events.ErrSourceClosed:
                //log and try to re-subscribe
		logger.Error("failed-getting-next-event", err)
		resubscribeChan <- err
		return
        default:
                //log and handle a nil event for any other error
		logger.Error("failed-getting-next-event", err)
		time.Sleep(retryPauseInterval)
		eventChan <- nil
	}
}
log.Printf("received event: %#v", event)
```
In the case there is an `ErrUnrecognizedEventType` error,  the client should skip
it and move to the next event. If the error is an `ErrSourceClosed`,  the client
should try to resubscribe to the event source. The example above uses a channel
to handle the re-subscription.

To access the event field values, you must convert the event to the right
type. You can use the `EventType` method to determine the type of the event,
for example:

``` go
if event.EventType() == models.EventTypeActualLRPCrashed {
  crashEvent := event.(*models.ActualLRPCrashedEvent)
  log.Printf("lrp has crashed. err: %s", crashEvent.CrashReason)
}
```

The following types of events are emitted:

## DesiredLRP events

### `DesiredLRPCreatedEvent`

When a new DesiredLRP is created, a
[DesiredLRPCreatedEvent](https://godoc.org/code.cloudfoundry.org/bbs/models#DesiredLRPCreatedEvent)
is emitted. The value of the `DesiredLrp` field contains information about the
DesiredLRP that was just created.

### `DesiredLRPChangedEvent`

When a DesiredLRP changes, a
[DesiredLRPChangedEvent](https://godoc.org/code.cloudfoundry.org/bbs/models#DesiredLRPChangedEvent)
is emitted. The value of the `Before` and `After` fields have information about the
DesiredLRP before and after the change.

### `DesiredLRPRemovedEvent`

When a DesiredLRP is deleted, a
[DesiredLRPRemovedEvent](https://godoc.org/code.cloudfoundry.org/bbs/models#DesiredLRPRemovedEvent)
is emitted. The field value of `DesiredLrp` will have information about the
DesiredLRP that was just removed.

## ActualLRP events

### `ActualLRPCreatedEvent`

When a new ActualLRP is created, a
[ActualLRPCreatedEvent](https://godoc.org/code.cloudfoundry.org/bbs/models#ActualLRPCreatedEvent)
is emitted. The value of the `ActualLrpGroup` field contains more information
about the ActualLRP.


### `ActualLRPChangedEvent`

When a ActualLRP changes, a
[ActualLRPChangedEvent](https://godoc.org/code.cloudfoundry.org/bbs/models#ActualLRPChangedEvent)
is emitted. The value of the `Before` and `After` fields contains information about the
ActualLRP state before and after the change.

### `ActualLRPRemovedEvent`

When a ActualLRP is removed, a
[ActualLRPRemovedEvent](https://godoc.org/code.cloudfoundry.org/bbs/models#ActualLRPRemovedEvent)
is emitted. The value of the `ActualLrp` field contains information about the
ActualLRP that was just removed.

### `ActualLRPCrashedEvent`

When a ActualLRP crashes a
[ActualLRPCrashedEvent](https://godoc.org/code.cloudfoundry.org/bbs/models#ActualLRPCrashedEvent)
is emitted. The event will have the following field values:

1. `ActualLRPKey`: The LRP key of the ActualLRP.
1. `ActualLRPInstanceKey`: The instance key of the ActualLRP.
1. `CrashCount`: The number of times the ActualLRP has crashed, including this latest crash.
1. `CrashReason`: The last error that caused the ActualLRP to crash.
1. `Since`: The timestamp when the ActualLRP last crashed, in nanoseconds in the Unix epoch.

## Task events

### `TaskCreatedEvent`

When a new Task is created, a
[TaskCreatedEvent](https://godoc.org/code.cloudfoundry.org/bbs/models#TaskCreatedEvent)
is emitted. The value of the `Task` field contains information about the
Task that was just created.

### `TaskChangedEvent`

When a Task changes, a
[TaskChangedEvent](https://godoc.org/code.cloudfoundry.org/bbs/models#TaskChangedEvent)
is emitted. The value of the `Before` and `After` fields have information about the
Task before and after the change.

### `TaskRemovedEvent`

When a Task is deleted, a
[TaskRemovedEvent](https://godoc.org/code.cloudfoundry.org/bbs/models#TaskRemovedEvent)
is emitted. The field value of `Task` will have information about the
Task that was just removed.

[back](README.md)
