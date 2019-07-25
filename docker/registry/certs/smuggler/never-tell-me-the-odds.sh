#!/bin/bash

set -xeuo pipefail
IFS=$'\n\t'

main(){
  create-registry-secret
  create-image-pull-secret
}

create-image-pull-secret(){
  local username password
  username="$(grep -E "signing_users:.*" -A 2 /config/bits-config |  awk -F: '/username:/{print $2}' | awk '{$1=$1};1')"
  password="$(grep -E "signing_users:.*" -A 2 /config/bits-config |  awk -F: '/password:/{print $2}' | awk '{$1=$1};1')"
  kubectl create secret docker-registry "$REGISTRY_CREDS_SECRET_NAME" \
    --docker-server="$REGISTRY_URL" \
    --docker-username="$username" \
    --docker-password="$password" \
    -n "$OPI_NAMESPACE"
}

create-registry-secret(){
  scf_secrets="$(kubectl get secret "$SECRET_NAME" --namespace="$SCF_NAMESPACE" --export -o yaml | grep -E 'cc-.*|internal-ca-.*')"

  cat <<EOT >> secret.yml
---
apiVersion: v1
kind: Secret
metadata:
  name: $SECRET_NAME
type: Opaque
data:
${scf_secrets}
EOT

  cat secret.yml

  kubectl apply -f secret.yml --namespace "$OPI_NAMESPACE"
}

main
