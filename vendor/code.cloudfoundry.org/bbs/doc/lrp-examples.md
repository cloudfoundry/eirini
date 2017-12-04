# LRP Examples

## Desiring an LRP

```go
client := bbs.NewClient("http://10.244.16.2:8889")
err = client.DesireLRP(logger, &models.DesiredLRP{
    ProcessGuid:          "some-guid",
    Domain:               "some-domain",
    RootFs:               "some-rootfs",
    Instances:            1,
    EnvironmentVariables: []*models.EnvironmentVariable{{Name: "FOO", Value: "bar"}},
    CachedDependencies: []*models.CachedDependency{
      {
        Name:      "app bits",
        From:      "blobstore.com/bits/app-bits",
        To:        "/usr/local/app",
        CacheKey:  "cache-key",
        LogSource: "log-source",
      },
      {
        Name:              "app bits with checksum",
        From:              "blobstore.com/bits/app-bits-checksum",
        To:                "/usr/local/app-checksum",
        CacheKey:          "cache-key",
        LogSource:         "log-source",
        ChecksumAlgorithm: "md5",
        ChecksumValue:     "checksum-value",
      },
    },
    Setup:          models.WrapAction(&models.RunAction{Path: "ls", User: "name"}),
    Action:         models.WrapAction(&models.RunAction{Path: "ls", User: "name"}),
    StartTimeoutMs: 15000,
    Monitor: models.WrapAction(models.EmitProgressFor(
      models.Timeout(
        &models.RunAction{
          Path: "ls",
          User: "name",
        },
        10*time.Second,
      ),
      "start-message",
      "success-message",
      "failure-message",
    )),
    DiskMb:      512,
    MemoryMb:    1024,
    MaxPids:     1024,
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
})
```

## Polling for ActualLRP Info

```go
for {
  lrpGroups, err := client.ActualLRPGroupsByProcessGuid(logger, "some-guid")
  if err != nil {
    log.Printf("failed to fetch lrps!")
    panic(err)
  }
  log.Printf("You have %d instances of your LRP", len(lrpGroups))
  time.Sleep(time.Second)
}
```

## Recieving a LRPCallbackResponse

To receive the LRPCallbackResponse, we first start an HTTP server.

```go
func taskCallbackHandler(w http.ResponseWriter, r *http.Request) {
    var taskResponse models.LRPCallbackResponse
	data, err := ioutil.ReadAll(r.Body)
	if err != nil {
	    log.Printf("failed to read task response")
		panic(err)
	}
    err = json.Unmarshal(data, &taskResponse)
	if err != nil {
	    log.Printf("failed to unmarshal json")
		panic(err)
	}
    log.Printf("here's the result from your LRPCallbackResponse:\n %s\n\n", taskResponse.Result)
}

http.HandleFunc("/", taskCallbackHandler)
go http.ListenAndServe("7890", nil)
```

Suppose this server is running on IP `10.244.16.6`. When the above task is desired, it will run on Diego, echo 'hello world' to the file `result-file.txt`, and complete successfully. Diego will then POST a JSON-encoded `LRPCallbackResponse` to the server. The `Result` field of the `LRPCallbackResponse` will be the contents of the `result-file.txt` file, namely 'hello world'.


## Retiring an ActualLRP

```go
client := bbs.NewClient(url)
err := client.RetireActualLRP(logger, &models.ActualLRPKey{
    ProcessGuid: "some-process-guid",
    Index: 0,
    Domain: "some-domain",
})
if err != nil {
    log.Printf("failed to retire actual lrp: " + err.Error())
}
```

## Removing a DesiredLRP

```go
client := bbs.NewClient(url)
err := client.RemoveDesiredLRP(logger, "some-process-guid")
if err != nil {
    log.Printf("failed to remove desired lrp: " + err.Error())
}
```

[back](README.md)
