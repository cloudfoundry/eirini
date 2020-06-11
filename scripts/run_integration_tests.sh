#!/bin/bash

set -euo pipefail

readonly BASEDIR="$(cd "$(dirname "$0")"/.. && pwd)"
export INTEGRATION_KUBECONFIG=${INTEGRATION_KUBECONFIG:-"$HOME/.kube/config"}
export GO111MODULE=on

main() {
  pushd "$BASEDIR"/integration >/dev/null || exit 1
  ginkgo -mod=vendor -p -r -keepGoing -tags=integration -randomizeAllSpecs -randomizeSuites -timeout=20m
  popd >/dev/null || exit 1
}

main "$@"
