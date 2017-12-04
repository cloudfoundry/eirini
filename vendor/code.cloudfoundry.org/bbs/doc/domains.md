# Overview of Domains

The consumer of Diego may organize LRPs into groupings called 'domains'.  These
are purely organizational (for example, for enabling multiple consumers to use
Diego without colliding) and have no implications on the ActualLRP's placement
or lifecycle.  It is possible to fetch all LRPs in a given domain.

## Domain Freshness

Diego periodically compares desired state (the set of DesiredLRPs) to actual
state (the set of ActualLRPs) and takes actions to keep the actual state in
sync with the desired state.  This eventual consistency model is at the core of
Diego's robustness.

In order to perform this responsibility safely, however, Diego must have some
way of knowing that its knowledge of the desired state is complete and
up-to-date.  In particular, consider a scenario where Diego's database has
crashed and must be repopulated.  In this context it is possible to enter a
state where the actual state (the ActualLRPs) are known to Diego but the
desired state (the DesiredLRPs) is not.  It would be catastrophic for Diego to
attempt to converge by shutting down all actual state!

To circumvent this, it is up to the consumer of Diego to inform Diego that its
knowledge of the desired state is up-to-date.  We refer to this as the
"freshness" of the desired state.  Consumers explicitly mark desired state as
*fresh* on a domain-by-domain basis.  Failing to do so will prevent Diego from
taking destructive actions to ensure eventual consistency (in particular, Diego
will refuse to stop extra instances with no corresponding desired state).

To maintain freshness, [POST](#upserting-a-domain) a request to the
`/v1/domains/upsert` endpoint.  The
consumer typically supplies a TTL and attempts to bump the freshness of the
domain before the TTL expires (verifying along the way, of course, that the
contents of Diego's DesiredLRP are up-to-date).

It is possible to opt out of this by updating the freshness with *no* TTL.  In
this case the freshness will never expire and Diego will always perform all its
eventual consistency operations.

> Note: only destructive operations performed during an eventual consistency
> convergence cycle are gated on freshness.  Diego will continue to start/stop
> instances when explicitly instructed to.

## <a name="api"></a>API

### Upserting a domain

To mark a domain as fresh for N seconds (ttl):

POST an
[UpsertDomainRequest](https://godoc.org/code.cloudfoundry.org/bbs/models#UpsertDomainRequest)
to `/v1/domains/upsert`, and receive an
[UpsertDomainResponse](https://godoc.org/code.cloudfoundry.org/bbs/models#UpsertDomainResponse).


You must repeat the POST before the `ttl` expires.  To make the domain never
expire, set the `ttl` field to `0` or omit the field.

### Golang Client API

```go
UpsertDomain(logger lager.Logger, domain string, ttl time.Duration) error
```

#### Inputs

* `domain string`: Name of the domain to declare fresh.
* `ttl time.Duration`: Duration of time for Diego to consider the domain fresh. The value `0` means to consider the domain fresh permanently, unless reset to a finite TTL by a later `UpsertDomain` call.

#### Output

* `error`:  Non-nil if an error occurred.


#### Example

```go
client := bbs.NewClient(url)
err := client.UpsertDomain(logger, "my-domain", 60*time.Second)
```


### Fetching all "fresh" Domains

To fetch all fresh domains:

POST an empty body to `/v1/domains/list`, and receive a
[DomainsResponse](https://godoc.org/code.cloudfoundry.org/bbs/models#DomainsResponse).


### Golang Client API

```go
Domains(logger lager.Logger) ([]string, error)
```

#### Inputs

None.


#### Output

* `[]string`: Slice of strings representing the current domains.
* `error`:  Non-nil if an error occurred.


#### Example

```go
client := bbs.NewClient(url)
domains, err := client.Domains(logger)
```
[back](README.md)
