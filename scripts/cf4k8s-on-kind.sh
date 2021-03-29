#!/bin/bash

set -euxo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"

CLUSTER_NAME=${CLUSTER_NAME:-cf-for-k8s}
CF_DOMAIN=${CF_DOMAIN:-vcap.me}
EIRINI_RELEASE_BASEDIR=${EIRINI_RELEASE_BASEDIR:-$HOME/workspace/eirini-release}
EIRINI_CI_BASEDIR=${EIRINI_CI_BASEDIR:-$HOME/workspace/eirini-ci}
EIRINI_RENDER_DIR=$(mktemp -d)

source "$SCRIPT_DIR/assets/cf4k8s/cc-commons.sh"

trap "rm -rf $EIRINI_RENDER_DIR" EXIT

use_local_cc="false"
use_local_eirini="false"
delete_kind_cluster="false"
while getopts "cde" opt; do
  case ${opt} in
    c)
      use_local_cc="true"
      ;;
    d)
      delete_kind_cluster="true"
      ;;
    e)
      use_local_eirini="true"
      ;;
  esac
done
shift $((OPTIND - 1))

values_file="$SCRIPT_DIR/values/$CLUSTER_NAME.cf-values.yml"

if [[ "$delete_kind_cluster" == "true" ]]; then
  kind delete cluster --name cf-for-k8s
  rm -rf "$SCRIPT_DIR/values/cf-for-k8s.cf-values.yml"
fi

if [[ ! -f "$values_file" ]]; then
  # ask early for pass passphrase if required
  docker_username=eiriniuser
  docker_password=$(pass eirini/docker-hub)
  docker_repo_prefix=eirini
else
  echo " ⚠️  WARNING: Using existing values file: $values_file! If you want a clean deployment consider deleting this file!"
fi

pushd $HOME/workspace/cf-for-k8s
{
  if ! kind get clusters | grep -q "$CLUSTER_NAME"; then
    # don't install the default CNI as it doesn't support NetworkPolicies
    # use the cluster config in assets until cf-for-k8s release > 2.1.1
    # which will contain the containerd configuration
    kind create cluster \
      --config <(yq e '.networking.disableDefaultCNI = true' $SCRIPT_DIR/assets/kind/cluster.yml) \
      --image kindest/node:v1.19.1 \
      --name "$CLUSTER_NAME"
  else
    kind export kubeconfig --name $CLUSTER_NAME
  fi

  if [[ ! -f "$values_file" ]]; then
    mkdir -p "$(dirname $values_file)"

    ./hack/generate-values.sh -d "$CF_DOMAIN" >"$values_file"

    cat <<EOF >>"$values_file"
app_registry:
  hostname: https://index.docker.io/v1/
  repository_prefix: "$docker_repo_prefix"
  username: "$docker_username"
  password: "$docker_password"

add_metrics_server_components: true
enable_automount_service_account_token: true
load_balancer:
  enable: false
metrics_server_prefer_internal_kubelet_address: true
remove_resource_requirements: true
use_first_party_jwt_tokens: true
EOF

  fi

  # generate eirini yamls
  "$EIRINI_RELEASE_BASEDIR/scripts/render-templates.sh" cf-system "$EIRINI_RENDER_DIR"

  if [[ "$use_local_eirini" == "true" ]]; then
    # patch generated eirini yamls into cf-for-k8s
    rm -rf "./build/eirini/_vendir/eirini"
    mv "$EIRINI_RENDER_DIR/templates" "./build/eirini/_vendir/eirini"
  fi

  # generate config/eirini/_ytt_lib/eirini/rendered.yml
  ./build/eirini/build.sh

  if [[ "$use_local_cc" == "true" ]]; then
    # build & bump cc
    build_ccng_image
    push-to-docker
    sed -i "s|ccng: .*$|ccng: $CCNG_IMAGE:$TAG|" "$HOME/workspace/capi-k8s-release/values/images.yml"
    "$HOME/workspace/capi-k8s-release/scripts/bump-cf-for-k8s.sh"
  fi

}
popd

# install Calico to get NetworkPolicy support
kapp deploy -y -a calico -f https://docs.projectcalico.org/manifests/calico.yaml

# deploy everything
kapp deploy -y -a cf -f <(ytt -f "$HOME/workspace/cf-for-k8s/config" -f "$values_file" $@)

cf api https://api.${CF_DOMAIN} --skip-ssl-validation
cf auth admin $(yq eval '.cf_admin_password' $values_file)
