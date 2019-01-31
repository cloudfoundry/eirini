# Diego BBS API Docs

Diego's Bulletin Board System (BBS) is the central data store and orchestrator of a Diego cluster. It communicates via protocol-buffer-encoded RPC-style calls over HTTP.

Diego clients communicate with the BBS via an [ExternalClient](https://godoc.org/github.com/cloudfoundry/bbs#ExternalClient) interface. This interface allows clients to create, read, update, delete, and subscribe to events about Tasks and LRPs.

## Table of Contents

- [API Overview](overview.md)
- [Overview of Domains](domains.md)
- [Overview of Tasks](tasks.md)
  - [Defining Tasks](defining-tasks.md)
  - [Task Examples](task-examples.md)
- [Overview of Long Running Processes](lrps.md) (LRPs)
  - [Defining LRPs](defining-lrps.md)
  - [LRP Examples](lrp-examples.md)
- [Overview of Cells](cells.md)
- [Actions](actions.md)
- [The Container Runtime Environment](environment.md)
- [How BBS API endpoints get revisioned](revisioning-bbs-api-endpoints.md)
- External API Reference
  - [Tasks](api-tasks.md)
  - [LRPs](api-lrps.md)
  - [Cells](api-cells.md)
  - [Events](events.md)
  - [Domains](domains.md#api)
- Internal API Reference
  - [Tasks](api-tasks-internal.md)
  - [LRPs](api-lrps-internal.md)
- [Fields common to Tasks and LRPs](common-models.md)
- [Description of BBS SQL schema](schema-description.md)
