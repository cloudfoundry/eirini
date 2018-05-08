# <div align="center"><img src="https://raw.githubusercontent.com/cloudfoundry-incubator/credhub/master/docs/images/logo.png" alt="CredHub"></div>

[![slack.cloudfoundry.org](https://slack.cloudfoundry.org/badge.svg)](https://slack.cloudfoundry.org)
[![GoDoc](https://godoc.org/github.com/golang/gddo?status.svg)](http://godoc.org/github.com/cloudfoundry-incubator/credhub-cli/credhub)

CredHub manages credentials like passwords, certificates, certificate authorities, ssh keys, rsa keys and arbitrary values (strings and JSON blobs). CredHub provides a CLI and API to get, set, generate and securely store such credentials.

* [CredHub Tracker](https://www.pivotaltracker.com/n/projects/1977341)

See additional repos for more info:

* [credhub](https://github.com/cloudfoundry-incubator/credhub) :     CredHub server code
* [credhub-acceptance-tests](https://github.com/cloudfoundry-incubator/credhub-acceptance-tests) : Integration tests
* [credhub-release](https://github.com/pivotal-cf/credhub-release) : BOSH release of CredHub server

### Installing the CLI

#### MacOS X with Homebrew
```bash
  brew install cloudfoundry/tap/credhub-cli
```

#### Linux and Windows
Download and install the desired release from the [release page](https://github.com/cloudfoundry-incubator/credhub-cli/releases).

### Building the CLI:

`make` (first time only to get dependencies, will also run specs)

`make build`

### Go Client

This repository contains a Go client library that can be used independently of the CredHub CLI.  Documentation for this library can be found [here](https://godoc.org/github.com/cloudfoundry-incubator/credhub-cli/credhub).


### Usage:

CredHub CLI can be used to manage credentials stored in a CredHub server. You must first target the CredHub server using the `api` command. Once targeted, you must login with either user or client credentials. Future commands will be sent to the targeted server. For additional information on how to perform CLI operations, you may review the examples shown [here][1] or review the help menus with the commands `credhub --help` and `credhub <command> --help`.

[1]:https://credhub-api.cfapps.io
