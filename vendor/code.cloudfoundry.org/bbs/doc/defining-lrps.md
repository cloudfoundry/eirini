### Defining LRPs

This document explains the fields available when defining a new LRP. For a higher-level overview of the Diego LRP API, see the [LRPs Overview](lrps.md).

```go
client := bbs.NewClient(url)
err := client.DesireLRP(logger, &models.DesiredLRP{
	ProcessGuid:          "some-guid",
	Domain:               "some-domain",
	RootFs:               "some-rootfs",
	Instances:            1,
	EnvironmentVariables: []*models.EnvironmentVariable{{Name: "FOO", Value: "bar"}},
	CachedDependencies: []*models.CachedDependency{
		{
			Name: "app bits",
			From: "blobstore.com/bits/app-bits",
			To: "/usr/local/app",
			CacheKey: "cache-key",
			LogSource: "log-source"
		},
		{
			Name: "app bits with checksum",
			From: "blobstore.com/bits/app-bits-checksum",
			To: "/usr/local/app-checksum",
			CacheKey: "cache-key",
			LogSource: "log-source",
			ChecksumAlgorithm: "md5",
			ChecksumValue: "checksum-value"
		},
	},
	Setup:          models.WrapAction(&models.RunAction{Path: "ls", User: "name"}),
	Action:         models.WrapAction(&models.RunAction{Path: "ls", User: "name"}),
	StartTimeoutMs: 15000,
	Monitor: models.WrapAction(models.EmitProgressFor(
		models.Timeout(
			&models.RunAction{
				Path: "ls",
				User: "name"
			},
			10*time.Second,
		),
		"start-message",
		"success-message",
		"failure-message",
	)),
	DiskMb:      512,
	MaxPids:     1024,
	MemoryMb:    1024,
	Privileged:  true,
	CpuWeight:   42,
	Ports:       []uint32{8080, 9090},
	Routes:      &models.Routes{"my-router": json.RawMessage(`{"foo":"bar"}`)},
	LogSource:   "some-log-source",
	LogGuid:     "some-log-guid",
	MetricsGuid: "some-metrics-guid",
	Annotation:  "some-annotation",
	Network: &models.Network{
		Properties: map[string]string{
			"some-key":       "some-value",
			"some-other-key": "some-other-value",
		},
	},
	EgressRules: []*models.SecurityGroupRule{{
		Protocol:     models.TCPProtocol,
		Destinations: []string{"1.1.1.1/32", "2.2.2.2/32"},
		PortRange:    &models.PortRange{Start: 10, End: 16000},
	}},
	ModificationTag:               &models.NewModificationTag("epoch", 0),
	LegacyDownloadUser:            "legacy-dan",
	TrustedSystemCertificatesPath: "/etc/somepath",
	VolumeMounts: []*models.VolumeMount{
		{
			Driver:        "my-driver",
			VolumeId:      "my-volume",
			ContainerPath: "/mnt/mypath",
			Mode:          models.BindMountMode_RO,
		},
	},
	PlacementTags: []string{"example-tag", "example-tag-2"},
	CheckDefinition: &models.CheckDefinition{
		Checks: []*models.Check{
			{
				HttpCheck: &models.HTTPCheck{
					Port:             12345,
					RequestTimeoutMs: 100,
					Path:             "/some/path",
				},
			},
		},
		LogSource: "health-check",
	}
})
```

#### LRP Identifiers

##### `ProcessGuid` [required]

It is up to the consumer of Diego to provide a *globally unique*
`ProcessGuid`.  To subsequently fetch the DesiredLRP and its ActualLRP you
refer to it by its `ProcessGuid`.

