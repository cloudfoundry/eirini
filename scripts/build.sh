#!/bin/bash

set -euo pipefail
IFS=$'\n\t'

readonly BASEDIR="$(cd "$(dirname "$0")"/.. && pwd)"

pushd "$BASEDIR/cmd/opi" || exit 1
    go build
popd || exit 1
