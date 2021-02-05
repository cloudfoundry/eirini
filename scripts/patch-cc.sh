#!/bin/bash

set -eu

CCNG_DIR="$HOME/workspace/capi-release/src/cloud_controller_ng"
TAG=$(tr -dc A-Za-z0-9 </dev/urandom | head -c 8)
CCNG_IMAGE="eirini/dev-ccng:$TAG"

build_ccng_image() {
  pushd "$CCNG_DIR"
  {
    pack build --builder "paketobuildpacks/builder:full" "$CCNG_IMAGE"
  }
  popd
}

publish-image() {
  if [[ "$(kubectl config current-context)" =~ "kind-" ]]; then
    load-into-kind
    return
  fi

  push-to-docker
}

push-to-docker() {
  docker push $CCNG_IMAGE
}

load-into-kind() {
  local current_context kind_cluster_name
  # assume we are pointed to a kind cluster
  current_context=$(yq r "$HOME/.kube/config" 'current-context')
  # strip the 'kind-' prefix that kind puts in the context name
  kind_cluster_name=${current_context#"kind-"}

  kind load docker-image --name "$kind_cluster_name" "$CCNG_IMAGE"
}

patch-cf-api-server() {
  local patch_file

  patch_file=$(mktemp)
  trap "rm $patch_file" EXIT

  cat <<EOF >>"$patch_file"
spec:
  template:
    spec:
      containers:
      - name: cf-api-server
        image: "$CCNG_IMAGE"
        imagePullPolicy: IfNotPresent
EOF

  kubectl -n cf-system patch deployment cf-api-server --patch "$(cat "$patch_file")"
}

main() {
  build_ccng_image
  publish-image
  patch-cf-api-server
}

main
