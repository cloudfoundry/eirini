#!/bin/bash

set -xeuo pipefail
IFS=$'\n\t'

main() {
  create-registry-secret
  create-image-pull-secret
}

create-image-pull-secret() {
  local username password
  username="$(grep -E "signing_users:.*" -A 2 /config/bits-config | awk -F: '/username:/{print $2}' | awk '{$1=$1};1')"
  password="$(grep -E "signing_users:.*" -A 2 /config/bits-config | awk -F: '/password:/{print $2}' | awk '{$1=$1};1')"
  # https://stackoverflow.com/a/45881259
  # There is no better way to upgrade the secret
  kubectl create secret docker-registry "$REGISTRY_CREDS_SECRET_NAME" \
    --docker-server="$REGISTRY_URL" \
    --docker-username="$username" \
    --docker-password="$password" \
    -n "$OPI_NAMESPACE" --dry-run -o yaml |
    kubectl apply -f -
}

get-secret() {
  local id secret
  id=${1}
  secret="$(kubectl get secret "$SECRET_NAME" --namespace="$SCF_NAMESPACE" -ojsonpath=\"{.data.$id}\")"
  echo "  $id: $secret"
}

get-secrets() {
  local id
  for id in cc-certs-volume cc-server-crt cc-server-crt-key cc-uploader-crt cc-uploader-crt-key internal-ca-cert eirini-client-crt eirini-client-crt-key; do
    get-secret $id
  done
}

create-registry-secret() {
  local scf_secrets secret_file_path
  scf_secrets="$(get-secrets)"
  secret_file_path=/tmp/secret.yml

  cat <<EOT >>$secret_file_path
---
apiVersion: v1
kind: Secret
metadata:
  name: $SECRET_NAME
type: Opaque
data:
${scf_secrets}
EOT

  cat $secret_file_path

  kubectl apply -f $secret_file_path --namespace "$OPI_NAMESPACE"
}

main
