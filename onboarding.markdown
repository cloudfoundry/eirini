# Onboarding a new Team Member

## Checklist

* Copy this section as tasks into a new chore
* Get access to [cloudfoundry.slack.com](https://slack.cloudfoundry.org/)
* Get access to the [tracker](https://www.pivotaltracker.com/n/projects/2172361) (ask the PM)
* [Add public SSH key to github.com](https://help.github.com/articles/connecting-to-github-with-ssh/) and verify
* Update [the onboarding document](https://github.com/cloudfoundry-incubator/bits-service/blob/master/docs/onboarding.markdown) if necessary

Additionally, for new members of the core team:

* Get onto to the github team / group (ask the PM)
* Create a [SL](https://control.softlayer.com) account (ask the PM)
* Create a [new VPN password](https://control.softlayer.com/account/user/profile) (DIY)
* Set up the [VPN client](http://knowledgelayer.softlayer.com/procedure/ssl-vpn-mac-os-x-1010) (DIY)
* Setup [`pass` access](https://github.com/cloudfoundry/eirini-private-config/tree/master#sensitive-passwords)
* Ask the PM to get added to the pairing board: https://pairup-ng.mybluemix.net/#eirini

## Project Overview

* [Inception doc](https://files.slack.com/files-pri/T02FL4A1X-FAED4MMSN/download/projecteirinipdf.pdf)
* [Demo]( https://files.slack.com/files-pri/T02FL4A1X-FADSGHCUR/download/eirini-demo.mp4)
* Code: https://code.cloudfoundry.org/eirini
* BOSH Release: https://github.com/andrew-edgar/eirini-release
* Pipeline: https://github.com/julzdiverse/eirini-release-ci
* Backlog: https://code.cloudfoundry.org/eirini/projects/1
* Pairing Board: https://eunomia.eu-de.mybluemix.net/
* Slack Channel: [#eirini-dev](https://cloudfoundry.slack.com/messages/C8RU3BZ26)

## Coding Guidelines

* Discuss whenever desired
* Some good [general advice](https://medium.com/@benbjohnson/standard-package-layout-7cdbc8391fc1)
* Comments
  - Generally avoid them
  - Codify things that need explanation (function names, interfaces, etc)
  - Only comment things that are surprising or deviate from a standard
* Tracker Updates
  - Mark git commits with the story number so that anyone picking up a story has a chance to see what code is in progress
  - If that isn't working, leave a comment in the story with a pointer to the commit
