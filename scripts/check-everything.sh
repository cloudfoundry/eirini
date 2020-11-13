#!/bin/bash

set -euo pipefail
IFS=$'\n\t'

RUN_DIR="$(cd "$(dirname "$0")" && pwd)"
EIRINI_DIR="$RUN_DIR/.."

EIRINIUSER_PASSWORD=""
if command -v pass &>/dev/null; then
  EIRINIUSER_PASSWORD="$(pass eirini/docker-hub)"
fi

export TELEPRESENCE_EXPOSE_PORT_START=10000
export TELEPRESENCE_SERVICE_NAME

clusterLock=$HOME/.kind-cluster.lock

ensure_kind_cluster() {
  local cluster_name
  cluster_name="$1"
  if ! kind get clusters | grep -q "$cluster_name"; then
    current_cluster="$(kubectl config current-context)" || true
    kindConfig=$(mktemp)
    cat <<EOF >>"$kindConfig"
kind: Cluster
apiVersion: kind.x-k8s.io/v1alpha4
networking:
  apiServerAddress: 172.17.0.1
EOF
    flock -x "$clusterLock" kind create cluster --name "$cluster_name" --config "$kindConfig" --wait 5m
    rm -f "$kindConfig"
    if [[ -n "$current_cluster" ]]; then
      kubectl config use-context "$current_cluster"
    fi
  fi
  kind export kubeconfig --name "$cluster_name" --kubeconfig "$HOME/.kube/$cluster_name.yml"
}

run_unit_tests() {
  echo "Running unit tests"

  export GO111MODULE=on
  "$RUN_DIR"/run_unit_tests.sh "$@"
}

run_integration_tests() {
  echo "Running integration tests on kind"

  local cluster_name="integration-tests"
  ensure_kind_cluster "$cluster_name"

  local service_name
  service_name=telepresence-$(uuidgen)

  local src_dir
  src_dir=$(mktemp -d)
  cp -a "$EIRINI_DIR" "$src_dir"
  cp "$HOME/.kube/$cluster_name.yml" "$src_dir"
  trap 'rm -rf $src_dir' EXIT

  KUBECONFIG=$HOME/.kube/integration-tests.yml telepresence \
    --method container \
    --new-deployment "$service_name" \
    --expose 10000 \
    --expose 10001 \
    --expose 10002 \
    --expose 10003 \
    --docker-run \
    --rm \
    -v "$src_dir":/usr/src/app \
    -v "$HOME"/.cache:/root/.cache \
    -e INTEGRATION_KUBECONFIG=/usr/src/app/integration-tests.yml \
    -e EIRINIUSER_PASSWORD="$EIRINIUSER_PASSWORD" \
    -e TELEPRESENCE_EXPOSE_PORT_START=10000 \
    -e TELEPRESENCE_SERVICE_NAME="$service_name" \
    -e NODES=4 \
    eirini/ci \
    /usr/src/app/scripts/run_integration_tests.sh "$@"
}

run_eats_helmless() {
  echo "Running EATs against helmless deployed eirini on kind"

  local cluster_name="eats-helmless"
  local kubeconfig="$HOME/.kube/$cluster_name.yml"

  ensure_kind_cluster "$cluster_name"
  if [[ "$redeploy" == "true" ]]; then
    skaffold delete -p helmless
    KUBECONFIG="$kubeconfig" "$RUN_DIR/skaffold" run -p helmless
  fi

  local service_name
  service_name=telepresence-$(uuidgen)

  src_dir=$(mktemp -d)
  cp -a "$EIRINI_DIR" "$src_dir"
  cp "$HOME/.kube/$cluster_name.yml" "$src_dir"
  trap 'rm -rf $src_dir' EXIT

  KUBECONFIG="$kubeconfig" telepresence \
    --method container \
    --new-deployment "$service_name" \
    --docker-run \
    --rm \
    -v "$src_dir":/usr/src/app \
    -v "$HOME"/.cache:/root/.cache \
    -e EIRINI_ADDRESS="https://eirini-api.eirini-core.svc.cluster.local:8085" \
    -e EIRINI_TLS_SECRET=eirini-certs \
    -e EIRINI_SYSTEM_NS=eirini-core \
    -e EIRINI_WORKLOADS_NS=eirini-workloads \
    -e EIRINIUSER_PASSWORD="$EIRINIUSER_PASSWORD" \
    -e INTEGRATION_KUBECONFIG="/usr/src/app/$cluster_name.yml" \
    eirini/ci \
    /usr/src/app/scripts/run_eats_tests.sh "$@"
}

