<h1 align="center">
  <img src="logo.jpg" alt="Eirini">
</h1>

<!-- A spacer -->
<div>&nbsp;</div>

[![Build Status](https://travis-ci.org/cloudfoundry-incubator/eirini.svg?branch=master)](https://travis-ci.org/cloudfoundry-incubator/eirini)
[![Maintainability](https://api.codeclimate.com/v1/badges/e624538795c9e66d8667/maintainability)](https://codeclimate.com/github/cloudfoundry-incubator/eirini/maintainability)
[![Test Coverage](https://api.codeclimate.com/v1/badges/e624538795c9e66d8667/test_coverage)](https://codeclimate.com/github/cloudfoundry-incubator/eirini/test_coverage)
[![Go Report Card](https://goreportcard.com/badge/github.com/cloudfoundry-incubator/eirini)](https://goreportcard.com/report/github.com/cloudfoundry-incubator/eirini)
[![Slack Status](https://slack.cloudfoundry.org/badge.svg)](https://slack.cloudfoundry.org)

## What is Eirini?

_Eirini_ is a thin layer of abstraction on top of Kubernetes that allows Cloud
Foundry to deploy applications as Pods on a Kubernetes cluster. Eirini uses the
Diego abstractions of _Long Running Processes (LRPs)_ and _Tasks_ to capture Cloud
Foundry's notion of long running processes and one-off tasks.

Deployment instructions are available at:
[cloudfoundry-incubator/eirini-release](https://github.com/cloudfoundry-incubator/eirini-release).

## Components

![Eirini Overview Diagram](docs/architecture/EiriniOverview.png)

---

Eirini is composed of:

- `api`: The main component, provides the REST API used by
  the [Cloud Controller](https://github.com/cloudfoundry/cloud_controller_ng/).
  It's responsible for starting LRPs and tasks.

- `event-reporter`: A Kubernetes reconciler that watches for LRP instance
  crashes and reports them to the [Cloud
  Controller](https://github.com/cloudfoundry/cloud_controller_ng/).

- `instance-index-env-injector`: A Kubernetes webhook that inserts the
  [`CF_INSTANCE_INDEX`](https://docs.cloudfoundry.org/devguide/deploy-apps/environment-variable.html#CF-INSTANCE-INDEX)
  environment variable into every LRP instance (pod).

- `task-reporter`: A Kubernetes reconciler that reports the outcome of tasks to
  the [Cloud Controller](https://github.com/cloudfoundry/cloud_controller_ng/)
  and deletes the underlying Kubernetes Jobs after a configurable TTL has
  elapsed.

- `eirini-controller`: A Kubernetes reconciler that acts on
  create/delete/update operations on Eirini's own Custom Resouce Definitions
  (CRDs). This is still experimental.

## CI Pipelines

We use Concourse. Our pipelines can be found
[here](https://jetson.eirini.cf-app.com/).

## Contributing

Please read [CONTRIBUTING.md](.github/contributing.md) for details.

## Have a question or feedback? Reach out to us!

We can be found in our Slack channel
[#eirini-dev](https://cloudfoundry.slack.com/archives/C8RU3BZ26) in the Cloud
Foundry workspace. Please hit us up with any questions you may have or to share
your experience with Eirini!
