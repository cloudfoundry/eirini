#!/bin/bash

set -euo pipefail
IFS=$'\n\t'

readonly BASEDIR="$(cd "$(dirname "$0")"/.. && pwd)"
export GO111MODULE=on

main() {
  run_tests
}

run_tests() {
  pushd "$BASEDIR" >/dev/null || exit 1
  ginkgo -mod=vendor -p -r -keepGoing --skipPackage=launcher,packs,integration -randomizeAllSpecs -randomizeSuites
  popd >/dev/null || exit 1
}

main "$@"
