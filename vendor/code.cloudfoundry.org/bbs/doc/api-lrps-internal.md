# Long Running Processes Internal API Reference

This reference does not cover the protobuf payload supplied to each endpoint.
Instead, it illustrates calls to the API via the Golang `bbs.InternalClient` interface.
Each method on that `InternalClient` interface takes a `lager.Logger` as the first argument to log errors generated within the client.
This first `Logger` argument will not be duplicated on the descriptions of the method arguments.

For detailed information on the types referred to below, see the [godoc documentation for the BBS models](https://godoc.org/code.cloudfoundry.org/bbs/models).

# ActualLRP APIs

## ClaimActualLRP

The cell calls `ClaimActualLRP` to report to the BBS that it has claimed an ActualLRP instance.

### BBS API Endpoint

POST an [ClaimActualLRPRequest](https://godoc.org/code.cloudfoundry.org/bbs/models#ClaimActualLRPRequest)
to `/v1/actual_lrps/claim`
and receive an [ActualLRPLifecycleResponse](https://godoc.org/code.cloudfoundry.org/bbs/models#ActualLRPLifecycleResponse).

### Golang Client API

```go
ClaimActualLRP(logger lager.Logger, processGuid string, index int, instanceKey *models.ActualLRPInstanceKey)
```

#### Inputs

* `processGuid`: The GUID of the corresponding DesiredLRP.
* `index int`: Index of the ActualLRP.
* `instanceKey *models.ActualLRPInstanceKey`: InstanceKey for the ActualLRP to claim.
  * `InstanceGuid string`: The GUID of the instance to claim.
  * `CellID string`: ID of the Cell claiming the ActualLRP.

#### Output

* `error`:  Non-nil if an error occurred.


#### Example

```go
client := bbs.NewClient(url)
err := client.ClaimActualLRP(logger, "some-guid", 0, &models.ActualLRPInstanceKey{
	InstanceGuid: "some-instance-guid",
	CellId: "some-cellID",
)
if err != nil {
    log.Printf("failed to claim actual lrp: " + err.Error())
}
```

## StartActualLRP

The cell calls `StartActualLRP` to report to the BBS that it has started an ActualLRP instance.

### BBS API Endpoint

POST an [StartActualLRPRequest](https://godoc.org/code.cloudfoundry.org/bbs/models#StartActualLRPRequest)
to `/v1/actual_lrps/start`
and receive an [ActualLRPLifecycleResponse](https://godoc.org/code.cloudfoundry.org/bbs/models#ActualLRPLifecycleResponse).

### Golang Client API

```go
StartActualLRP(logger lager.Logger, key *models.ActualLRPKey, instanceKey *models.ActualLRPInstanceKey, netInfo *models.ActualLRPNetInfo) error
```

#### Inputs

* `key *models.ActualLRPKey`: [ActualLRPKey](https://godoc.org/code.cloudfoundry.org/bbs/models#ActualLRPKey) for the instance. Includes the LRP process guid, index, and LRP domain.
* `instanceKey *models.ActualLRPInstanceKey`: [ActualLRPInstanceKey](https://godoc.org/code.cloudfoundry.org/bbs/models#ActualLRPInstanceKey) for the ActualLRP to start.
  * `InstanceGuid string`: The GUID of the instance to start.
  * `CellID string`: ID of the Cell starting the ActualLRP.
* `netInfo *models.ActualLRPNetInfo`: [ActualLRPNetInfo](https://godoc.org/code.cloudfoundry.org/bbs/models#ActualLRPNetInfo) containing updated networking information for the ActualLRP.

#### Output

* `error`:  Non-nil if an error occurred.


#### Example

```go
client := bbs.NewClient(url)
err := client.StartActualLRP(logger, &models.ActualLRPKey{
	   ProcessGuid: "some-guid",
	   Index: 0,
	   Domain: "some-domain",
	},
	&models.ActualLRPInstanceKey{
	    InstanceGuid: "some-instance-guid",
	    CellId: "some-cellID",
	},
	&models.ActualLRPNetInfo{
	    Address: "1.2.3.4",
	    models.NewPortMapping(10,20),
	    InstanceAddress: "2.2.2.2",
	},
)
if err != nil {
    log.Printf("failed to start actual lrp: " + err.Error())
}
```

## CrashActualLRP

The cell calls `CrashActualLRP` to report to the BBS that an ActualLRP instance it was running has crashed.

### BBS API Endpoint

POST an [CrashActualLRPRequest](https://godoc.org/code.cloudfoundry.org/bbs/models#CrashActualLRPRequest)
to `/v1/actual_lrps/crash`
and receive an [ActualLRPLifecycleResponse](https://godoc.org/code.cloudfoundry.org/bbs/models#ActualLRPLifecycleResponse).

### Golang Client API

```go
CrashActualLRP(logger lager.Logger, key *models.ActualLRPKey, instanceKey *models.ActualLRPInstanceKey, errorMessage string) error
```

#### Inputs

* `key *models.ActualLRPKey`: [ActualLRPKey](https://godoc.org/code.cloudfoundry.org/bbs/models#ActualLRPKey) for the instance. Includes the LRP process guid, index, and LRP domain.
* `instanceKey *models.ActualLRPInstanceKey`: [ActualLRPInstanceKey](https://godoc.org/code.cloudfoundry.org/bbs/models#ActualLRPInstanceKey) for the ActualLRP to crash.
  * `InstanceGuid string`: The GUID of the instance to crash.
  * `CellID string`: ID of the Cell crashing the ActualLRP.
* `errorMessage string`: The error message describing the reason for the crash.

#### Output

* `error`:  Non-nil if an error occurred.


#### Example

```go
client := bbs.NewClient(url)
err := client.CrashActualLRP(logger, &models.ActualLRPKey{
	   ProcessGuid: "some-guid",
	   Index: 0,
	   Domain: "some-domain",
	},
	&models.ActualLRPInstanceKey{
	    InstanceGuid: "some-instance-guid",
	    CellId: "some-cellID",
	},
	"Crashed Reason",
)
if err != nil {
    log.Printf("failed to crash actual lrp: " + err.Error())
}
```

## FailActualLRP

The auctioneer calls `FailActualLRP` to report to the BBS that it has failed to place an ActualLRP instance.

### BBS API Endpoint

POST an [FailActualLRPRequest](https://godoc.org/code.cloudfoundry.org/bbs/models#FailActualLRPRequest)
to `/v1/actual_lrps/fail`
and receive an [ActualLRPLifecycleResponse](https://godoc.org/code.cloudfoundry.org/bbs/models#ActualLRPLifecycleResponse).

### Golang Client API

```go
FailActualLRP(logger lager.Logger, key *models.ActualLRPKey, errorMessage string) error
```

#### Inputs

* `key *models.ActualLRPKey`: [ActualLRPKey](https://godoc.org/code.cloudfoundry.org/bbs/models#ActualLRPKey) for the instance. Includes the LRP process guid, index, and LRP domain.
* `errorMessage string`: The error message describing the reason for the placement failure.

#### Output

* `error`:  Non-nil if an error occurred.

#### Example

```go
client := bbs.NewClient(url)
err := client.FailActualLRP(logger, &models.ActualLRPKey{
	   ProcessGuid: "some-guid",
	   Index: 0,
	   Domain: "some-domain",
	},
	"Failure Reason",
)
if err != nil {
    log.Printf("failed to fail actual lrp: " + err.Error())
}
```

## RemoveActualLRP

The cell calls `RemoveActualLRP` to remove from the BBS an ActualLRP instance it has claimed but for which it no longer has a container.

### BBS API Endpoint

POST an [RemoveActualLRPRequest](https://godoc.org/code.cloudfoundry.org/bbs/models#RemoveActualLRPRequest)
to `/v1/actual_lrps/remove`
and receive an [ActualLRPLifecycleResponse](https://godoc.org/code.cloudfoundry.org/bbs/models#ActualLRPLifecycleResponse).

### Golang Client API

```go
RemoveActualLRP(logger lager.Logger, processGuid string, index int, instanceKey *models.ActualLRPInstanceKey) error
```

#### Inputs

* `processGuid`: The GUID of the corresponding DesiredLRP.
* `index int`: Index of the ActualLRP.
* `instanceKey *models.ActualLRPInstanceKey`: [ActualLRPInstanceKey](https://godoc.org/code.cloudfoundry.org/bbs/models#ActualLRPInstanceKey) for the ActualLRP to remove. If present, must match the key in the BBS record. If nil, the ActualLRP is removed without requiring a match.

#### Output

* `error`:  Non-nil if an error occurred.

#### Example

```go
client := bbs.NewClient(url)
err := client.RemoveActualLRP(logger, "some-guid", 0, &models.ActualLRPInstanceKey{
	InstanceGuid: "some-instance-guid",
	CellId: "some-cellID",
)
)
if err != nil {
    log.Printf("failed to remove an actual lrp: " + err.Error())
}
```

## EvacuateClaimedActualLRP

The cell calls `EvacuateClaimedActualLRP` to evacuate an ActualLRP it has claimed but not yet started.

### BBS API Endpoint

POST an [EvacuateClaimedActualLRPRequest](https://godoc.org/code.cloudfoundry.org/bbs/models#EvacuateClaimedActualLRPRequest)
to `/v1/actual_lrps/evacuate_claimed`
and receive an [EvacuationResponse](https://godoc.org/code.cloudfoundry.org/bbs/models#EvacuationResponse).

### Golang Client API

```go
EvacuateClaimedActualLRP(logger lager.Logger, key *models.ActualLRPKey, instanceKey *models.ActualLRPInstanceKey) (bool, error)
```

#### Inputs

* `key *models.ActualLRPKey`: [ActualLRPKey](https://godoc.org/code.cloudfoundry.org/bbs/models#ActualLRPKey) for the instance. Includes the LRP process guid, index, and LRP domain.
* `instanceKey *models.ActualLRPInstanceKey`: [ActualLRPInstanceKey](https://godoc.org/code.cloudfoundry.org/bbs/models#ActualLRPInstanceKey) for the claimed ActualLRP to evacuate.

#### Output

* `bool`: Flag indicating whether to keep the container. If `true`, keep the container. If `false`, destroy it.
* `error`:  Non-nil if an error occurred.

#### Example

```go
client := bbs.NewClient(url)
keepContainer, err := client.EvacuateClaimedActualLRP(logger, &models.ActualLRPKey{
	       ProcessGuid: "some-guid",
	       Index: 0,
	       Domain: "some-domain",
	},
	&models.ActualLRPInstanceKey{
	    InstanceGuid: "some-instance-guid",
	    CellId: "some-cellID",
)
if err != nil {
    log.Printf("failed to evacuate claimed actual lrp: " + err.Error())
}
```

## EvacuateCrashedActualLRP

The cell calls `EvacuateCrashedActualLRP` to report that an ActualLRP has crashed during evacuation.

### BBS API Endpoint

POST an [EvacuateCrashedActualLRPRequest](https://godoc.org/code.cloudfoundry.org/bbs/models#EvacuateCrashedActualLRPRequest)
to `/v1/actual_lrps/evacuate_crashed`
and receive an [EvacuationResponse](https://godoc.org/code.cloudfoundry.org/bbs/models#EvacuationResponse).

### Golang Client API

```go
EvacuateCrashedActualLRP(logger lager.Logger, key *models.ActualLRPKey, instanceKey *models.ActualLRPInstanceKey, errorMessage string) (bool, error)
```

#### Inputs

* `key *models.ActualLRPKey`: [ActualLRPKey](https://godoc.org/code.cloudfoundry.org/bbs/models#ActualLRPKey) for the instance. Includes the LRP process guid, index, and LRP domain.
* `instanceKey *models.ActualLRPInstanceKey`: [ActualLRPInstanceKey](https://godoc.org/code.cloudfoundry.org/bbs/models#ActualLRPInstanceKey) for the crashed ActualLRP to evacuate.
* `errorMessage string`: The error message describing the reason for the crash.

#### Output

* `bool`: Flag indicating whether to keep the container. If `true`, keep the container. If `false`, destroy it.
* `error`:  Non-nil if an error occurred.

#### Example

```go
client := bbs.NewClient(url)
keepContainer, err := client.EvacuateCrashedActualLRP(logger, &models.ActualLRPKey{
	       ProcessGuid: "some-guid",
	       Index: 0,
	       Domain: "some-domain",
	},
	&models.ActualLRPInstanceKey{
	    InstanceGuid: "some-instance-guid",
	    CellId: "some-cellID",
	"some error message",
)
if err != nil {
    log.Printf("failed to evacuate crashed actual lrp: " + err.Error())
}
```

## EvacuateStoppedActualLRP

The cell calls `EvacuateStoppedActualLRP` to report that an ActualLRP has stopped during evacuation.

### BBS API Endpoint

POST an [EvacuateStoppedActualLRPRequest](https://godoc.org/code.cloudfoundry.org/bbs/models#EvacuateStoppedActualLRPRequest)
to `/v1/actual_lrps/evacuate_stopped`
and receive an [EvacuationResponse](https://godoc.org/code.cloudfoundry.org/bbs/models#EvacuationResponse).

### Golang Client API

```go
EvacuateStoppedActualLRP(logger lager.Logger, key *models.ActualLRPKey, instanceKey *models.ActualLRPInstanceKey) (bool, error)
```

#### Inputs

* `key *models.ActualLRPKey`: [ActualLRPKey](https://godoc.org/code.cloudfoundry.org/bbs/models#ActualLRPKey) for the instance. Includes the LRP process guid, index, and LRP domain.
* `instanceKey *models.ActualLRPInstanceKey`: [ActualLRPInstanceKey](https://godoc.org/code.cloudfoundry.org/bbs/models#ActualLRPInstanceKey) for the stopped ActualLRP to evacuate.

#### Output

* `bool`: Flag indicating whether to keep the container. If `true`, keep the container. If `false`, destroy it.
* `error`:  Non-nil if an error occurred.

#### Example

```go
client := bbs.NewClient(url)
keepContainer, err := client.EvacuateStoppedActualLRP(logger, &models.ActualLRPKey{
	       ProcessGuid: "some-guid",
	       Index: 0,
	       Domain: "some-domain",
	},
	&models.ActualLRPInstanceKey{
	    InstanceGuid: "some-instance-guid",
	    CellId: "some-cellID",
	"some error message",
)
if err != nil {
    log.Printf("failed to evacuate stopped actual lrp: " + err.Error())
}
```

## EvacuateRunningActualLRP

The cell calls `EvacuateRunningActualLRP` to evacuate an ActualLRP it has started.

### BBS API Endpoint

POST an [EvacuateRunningActualLRPRequest](https://godoc.org/code.cloudfoundry.org/bbs/models#EvacuateRunningActualLRPRequest)
to `/v1/actual_lrps/evacuate_running`
and receive an [EvacuationResponse](https://godoc.org/code.cloudfoundry.org/bbs/models#EvacuationResponse).

### Golang Client API

```go
EvacuateRunningActualLRP(logger lager.Logger, key *models.ActualLRPKey, instanceKey *models.ActualLRPInstanceKey) (bool, error)
```

#### Inputs

* `key *models.ActualLRPKey`: [ActualLRPKey](https://godoc.org/code.cloudfoundry.org/bbs/models#ActualLRPKey) for the instance. Includes the LRP process guid, index, and LRP domain.
* `instanceKey *models.ActualLRPInstanceKey`: [ActualLRPInstanceKey](https://godoc.org/code.cloudfoundry.org/bbs/models#ActualLRPInstanceKey) for the running ActualLRP to evacuate.

#### Output

* `bool`: Flag indicating whether to keep the container. If `true`, keep the container. If `false`, destroy it.
* `error`:  Non-nil if an error occurred.

#### Example

```go
client := bbs.NewClient(url)
keepContainer, err := client.EvacuateRunningActualLRP(logger, &models.ActualLRPKey{
	       ProcessGuid: "some-guid",
	       Index: 0,
	       Domain: "some-domain",
	},
	&models.ActualLRPInstanceKey{
	    InstanceGuid: "some-instance-guid",
	    CellId: "some-cellID",
	"some error message",
)
if err != nil {
    log.Printf("failed to evacuate running actual lrp: " + err.Error())
}
```

## RemoveEvacuatingActualLRP

The cell calls `EvacuateRunningActualLRP` to remove an evacuating ActualLRP for which it no longer has a container.

### BBS API Endpoint

POST an [RemoveEvacuatingActualLRPRequest](https://godoc.org/code.cloudfoundry.org/bbs/models#RemoveEvacuatingActualLRPRequest) to `/v1/actual_lrps/remove_evacuating`, and receive an [RemoveEvacuatingActualLRPResponse](https://godoc.org/code.cloudfoundry.org/bbs/models#RemoveEvacuatingActualLRPResponse).

### Golang Client API

```go
RemoveEvacuatingActualLRP(logger lager.Logger, key *models.ActualLRPKey, instanceKey *models.ActualLRPInstanceKey) error
```

#### Inputs

* `key *models.ActualLRPKey`: [ActualLRPKey](https://godoc.org/code.cloudfoundry.org/bbs/models#ActualLRPKey) for the instance. Includes the LRP process guid, index, and LRP domain.
* `instanceKey *models.ActualLRPInstanceKey`: [ActualLRPInstanceKey](https://godoc.org/code.cloudfoundry.org/bbs/models#ActualLRPInstanceKey) for the evacuating ActualLRP to remove.

#### Output

* `error`:  Non-nil if an error occurred.

#### Example

```go
client := bbs.NewClient(url)
err := client.RemoveEvacuatingActualLRP(logger, &models.ActualLRPKey{
	       ProcessGuid: "some-guid",
	       Index: 0,
	       Domain: "some-domain",
	},
	&models.ActualLRPInstanceKey{
	    InstanceGuid: "some-instance-guid",
	    CellId: "some-cellID",
	"some error message",
)
if err != nil {
    log.Printf("failed to remove evacuating actual lrp: " + err.Error())
}
```
[back](README.md)
