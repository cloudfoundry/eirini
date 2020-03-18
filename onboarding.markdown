# Onboarding a new Team Member

## Checklist
* Clone the [template chore](https://www.pivotaltracker.com/story/show/171856898) in Tracker and follow the task list
* Update the template if necessary

## Project Overview

* [Inception doc](https://files.slack.com/files-pri/T02FL4A1X-FAED4MMSN/download/projecteirinipdf.pdf)
* [Demo]( https://files.slack.com/files-pri/T02FL4A1X-FADSGHCUR/download/eirini-demo.mp4)
* [Talks about Eirini](http://eirini.cf/#/talks)
* [Onboarding presentation](https://cloudfoundry.slack.com/files/UACLP8DGC/FPJDTH885/projecteirini_v2.0.1.key)
* [Kubernetes tutorial](https://kubernetes.io/docs/tutorials/hello-minikube/)
* [Code](https://code.cloudfoundry.org/eirini)
* [Release](https://code.cloudfoundry.org/eirini-release)
* [CI Pipeline](https://jetson.eirini.cf-app.com/teams/main/pipelines/ci)
* [Release Pipeline](https://jetson.eirini.cf-app.com/teams/main/pipelines/eirini-release)
* [SCF](https://github.com/SUSE/scf) and [fissile](https://github.com/cloudfoundry-incubator/fissile)
* [Slack Channel #eirini-dev](https://cloudfoundry.slack.com/messages/C8RU3BZ26)

## Coding Guidelines

* Discuss whenever desired
* We use linters and formatters whenever we can (golangci-lint, shfmt, etc)
* Comments
  - Generally avoid them
  - Codify things that need explanation (function names, interfaces, etc)
  - Only comment things that are surprising or deviate from a standard
* Tracker Updates
  - Mark git commits with the story number so that anyone picking up a story has a chance to see what code is in progress
  - If that isn't working, leave a comment in the story with a pointer to the commit
