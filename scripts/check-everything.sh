#!/bin/bash

set -euo pipefail
IFS=$'\n\t'

RUN_DIR="$(cd "$(dirname "$0")" && pwd)"
EIRINI_DIR="$RUN_DIR/.."
EIRINI_RELEASE_BASEDIR="$EIRINI_DIR/../eirini-release"
EIRINI_CONTROLLER_BASEDIR="$EIRINI_DIR/../eirini-controller"

if [ -z ${EIRINIUSER_PASSWORD+x} ]; then
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
  local cluster_name="integration-tests"
  export KUBECONFIG="$HOME/.kube/$cluster_name.yml"
  ensure_kind_cluster "$cluster_name"

  echo "#########################################"
  echo "Running integration tests on $(kubectl config current-context)"
  echo "#########################################"
  echo

  local service_name
  service_name=telepresence-$(uuidgen)

  local src_dir
  src_dir=$(mktemp -d)
  cp -a "$EIRINI_DIR" "$src_dir"
  cp "$KUBECONFIG" "$src_dir"
  trap "rm -rf $src_dir" EXIT

  kubectl apply -f "$EIRINI_CONTROLLER_BASEDIR/deployment/helm/templates/core/lrp-crd.yml"
  kubectl apply -f "$EIRINI_CONTROLLER_BASEDIR/deployment/helm/templates/core/task-crd.yml"

  telepresence \
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
    -e INTEGRATION_KUBECONFIG="/usr/src/app/$(basename $KUBECONFIG)" \
    -e EIRINIUSER_PASSWORD="$EIRINIUSER_PASSWORD" \
    -e TELEPRESENCE_EXPOSE_PORT_START=10000 \
    -e TELEPRESENCE_SERVICE_NAME="$service_name" \
    -e NODES=4 \
    eirini/ci \
    /usr/src/app/scripts/run_integration_tests.sh "$@"
}

run_eats() {
  local cluster_name="eats"
  export KUBECONFIG="$HOME/.kube/$cluster_name.yml"
  ensure_kind_cluster "$cluster_name"

  echo "#########################################"
  echo "Running EATs against deployed eirini on $(kubectl config current-context)"
  echo "#########################################"
  echo

  if [[ "$redeploy" == "true" ]]; then
    regenerate_secrets
    redeploy_wiremock
    redeploy_prometheus
    redeploy_eirini
  fi

  local service_name
  service_name=telepresence-$(uuidgen)

  local src_dir
  src_dir=$(mktemp -d)
  cp -a "$EIRINI_DIR" "$src_dir"
  cp "$KUBECONFIG" "$src_dir"
  trap "rm -rf $src_dir" EXIT

  telepresence \
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
    -e INTEGRATION_KUBECONFIG="/usr/src/app/$(basename $KUBECONFIG)" \
    eirini/ci \
    /usr/src/app/scripts/run_eats_tests.sh "$@"
}

regenerate_secrets() {
  wiremock_keystore_password=${WIREMOCK_KEYSTORE_PASSWORD:-$(pass eirini/ci/wiremock-keystore-password)}
  "$EIRINI_RELEASE_BASEDIR/scripts/generate-secrets.sh" "*.eirini-core.svc" "$wiremock_keystore_password"

}

redeploy_wiremock() {
  kapp -y delete -a wiremock
  kapp -y deploy -a wiremock -f "$EIRINI_RELEASE_BASEDIR/scripts/assets/wiremock.yml"
}

redeploy_prometheus() {
  kapp -y delete -a prometheus
  helm repo add prometheus-community https://prometheus-community.github.io/helm-charts
  helm repo update
  helm -n eirini-core template prometheus prometheus-community/prometheus | kapp -y deploy -a prometheus -f -
}

redeploy_eirini() {
  render_dir=$(mktemp -d)
  trap "rm -rf $render_dir" EXIT
  ca_bundle="$(kubectl get secret -n eirini-core eirini-instance-index-env-injector-certs -o jsonpath="{.data['tls\.ca']}")"
  "$EIRINI_RELEASE_BASEDIR/scripts/render-templates.sh" eirini-core "$render_dir" \
    --values "$EIRINI_RELEASE_BASEDIR/scripts/assets/value-overrides.yml" \
    --set "webhook_ca_bundle=$ca_bundle,resource_validator_ca_bundle=$ca_bundle"
  kbld -f "$render_dir" -f "$RUN_DIR/kbld-local-eirini.yml" >"$render_dir/rendered.yml"
  for img in $(grep -oh "kbld:.*" "$render_dir/rendered.yml"); do
    kind load docker-image --name eats "$img"
  done
  kapp -y delete -a eirini
  kapp -y deploy -a eirini -f "$render_dir/rendered.yml"
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

  if [[ "$run_eats" == "true" ]]; then
    run_eats "$@"
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
  tmux split-window -v -p 50 "/bin/bash -c \"$0 -e $do_not_deploy; bash --init-file <(echo 'history -s $0 -e $do_not_deploy')\""
  tmux select-pane -L
  tmux split-window -v -p 50 "/bin/bash -c \"$0 -l; bash --init-file <(echo 'history -s $0 -l')\""
}

main() {
  USAGE=$(
    cat <<EOF
Usage: check-everything.sh [options]
Options:
  -a  run all tests (default)
  -e  EATs tests
  -h  this help
  -i  integration tests
  -l  golangci-lint
  -n  do not redeploy eirini when running eats
  -u  unit tests
EOF
  )

  local run_eats="false" \
    run_unit_tests="false" \
    run_integration_tests="false" \
    run_linter="false" \
    redeploy="true" \
    run_subset="false"

  while getopts "auiefrnhl" opt; do
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
        run_eats="true"
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
