# Onboarding a new Team Member

## Checklist

* Copy this section as tasks into a new chore
* Get access to [cloudfoundry.slack.com](https://slack.cloudfoundry.org/)
* Get access to the [tracker](https://www.pivotaltracker.com/n/projects/2172361) (ask the anchor)
* [Add public SSH key to github.com](https://help.github.com/articles/connecting-to-github-with-ssh/) and verify
* Ask the anchor to be invited to the retro
* Ask the PM to be invited to the IPM
* Update [the onboarding document](https://github.com/cloudfoundry-incubator/eirini/blob/master/onboarding.markdown) if necessary

Additionally, for new members of the core team:

* Join the [Eirini](https://github.com/orgs/cloudfoundry-incubator/teams/eirini/members) and [CI](https://github.com/orgs/cf-cube-ci/teams/cube/members) teams
* Ask @smoser-ibm to create a [SL](https://control.softlayer.com) account for you
* Create a [new VPN password](https://control.softlayer.com/account/user/profile)
* Set up the [VPN client](http://knowledgelayer.softlayer.com/procedure/ssl-vpn-mac-os-x-1010)
* Setup [`pass` access](https://github.com/cloudfoundry/eirini-private-config/tree/master#sensitive-passwords)
* Add yourself to the [pairing board](https://pairup-ng.mybluemix.net/#eirini)

## Project Overview

* [Inception doc](https://files.slack.com/files-pri/T02FL4A1X-FAED4MMSN/download/projecteirinipdf.pdf)
* [Demo]( https://files.slack.com/files-pri/T02FL4A1X-FADSGHCUR/download/eirini-demo.mp4)
* [Code](https://code.cloudfoundry.org/eirini)
* [Release](https://code.cloudfoundry.org/eirini-release)
* [Core CI Pipeline](https://ci.flintstone.cf.cloud.ibm.com/teams/eirini/pipelines/ci)
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
