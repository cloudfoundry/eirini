#!/bin/bash

set -euo pipefail

readonly BASEDIR="$(cd "$(dirname "$0")"/.. && pwd)"
export GO111MODULE=on
if [ -z ${EIRINIUSER_PASSWORD+x} ]; then
  EIRINIUSER_PASSWORD="$(pass eirini/docker-hub)"
fi

nodes=""
if [[ "${NODES:-}" != "" ]]; then
  nodes="-nodes $NODES"
fi

build() {
  ginkgo build -mod=vendor -r
}

run() {
  export EIRINI_BINS_PATH
  EIRINI_BINS_PATH=$(mktemp -d)
  trap "rm -rf $EIRINI_BINS_PATH" EXIT

  ginkgo -mod=vendor -r -p $nodes -keepGoing -randomizeAllSpecs -randomizeSuites -timeout=20m "$@" $(find . -name "*.test")
}

main() {
  action=${1:-"run"}
  if [ $# -gt 0 ]; then
    shift
  fi

  pushd "$BASEDIR"/tests/integration >/dev/null || exit 1
  {
    ${action}
  }
  popd >/dev/null || exit 1
}

main "$@"
