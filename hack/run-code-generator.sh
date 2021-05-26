#!/bin/bash

set -euo pipefail

EIRINI_ROOT=$(dirname "${BASH_SOURCE[0]}")/..
EIRINI_TMP_CRD="$EIRINI_ROOT/code.cloudfoundry.org/crds"
EIRINI_RELEASE="$EIRINI_ROOT/../eirini-release"
CODEGEN_PKG=${CODEGEN_PKG:-$(
  cd "${EIRINI_ROOT}"
  ls -d -1 ./vendor/k8s.io/code-generator 2>/dev/null || echo ../code-generator
)}

CONTROLLERGEN_PKG=${CONTROLLERGEN_PKG:-$(
  cd "${EIRINI_ROOT}"
  ls -d -1 ./vendor/sigs.k8s.io/controller-tools 2>/dev/null || echo ../controller-tools
)}

cleanup() {
  rm -rf $EIRINI_ROOT/code.cloudfoundry.org
}

trap cleanup EXIT

rm -rf $EIRINI_ROOT/pkg/generated

# generate the code with:
# --output-base    because this script should also be able to run inside the vendor dir of
#                  k8s.io/kubernetes. The output-base is needed for the generators to output into the vendor dir
#                  instead of the $GOPATH directly. For normal projects this can be dropped.
/bin/bash "${CODEGEN_PKG}/generate-groups.sh" all \
  code.cloudfoundry.org/eirini/pkg/generated code.cloudfoundry.org/eirini/pkg/apis \
  eirini:v1 \
  --output-base "$(dirname "${BASH_SOURCE[0]}")/.." \
  --go-header-file "${EIRINI_ROOT}/hack/boilerplate.go.txt"

cp -R $EIRINI_ROOT/code.cloudfoundry.org/eirini/pkg/generated $EIRINI_ROOT/pkg
cp -R $EIRINI_ROOT/code.cloudfoundry.org/eirini/pkg/apis/* $EIRINI_ROOT/pkg/apis/

# CRD Generation

mkdir -p "$EIRINI_TMP_CRD"

pushd "$EIRINI_ROOT"
{
  go run vendor/sigs.k8s.io/controller-tools/cmd/controller-gen/main.go crd output:dir="$EIRINI_TMP_CRD" paths=./pkg/apis/...
  cp "$EIRINI_TMP_CRD/eirini.cloudfoundry.org_lrps.yaml" "$EIRINI_RELEASE/helm/templates/core/lrp-crd.yml"
  cp "$EIRINI_TMP_CRD/eirini.cloudfoundry.org_tasks.yaml" "$EIRINI_RELEASE/helm/templates/core/task-crd.yml"
}
popd