- The `ProcessGuid` must include only the characters `a-z`, `A-Z`, `0-9`, `_` and `-`.
- The `ProcessGuid` must not be empty
- If you attempt to create a DesiredLRP with a `ProcessGuid` that matches that
  of an existing DesiredLRP, Diego will attempt to update the existing
  DesiredLRP.  This is subject to the rules described in [updating
  DesiredLRPs](lrps.md#updating-desiredlrps) below.


##### `Domain` [required]

The consumer of Diego may organize LRPs into groupings called 'domains'.  These
are purely organizational (for example, for enabling multiple consumers to use
Diego without colliding) and have no implications on the ActualLRP's placement
or lifecycle.  It is possible to fetch all LRPs in a given domain.

- It is an error to provide an empty `Domain` field.

#### Instances

##### `Instances` [required]

Diego can run and manage multiple instances (`ActualLRP`s) for each
`DesiredLRP`. `Instances` specifies the number of desired instances and must
not be less than zero.

#### Container Contents and Environment

##### `RootFs` [required]

The `RootFs` field specifies the root filesystem to mount into the container.
Diego can be configured with a set of *preloaded* RootFSes.
These are named root filesystems that are already on the Diego Cells.

Preloaded root filesystems look like:

```
"rootfs": "preloaded:ROOTFS-NAME"
```

Diego's [BOSH release](https://github.com/cloudfoundry/diego-release) ships with the
`cflinuxfs2` filesystem root filesystem built to work with the Cloud Foundry buildpacks, which can be specified via
```
"rootfs": "preloaded:cflinuxfs2"
```

It is possible to provide a custom root filesystem by specifying a Docker image for `RootFs`:

```
"rootfs": "docker:///docker-user/docker-image#docker-tag"
```

To pull the image from a different registry than the default (Docker Hub), specify it as the host in the URI string, e.g.:

```
"rootfs": "docker://index.myregistry.gov/docker-user/docker-image#docker-tag"
```

> You *must* provide the dockerimage `RootFs` uri as above, including the leading `docker://`!

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

A set of tags that will be used to schedule the LRP on specific cells.
An LRP will only be placed on a cell if the tags in the LRP's `PlacementTags`
are exactly the same as the tags advertised by the given cell.

For example:
- An LRP with the placement tags ["tag-1"] will match only a cell advertising ["tag-1"]. It will not match a cell advertising ["tag-1", "tag-2"] or [].
- An LRP with no placement tags will only match a cell advertising no tags.

#### Container Limits

##### `CpuWeight` [optional]

To control the CPU shares provided to a container, set `CpuWeight`.
This must be a positive number between `1` and `100`, inclusive.
The `CpuWeight` enforces a relative fair share of the CPU among containers.
It's best explained with examples.
Consider the following scenarios (we shall assume that each container is running
a busy process that is attempting to consume as many CPU resources as possible):

- Two containers, with equal values of `CpuWeight`: both containers will receive equal shares of CPU time.
- Two containers, one with `"cpu_weight": 50` and the other with `"cpu_weight": 100`: the later will get (roughly) 2/3 of the CPU time, the former 1/3.

##### `DiskMb` [optional]

A disk quota applied to the entire container.
Any data written on top of the RootFS counts against the Disk Quota.
Processes that attempt to exceed this limit will not be allowed to write to disk.

- `DiskMb` must be an integer >= 0
- If set to 0 no disk constraints are applied to the container
- The units are megabytes

##### `MaxPids` [optional]

A maximum process limit is applied to the container. If the number of processes running on the container reach the limit, new processes spawned will fail.

- The `MaxPids` value must be an integer greater than or equal to 0.
- If set to 0, no process limit is applied to the container.

##### `MemoryMb` [optional]

A memory limit applied to the entire container.
If the aggregate memory consumption by all processs running in the container exceeds this value, the container will be destroyed.

- `MemoryMb` must be an integer >= 0
- If set to 0 no memory constraints are applied to the container
- The units are megabytes

##### `Privileged` [optional]

If false, Diego will create a container that is in a user namespace.
Processes that succesfully obtain escalated privileges (i.e. root access) will actually only
be root within the user namespace and will not be able to maliciously modify the host VM.
If true, Diego creates a container with no user namespace -- escalating to root gives the user *real* root access.

#### Actions

When an LRP instance is instantiated, a container is created with the specified `RootFs` mounted.
Diego is responsible for performing any container setup necessary to successfully launch processes and monitor said processes.

##### `Setup` [optional]

After creating a container, Diego will first run the action specified in the `Setup` field.
This field is optional and is typically used to download files and run (short-lived) processes that configure the container.
For more details on the available actions see [actions](actions.md).

- If the `Setup` action fails the `ActualLRP` is considered to have crashed and will be restarted

##### `Action` [required]

After completing any `Setup` action, Diego will launch the `Action` action.
This `Action` is intended to launch any long-running processes.
For more details on the available actions see [actions](actions.md).

##### `CheckDefinition` [optional] [experiemental]

`CheckDefinition` provides a more structured way to declare healthchecks. It is up to the `Rep` whether to use the `Monitor` action or the `CheckDefinition`. See `enable_declarative_healthcheck` property in the [Rep job spec](https://github.com/cloudfoundry/diego-release/blob/develop/jobs/rep/spec)

**NOTE**: The `CheckDefinition` is still an experimental feature.

###### `Checks` [repeated]

A list of health checks. Each healthcheck can be either a `TCPCheck` or `HTTPCheck`. It is an error to have both set.

###### `LogSource` [optional]

If provided, logs emitted by this process will be tagged with the value of the
`LogSource` string.  Otherwise, a default tag of `HEALTH` is used.

###### `HTTPCheck`

Defines an http health check.

- The `Port` must be a nonzero value and no greater than 65535.
- `RequestTimeoutMs` is the timeout in ms for the entire http request (includes tcp connection establish time, sending request and receiving the response).
- `Path` is the http request path used to in the http request.

`HTTPCheck` can fail for the following reaosons:

1. a connection cannot be established to the given port
2. a timeout error is encountered before the response body is received
3. a non-200 status code is received

###### `TCPCheck`

A TCP health check.

- The `Port` must be a nonzero value and no greater than 65535.
- `ConnectTimeoutMs` is the timeout in ms for establishing the TCP connection.

`TCPCheck` can fail for the following reaosons:

1. a connection cannot be established to the given port
2. a timeout error is encountered while establishing the tcp connection

##### `Monitor` [optional]

If provided, Diego will monitor the long running processes encoded in `Action` by periodically invoking the `Monitor` action.
If the `Monitor` action returns succesfully (exit status code 0), the container is deemed "healthy", otherwise the container is deemed "unhealthy".
Monitoring is quite flexible in Diego and is outlined in more detail [here](lrps.md#monitoring-health).

##### `StartTimeoutMs` [required]

If provided, Diego will give the `Action` action up to `StartTimeoutMs` seconds to become healthy before marking the LRP as failed.

The `DeprecatedStartTimeoutS` field has been deprecated in favor of
`StartTimeoutMs`. The `StartTimeoutMs` field is required and will be translated
into `DeprecatedStartTimeoutS` for older clients.

##### `LegacyDownloadUser` [optional]

For backwards compatibility, `LegacyDownloadUser` specifies the user for a
`DownloadAction`.

#### Networking

Diego can open and expose arbitrary `Ports` inside the container.
There are plans to generalize this support and make it possible to build custom service discovery solutions on top of Diego.
The API is likely to change in backward-incompatible ways as we work these requirements out.

By default network access for any container is limited but some LRPs might need specific network access and that can be setup using `EgressRules` field.
Rules are evaluated in reverse order of their position, i.e., the last one takes precedence.

##### `Ports` [optional]

`Ports` is a list of ports to open in the container.
Processes running in the container can bind to these ports to receive incoming traffic.
These ports are only valid within the container namespace and an arbitrary host-side port is created when the container is created.
This host-side port is made available on the `ActualLRP`.

##### `Routes` [optional]

`Routes` is a map where the keys identify route providers and the values hold information for the providers to consume.
The information in the map must be valid JSON but is not proessed by Diego.
The total length of the routing information must not exceed 4096 bytes.

##### `EgressRules` [optional]

See description of [EgressRules](common-models.md#egressrules-optional)

#### Logging

Diego uses [loggregator](https://github.com/cloudfoundry/loggregator) to emit logs generated by container processes to the user.

##### `LogGuid` [optional]

`LogGuid` controls the loggregator guid associated with logs coming from LRP processes.
One typically sets the `LogGuid` to the `ProcessGuid` though this is not strictly necessary.

##### `LogSource` [optional]

`LogSource` is an identifier emitted with each log line.
Individual `RunAction`s can override the `LogSource`.
This allows a consumer of the log stream to distinguish between the logs of different processes.

##### `MetricsGuid` [optional]

The `MetricsGuid` field sets the `ApplicationId` on container metris coming from the DesiredLRP.

##### Attaching Arbitrary Metadata

##### `Annotation` [optional]

Diego allows arbitrary annotations to be attached to a DesiredLRP.
The annotation must not exceed 10 kilobytes in size.

[back](README.md)
