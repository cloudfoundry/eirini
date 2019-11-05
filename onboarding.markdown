# Onboarding a new Team Member

## Checklist

* Copy this section as tasks into a new chore
* Get access to [cloudfoundry.slack.com](https://slack.cloudfoundry.org/)
* Get access to the [tracker](https://www.pivotaltracker.com/n/projects/2172361) (ask the anchor)
* [Add public SSH key to github.com](https://help.github.com/articles/connecting-to-github-with-ssh/) and verify
* Ask the anchor to be invited to the retro
* Ask the PM to be invited to the IPM
* Ask the anchor to be [added to the eirini@cloudfoundry.org](https://groups.google.com/a/cloudfoundry.org/forum/#!managemembers/eirini/members) mailing list
* Ask a team member to add you to the GCP Project
* Ask the anchor to be added to the [CF Eirini team](https://github.com/orgs/cloudfoundry-incubator/teams/eirini/members) and the [CI](https://github.com/orgs/cf-cube-ci/teams/cube/members) team
* Ask @smoser-ibm to create a [SL](https://control.softlayer.com) account for you
* Setup [`pass` access](https://github.com/cloudfoundry/eirini-private-config/tree/master#sensitive-passwords)
* Add yourself to the [pairing board](https://pairup-ng.mybluemix.net/#eirini)
* Update [the onboarding document](https://github.com/cloudfoundry-incubator/eirini/blob/master/onboarding.markdown) if necessary

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
