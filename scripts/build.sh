#!/bin/bash

set -euo pipefail
IFS=$'\n\t'

readonly BASEDIR="$(cd "$(dirname "$0")"/.. && pwd)"

pushd "$BASEDIR/cmd/opi" > /dev/null || exit 1
  go build
popd > /dev/null || exit 1
