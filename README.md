<h1 align="center">
  <img src="logo.jpg" alt="Eirini">
</h1>

<!-- A spacer -->
<div>&nbsp;</div>

[![Build Status](https://travis-ci.org/cloudfoundry-incubator/eirini.svg?branch=master)](https://travis-ci.org/cloudfoundry-incubator/eirini)

## What is Eirini?

*Eirini* is a Kubernetes backend for Cloud Foundry. It deploys CF applications to a Kubernetes backend, using OCI images and Kubernetes `StatefulSet`s.

Since scheduling is increasingly commoditized, Eirini provides an _Orchestrator Provider Interface (OPI)_ layer, that abstracts away orchestration from Cloud Foundry's control plane. This means Eirini is not solely a Kube backend at all, but that it is a generic backend for any scheduler! This means it could schedule to Diego, Kubernetes, Swarm and other orchestration providers, as long as there is an implementation of the OPI layer for the target platform.

To offer a generic orchestrator interface, Eirini uses the Diego abstractions of _Long Running Processes (LRPs)_ and _Tasks_ to capture Cloud Foundry's notion of long running processes and one-off tasks.

Deployment instructions are available at: [cloudfoundry-incubator/eirini-release](https://github.com/cloudfoundry-incubator/eirini-release).

## Orchestrator Provider Interface (OPI)

The really great thing about Diego is the high level abstractions above the level of containers and pods. Specifically, these are _Long Running Processes (LRPs)_ and _Tasks_. Actually, LRPs and Tasks are most of what you need to build a PaaS, and they're cross-cutting concepts that map nicely to all current orchestrators (for example to LRPs/Tasks directly in Diego, to Deployments/Jobs in Kube, and to Services/Containers in Swarm).

Currently Eirini strictly provides a Kubernetes implementation of the OPI. However, this can be easily extended to support other orchestration platforms.

## Components

Eirini is composed of:

- `opi`: The main component, provides the REST API (implementing OPI) used by the Cloud Controller. It's responsible for starting LRPs and tasks.
- `event-reporter`
- `instance-index-env-injector`
- `metrics-collector`
- `route-collector`
- `route-pod-informer`
- `route-statefulset-informer`
- `staging-reporter`
- `task-reporter`
- `eirini-controller`

## Have a question or feedback? Reach out to us!

We can be found in our Slack channel [#eirini-dev](https://cloudfoundry.slack.com/archives/C8RU3BZ26) in the Cloud Foundry workspace. Please hit us up with any questions you may have or to share your experience with Eirini!
