##### `EnvironmentVariables` [optional]

Clients may define environment variables at the container level, which all processes running in the container will receive. For example:

```go
EnvironmentVariables: []*models.EnvironmentVariable{
  {
    Name:  "FOO",
    Value: "BAR",
  },
  {
    Name:  "LANG",
    Value: "en_US.UTF-8",
  },
}
```

For more details on the environment variables provided to processes in the container, see the section on the [Container Runtime Environment](environment.md)

##### `CachedDependencies` [optional]

List of dependencies to cache on the Diego Cell and then to bind-mount into the container at the specified location. For example:

```go
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
```

The `ChecksumAlgorithm` and `ChecksumValue` are optional and used to validate the downloaded binary.  They must be used together.

##### `VolumeMounts` [optional]

Volume Mounts are used to specify persistent storage to be attached to a container in either a Task or LRP.

You can define the specific storage subsystem driver, volumeId, path in the container, bind mount mode as well as
some driver specific configuration information.

See the model documentation for VolumeMount [here](https://godoc.org/code.cloudfoundry.org/bbs/models#VolumeMount)

```go
VolumeMounts: []*models.VolumeMount{
  {
    Driver:        "my-driver",
    VolumeId:      "my-volume",
    ContainerPath: "/mnt/mypath",
    Mode:          models.BindMountMode_RO,
  },
}
```

##### `EgressRules` [optional]

List of firewall rules applied to the Task container. If traffic originating inside the container has a destination matching one of the rules, it is allowed egress. For example,

```go
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
    Annotations:  []string{"sec-group-banana", "sec-group-avocado"},
  },
}
```

This list of rules allows all outgoing TCP traffic bound for ports 1 though 1024 and UDP traffic to subnet 8.8.0.0/16 on port 53. Syslog messages are emitted for new connections matching the TCP rule.

###### `Protocol` [required]

The protocol type of the rule can be one of the following values: `tcp`, `udp`,`icmp`, or `all`.

###### `Destinations` [required]

List of string representing a single IPv4 address (`1.2.3.4`), a range of IPv4 addresses (`1.2.3.4-2.3.4.5`), or an IPv4 subnet in CIDR notation (`1.2.3.4/24`).


###### `Ports` and `PortRange` [optional]

The `Ports` field is a list of integers between 1 and 65535 that correspond to destination ports.
The `PortRange` field is a struct with a `Start` field and an `End` field, both integers between 1 and 65535. These values are required and signify the start and end of the port range, inclusive.

- Either `Ports` or `PortRange` must be provided for protocol `tcp` and `udp`.
- It is an error to provide both.

###### `IcmpInfo` [optional]

The `IcmpInfo` field stores two fields with parameters that pertain to ICMP traffic:

- `Type` [required]: integer between 0 and 255
- `Code` [required]: integer

```go
rule := &SecurityGroupRule{
  Protocol: "icmp",
  IcmpInfo: &ICMPInfo{Type: 8, Code: 0}
}
```

- `IcmpInfo` is required for protocol `icmp`.
- It is an error to provide for other protocols.

###### `Log` [optional]

If true, the system will log new outgoing connections that match the rule.

- `Log` is optional for all protocol types (`tcp`, `udp`, `icmp`, and `all`.
- To ensure that they apply first, put all rules with `Log` set to true at the **end** of the rule list.

###### `Annotations` [optional]

Array of client-specified annotations or comments to store on the rule.


##### Examples of Egress rules

---

Protocol `all`:

```go
all := &SecurityGroupRule{
    Protocol: "all",
    Destinations: []string{"1.2.3.4"},
    Log: true,
    Annotations:  []string{"sg-1234"},
}
```

---

Protocol `tcp`:

```go
tcp := &SecurityGroupRule{
    Protocol: "tcp",
    Destinations: []string{"1.2.3.4-2.3.4.5"},
    Ports: []int[80, 443],
    Log: true,
}
```

---

Protocol `udp`:

```go
udp := &SecurityGroupRule{
    Protocol: "udp",
    Destinations: []string{"1.2.3.4/8"},
    PortRange: {
        Start: 8000,
        End: 8085,
    },
}
```

---

Protocol `icmp`:

```go
icmp := &SecurityGroupRule{
    Protocol: "icmp",
    Destinations: []string{"1.2.3.4", "2.3.4.5/6"},
    IcmpInfo: {
        Type: 1,
        Code: 40,
    },
}
```

[back](README.md)
