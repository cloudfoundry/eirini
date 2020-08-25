#!/bin/bash

set -euo pipefail
IFS=$'\n\t'

RUN_DIR="$(cd "$(dirname "$0")" && pwd)"

export GO111MODULE=on
"$RUN_DIR"/run_unit_tests.sh

readonly kubeconfig=$(mktemp)
trap "rm $kubeconfig" EXIT
if ! kind get clusters | grep -q integration-tests; then
  current_cluster="$(kubectl config current-context)"
  kind create cluster --name integration-tests
  kubectl config use-context "$current_cluster"
fi
kind get kubeconfig --name integration-tests >$kubeconfig
INTEGRATION_KUBECONFIG=$kubeconfig "$RUN_DIR"/run_integration_tests.sh

echo "Running Linter"
cd "$RUN_DIR"/.. || exit 1
golangci-lint run
