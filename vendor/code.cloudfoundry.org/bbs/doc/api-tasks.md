# Tasks API Reference

This reference does not cover the protobuf payload supplied to each endpoint.

For detailed information on the structs and types listed see [models documentation](https://godoc.org/code.cloudfoundry.org/bbs/models)

# Tasks APIs

## DesireTask
	{Path: "/v1/tasks/desire", Method: "POST", Name: DesireTaskRoute_r0}, // Deprecated

### BBS API Endpoint
Post a DesireTaskRequest to "/v1/tasks/desire"

### Golang Client API
```go
func (c *client) DesireTask(logger lager.Logger, taskGuid, domain string, taskDef *models.TaskDefinition) error
```

#### Input
* `logger lager.Logger`
  * The logging sink
* `taskGuid string`
  * The task Guid
* `domain string`
  * The Domain
* `taskDef *models.TaskDefinition`
  * See the [Defining Tasks page](defining-tasks.md) for how to create a Task

#### Output
* `error`
  * Non-nil if error occurred

#### Example
See the [Defining Tasks page](defining-tasks.md) for how to create a Task

## Tasks
Lists all Tasks

### BBS API Endpoint
Post a TasksRequest to "/v1/tasks/list.r2"

DEPRECATED:
* Post a TasksRequest to "/v1/tasks/list.r1"
* Post a TasksRequest to "/v1/tasks/list"

### Golang Client API
```go
func (c *client) Tasks(logger lager.Logger) ([]*models.Task, error)
```

#### Input
* `logger lager.Logger`
  * The logging sink

#### Output
* `[]*models.Task`
  * [See Task Documentation](https://godoc.org/code.cloudfoundry.org/bbs/models#Task)
* `error`
  * Non-nil if error occurred

#### Example
```go
client := bbs.NewClient(url)
tasks, err := client.Tasks(logger)
if err != nil {
    log.Printf("failed to retrieve tasks: " + err.Error())
}
```

## TasksByDomain
Lists all Tasks of the given domain

### BBS API Endpoint
Post a TasksRequest to "/v1/tasks/list.r2"

DEPRECATED:
* Post a TasksRequest to "/v1/tasks/list.r1"
* Post a TasksRequest to "/v1/tasks/list"

### Golang Client API
```go
func (c *client) TasksByDomain(logger lager.Logger, domain string) ([]*models.Task, error)
```

#### Input
* `logger lager.Logger`
  * The logging sink
* `domain string`
  * The domain

#### Output
* `[]*models.Task`
  * [See Task Documentation](https://godoc.org/code.cloudfoundry.org/bbs/models#Task)
* `error`
  * Non-nil if error occurred

#### Example
```go
client := bbs.NewClient(url)
tasks, err := client.TasksByDomain(logger, "the-domain")
if err != nil {
    log.Printf("failed to retrieve tasks: " + err.Error())
}
```

## TasksByCellID
Lists all Tasks on the given cell

### BBS API Endpoint
Post a TasksRequest to "/v1/tasks/list.r2"

DEPRECATED:
* Post a TasksRequest to "/v1/tasks/list.r1"
* Post a TasksRequest to "/v1/tasks/list"

### Golang Client API
```go
func (c *client) TasksByCellID(logger lager.Logger, cellId string) ([]*models.Task, error)
```

#### Input
* `logger lager.Logger`
  * The logging sink
* `cellId string`
  * The CellID

#### Output
* `[]*models.Task`
  * [See Task Documentation](https://godoc.org/code.cloudfoundry.org/bbs/models#Task)
* `error`
  * Non-nil if error occurred

#### Example
```go
client := bbs.NewClient(url)
tasks, err := client.TasksByCellID(logger, "my-cell")
if err != nil {
    log.Printf("failed to retrieve tasks: " + err.Error())
}
```



## TaskByGuid
Returns the Task with the given guid

### BBS API Endpoint
Post a TaskByGuidRequest to "/v1/tasks/get_by_task_guid.r2"

DEPRECATED:
* Post a TaskByGuidRequest to "/v1/tasks/get_by_task_guid.r1"
* Post a TaskByGuidRequest to "/v1/tasks/get_by_task_guid"

### Golang Client API
```go
func (c *client) TaskByGuid(logger lager.Logger, taskGuid string) (*models.Task, error)
```

#### Input
* `logger lager.Logger`
  * The logging sink
* `taskGuid string`
  * The task Guid

#### Output
* `*models.Task`
  * [See Task Documentation](https://godoc.org/code.cloudfoundry.org/bbs/models#Task)
* `error`
  * Non-nil if error occurred

#### Example
```go
client := bbs.NewClient(url)
task, err := client.TaskByGuid(logger, "the-task-guid")
if err != nil {
    log.Printf("failed to retrieve task: " + err.Error())
}
```

## CancelTask
Cancels the Task with the given task guid

### BBS API Endpoint
Post a TaskGuidRequest to "/v1/tasks/cancel"

### Golang Client API
```go
func (c *client) CancelTask(logger lager.Logger, taskGuid string) error
```

#### Input
* `logger lager.Logger`
  * The logging sink
* `taskGuid string`
  * The task Guid

#### Output
* `error`
  * Non-nil if error occurred

#### Example
```go
client := bbs.NewClient(url)
err := client.CancelTask(logger, "the-task-guid")
if err != nil {
    log.Printf("failed to cancel task: " + err.Error())
}
```

## ResolvingTask
Resolves a Task with the given guid

### BBS API Endpoint
Post a TaskGuidRequest to "/v1/tasks/resolving"

### Golang Client API
```go
func (c *client) ResolvingTask(logger lager.Logger, taskGuid string) error
```

#### Input
* `logger lager.Logger`
  * The logging sink
* `taskGuid string`
  * The task Guid

#### Output
* `error`
  * Non-nil if error occurred

#### Example
```go
client := bbs.NewClient(url)
err := client.ResolvingTask(logger, "the-task-guid")
if err != nil {
    log.Printf("failed to resolving task: " + err.Error())
}
```

## DeleteTask
Deletes a completed task with the given guid

### BBS API Endpoint
Post a TaskGuidRequest to "/v1/tasks/delete"

### Golang Client API
```go
func (c *client) DeleteTask(logger lager.Logger, taskGuid string) error
```

#### Input
* `logger lager.Logger`
  * The logging sink
* `taskGuid string`
  * The task Guid

#### Output
* `error`
  * Non-nil if error occurred

#### Example
```go
client := bbs.NewClient(url)
err := client.DeleteTask(logger, "the-task-guid")
if err != nil {
    log.Printf("failed to delete task: " + err.Error())
}
```
[back](README.md)
