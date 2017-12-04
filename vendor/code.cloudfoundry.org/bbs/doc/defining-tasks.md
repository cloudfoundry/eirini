## Defining Tasks

This document explains the fields available when defining a new Task. For a higher-level overview of the Diego Task API, see the [Tasks Overview](tasks.md).

```go
client := bbs.NewClient(url)
err := client.DesireTask(
  "task-guid", // 'guid' parameter
  "domain",    // 'domain' parameter
  &models.TaskDefinition{
    RootFs: "docker:///docker.com/docker",
    EnvironmentVariables: []*models.EnvironmentVariable{
      {
        Name:  "FOO",
        Value: "BAR",
      },
    },
    CachedDependencies: []*models.CachedDependency{
      {
        Name: "app bits",
        From: "https://blobstore.com/bits/app-bits",
        To: "/usr/local/app",
        CacheKey: "cache-key",
        LogSource: "log-source",
        ChecksumAlgorithm: "md5",
        ChecksumValue: "the-checksum",
      },
    },
    Action: models.WrapAction(&models.RunAction{
      User:           "user",
      Path:           "echo",
      Args:           []string{"hello world"},
      ResourceLimits: &models.ResourceLimits{},
    }),
    MemoryMb:    256,
    DiskMb:      1024,
    MaxPids:     1024,
    CpuWeight:   42,
    Privileged:  true,
    LogGuid:     "123",
    LogSource:   "APP",
    MetricsGuid: "456",
    CompletionCallbackUrl: "http://36.195.164.128:8080",
    ResultFile:  "some-file.txt",
    EgressRules: []*models.SecurityGroupRule{
      {
        Protocol:     "tcp",
        Destinations: []string{"0.0.0.0/0"},
        PortRange: &models.PortRange{
          Start: 1,
          End:   1024,
        },
        Log: true,
      },
      {
        Protocol:     "udp",
        Destinations: []string{"8.8.0.0/16"},
        Ports:        []uint32{53},
      },
    },
    Annotation:                    "place any label/note/thing here",
    TrustedSystemCertificatesPath: "/etc/somepath",
    VolumeMounts: []*models.VolumeMount{
      {
        Driver:        "my-driver",
        VolumeId:      "my-volume",
        ContainerPath: "/mnt/mypath",
        Mode:          models.BindMountMode_RO,
      },
    }
    PlacementTags: []string{"tag-1", "tag-2"},
  }
)
```

### Task Identifiers

#### `guid` [required]

Diego clients must provide each Task with a unique Task identifier. Use this identifier to refer to the Task later.

- It is an error to create a Task with a guid matching that of an existing Task.
- The `guid` must include only the characters `a-z`, `A-Z`, `0-9`, `_` and `-`.
- The `guid` must not be empty.


#### `domain` [required]

Diego clients must label their Tasks with a domain. These domains partition the Tasks into logical groups, which clients may retrieve via the BBS API. Task domains are purely organizational (for example, for enabling multiple clients to use Diego without accidentally interfering with each other) and do not affect the Task's placement or lifecycle.

- It is an error to provide an empty `domain`.


### Task Definition Fields

#### Container Contents and Environment

##### `RootFs` [required]

The `RootFs` field specifies the root filesystem to use inside the container. One class of root filesystems are the `preloaded` root filesystems, which are directories colocated on the Diego Cells and registered with their cell reps. Clients specify a preloaded root filesystem in the form:

```go
RootFs: "preloaded:ROOTFS-NAME"
```

Cloud Foundry buildpack-based apps use the `cflinuxfs2` preloaded filesystem, built to work with Cloud Foundry buildpacks:

```go
RootFs: "preloaded:cflinuxfs2"
```

Clients may also provide a root filesystem based on a Docker image:

```go
RootFs: "docker:///docker-org/docker-image#docker-tag"
```

To pull the image from a different registry than Docker Hub, specify it as the host in the URI string, e.g.:

```go
RootFs: "docker://index.myregistry.gov/docker-org/docker-image#docker-tag"
```

##### `ImageUsername` [optional]

The `ImageUsername` field specifies the username to be used when fetching the
container image defined by the `RootFs` field from the image repository.

Setting `ImageUsername` requires the `ImagePassword` to also be set.

##### `ImagePassword` [optional]

The `ImagePassword` field specifies the password to be used when fetching the
container image defined by the `RootFs` field from the image repository.

Setting `ImagePassword` requires the `ImageUsername` to also be set.

##### `EnvironmentVariables` [optional]

