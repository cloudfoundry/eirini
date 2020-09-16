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
  if [[ "$redeploy" == "true" ]]; then
    KUBECONFIG="$kubeconfig" "$EIRINI_RELEASE_DIR/deploy/scripts/cleanup.sh" || true
    KUBECONFIG="$kubeconfig" "$EIRINI_RELEASE_DIR/deploy/scripts/deploy.sh"
  fi

  EIRINI_IP="$(KUBECONFIG="$kubeconfig" kubectl get nodes -o jsonpath='{.items[0].status.addresses[0].address}')"

  EIRINI_ADDRESS="https://$EIRINI_IP" \
    EIRINI_TLS_SECRET=eirini-certs \
    EIRINI_SYSTEM_NS=eirini-core \
    INTEGRATION_KUBECONFIG="$kubeconfig" \
    $RUN_DIR/run_eats_tests.sh
}

run_linter() {
  echo "Running Linter"
  cd "$RUN_DIR"/.. || exit 1
  golangci-lint run
}

run_subset() {
  if [[ "$run_unit_tests" == "true" ]]; then
    run_unit_tests
  fi

  if [[ "$run_integration_tests" == "true" ]]; then
    run_integration_tests
  fi

  if [[ "$run_eats" == "true" ]]; then
    run_eats
  fi

  if [[ "$run_linter" == "true" ]]; then
    run_linter
  fi
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
  -e  EATs tests
  -h  this help
  -i  integration tests
  -l  golangci-lint
  -n  do not redeploy helmless when running eats
  -u  unit tests
EOF
  )

  local cluster_name \
    additional_values \
    run_eats="false" \
    run_unit_tests="false" \
    run_integration_tests="false" \
    run_linter="false" \
    skip_docker_build="false" \
    redeploy="true"

  additional_values=""
  while getopts "auienh" opt; do
    case ${opt} in
      n)
        redeploy="false"
        ;;
      a)
        run_everything
        exit 0
        ;;
      u)
        run_unit_tests="true"
        ;;
      i)
        run_integration_tests="true"
        ;;
      e)
        run_eats="true"
        ;;
      l)
        run_linter="true"
        ;;
      h)
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
  if [[ $((OPTIND - 1)) -eq 0 ]]; then
    run_everything
  fi
  run_subset
}

main $@
