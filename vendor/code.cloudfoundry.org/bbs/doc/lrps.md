# Overview of LRPs: Long Running Processes

Diego can distribute and monitor multiple instances of a Long Running Process (LRP).  These instances are distributed across Diego Cells and restarted automatically if they crash or disappear.  The instances are identical (though each instance is given a unique index (in the range `0, 1, ...N-1`) and a unique instance guid).

LRPs are described by providing Diego with a `DesiredLRP`.  The `DesiredLRP` can be thought of as a manifest that describes how an LRP should be executed and monitored.

The instances that end up running on Diego cells are referred to as `ActualLRP`s.  The `ActualLRP`s contain information about the state of the instance and about the host Cell the instance is running on.

When describing a property common to both `DesiredLRP`s and `ActualLRP`s (e.g. the `process_guid`) we may refer to both notions collectively simply as LRPs.

Diego is continually monitoring and reconciling desired state and actual state.  As such it is important to ensure that the desired state is up-to-date and accurate.  This is covered in detail in the section below on [Domain Freshness](#domain-freshness).

First, let's discuss DesiredLRPs.

## The LRP API

We recommend interacting with Diego's LRP functionality through the
ExternalDesiredLRPClient interface. The calls exposed to external clients are
specifically documented [here](https://godoc.org/github.com/cloudfoundry/bbs#ExternalDesiredLRPClient).

## The LRP Lifecycle

Here is a happy path overview of the LRP lifecycle:

- The Nsync listens for desired app requests and creates/modifies the DesiredLRPs via the BBS
- The Converger compares DesiredLRPs and their ActualLRPs and takes action to enforce the desired state
- Starts are requested and sent to the Auctioneer
    - The Rep starts the ActualLRP and updates its state in the BBS
- Stops are sent directly to the Reps
    - The Rep stops the ActualLRP and updates its state in the BBS

### ActualLRP States
State | Meaning
------|--------
`UNCLAIMED` | The ActualLRP is being scheduled by an auction.
`CLAIMED` | The ActualLRP has been assigned to a Cell and is being started.
`RUNNING` | The ActualLRP is running on a Cell and is ready to receive traffic/work.
`CRASHED`| The ActualLRP has crashed and is no longer on a Cell. It should be restarted (eventually).

## Defining LRPs

See [Defining LRPs](defining-lrps.md) for more details on the fields that should be provided
when submitting a DesiredLRP to a Client's `DesireLRP` method.

## Updating DesiredLRPs

Only a subset of the DesiredLRP's fields may be updated dynamically.  In particular, changes that require the process to be restarted are not allowed - instead, you should submit a new DesiredLRP and orchestrate the upgrade path from one LRP to the next.  This provides the consumer of Diego the flexibility to pick the most appropriate upgrade strategy (blue-green, etc...)

It is possible, however, to dynamically modify the number of instances, and the routes associated with the LRP.  Diego's API makes this explicit -- when updating a DesiredLRP you provide a `DesiredLRPUpdateRequest`:

```
{
    "instances": 17,
    "routes": {
        "cf-router": [
            {
                "hostnames": ["a.example.com", "b.example.com"],
                "port": 8080
            }, {
                "hostnames": ["c.example.com"],
                "port": 5050
            }
        ],
        "some-other-router": "any opaque json payload"
    },
    "annotation": "arbitrary metadata"
}
```

These may be provided simultaneously in one request, or independently over several requests.


## Monitoring Health

It is up to the consumer to tell Diego how to monitor an LRP instance.  If provided, Diego uses the `monitor` action to ascertain when an LRP is up.

Typically, an ActualLRP instance begins in an unhealthy state (`CLAIMED`).  At this point the `monitor` action is polled every 0.5 seconds.  Eventually the `monitor` action succeeds and the instance enters a healthy state (`RUNNING`).  At this point the `monitor` action is polled every 30 seconds.  If the `monitor` action subsequently fails, the ActualLRP is considered crashed.  Diego's consumer is free to define an arbitrary `monitor` action - a `monitor` action may check that a port is accepting connections, or that a URL returns a happy status code, or that a file is present in the container.  In fact, a single `monitor` action might be a composition of other actions that can monitor multiple processes running in the container.

Normally, the `action` action on the DesiredLRP does not exit.  It is possible, however, to launch and daemonize a process in Diego.  If the `action` action exits succesfully Diego assumes the process is a daemon and continues monitoring it with the `monitor` action.  If the `action` action fails (e.g. exit with non-zero status code for a `RunAction`) Diego assumes the ActualLRP has failed and schedules it to be restarted.

Finally, it is possible to opt out of monitoring.  If no `monitor` action is specified then the health of the ActualLRP is dependent on the `action` continuing to run indefinitely.  The ActualLRP is considered `RUNNING` as soon as the `action` action begins, and is considered to have failed if the `action` action ever exits.

> Note that Diego does not currently stream back logs for processes that daemonize.

## Fetching DesiredLRPs

Diego allows consumers to fetch DesiredLRPs -- the response object (`DesiredLRPResponse`) is identical to the `DesiredLRPCreateRequest` object described above.

When fetching DesiredLRPs one can fetch *all* DesiredLRPs in Diego, all DesiredLRPs of a given `domain`, and a specific DesiredLRP by `process_guid`.

The fact that a DesiredLRP is present in Diego does not mean that the corresponding ActualLRP instances are up and running.  Diego converges on the desired state and starting/stopping ActualLRPs may take time.  The presence of a DesiredLRP in Diego signifies the consumer's intent for Diego to run instances - not that those instances are currently running.  For that you must fetch the ActualLRPs.

## Fetching ActualLRPs

As outlined above, DesiredLRPs represent the consumer's intent for Diego to run instances.  To fetch instances, consumers must [fetch ActualLRPs](api-lrps.md#actuallrp-apis).

When fetching ActualLRPs, one can fetch *all* ActualLRPs in Diego, all ActualLRPs of a given `domain`, all ActualLRPs for a given DesiredLRP by `process_guid`, and all ActualLRPs at a given *index* for a given `process_guid`.

In all cases, the consumer is given an array of `ActualLRPResponse`:

```
[
    {
        "process_guid": "some-process-guid",
        "instance_guid": "some-instnace-guid",
        "cell_id": "some-cell-id",
        "domain": "some-domain",
        "index": 15,
        "state": "UNCLAIMED", "CLAIMED", "RUNNING" or "CRASHED"

        "address": "10.10.11.11",
        "ports": [
            {"container_port": 8080, "host_port": 60001},
            {"container_port": 5000, "host_port": 60002},
        ],

        "placement_error": "insufficient resources",

        "since": 1234567
    },
    ...
]
```

Let's describe each of these fields in turn.

### ActualLRP Identifiers

#### `process_guid`

The `process_guid` for this ActualLRP -- this is used to correlate ActualLRPs with DesiredLRPs.

#### `instance_guid`

An arbitrary identifier unique to this ActualLRP instance.

#### `cell_id`

The identifier of the Diego Cell running the ActualLRP instance.

#### `domain`

The `domain` associated with this ActualLRP's DesiredLRP.

#### `index`

The `index` of the ActualLRP - an integer between `0` and `N-1` where `N` is the desired number of instances.

#### `state`

The state of the ActualLRP.

When an ActualLRP is first created, it enters the `UNCLAIMED` state.

Once the ActualLRP is placed onto a Cell it enters the `CLAIMED` state.  During this time a container is being created and the various processes inside the container are being spun up.

When the `action` action begins running, Diego begins periodically running the `monitor` action.  As soon as the `monitor` action reports that the processes are healthy the ActualLRP will transition into the `RUNNING` state.

#### `placement_error`

When an ActualLRP cannot be placed because there are no resources to place it, the `placement_error` is populated with the reason.

> `placement_error` is only populated when the ActualLRP is in the `UNCLAIMED` state.

#### `since`

The last modified time of the ActualLRP represented as the number of nanoseconds elapsed since January 1, 1970 UTC.

#### Networking

#### `address`

`address` contains the externally accessible IP of the host running the container.

> `address` is only populated when the ActualLRP enters the `RUNNING` state.

#### `ports`

`ports` is an array containing mappings between the `container_port`s requested in the DesiredLRP and the `host_port`s associated with said `container_port`s.  In the example above to connect to the process bound to port `5000` inside the container, a request must be made to `10.10.11.11:60002`.

> `ports` is only populated when the ActualLRP enters the `RUNNING` state.

## Killing ActualLRPs

Diego supports killing the `ActualLRP`s for a given `process_guid` at a given `index`.  This is documented [here](api-lrps.md#retireactuallrp).  Note that this does not change the *desired* state -- Diego will simply shut down the `ActualLRP` at the given `index` and will eventually converge on desired state by restarting the (now-missing) instance.  To permanently scale down a DesiredLRP you must update the `instances` field on the DesiredLRP.


## Domain Freshness

Diego periodically compares desired state (the set of DesiredLRPs) to actual state (the set of ActualLRPs) and takes actions to keep the actual state in sync with the desired state.  This eventual consistency model is at the core of Diego's robustness.

In order to perform this responsibility safely, however, Diego must have some way of knowing that its knowledge of the desired state is complete and up-to-date.  In particular, consider a scenario where Diego's database has crashed and must be repopulated.  In this context it is possible to enter a state where the actual state (the ActualLRPs) are known to Diego but the desired state (the DesiredLRPs) is not.  It would be catastrophic for Diego to attempt to converge by shutting down all actual state!

To circumvent this, it is up to the consumer of Diego to inform Diego that its knowledge of the desired state is up-to-date.  We refer to this as the "freshness" of the desired state.  Consumers explicitly mark desired state as *fresh* on a domain-by-domain basis.  Failing to do so will prevent Diego from taking actions to ensure eventual consistency (in particular, Diego will refuse to stop extra instances with no corresponding desired state).

To maintain freshness you perform a simple [POST](domains.md#upserting-a-domain).  The consumer typically supplies a TTL and attempts to bump the freshness of the domain before the TTL expires (verifying along the way, of course, that the contents of Diego's DesiredLRP are up-to-date).

It is possible to opt out of this by updating the freshness with *no* TTL.  In this case the freshness will never expire and Diego will always perform all its eventual consistency operations.

> Note: only destructive operations performed during an eventual consistency convergence cycle are gated on freshness.  Diego will continue to start/stop instances when explicitly instructed to.

[back](README.md)
