#!/bin/bash

set -euo pipefail

readonly BASEDIR="$(cd "$(dirname "$0")"/.. && pwd)"
export GO111MODULE=on
export EIRINIUSER_PASSWORD="${EIRINIUSER_PASSWORD:-$(pass eirini/docker-hub)}"

main() {
  pushd "$BASEDIR"/integration >/dev/null || exit 1
  ginkgo -mod=vendor -p -r -keepGoing -tags=integration -randomizeAllSpecs -randomizeSuites -timeout=20m -skipPackage='eats' "$@"
  popd >/dev/null || exit 1
}

main "$@"
