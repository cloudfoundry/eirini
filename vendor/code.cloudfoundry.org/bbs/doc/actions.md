# Available Actions

[Tasks](tasks.md) and [LRPs](lrps.md) express their work in terms of composable Actions. The following types of Action are available:

- Basic Actions:
    - [`RunAction`](#runaction): Runs a process in the container.
    - [`DownloadAction`](#downloadaction): Fetches an archive (`.tar`, `.tgz`, or `.zip`) and extracts it into the container.
    - [`UploadAction`](#uploadaction): Uploads a single file in the container to a URL via POST.
- Combining Actions:
    - [`SerialAction`](#serialaction): Runs several actions sequentially.
    - [`ParallelAction`](#parallelaction): Runs several actions in parallel.
    - [`CodependentAction`](#codependentaction): Runs sevenal actions in parallel, cancelling all other actions if one exits.
- Wrapping Actions:
    - [`EmitProgressAction`](#emitprogressaction): Wraps another action, logging messages when the wrapped action starts and finishes.
    - [`TimeoutAction`](#timeoutaction): Wraps another action, cancelling it if it does not finish within the timeout.
    - [`TryAction`](#tryaction): Runs the wrapped action, ignoring any error it generates.


## `RunAction`

The `RunAction` runs a process in the container, emitting stdout and stderr to the logging system. When the process finishes, a log line with its exit status will also be emitted to the logging system.

```go
action := &models.RunAction{
  Path: "/path/to/executable",
  Args: []string{"some", "args to", "pass in"},
  Dir: "/path/to/working/directory",
  User: "username",
  EnvironmentVariables: []*models.EnvironmentVariable{
    {
      Name: "ENVNAME",
      Value: "ENVVALUE",
    },
  },
  ResourceLimits: &models.ResourceLimits{
    Nofile: 1000,
  },
  LogSource: "some-log-source",
  SuppressLogOutput: false,
}
```

#### `Path` [required]

The path to the executable.


#### `Args` [optional]

An array of arguments for the executable specified in the `Path` value.


#### `Dir` [optional]

The working directory in which to execute the process.


#### `User` [required]

The user as which to run the process. Only user names may be specified, not numerical user IDs. The user should already exist in the rootfs used for the container.


#### `EnvironmentVariables` [optional]

A list of environment variables. These override any container-level environment variables.


#### `ResourceLimits` [optional]

A set of resource limits to apply to the process. Supported limits:

- `Nofile`: Number of file descriptors the process may allocate.
- `NProc`: Number of processes per user. **Note** this field is deprecated and is ignored


#### `LogSource` [optional]

If provided, logs emitted by this process will be tagged with the provided
`LogSource`.  Otherwise, the container-level `LogSource` is used.


#### `SuppressLogOutput` [optional]

If set, no logs will be emitted for the action.


## `DownloadAction`

The `DownloadAction` downloads an archive and extracts it to a specified
location in the container.

```go
action := &models.DownloadAction{
  From: "http://some/endpoint",
  To: "/some/container/path",
  User: "username",
  Artifact: "download name",
  CacheKey: "some-cache-key",
  LogSource: "some-log-source",
  ChecksumAlgorithm: "md5",
  ChecksumValue: "some-checksum-value",
}
```

#### `From` [required]

The URL from which to fetch the archive. The downloaded asset must be a tar archive, a
gzipped tar archive, or a zip archive.


#### `To` [required]

The absolute path into which to extract the archive.


#### `User` [required]

The user as which to run the process. Only user names may be specified, not numerical user IDs. The user should already exist in the rootfs used for the container.


#### `Artifact` [optional]

If specified, additional logs will be emitted to signify the progress of the
download action, including the number of bytes downloaded.


#### `CacheKey` [optional]

If specified, the Diego cell will cache the downloaded asset. Its cached-downloader stores values from the `ETag` and `Last-Modified` headers in the download response and supplies them as `If-None-Match` and `If-Modified-Since` headers in subsequent download requests. If it does not receive a `304 Not Modified` status code for those requests, it invalidates its cache.


#### `LogSource` [optional]

If provided, logs emitted by this action will be tagged with the provided
`LogSource`.  Otherwise the container-level `LogSource` is used.

#### `ChecksumAlgorithm` [optional]

If provided, the ChecksumValue must also be set.  It defines the checksum algorithm used to validate downloaded contents.  Must be one of `md5`, `sha1`, or `sha256`.

#### `ChecksumValue` [optional]

If provided, the ChecksumAlgorithm must also be set.  It provides the checksum to validate against.

## `UploadAction`

The `UploadAction` uploads a file from the container to the specified location.

```go
action := &models.UploadAction{
  To: "http://some/endpoint",
  From: "/some/container/file",
  User: "username",
  Artifact: "upload name",
  LogSource: "some-log-source",
}
```

#### `From` [required]

The path to a file in the container. Relative paths will be based on the home directory of the specified user.


#### `To` [required]

A URL to which to upload the file as the body of an HTTP POST request.


#### `User` [required]

The container-side user that uploads the file. Only user names may be specified, not numerical user IDs. The user should already exist in the rootfs used for the container, and should have access to the file specified in the `From` field.


#### `Artifact` [optional]

If specified, additional logs will be emitted to signify the progress of the
upload action, including the number of bytes uploaded.


#### `LogSource` [optional]

If provided, logs emitted by this action will be tagged with the provided
`LogSource`.  Otherwise the container-level `LogSource` is used.


## `SerialAction`

The `SerialAction` runs a sequence of actions in serial, stopping if one errors.

```go
action := &models.SerialAction{
  Actions: []*models.Action{
    action1,
    action2,
  },
  LogSource: "log-source",
}
```

#### `Actions` [required]

An array of Actions to run in series. If one of the actions fails, the Serial action returns with the failure, and subsequent actions are not run.


#### `LogSource` [optional]

If provided, logs emitted by this action and its subactions will be tagged with
the provided `LogSource`.  Otherwise the container-level `LogSource` is used.


## `ParallelAction`

The `ParallelAction` runs a sequence of actions in parallel and waits for them all to finish.

```go
action := &models.ParallelAction{
  Actions: []*models.Action{
    action1,
    action2,
  },
  LogSource: "log-source",
}
```

#### `Actions` [required]

An array of Actions to run in parallel. If one of the actions fails, the Parallel action itself fails, and returns with the first detected error of all the failed actions.


#### `LogSource` [optional]

If provided, logs emitted by this action and its subactions will be tagged with
the provided `LogSource`.  Otherwise the container-level `LogSource` is used.


## `CodependentAction`

The `CodependentAction` runs a sequence of actions in parallel, cancelling the other actions if one of them finishes. The Codependent action always returns an error, as it intends its actions to run indefinitely.

```go
action := &models.CodependentAction{
  Actions: []*models.Action{
    action1,
    action2,
  },
  LogSource: "log-source",
}
```

#### `Actions` [required]

An array of Actions to run in series. If one of the actions fails, the Codependent action cancels the other actions and returns with the failures.


#### `LogSource` [optional]

If provided, logs emitted by this action and its subactions will be tagged with
the provided `LogSource`.  Otherwise the container-level `LogSource` is used.


## `EmitProgressAction`

The `EmitProgressAction` emits additional logging around the wrapped action.

```go
action := &models.EmitProgressAction{
  Action: differentAction,
  StartMessage: "some message at start",
  SuccessMessage: "some message on success",
  FailureMessagePrefix: "some message to prefix failure",
  LogSource: "some-log-source",
}
```

#### `Action` [required]

The action to run.


#### `StartMessage` [optional]

If present, a message to emit before the action runs.


#### `SuccessMessage` [optional]

If present, a message to emit when the action succeeds.


#### `FailureMessagePrefix` [optional]

If present, a message to emit when the action fails. The corresponding error if
emittable will be appended with a ':' separator.


#### `LogSource` [optional]

If provided, logs emitted by this action and its subaction will be tagged with
the provided `LogSource`.  Otherwise the container-level `LogSource` is used.


## `TimeoutAction`

The `TimeoutAction` cancels the wrapped action if it does not finish within the timeout.

```go
action := &models.TimeoutAction{
  Action: differentAction,
  TimeoutMs: int64(10 * time.Second) / 1000000,
  LogSource: "log-source",
}
```

The `DeprecatedTimeoutNs` field has been deprecated in favor of `TimeoutMs`.
The `TimeoutMs` field is required and will be translated into `DeprecatedTimeoutNs` for older clients.

#### `Action` [required]

The action to run.


#### `Timeout` [required]

The number of nanoseconds to wait for the wrapped action to succeed. If the action does not return before this duration, the action is cancelled. Must be greater than `0`.


#### `LogSource` [optional]

If provided, logs emitted by this action and its subaction will be tagged with
the provided `LogSource`.  Otherwise the container-level `LogSource` is used.


## `TryAction`

The `TryAction` absorbs any error generated by the wrapped action.

```go
action := &model.TryAction{
  Action: differentAction,
  LogSource: "some-log-source",
}
```

#### `Action` [required]

The action to run.


#### `LogSource` [optional]

If provided, logs emitted by this action and its subaction will be tagged with
the provided `LogSource`.  Otherwise the container-level `LogSource` is used.


[back](README.md)
