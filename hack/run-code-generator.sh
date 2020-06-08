#!/bin/bash

set -euo pipefail

EIRINI_ROOT=$(dirname "${BASH_SOURCE[0]}")/..
CODEGEN_PKG=${CODEGEN_PKG:-$(
  cd "${EIRINI_ROOT}"
  ls -d -1 ./vendor/k8s.io/code-generator 2>/dev/null || echo ../code-generator
)}

rm -rf $EIRINI_ROOT/pkg/generated

# generate the code with:
# --output-base    because this script should also be able to run inside the vendor dir of
#                  k8s.io/kubernetes. The output-base is needed for the generators to output into the vendor dir
#                  instead of the $GOPATH directly. For normal projects this can be dropped.
/bin/bash "${CODEGEN_PKG}/generate-groups.sh" all \
  code.cloudfoundry.org/eirini/pkg/generated code.cloudfoundry.org/eirini/pkg/apis \
  lrp:v1 \
  --output-base "$(dirname "${BASH_SOURCE[0]}")/.." \
  --go-header-file "${EIRINI_ROOT}/hack/boilerplate.go.txt"

cp -R $EIRINI_ROOT/code.cloudfoundry.org/eirini/pkg/generated $EIRINI_ROOT/pkg
cp -R $EIRINI_ROOT/code.cloudfoundry.org/eirini/pkg/apis/* $EIRINI_ROOT/pkg/apis/

cleanup() {
  rm -rf $EIRINI_ROOT/code.cloudfoundry.org
}

trap cleanup EXIT
