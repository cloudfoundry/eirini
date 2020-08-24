#!/bin/bash

set -euo pipefail

readonly BASEDIR="$(cd "$(dirname "$0")"/.. && pwd)"
export GO111MODULE=on
export EIRINIUSER_PASSWORD="${EIRINIUSER_PASSWORD:-$(pass eirini/docker-hub)}"

main() {
  pushd "$BASEDIR"/tests/eats >/dev/null || exit 1
  ginkgo -mod=vendor -p -r -keepGoing -randomizeAllSpecs -randomizeSuites -timeout=20m "$@"
  popd >/dev/null || exit 1
}

main "$@"
