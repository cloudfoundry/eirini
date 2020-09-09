#!/bin/bash

set -euo pipefail

readonly BASEDIR="$(cd "$(dirname "$0")"/.. && pwd)"
export GO111MODULE=on
export EIRINIUSER_PASSWORD="${EIRINIUSER_PASSWORD:-$(pass eirini/docker-hub)}"

main() {
  pushd "$BASEDIR"/tests/integration >/dev/null || exit 1
  ginkgo -mod=vendor -p -r -keepGoing -tags=integration -randomizeAllSpecs -randomizeSuites -skipPackage instance-index-injector -timeout=20m "$@"
  popd >/dev/null || exit 1
}

main "$@"
