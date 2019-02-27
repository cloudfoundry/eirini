#!/bin/bash

set -euo pipefail
IFS=$'\n\t'

RUN_DIR="$(cd "$(dirname "$0")" && pwd)"

"$RUN_DIR"/run_unit_tests.sh
"$RUN_DIR"/build.sh
"$RUN_DIR"/run_integration_tests.sh

echo "Running Linter"
cd "$RUN_DIR"/.. || exit 1
golangci-lint run
