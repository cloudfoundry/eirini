#!/bin/bash

set -euxo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"

CLUSTER_NAME=${CLUSTER_NAME:-cf-for-k8s-kind}
CF_DOMAIN=${CF_DOMAIN:-vcap.me}
EIRINI_RELEASE_BASEDIR=${EIRINI_RELEASE_BASEDIR:-$HOME/workspace/eirini-release}
EIRINI_RENDER_DIR=$(mktemp -d)

trap "rm -rf $EIRINI_RENDER_DIR" EXIT

values_file="$SCRIPT_DIR/values/$CLUSTER_NAME.cf-values.yml"

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
    kind create cluster --config=./deploy/kind/cluster.yml --image kindest/node:v1.19.1 --name "$CLUSTER_NAME"
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
  # "$EIRINI_RELEASE_BASEDIR/scripts/render-templates.sh" cf-system "$EIRINI_RENDER_DIR"

  # patch generated eirini yamls into cf-for-k8s
  # rm -rf "./build/eirini/_vendir/eirini"
  # mv "$EIRINI_RENDER_DIR/templates" "./build/eirini/_vendir/eirini"

  # generate config/eirini/_ytt_lib/eirini/rendered.yml
  ./build/eirini/build.sh

  # deploy everything
  kapp deploy -y -a cf -f <(ytt -f config -f "$values_file")
}
popd

cf api https://api.${CF_DOMAIN} --skip-ssl-validation
cf auth admin $(yq r $values_file cf_admin_password)
