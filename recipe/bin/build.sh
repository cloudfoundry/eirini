#!/bin/bash

set -euo pipefail

readonly BASEDIR="$(cd $(dirname $0)/.. && pwd)"
readonly TAG="${1?Provide a tag please}"

main() {
  build-image downloader
  build-image executor
  build-image uploader
}

build-image() {
  docker build -t "eirini/recipe-${1}:${TAG}" -f ${BASEDIR}/image/Dockerfile-${1} .
}

main
