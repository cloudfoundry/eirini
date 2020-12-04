# Eirini test suites and their scope

In the eirini project we have multiple test suites to cover all the functionality. This document explains the scope of each test suite
and how one can run it both locally and on CI.

## Unit tests

Unit tests live next to the packages the test. You can trigger unit tests by runnint the [run_unit_tests.sh script](https://github.com/cloudfoundry-incubator/eirini/blob/master/scripts/run_unit_tests.sh).

## EATs (Eirini Acceptance Tests)

This suite needs a deployed Eirini to run against. Currently there are 2 ways to deploy Eirini. There are [helm templates](https://github.com/cloudfoundry-incubator/eirini-release/tree/master/helm) and there are [plain yaml files](https://github.com/cloudfoundry-incubator/eirini-release/tree/master/deploy).

Assuming you have a running kubernetes cluster, the following commands should run EATs for you:

```
$ cd eirini-release
$ ./deploy/scripts/deploy.sh
$ cd ../eirini
$ ./scripts/run_eats_tests.sh
```

EATs is an end-to-end suite, meaning it should check only final results/artifacts and not intermediate states (e.g. don't test that a LRP CR exists and a Pod is created, check its container is running as expected).

TODO:
  We seem to use IsUsingDeployedEirini in EATs to skip some tests that need a fake capi server which the components need to talk back to. We should find a setup that gets rid of this code switch (relevant stories: #174354675, #174193327).

## Integration tests

This test suite should be run against an empty Kubernetes cluster (no Eirini component deployed). Each test should run a single Eirini component (after building it from source) and point it to an empty cluster. If any dependencies are needed (e.g. CRDs) the test suite should deploy them on the cluster before the tests start and cleanup after they are done.

You can run this test suite like this:

```
$ cd eirini
$ export INTEGRATION_KUBECONFIG=path_to_your_kubeconfig # Skip if you want to use your default kubeconfig
$ ./scripts/run_integration_tests.sh
```

This suite should be possible to run locally
