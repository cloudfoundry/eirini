#!/bin/bash

set -euo pipefail
IFS=$'\n\t'

RUN_DIR="$(cd "$(dirname "$0")" && pwd)"
EIRINI_RELEASE_DIR="$HOME/workspace/eirini-release"

ensure_kind_cluster() {
  if ! kind get clusters | grep -q integration-tests; then
    current_cluster="$(kubectl config current-context)" || true
    kind create cluster --name integration-tests
    if [[ -n "$current_cluster" ]]; then
      kubectl config use-context "$current_cluster"
    fi
  fi
  kind get kubeconfig --name integration-tests >$kubeconfig
}

run_unit_tests() {
  echo "Running unit tests"

  export GO111MODULE=on
  "$RUN_DIR"/run_unit_tests.sh
}

run_integration_tests() {
  echo "Running integration tests on kind"

  ensure_kind_cluster
  INTEGRATION_KUBECONFIG=$kubeconfig "$RUN_DIR"/run_integration_tests.sh
}

run_eats() {
  echo "Running EATs against helmless deployed eirini on kind"

  ensure_kind_cluster
  KUBECONFIG="$kubeconfig" "$EIRINI_RELEASE_DIR/deploy/scripts/cleanup.sh" || true
  KUBECONFIG="$kubeconfig" "$EIRINI_RELEASE_DIR/deploy/scripts/deploy.sh"

  EIRINI_IP="$(KUBECONFIG="$kubeconfig" kubectl get nodes -o jsonpath='{.items[0].status.addresses[0].address}')"

  EIRINI_ADDRESS="https://$EIRINI_IP" \
    EIRINI_TLS_SECRET=eirini-certs \
    EIRINI_SYSTEM_NS=eirini-core \
    HELMLESS=true \
    INTEGRATION_KUBECONFIG="$kubeconfig" \
    $RUN_DIR/run_eats_tests.sh
}

run_linter() {
  echo "Running Linter"
  cd "$RUN_DIR"/.. || exit 1
  golangci-lint run
}

run_everything() {
  run_unit_tests
  run_integration_tests
  run_eats
  run_linter
}

main() {
  readonly kubeconfig=$(mktemp)
  trap "rm $kubeconfig" EXIT

  USAGE=$(
    cat <<EOF
Usage: check-everything.sh [options]
Options:
  -a  run all tests (default)
  -u  unit tests
  -i  integration tests
  -e  EATs tests
  -l  golangci-lint
  -h  this help
EOF
  )

  local cluster_name additional_values skip_docker_build="false"
  additional_values=""
  while getopts "auieh" opt; do
    case ${opt} in
      a)
        run_everything
        exit 0
        ;;
      u)
        run_unit_tests
        exit 0
        ;;
      i)
        run_integration_tests
        exit 0
        ;;
      e)
        run_eats
        exit 0
        ;;
      l)
        run_linter
        exit 0
        ;;
      h)
        echo hello
        echo "$USAGE"
        exit 0
        ;;
      \?)
        echo "Invalid option: $OPTARG" 1>&2
        echo "$USAGE"
        exit 1
        ;;
      :)
        echo "Invalid option: $OPTARG requires an argument" 1>&2
        echo "$USAGE"
        exit 1
        ;;
    esac
  done
  shift $((OPTIND - 1))
  run_everything
}

main $@
