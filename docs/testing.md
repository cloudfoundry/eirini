# Eirini test suites and their scope

In the eirini project we have multiple test suites to cover all the functionality. This document explains the scope of each test suite
and how one can run it both locally and on CI.

## Dependencies

To run the tests you need a Linux machine with a few things installed:

1. [telepresence](https://www.telepresence.io/reference/install)
1. [helm v2](https://v2.helm.sh/docs/install/)
1. [skaffold](https://skaffold.dev/docs/install/)
1. [kind](https://kind.sigs.k8s.io/docs/user/quick-start/#installation)
1. [kubectl](https://kubernetes.io/docs/tasks/tools/install-kubectl/)
1. [docker](https://docs.docker.com/engine/install/)

If you'd like to spin up a VM rather than installing all these tools on your local machine, you can use our [eirini-station Vagrant machine](https://github.com/eirini-forks/eirini-station)

## Unit tests

Unit tests live next to the packages the test. You can trigger unit tests by running:

```shell
./scripts/check-everything.sh -u
```

## Integration tests

This test suite should be run against an empty Kubernetes cluster (no Eirini component deployed). Each test should run a single Eirini component (after building it from source) and point it to an empty cluster. If any dependencies are needed (e.g. CRDs) the test suite should deploy them on the cluster before the tests start and cleanup after they are done.

You can run this test suite like this:

```
./scripts/check-everything.sh -i
```

This suite should be possible to run locally

## EATs (Eirini Acceptance Tests)

This suite needs a deployed Eirini to run against. Currently there are 2 ways to deploy Eirini. There are [helm templates](https://github.com/cloudfoundry-incubator/eirini-release/tree/master/helm) and there are [plain yaml files](https://github.com/cloudfoundry-incubator/eirini-release/tree/master/deploy).

Assuming you have a running kubernetes cluster, the following commands should run EATs for you:

```shell
./scripts/check-everything.sh -e
```

EATs is an end-to-end suite, meaning it should check only final results/artifacts and not intermediate states (e.g. don't test that a LRP CR exists and a Pod is created, check its container is running as expected).

**NOTE:** Currently some CRD EATs actually do check StatefulSets and Pods. This is necessary because integration tests run binaries locally and running the crd controller binary in parallel causes the test to interfere with each other. This should be changed in the future.

## Linting

We use the [golanglint-ci](https://github.com/golangci/golangci-lint) for linting. If you'd like to run it use:

```shell
./scripts/check-everything.sh -l
```
