# Cells API Reference

This reference does not cover the protobuf payload supplied to each endpoint.
Instead, it illustrates calls to the API via the Golang `bbs.Client` interface.
Each method on that `Client` interface takes a `lager.Logger` as the first argument to log errors generated within the client.
This first `Logger` argument will not be duplicated on the descriptions of the method arguments.

For detailed information on the types referred to below, see the [godoc documentation for the BBS models](https://godoc.org/code.cloudfoundry.org/bbs/models).


# Cells APIs

## Cells

### BBS API Endpoint

POST an empty request to `/v1/cells/list.r1` and receive a
[CellsResponse](https://godoc.org/code.cloudfoundry.org/bbs/models#CellsResponse).

#### Deprecated Endpoints

* Make a GET request to `/v1/cells/list.r1` and receive a
[CellsResponse](https://godoc.org/code.cloudfoundry.org/bbs/models#CellsResponse).

### Golang Client API

```go
Cells(logger lager.Logger) ([]*models.CellPresence, error)
```

#### Input

None.

#### Output

* `[]*models.CellPresence`: Slice of [`models.CellPresence`](https://godoc.org/code.cloudfoundry.org/bbs/models#CellPresence) pointers.
* `error`:  Non-nil if an error occurred.


#### Example

```go
client := bbs.NewClient(url)
cells, err := client.Cells(logger)
```
[back](README.md)
