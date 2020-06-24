#!/bin/bash

set -euo pipefail

IFS=$'\n\t'

readonly USAGE="Usage: patch-me-if-you-can.sh -c <cluster-name> [ -s ] [ <component-name> ... ]"
readonly SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" >/dev/null 2>&1 && pwd)"
readonly EIRINI_BASEDIR=$(realpath "$SCRIPT_DIR/..")
readonly EIRINI_RELEASE_BASEDIR=$(realpath "$SCRIPT_DIR/../../eirini-release")
readonly EIRINI_PRIVATE_CONFIG_BASEDIR=$(realpath "$SCRIPT_DIR/../../eirini-private-config")
readonly EIRINI_CI_BASEDIR="$HOME/workspace/eirini-ci"
readonly CF4K8S_DIR="$HOME/workspace/cf-for-k8s"
readonly CAPIK8SDIR="$HOME/workspace/capi-k8s-release"

main() {
  if [ "$#" == "0" ]; then
    echo "$USAGE"
    exit 1
  fi

  local cluster_name skip_docker_build="false"
  while getopts ":c:s" opt; do
    case ${opt} in
      c)
        cluster_name=$OPTARG
        ;;
      s)
        skip_docker_build="true"
        ;;
      \?)
        echo "Invalid option: $OPTARG" 1>&2
        echo "$USAGE"
        ;;
      :)
        echo "Invalid option: $OPTARG requires an argument" 1>&2
        echo "$USAGE"
        ;;
    esac
  done
  shift $((OPTIND - 1))

  if [ -z "$cluster_name" ]; then
    echo "Cluster name not provided"
    echo "$USAGE"
    exit 1
  fi

  if [[ "$(current_cluster_name)" =~ "gke_.*${cluster_name}\$" ]]; then
    echo "Your current cluster is $(current_cluster_name), but you want to update $cluster_name. Please target $cluster_name"
    echo "gcloudcluster $cluster_name"
    exit 1
  fi

  local extra_args
  if [ "$skip_docker_build" != "true" ]; then
    if [ "$#" == "0" ]; then
      echo "No components specified. Nothing to do."
      echo "If you want to helm upgrade without building containers, please pass the '-s' flag"
      exit 0
    fi
    local component
    for component in "$@"; do
      if is_cloud_controller $component; then
        custom_ccng_values_file=$(mktemp)
        build_ccng_image $custom_ccng_values_file
        extra_args=("-f" "$custom_ccng_values_file")
      else
        update_component "$component"
      fi
    done
  fi

  pull_private_config
  patch_cf_for_k8s
  deploy_cf "$cluster_name" "${extra_args[@]}"
}

is_cloud_controller() {
  local component
  component="$1"
  [[ "$component" =~ cloud.controller ]] || [[ "$component" =~ "ccng" ]] || [[ "$component" =~ "capi" ]] || [[ "$component" =~ "cc" ]]
}

build_ccng_image() {
  export IMAGE_DESTINATION_KPACK_WATCHER="docker.io/eirini/dev-kpack-watcher"
  export IMAGE_DESTINATION_CCNG="docker.io/eirini/dev-ccng"
  git -C "$CAPIK8SDIR" checkout values/images.yml
  "$CAPIK8SDIR"/scripts/build-into-values.sh "$CAPIK8SDIR/values/images.yml"
  "$CAPIK8SDIR"/scripts/bump-cf-for-k8s.sh
}

update_component() {
  local component
  component=$1

  echo "--- Patching component $component ---"
  docker_build "$component"
  docker_push "$component"
  update_image_in_helm_chart "$component"
}

docker_build() {
  echo "Building docker image for $1"
  pushd "$EIRINI_BASEDIR"
  docker build . -f "$EIRINI_BASEDIR/docker/$component/Dockerfile" \
    --build-arg GIT_SHA=big-sha \
    --tag "eirini/$component:patch-me-if-you-can"
  popd
}

docker_push() {
  echo "Pushing docker image for $1"
  pushd "$EIRINI_BASEDIR"
  docker push "eirini/$component:patch-me-if-you-can"
  popd
}

update_image_in_helm_chart() {
  echo "Applying docker image of $1 to kubernetes cluster"
  pushd "$EIRINI_RELEASE_BASEDIR/helm/eirini/templates"
  local file new_image_ref
  file=$(rg -l "image: eirini/${1}")
  new_image_ref="$(docker inspect --format='{{index .RepoDigests 0}}' "eirini/${1}:patch-me-if-you-can")"
  sed -i -e "s|image: eirini/${1}.*$|image: ${new_image_ref}|g" "$file"
  popd
}

patch_cf_for_k8s() {
  local build_path eirini_values eirini_custom_values
  rm -rf "$CF4K8S_DIR/build/eirini/_vendir/eirini"

  build_path="$CF4K8S_DIR/build/eirini/"
  eirini_values="$build_path/eirini-values.yml"
  eirini_custom_values="$build_path/eirini-custom-values.yml"

  cat >"$build_path/add-namespaces-overlay.yml" <<EOF
#@ load("@ytt:overlay", "overlay")

#@overlay/match by=overlay.subset({"kind":"Namespace", "metadata":{"name":"cf-workloads"}})
#@overlay/remove
---

EOF

  cat >>"$eirini_custom_values" <<EOF
---
opi:
  serviceName: eirini
  lrpController:
    tls:
      secretName: "eirini-internal-tls-certs"
      keyPath: "tls.key"
      caPath: "tls.ca"
      certPath: "tls.crt"
  tasks:
    tls:
      taskReporter:
          secretName: "eirini-internal-tls-certs"
          keyPath: "tls.key"
          caPath: "tls.ca"
          certPath: "tls.crt"

EOF

  yq merge --inplace "$eirini_values" "$eirini_custom_values"
  rm "$eirini_custom_values"

  cp -r "$EIRINI_RELEASE_BASEDIR/helm/eirini" "$CF4K8S_DIR/build/eirini/_vendir/"

  "$CF4K8S_DIR"/build/eirini/build.sh
}

deploy_cf() {
  local cluster_name
  cluster_name="$1"
  shift 1
  kapp deploy -a cf -f <(
    ytt -f "$CF4K8S_DIR/config" \
      -f "$EIRINI_CI_BASEDIR/cf-for-k8s" \
      -f "$EIRINI_PRIVATE_CONFIG_BASEDIR/environments/kube-clusters/"${cluster_name}"/default-values.yml" \
      -f "$EIRINI_PRIVATE_CONFIG_BASEDIR/environments/kube-clusters/"${cluster_name}"/loadbalancer-values.yml" \
      $@
  ) -y
}

pull_private_config() {
  pushd "$EIRINI_PRIVATE_CONFIG_BASEDIR"
  git pull --rebase
  popd
}

current_cluster_name() {
  kubectl config current-context | cut -d / -f 1
}

main "$@"
