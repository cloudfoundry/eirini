[![Build Status](https://travis-ci.org/cloudfoundry-incubator/eirini.svg?branch=master)](https://travis-ci.org/cloudfoundry-incubator/eirini)

# Eirini - what this?

Eirini is a Kubernetes backend for Cloud Foundry. It deploys CF apps to a kube backend, using OCI images and Kube deployments.

_But there's more!_

Eirini exports staged CF images as docker images. So you can schedule them however you'd like. *And separately* it gives you the nice integrated `cf push` flow, with CF Apps mapped directly to kube Deployment objects. In other words it decouples buildpack staging and stateless-multitenant-app running.

_But there's more!_

Eirini uses a little abstraction library, "OPI", which means it's not actually a Kube backend at all: it's a generic backend for any scheduler! This means it can schedule to diego/kube/swarm and whatever else is cool next year.

It uses the diego abstractions -- LRPs and Tasks -- in order to support generic orchestrators.

An experimental BOSH release for this is available at https://github.com/cloudfoundry-incubator/eirini-release

# y tho, y?

Scheduling is increasingly commodotised, it makes sense to ask how easy/hard it'd be to abstract our way out of it now.

# What components?

Eirini has the following components, the first two are available as subcommands of the `eirini` binary:
 
 - `Bifrost` converts and transfers cloud controller app specific requests to OPI specific objects and runs them in Kubernetes. It relies on the `Registry` to serve OCI images for droplets, and `OPI` to abstract the communication with Kube. 
 - `OPI` or the "orchestrator provider interface" provides a declarative abstraction over multiple schedulers inspired by Diego's LRP/Task model and Bosh's CPI concept.
 - `St8ger` implements Staging by running Kubernetes/OPI one-off tasks
 
# Tell me more 'bout OPI

The really great thing about Diego is the high level abstractions above the level of containers and pods. Specifically, these are LRPs and Tasks. Actually, LRPs and Tasks are most of what you need to build both a PaaS and quite a lot of other things. And they're cross-cutting concepts that map nicely to all current orchestrators (for example to LRPs/Tasks directly in Diego, to Deployments/One-Off Tasks in Kube, and to Services and Containers in Swarm).

One of the great things about Bosh is the CPI abstraction that lets it work on any IaaS. But so far Cloud Foundry has been tightly coupled to one specific Orchestrator (Diego). This was fine for fast iteration, but now orchestration is increasingly commodotised it makes a lot of sense to abstract ourselves away from the details of scheduling so an operator can use whatever orchestrator he or she wants and higher level systems can support all of them for free.

OPI uses the LRP/Task abstractions to do that.

# The OPI config file

In order to start OPI you need to call the `connect` command and provide a OPI config YAML file:

`$ opi connect --config path/to/config.yml`

```yaml
opi:
  kube_config: "path/to/kube/config/file"
  kube_namespace: "the kubernetes namespace used by the cf deployment"
  kube_endpoint: "the kubernetes endpoint where to schedule workload to"
  api_endpoint: "the CF API endpoint (eg. api.bosh-lite.com)"
  cf_username: "cf admin user"
  cf_password: "cf admin password"
  cc_internal_user: "cloud controller internal user"
  cc_internal_password: "cloud controller internal password"
  external_eirini_address: "the external eirini address"
  stager_image_tag: "The tag of the recipe image, which is used to stage an app. If empty, latest is used."
```

# Development

* `go get code.cloudfoundry.org/eirini`
* `cd` into the package you want to test
* `ginkgo`

Some integration tests require a running [`minikube`](https://github.com/kubernetes/minikube#installation).
