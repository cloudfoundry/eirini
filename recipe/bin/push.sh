#!/bin/bash

set -euo pipefail

readonly BASEDIR="$(cd $(dirname $0)/.. && pwd)"
readonly DOCKER_USER="${1?Provide a user please}"
readonly TAG="${2?Provide a tag please}"

main() {
  push-image downloader
  push-image executor
  push-image uploader
}

push-image() {
  docker push "${DOCKER_USER}/recipe-${1}:${TAG}"
}

main
