# Running Integration Tests

## Run `build` tests

This suite is running the OPI build tests. It is basically testing if OPI can properly be build and run.

`$ ginkgo -- -valid_opi_config=<path-to-opi-config>`

You can find an example of the opi-config file in the `build` folder.

## Run `launch` tests

This suite is testing the functionality of the launcher. The launcher is responsible to start CF apps running in a OCI container. 

`$ ginkgo`

## Run `statefulsets` tests

This suite is testing edge cases of the `statefulset` functionality that can not be covered by unit-tests. 

`$ ginkgo`
