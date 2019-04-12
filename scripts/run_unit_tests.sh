#!/bin/bash

set -euo pipefail
IFS=$'\n\t'

readonly BASEDIR="$(cd "$(dirname "$0")"/.. && pwd)"

main(){
  run_tests
}

run_tests() {
  pushd "$BASEDIR" > /dev/null || exit 1
    ginkgo -r -keepGoing --skipPackage=launcher,packs,integration -randomizeAllSpecs
  popd > /dev/null || exit 1
}

main "$@"