run_eats_helmful() {
  echo "Running EATs against helm deployed eirini on kind"

  local cluster_name="eats-helmful"
  local kubeconfig="$HOME/.kube/$cluster_name.yml"

  ensure_kind_cluster "$cluster_name"
  if [[ "$redeploy" == "true" ]]; then
    skaffold delete -p helm || true # helm will fail if nothing is deployed
    KUBECONFIG="$kubeconfig" "$RUN_DIR/skaffold" run -p helm
  fi

  local service_name
  service_name=telepresence-$(uuidgen)

  src_dir=$(mktemp -d)
  cp -a "$EIRINI_DIR" "$src_dir"
  cp "$HOME/.kube/$cluster_name.yml" "$src_dir"
  trap 'rm -rf $src_dir' EXIT

  KUBECONFIG="$kubeconfig" telepresence \
    --method container \
    --new-deployment "$service_name" \
    --docker-run \
    --rm \
    -v "$src_dir":/usr/src/app \
    -v "$HOME"/.cache:/root/.cache \
    -e EIRINI_ADDRESS="https://eirini-opi.eirini-core.svc.cluster.local:8085" \
    -e EIRINI_TLS_SECRET=eirini-certs \
    -e EIRINI_SYSTEM_NS=eirini-core \
    -e EIRINI_WORKLOADS_NS=eirini \
    -e EIRINIUSER_PASSWORD="$EIRINIUSER_PASSWORD" \
    -e INTEGRATION_KUBECONFIG="/usr/src/app/$cluster_name.yml" \
    eirini/ci \
    /usr/src/app/scripts/run_eats_tests.sh "$@"
}

run_linter() {
  echo "Running Linter"
  cd "$RUN_DIR"/.. || exit 1
  golangci-lint run
}

run_subset() {
  if [[ "$run_unit_tests" == "true" ]]; then
    run_unit_tests "$@"
  fi

  if [[ "$run_integration_tests" == "true" ]]; then
    run_integration_tests "$@"
  fi

  if [[ "$run_eats_helmless" == "true" ]]; then
    run_eats_helmless "$@"
  fi

  if [[ "$run_eats_helmful" == "true" ]]; then
    run_eats_helmful "$@"
  fi

  if [[ "$run_linter" == "true" ]]; then
    run_linter
  fi
}

RED=1
GREEN=2
print_message() {
  message=$1
  colour=$2
  printf "\\r\\033[00;3%sm[%s]\\033[0m\\n" "$colour" "$message"
}

run_everything() {
  print_message "about to run tests in parallel, it will be awesome" $GREEN
  print_message "ctrl-d panes when they are done" $RED
  local do_not_deploy="-n "
  if [[ "$redeploy" == "true" ]]; then
    do_not_deploy=""
  fi
  tmux new-window -n eirini-tests "/bin/bash -c \"$0 -u; bash --init-file <(echo 'history -s $0 -u')\""
  tmux split-window -h -p 50 "/bin/bash -c \"$0 -i $do_not_deploy; bash --init-file <(echo 'history -s $0 -i $do_not_deploy')\""
  tmux split-window -v -p 50 "/bin/bash -c \"$0 -ef $do_not_deploy; bash --init-file <(echo 'history -s $0 -ef $do_not_deploy')\""
  tmux select-pane -L
  tmux split-window -v -p 50 "/bin/bash -c \"$0 -l; bash --init-file <(echo 'history -s $0 -l')\""
}

main() {
  USAGE=$(
    cat <<EOF
Usage: check-everything.sh [options]
Options:
  -a  run all tests (default)
  -e  EATs tests helmless
  -f  EATs test helmful
  -h  this help
  -i  integration tests
  -l  golangci-lint
  -n  do not redeploy eirini when running eats
  -u  unit tests
EOF
  )

  local run_eats_helmless="false" \
    run_eats_helmful="false" \
    run_unit_tests="false" \
    run_integration_tests="false" \
    run_linter="false" \
    redeploy="true" \
    run_subset="false"

  while getopts "auiefnhl" opt; do
    case ${opt} in
      n)
        redeploy="false"
        ;;
      a)
        run_subset="false"
        ;;
      u)
        run_unit_tests="true"
        run_subset="true"
        ;;
      i)
        run_integration_tests="true"
        run_subset="true"
        ;;
      e)
        run_eats_helmless="true"
        run_subset="true"
        ;;
      f)
        run_eats_helmful="true"
        run_subset="true"
        ;;
      l)
        run_linter="true"
        run_subset="true"
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

  if [[ "$run_subset" == "true" ]]; then
    run_subset "$@"
  else
    run_everything
  fi
}

main "$@"
