#!/bin/bash

set -euo pipefail
IFS=$'\n\t'

readonly BASEDIR="$(cd "$(dirname "$0")"/.. && pwd)"

pushd "$BASEDIR" > /dev/null || exit 1
  go build -o /dev/null ./cmd/opi
  go build -o /dev/null ./cmd/rootfs-patcher
popd > /dev/null || exit 1
