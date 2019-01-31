# Overview of Tasks

Diego can run one-off work in the form of Tasks. When a Task is submitted, Diego allocates resources for the Task on a Cell, runs the Task on that Cell, and then reports the result of the Task. Tasks are guaranteed to run at most once.


## The Task API

We recommend interacting with Diego's Task functionality through the ExternalTaskClient interface. The calls exposed to external clients are specifically documented [here](https://godoc.org/github.com/cloudfoundry/bbs#ExternalTaskClient).


## The Task Lifecycle

Tasks in Diego undergo a lifecycle encoded in the Task state:

- When first created, a Task's state is `PENDING`. 
- When the `PENDING` Task is allocated to a Diego Cell, the Cell sets the Task's state to `RUNNING` state, and populates the Task's `CellId` field with its own Cell ID.
- On failed attempts to place the task on a cell, the `RejectionCount` field is incremented, and the `RejectionReason` field is populated. The maximum number of attempts to place a task is configured in the BBS.
- When the Task completes, the Cell sets the `Failed`, `FailureReason`, and `Result` fields on the Task as appropriate, and sets the Task's state to `COMPLETED`.

At this point it is up to the Diego client to detect and resolve the completed Task. It can do this either by having set a completion callback URL on the Task when defined, or by polling for the Task and resolving and deleting it itself.

To prevent two Task clients from operating on the same completed Task at once, the BBS provides the `RESOLVING` state on the Task. Any client intending to delete the Task must first successfully move it from the `COMPLETED` state to the `RESOLVING` state. For example, when Diego itself calls the completion callback URL on the Task, it first must transition the Task into the `RESOLVING` state. External clients should adhere to the same convention.

Diego will automatically delete completed Tasks that remain unresolved after 2 minutes.


## Defining Tasks

When submitting a task, a valid `guid`, `domain`, and `TaskDefinition` should be provided to [a Client's DesireTask method](https://github.com/cloudfoundry/bbs/blob/master/client.go#L87). See [Defining Tasks](defining-tasks.md) for more detail on the `TaskDefinition` fields.


## Retreiving Tasks

The `ExternalTaskClient` can be used to retrieve a Task from the BBS API. Fields on the Task not present in the `TaskDefinition` represent its status. The returned task has the following additional attributes:

### `State`

The `State` field determines where in its [lifecycle](#the_task_lifecycle) the Task is.


### `CellId`

`CellId` identifies which of Diego's Cells has accepted the Task workload.


### `RejectionCount` and `RejectionReason`

- `RejectionCount` shows the number of times that placement has failed for this task.
- `RejectionReason` shows the reason for the most recent placement failure.


### `CreatedAt`, `UpdatedAt`, and `FirstCompletedAt`

Timestamps in nanoseconds since the start of UNIX epoch time (1970-01-01).

- `CreatedAt` is the time at which the Task was created.
- `UpdatedAt` is the time at which the Task was last updated.
- `FirstCompletedAt` is the time at which the Task first entered the `COMPLETED` state.

The `FirstCompletedAt` timestamp is used to determine when a Task should be deleted during Task convergence after remaining unresolved for over 2 minutes.


## Receiving the Task Result

If the client specifies a `CompletionCallbackUrl` on the original Task definition, a `TaskCallbackResponse` will be sent back as JSON to the specified URL when the task is completed.

This JSON-encoded `TaskCallbackResponse` looks like the following:

```json
{
  "task_guid": "some-guid",
  "failed": false,
  "failure_reason": "some failure",
  "result": "first 10KB of ResultFile",
  "annotation": "arbitrary",
  "created_at: 4567890434758937
}
```


#### `Failed`

Once a Task enters the `COMPLETED` state, `Failed` will be a boolean indicating whether the Task completed succesfully or unsuccesfully.


#### `FailureReason`

If `Failed` is `true`, `FailureReason` will be a short string indicating why the Task failed.  Sometimes, in the case of a `RunAction` that has failed this may read `exit status 1`. More detailed debugging of the Task may require retrieving the log messages from the Loggregator system.


#### `Result`

If `ResultFile` was specified and the Task has completed succesfully, `Result` will include the first 10KB of the `ResultFile`.


#### `Annotation`

This is the arbitrary string that was specified in the TaskDefinition.


[back](README.md)
