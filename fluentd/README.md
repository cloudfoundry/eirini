
# Supported tags and respective `Dockerfile` links

- `latest` [(Dockerfile)][latest-dockerfile]

# Usage

This image uses fluentd to read logs from k8s nodes and write them into a
loggregator agent. It is intended to be used as part of the [loggregator k8s
deployment][loggregator-k8s-deployment].

## Tests

1. Run `bundle install`
1. Run `rspec` to run the tests for the loggregator fluentd plugin.

[latest-dockerfile]: https://github.com/cloudfoundry/loggregator-ci/blob/master/docker-images/fluentd/Dockerfile
[loggregator-k8s-deployment]: https://code.cloudfoundry.org/loggregator-k8s-deployment
