#!/bin/bash

set -euo pipefail

readonly BASEDIR="$(cd $(dirname $0)/.. && pwd)"
readonly TAG="${1?Provide a tag please}"
readonly BASEIMAGE="${2:-packs/cf}"

source "${BASEDIR}/bin/functions.sh"

main