See description of [Environment Variables](common-models.md#environmentvariables-optional)

##### `CachedDependencies` [optional]

See description of [Cached Dependencies](common-models.md#cacheddependencies-optional)

##### `TrustedSystemCertificatesPath` [optional]

An absolute path inside the container's filesystem where trusted system certificates will be provided if an operator has specified them.

##### `VolumeMounts` [optional]

See description of [Volume Mounts](common-models.md#volumemounts-optional)

##### `PlacementTags` [optional]

A set of tags that will be used to schedule the Task on specific cells.
An Task will only be placed on a cell if the tags in the Task's `PlacementTags`
are exactly the same as the tags advertised by the given cell.

For example:
- An Task with the placement tags ["tag-1"] will match only a cell advertising ["tag-1"]. It will not match a cell advertising ["tag-1", "tag-2"] or [].
- An Task with no placement tags will only match a cell advertising no tags.

#### Container Limits

##### `CpuWeight` [optional]

To control the CPU shares provided to a container, set `CpuWeight`. This must be a positive number in the range `1-100`. The `CpuWeight` enforces a relative fair share of the CPU among containers per unit time. To explain, suppose that container A and container B each runs a busy process that attempts to consume as much CPU as possible.

- If A and B each has `CpuWeight: 100`, their processes will receive approximately equal amounts of CPU time.
- If A has `CpuWeight: 25` and B has `CpuWeight: 75`, A's process will receive about one quarter of the CPU time, and B's process will receive about three quarters of it.


##### `DiskMb` [optional]

A disk quota in mebibytes applied to the container. Data written on top of the container's root filesystem counts against this quota. If it is exceeeded, writes will fail, but the container runtime will not kill processes in the container directly.

- The `DiskMb` value must be an integer greater than or equal to 0.
- If set to 0, no disk quota is applied to the container.

##### `MaxPids` [optional]

A maximum process limit is applied to the container. If the number of processes running on the container reach the limit, new processes spawned will fail.

- The `MaxPids` value must be an integer greater than or equal to 0.
- If set to 0, no process limit is applied to the container.

##### `MemoryMb` [optional]

A memory limit in mebibytes applied to the container.  If the total memory consumption by all processs running in the container exceeds this value, the container will be destroyed.

- The `MemoryMb` value must be an integer greater than or equal to 0.
- If set to 0, no memory quota is applied to the container.


##### `Privileged` [optional]

- If false, Diego will create a container that is in a user namespace.  Processes that run as root will actually be root only within the user namespace and will not have administrative privileges on the host system.
- If true, Diego creates a container without a user namespace, so that container root corresponds to root on the host system.


#### Actions

##### `Action` [required]

Encodes the action to execute when running the Task.  For more details, see the section on [Actions](actions.md).


#### Task Completion and Output

When the `Action` on a Task finishes, the Task is marked as `COMPLETED`.

##### `ResultFile` [optional]

If specified on a TaskDefinition, Diego retrieves the contents of this file from the container when the Task completes successfully. The retrieved contents are made available in the `Result` field of the [Task](https://godoc.org/code.cloudfoundry.org/bbs/models#Task) returned in a  [TaskResponse](https://godoc.org/code.cloudfoundry.org/bbs/models#TaskResponse).

- Diego only returns the first 10 kilobytes of the `ResultFile`.  If you need to communicate back larger datasets, consider using an `UploadAction` to upload the result file to another service.


##### `CompletionCallbackUrl` [optional]

Diego clients have several ways to learn that a Task has `COMPLETED`: they can poll the Task, subscribe to the Task event stream, or register a callback.

If a `CompletionCallbackUrl` is provided, Diego will send a `POST` request to the provided URL when the Task completes.  The body of the `POST` will include the [TaskResponse](https://godoc.org/code.cloudfoundry.org/bbs/models#TaskResponse).

- Almost any response from the callback will resolve the Task, thereby removing it from the BBS.
- If the callback responds with status code '503 Service Unavailable' or '504 Gateway Timeout', however, Diego will immediately retry the callback up to 3 times.

- If these status codes persist, if the callback times out, or if a connection cannot be established, Diego will try again after a short period of time, typically 30 seconds.
- After about 2 minutes without a successful response from the callback URL, Diego will give up on the task and delete it.

#### Networking

By default network access for any container is limited but some tasks may need specific network access and that can be setup using `egress_rules` field.


##### `EgressRules` [optional]

See description of [EgressRules](common-models.md#egressrules-optional)

---

#### Logging

Diego emits container metrics and logs generated by container processes to the [Loggregator](https://github.com/cloudfoundry/loggregator) system, in the form of [dropsonde log messages](https://github.com/cloudfoundry/dropsonde-protocol/blob/master/events/log.proto) and [container metrics](https://github.com/cloudfoundry/dropsonde-protocol/blob/master/events/metric.proto).

##### `LogGuid` [optional]

The `LogGuid` field sets the `AppId` on log messages coming from the Task.


##### `LogSource` [optional]

The `LogSource` field sets the default `SourceType` on the log messages. Individual Actions on the Task may override this field, so that different actions may be distinguished by the `SourceType` values on log messages.


##### `MetricsGuid` [optional]

The `MetricsGuid` field sets the `ApplicationId` on container metris coming from the Task.


#### Storing Arbitrary Metadata

##### `Annotation` [optional]

Diego allows arbitrary annotations to be attached to a Task.  The annotation may not exceed 10 kilobytes in size.



[back](README.md)
