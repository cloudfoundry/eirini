#!/bin/bash

set -euo pipefail
IFS=$'\n\t'

RUN_DIR="$(cd "$(dirname "$0")" && pwd)"
EIRINI_RELEASE_DIR="$HOME/workspace/eirini-release"

ensure_kind_cluster() {
  local cluster_name
  cluster_name="$1"
  if ! kind get clusters | grep -q "$cluster_name"; then
    current_cluster="$(kubectl config current-context)" || true
    kind create cluster --name "$cluster_name"
    if [[ -n "$current_cluster" ]]; then
      kubectl config use-context "$current_cluster"
    fi
  fi
  kind get kubeconfig --name "$cluster_name" >$kubeconfig
}

run_unit_tests() {
  echo "Running unit tests"

  export GO111MODULE=on
  "$RUN_DIR"/run_unit_tests.sh
}

run_integration_tests() {
  echo "Running integration tests on kind"

  ensure_kind_cluster "integration-tests"
  INTEGRATION_KUBECONFIG=$kubeconfig "$RUN_DIR"/run_integration_tests.sh
}

run_eats_helmless_single_ns() {
  echo "Running EATs against single NS helmless deployed eirini on kind"

  run_eats_helmless "false"
}

run_eats_helmless_multi_ns() {
  echo "Running EATs against multi NS helmless deployed eirini on kind"

  run_eats_helmless "true"
}

run_eats_helmful_single_ns() {
  echo "Running EATs against single NS helm deployed eirini on kind"

  run_eats_helmful "false"
}

run_eats_helmful_multi_ns() {
  echo "Running EATs against multi NS helm deployed eirini on kind"

  run_eats_helmful "true"
}

run_eats_helmless() {
  local multi_ns_support
  multi_ns_support="$1"

  ensure_kind_cluster "eats-helmless"
  if [[ "$redeploy" == "true" ]]; then
    KUBECONFIG="$kubeconfig" "$EIRINI_RELEASE_DIR/deploy/scripts/cleanup.sh" || true
    KUBECONFIG="$kubeconfig" USE_MULTI_NAMESPACE="$multi_ns_support" "$EIRINI_RELEASE_DIR/deploy/scripts/deploy.sh"
  fi

  EIRINI_ADDRESS="https://eirini-api.eirini-core.svc.cluster.local:8085" \
    EIRINI_TLS_SECRET=eirini-certs \
    EIRINI_SYSTEM_NS=eirini-core \
    EIRINI_WORKLOADS_NS=eirini-workloads \
    INTEGRATION_KUBECONFIG="$kubeconfig" \
    USE_MULTI_NAMESPACE="$multi_ns_support" \
    $RUN_DIR/run_eats_tests.sh
}

run_eats_helmful() {
  local multi_ns_support
  multi_ns_support="$1"

  ensure_kind_cluster "eats-helmful"
  if [[ "$redeploy" == "true" ]]; then
    KUBECONFIG="$kubeconfig" "$EIRINI_RELEASE_DIR/helm/scripts/helm-cleanup.sh"
    KUBECONFIG="$kubeconfig" USE_MULTI_NAMESPACE="$multi_ns_support" "$EIRINI_RELEASE_DIR/helm/scripts/helm-deploy-eirini.sh"
  fi

  EIRINI_ADDRESS="https://eirini-opi.cf.svc.cluster.local:8085" \
    EIRINI_TLS_SECRET=eirini-certs \
    EIRINI_SYSTEM_NS=cf \
    EIRINI_WORKLOADS_NS=eirini \
    INTEGRATION_KUBECONFIG="$kubeconfig" \
    USE_MULTI_NAMESPACE="$multi_ns_support" \
    "$RUN_DIR/run_eats_tests.sh"
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

  if [[ "$run_eats_helmless_single_ns" == "true" ]]; then
    run_eats_helmless_single_ns
  fi

  if [[ "$run_eats_helmful_single_ns" == "true" ]]; then
    run_eats_helmful_single_ns
  fi

  if [[ "$run_eats_helmless_multi_ns" == "true" ]]; then
    run_eats_helmless_multi_ns
  fi

  if [[ "$run_eats_helmful_multi_ns" == "true" ]]; then
    run_eats_helmful_multi_ns
  fi

  if [[ "$run_linter" == "true" ]]; then
    run_linter
  fi
}

run_everything() {
  run_unit_tests
  run_integration_tests
  run_eats_helmless_single_ns
  run_eats_helmful_single_ns
  run_eats_helmless_multi_ns
  run_eats_helmful_multi_ns
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
  -e  EATs tests (helmless / single NS)
  -f  EATs test (helmful / single NS)
  -E  EATs tests (helmless / multi NS)
  -F  EATs test (helmful / multi NS)
  -h  this help
  -i  integration tests
  -l  golangci-lint
  -n  do not redeploy eirini when running eats
  -u  unit tests
EOF
  )

  local additional_values \
    run_eats_helmless_single_ns="false" \
    run_eats_helmful_single_ns="false" \
    run_eats_helmless_multi_ns="false" \
    run_eats_helmful_multi_ns="false" \
    run_unit_tests="false" \
    run_integration_tests="false" \
    run_linter="false" \
    skip_docker_build="false" \
    redeploy="true"

  additional_values=""
  while getopts "auieEfFnhl" opt; do
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
        run_eats_helmless_single_ns="true"
        ;;
      f)
        run_eats_helmful_single_ns="true"
        ;;
      E)
        run_eats_helmless_multi_ns="true"
        ;;
      F)
        run_eats_helmful_multi_ns="true"
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
