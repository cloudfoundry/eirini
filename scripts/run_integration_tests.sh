#!/bin/bash

set -euo pipefail

readonly BASEDIR="$(cd "$(dirname "$0")"/.. && pwd)"
INTEGRATION_KUBECONFIG=${INTEGRATION_KUBECONFIG:?must be a path to a valid kubeconfig}

main(){
  pushd "$BASEDIR"/integration > /dev/null || exit 1
    ginkgo -tags=integration
  popd > /dev/null || exit 1
}

main "$@"
