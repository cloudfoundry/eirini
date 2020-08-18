#!/bin/bash

set -euo pipefail

# Time the tests
start=$(($(date +%s%N) / 1000000))

readonly BASEDIR="$(cd "$(dirname "$0")"/.. && pwd)"
readonly POD_NAME="eirini-integration-tests-$(openssl rand -hex 5)"
readonly SKIP_CLEANUP="${SKIP_CLEANUP:-false}"
export EIRINIUSER_PASSWORD="${EIRINIUSER_PASSWORD:-$(pass eirini/docker-hub)}"

echo "Running tests in pod ${POD_NAME}"

# Formats the 2 arguments (start time and end time in milliseconds) to a human readable format
format_time() {
  start_time=$1
  end_time=$2
  total_milliseconds=$((end_time - start_time))

  milliseconds=$((total_milliseconds % 1000))
  seconds=$((total_milliseconds / 1000 % 60))
  minutes=$((total_milliseconds / 60000 % 60))
  hours=$((total_milliseconds / 3600000 % 24))

  echo "Total time: $hours hours, $minutes minutes, $seconds seconds, $milliseconds milliseconds"
}

is_init_container_running() {
  local pod_name container_name
  pod_name="$1"
  container_name="$2"
  if [[ "$(kubectl get pods "${pod_name}" \
    --output jsonpath="{.status.initContainerStatuses[?(@.name == \"${container_name}\")].state.running}")" != "" ]]; then
    return 0
  fi
  return 1
}

# Deletes the testing pod unless SKIP_CLEANUP is set to a non "false" value
cleanup() {
  rv=$?
  if [ ! "$SKIP_CLEANUP" == "false" ]; then
    echo "Skipping clean up because SKIP_CLEANUP is set. Cleanup pod ${POD_NAME} manually if you must."
    exit $rv
  fi
  echo "Cleaning up pod ${POD_NAME}"
  # Higher grace period so the tests have time to cleanup kubernetes resources
  kubectl delete pod $POD_NAME --wait --ignore-not-found --grace-period=120
  exit $rv
}
trap "cleanup" EXIT

kubectl apply -f "$BASEDIR"/scripts/assets/test-pod-rbac.yml
goml set -d -f <(goml set -d -f ./scripts/assets/test-pod.yml -p spec.containers.0.env.name:EIRINIUSER_PASSWORD.value -v "$EIRINIUSER_PASSWORD") -p metadata.name -v $POD_NAME | kubectl apply -f -

timeout=30
while ! kubectl get pods $POD_NAME; do
  sleep 1
  timeout=$((timeout - 1))
done
if [[ "${timeout}" == 0 ]]; then
  exit 1
fi

timeout=30
until is_init_container_running $POD_NAME "wait-for-code" || [[ "$timeout" == "0" ]]; do
  sleep 1
  timeout=$((timeout - 1))
done
if [[ "${timeout}" == 0 ]]; then
  exit 1
fi

kubectl cp "$BASEDIR" $POD_NAME:/eirini-code -c wait-for-code
kubectl cp "$(mktemp)" $POD_NAME:/eirini-code/tests-can-start -c wait-for-code

kubectl wait pod $POD_NAME --for=condition=Ready
# Tail the test logs.
container_name="tests"
kubectl logs $POD_NAME \
  --follow \
  --container "${container_name}"

format_time $start $(($(date +%s%N) / 1000000))

# Wait for the container to terminate and then exit the script with the container's exit code.
jsonpath="{.status.containerStatuses[?(@.name == \"${container_name}\")].state.terminated.exitCode}"
while true; do
  exit_code="$(kubectl get pod $POD_NAME --output "jsonpath=${jsonpath}")"
  if [[ -n "${exit_code}" ]]; then
    exit "${exit_code}"
  fi
  sleep 1
done
