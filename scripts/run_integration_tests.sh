#!/bin/bash

set -euo pipefail

readonly BASEDIR="$(cd "$(dirname "$0")"/.. && pwd)"
INTEGRATION_KUBECONFIG=${INTEGRATION_KUBECONFIG:?must be a path to a valid kubeconfig}
export GO111MODULE=on

main(){
  pushd "$BASEDIR"/integration > /dev/null || exit 1
    ginkgo -mod=vendor -race -p -r -keepGoing -tags=integration -randomizeAllSpecs
  popd > /dev/null || exit 1
}

main "$@"
