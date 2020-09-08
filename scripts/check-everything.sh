#!/bin/bash

set -euo pipefail
IFS=$'\n\t'

RUN_DIR="$(cd "$(dirname "$0")" && pwd)"
EIRINI_RELEASE_DIR="$HOME/workspace/eirini-release"

echo "Running unit tests"
export GO111MODULE=on
"$RUN_DIR"/run_unit_tests.sh

echo "Running integration tests on kind"
readonly kubeconfig=$(mktemp)
trap "rm $kubeconfig" EXIT
if ! kind get clusters | grep -q integration-tests; then
  current_cluster="$(kubectl config current-context)"
  kind create cluster --name integration-tests
  kubectl config use-context "$current_cluster"
fi
kind get kubeconfig --name integration-tests >$kubeconfig
INTEGRATION_KUBECONFIG=$kubeconfig "$RUN_DIR"/run_integration_tests.sh

echo "Running EATs against helmless deployed eirini on kind"

"$EIRINI_RELEASE_DIR/deploy/scripts/deploy.sh"
trap "$EIRINI_RELEASE_DIR/deploy/scripts/cleanup.sh" EXIT

EIRINI_IP="$(kubectl get nodes -o jsonpath='{.items[0].status.addresses[0].address}')"
echo -n "Waiting for eirini to start: "
while ! nc -z "$EIRINI_IP" 443; do
  echo -n "."
  sleep 0.5
done
echo " Up!"

EIRINI_ADDRESS="https://$EIRINI_IP" \
  EIRINI_TLS_SECRET=eirini-certs \
  EIRINI_SYSTEM_NS=eirini-core \
  HELMLESS=true \
  KUBECONFIG="$kubeconfig" \
  $RUN_DIR/run_eats_tests.sh

echo "Running Linter"
cd "$RUN_DIR"/.. || exit 1
golangci-lint run
