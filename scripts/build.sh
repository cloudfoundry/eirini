#!/bin/bash

set -euo pipefail
IFS=$'\n\t'

readonly BASEDIR="$(cd "$(dirname "$0")"/.. && pwd)"

export GO111MODULE=on
pushd "$BASEDIR" > /dev/null || exit 1
  go build -mod vendor -o /dev/null ./cmd/opi
  go build -mod vendor -o /dev/null ./cmd/rootfs-patcher
  go build -mod vendor -o /dev/null ./cmd/bits-waiter
popd > /dev/null || exit 1
