#!/bin/bash

set -euo pipefail

readonly BASEDIR="$(cd $(dirname $0)/.. && pwd)"
readonly DOCKER_USER="${1?Provide a docker user please}"
readonly TAG="${2?Provide a tag please}"

main() {
  build-image downloader
  build-image executor
  build-image uploader
}

build-image() {
  docker build -t "${DOCKER_USER}/recipe-${1}:${TAG}" -f ${BASEDIR}/image/Dockerfile-${1} .
}

main
