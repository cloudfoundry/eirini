# buildpackapplifecycle

**Note**: This repository should be imported as `code.cloudfoundry.org/buildpackapplifecycle`.

The buildpack lifecycle implements the traditional Cloud Foundry deployment
strategy.

The **Builder** downloads buildpacks and app bits, and produces a droplet.

The **Launcher** runs the start command using a standard rootfs and
environment.

Read about the app lifecycle spec here: https://github.com/cloudfoundry/diego-design-notes#app-lifecycles

### Running tests

On linux or windows, please use `ginkgo -r` from this directory.

On a mac, you should use docker to run the tests on a linux machine

```
$ docker build -f Dockerfile.linux.test -t buildpackapplifecycle .
$ docker run -it buildpackapplifecycle
```
