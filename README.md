[![Build Status](https://travis-ci.org/cloudfoundry-incubator/eirini.svg?branch=master)](https://travis-ci.org/cloudfoundry-incubator/eirini)

# What is Eirini?

*Eirini* is a Kubernetes backend for Cloud Foundry.
It deploys CF apps to a kube backend, using OCI images and Kube deployments.

Eirini gives you the nice integrated `cf push` flow,
with CF Apps mapped directly to kube `StatefulSet`.
In other words it decouples buildpack staging and stateless-multitenant-app running.

Since scheduling is increasingly commoditized, Eirini provides an "Orchestrator Provider Interface (OPI)" layer, that abstracts away orchestration from Cloud Foundry's control plane. This means Eirini is not solely a Kube backend at all, but that it is a generic backend for any scheduler! This means it could schedule to Diego, Kube, Swarm and other orchestration providers, as long as there is an implementation of the OPI layer for the target platform.

To offer a generic orchestrator interface, Eirini uses the Diego abstractions of LRPs and Tasks to capture Cloud Foundry's notion of long running processes and one-off tasks.

Deployment instructions are available at: [cloudfoundry-incubator/eirini-release](https://github.com/cloudfoundry-incubator/eirini-release)

# Eirini Components

Eirini has the following components, the first two are available as subcommands of the `eirini` binary:
 
 - `Bifrost` converts and transfers cloud controller app specific requests to OPI specific objects and runs them in Kubernetes. It relies on the [`bits-service`](https://github.com/cloudfoundry-incubator/bits-service) to serve OCI images for droplets, and `OPI` to abstract the communication with Kube.
 - `OPI` or the "Orchestrator Provider Interface" provides a declarative abstraction over multiple schedulers inspired by Diego's LRP/Task model and Bosh's CPI concept.
 - `Stager` implements staging by running Kubernetes/OPI one-off tasks
 
# Orchestrator Provider Interface (OPI)

The really great thing about Diego is the high level abstractions above the level of containers and pods.
Specifically, these are Long Running Processes (LRPs) and Tasks.
Actually, LRPs and Tasks are most of what you need to build a PaaS,
and they're cross-cutting concepts that map nicely to all current orchestrators
(for example to LRPs/Tasks directly in Diego,
to Deployments/Jobs in Kube,
and to Services/Containers in Swarm).

One of the great things about BOSH is the CPI abstraction that lets it work on any IaaS.
Cloud Foundry however, has been tightly coupled to one specific Orchestrator (Diego).

Currently Eirini strictly provides a Kubernetes implementation of the OPI.
However, this can be easily extended to support other orchestration platforms.

# Configuring OPI with Cloud Foundry and Kubernetes

In order to start OPI with the Kubernetes orchestration backend, you need to call the `connect` command and provide an OPI config YAML file:

`$ opi connect --config path/to/config.yml`

```yaml
opi:
  kube_config: "path/to/kube/config/file"
  kube_namespace: "the kubernetes namespace used by the cf deployment"
  kube_endpoint: "the kubernetes endpoint where to schedule workload to"
  api_endpoint: "the CF API endpoint (eg. api.bosh-lite.com)"
  cf_username: "cf admin user"
  cf_password: "cf admin password"
  external_eirini_address: "the external eirini address"
  stager_image_tag: "The tag of the recipe image, which is used to stage an app. If empty, latest is used."
```

# Development

Eirini is a Golang project.
You can simply get the code in your `GOPATH` and start development by running unit tests and integration tests like below
(Some integration tests require a running [`minikube`](https://github.com/kubernetes/minikube#installation)).

* `go get code.cloudfoundry.org/eirini`
* `cd` into the package you want to test
* `ginkgo`

For details on how you can contribute to the Eirini project,
please read the [CONTRIBUTING](CONTRIBUTING.md) document.
