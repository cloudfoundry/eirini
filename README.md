# What this?

Cube is a Kubernetes backend for Cloud Foundry. It syncs CF apps to a kube
backend in exactly the same way that the diego `nsync` component works, except
using OCI images and Kube deployments.

_But there's more!_

Cube exports staged CF images as docker images. So you can schedule them
however you'd like. *And separately* it gives you the nice integrated `cf push` flow,
with CF Apps mapped directly to kube Deployment objects. In other words it decouples buildpack
staging and stateless-multitenant-app running.

_But there's more!_

Cube uses a little abstraction library, "OPI", which means it's not actually a
Kube backend at all: it's a generic backend for any scheduler! This means it
can schedule to diego/kube/swarm and whatever else is cool next year.

It uses the diego abstractions -- LRPs and Tasks -- in order to support generic
orchestrators.

An experimental BOSH release for this is available at https://github.com/andrew-edgar/cube-release

# y tho, y?

Partly a fun experiment, partly a proof of concept. Scheduling is increasingly
commodotised, it makes sense to ask how easy/hard it'd be to abstract our way
out of it now.

# What components?

Cube has the following components, the first two are available as subcommands of the `cube` binary:
 
 - `Sink` provides a convergence loop that pulls desired apps from the Cloud Controller and creates corresponding Kubernetes resources. It relies on the `Registry` to serve OCI images for droplets, and `OPI` to abstract the communication with Kube. (Example: `cube sink --ccApi <api url> --ccPass <internal admin user password>`)
 - `Registry` is an OCI registry vending images based on droplets. Eventually this would be nice to move in to Cloud Controller. (Example: `cube registry --rootfs </path/to/rootfs.tar>`)
 - `OPI` or the "orchestrator provider interface" provides a declarative abstraction over multiple schedulers inspired by Diego's LRP/Task model and Bosh's CPI concept.
 - `St8ge` implements Staging by running Kubernetes/OPI one-off tasks
 
# Tell me more 'bout OPI

The really great thing about Diego is the high level abstractions above the
level of containers and pods. Specifically, these are LRPs and Tasks. Actually,
LRPs and Tasks are most of what you need to build both a PaaS and quite a lot
of other things. And they're cross-cutting concepts that map nicely to all
current orchestrators (for example to LRPs/Tasks directly in Diego, to
Deployments/One-Off Tasks in Kube, and to Services and Containers in Swarm).

One of the great things about Bosh is the CPI abstraction that lets it work on
any IaaS. But so far Cloud Foundry has been tightly coupled to one specific
Orchestrator (Diego). This was fine for fast iteration, but now orchestration is
increasingly commodotised it makes a lot of sense to abstract ourselves away
from the details of scheduling so an operator can use whatever orchestrator he
or she wants and higher level systems can support all of them for free.

OPI uses the LRP/Task abstractions to do that.
