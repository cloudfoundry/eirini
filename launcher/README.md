# Launcher

The launcher package contains all necessary code to run a cf-app on kube. It is used by the `k8s` package and kube deployments. 

## The `launchcmd`

This piece of code wraps `code.cloudfoundry.org/buildpackapplifecycle/launcher`. It is responsible to parse the `startup_command` from the `staging_info.yml` inside cf-app if no startup command was provided. As we use `packs` to do the staging, it is usually the case that no startup command is provided.  

The resulting binary of `launchcmd` is provided together with `code.cloudfoundry.org/buildpackapplifecycle/launcher` binary in the `cubefs.tar`. The `cubefs` is the basic `cflinuxfs2` + `cube specific binaries` (`launch` and `launcher`).

## The launcher

The launcher just provides some environment setup and the right launch command for an `opi.LRP`, which is required to run a cf-app successfully. 

## Building the cubefs.tar file

The `cubefs.tar` file needs to be provided to the registry job. To build it you just need to make sure you have `go` and `docker` installed on your machine and run the `launcher/bin/build-cubefs.sh` script.

*NOTES:*

- don't forget to init submodules
- the `buildpackapplifecycle` package has a comment with an import in `launcher/package.go`. This causes issues when creating the binary. You will need to remove it. 


