#!/bin/bash

set -xeuo pipefail
IFS=$'\n\t'

declare -A key_map
key_map=(["cc-server-crt"]="tls.crt" ["cc-server-crt-key"]="tls.key" ["internal-ca-cert"]="ca.crt" ["eirini-client-crt"]="tls.crt" ["eirini-client-crt-key"]="tls.key")

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
  local id new_id secret
  id=${1}
  new_id=${2}
  secret="$(kubectl get secret "$SECRET_NAME" --namespace="$SCF_NAMESPACE" -ojsonpath=\"{.data.$id}\")"
  echo "  $new_id: $secret"
}

get-secrets() {
  local id
  for id in $@; do
    get-secret $id ${key_map[$id]}
  done
}

create-registry-secret() {
  cc_keys=("cc-server-crt" "cc-server-crt-key" "internal-ca-cert")
  apply-secret "cc-uploader-certs" ${cc_keys[@]}

  eirini_keys=("eirini-client-crt" "eirini-client-crt-key" "internal-ca-cert")
  apply-secret "eirini-client-certs" ${eirini_keys[@]}
}

apply-secret() {
  local secret_name scf_secrets
  secret_name="$1"
  shift 1
  scf_secrets="$(get-secrets $@)"
  secret_file_path=/tmp/secret.yml

  cat <<EOT >>$secret_file_path
---
apiVersion: v1
kind: Secret
metadata:
  name: $secret_name
type: Opaque
data:
${scf_secrets}
EOT

  cat $secret_file_path

  kubectl apply -f $secret_file_path --namespace "$OPI_NAMESPACE"
}

main
