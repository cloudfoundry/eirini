# BBS Conventions for Revising API Endpoints

The goal of this doc is to socialize conventions regarding the update of API endpoints in the BBS.
Specifically, we want to establish conventions for adding revision numbers to endpoint routes and handler names.

## API Endpoint and Handler Naming Conventions
BBS uses a library called [rata](https://github.com/tedsuo/rata) to create a mapping of endpoints to handlers.
Specifically, the library maps endpoints (like `/v1/desired_lrps/list`) and methods (like `GET` or `POST`) to a handler using a string like `DesiredLRPsRoute_r0`.
As the Diego team updates API endpoints in the BBS, we communicate changes to functionality using revisions in the endpoint route -- for example, `/v1/desired_lrps/list.r1`.
When we add the new revision, we update the mapping key to be called `DesiredLRPsRoute_r1`.

To maintain backwards compatibility, BBS continues to support older versions of API endpoints for some window of time.
Updating the version number in the beginning of the path (e.g. `/v1/`) is rare, because a bump to that number implies a much larger change, such as a rethinking of the data model.
Most updates are typically less impactful, such as adding a new field to an existing resource.
So, instead we append "revision" numbers to the api route.

If you're planning to introduce or update a BBS API endpoint, please adhere to the following conventions:
- The first version of the API endpoint should not explicitly include a revision number, and should start with the following format: `/v1/path/function` which maps to the handler key `PathFunction_r0`
- The next update to the API endpoint should add a revision number: `/v1/path/function.r1`, which maps to the handler key `PathFunction_r1`.
- The older endpoint (`/v1/path/function`) should continue to be supported, but the code should mark the endpoint as deprecated,
  both using a comment in the code, as well as an update to the [`deprecations.md` document](https://github.com/cloudfoundry/diego-release/blob/develop/docs/deprecations.md#bbs-1).
- Repeat as necessary

### Major Release Process
One operational goal of diego-release is to ensure that operators can successfully upgrade diego-release,
so long as they hit each major version bump.
For example, to upgrade from diego-release v0.1488.0 to diego-release v2.14.0, an operator should be able to upgrade cleanly by first upgrading to some v1.x.y of diego-release before going to v2.14.0.
To be explicit, directly upgrading from v0.1488.0 to v2.14.0 is not supported.

To support that upgrade workflow, depreacted endpoints in the BBS should remain intact for at least one entire major version.
For example, if an endpoint `/v1/desired_lrps/list` was deprecated during diego-release v0.1488.0 in favor of `/v1/desired_lrps/list.r1`,
it will continue to exist for the lifetime of diego-release v1.x.y.
When the Diego team cuts v2.0.0, deprecated endpoints will be removed from BBS.

At that time, the latest revision of the endpoint will be the only functional one.
In the example above, that endpoint would be `/v1/desired_lrps/list.r1`.
This route will not change at this time.
Similarly, the handler key `DesiredLRPsRoute_r1` will also remain the same.

For example, the table below shows how revisions `r0` through `r3` would be supported in sequential major releases of diego-release
based on when each was introduced:

| diego-release 0.x.y              | diego-release 1.x.y              | diego-release 2.x.y        | diego-release 3.x.y        |
| -------------------------------- | -------------------------------- | -------------------------- | -------------------------- |
| `/v1/desired_lrps/list`          | `/v1/desired_lrps/list`          | `/v1/desired_lrps/list.r2` | `/v1/desired_lrps/list.r3` |
| `/v1/desired_lrps/list.r1` (new) | `/v1/desired_lrps/list.r1`       | `/v1/desired_lrps/list.r3` |                            |
|                                  | `/v1/desired_lrps/list.r2` (new) |                            |                            |
|                                  | `/v1/desired_lrps/list.r3` (new) |                            |                            |

# See Also
- To find more documentation about specific endpoints, look in the [table of contents](README.md) for links to API references.
- To find information about endpoints that are deprecated (that is, still supported but planned to be removed in the next major release), look at [this document in diego-release](https://github.com/cloudfoundry/diego-release/blob/develop/docs/deprecations.md#bbs-1).
