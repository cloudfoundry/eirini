#!/bin/bash

set -euxo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"

CLUSTER_NAME=${CLUSTER_NAME:-cf-for-k8s-kind}
CF_DOMAIN=${CF_DOMAIN:-vcap.me}
EIRINI_RELEASE_BASEDIR=${EIRINI_RELEASE_BASEDIR:-$HOME/workspace/eirini-release}

values_file="$SCRIPT_DIR/values/$CLUSTER_NAME.cf-values.yml"

if [[ ! -f "$values_file" ]]; then
  # ask early for pass passphrase if required
  docker_username=eiriniuser
  docker_password=$(pass eirini/docker-hub)
  docker_repo_prefix=eirini
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

  # patch current eirini helm into cf-for-k8s
  rm -rf "./build/eirini/_vendir/eirini"
  cp -r "$EIRINI_RELEASE_BASEDIR/helm/eirini" "./build/eirini/_vendir/"
  ./build/eirini/build.sh

  kapp deploy -y -a cf -f <(ytt -f config -f "$values_file")
}
popd

cf api https://api.${CF_DOMAIN} --skip-ssl-validation
cf auth admin $(yq r $values_file cf_admin_password)
