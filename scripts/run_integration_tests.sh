#!/bin/bash

readonly OPI_CONFIG_FILE="${1?OPI config file must be provided!}"
readonly BASEDIR="$(cd "$(dirname "$0")"/.. && pwd)"


main(){
  if [ "$1" = "help" ]; then
    help
  fi
  
  run_tests
}

run_tests() {
  pushd "$BASEDIR"/integration || exit 1
    ginkgo -tags=integration -- -valid_opi_config="$OPI_CONFIG_FILE"
  popd || exit 1
}

help() {
  cat << EOF 
  Usage:
    $ ./run_integration_tests.sh <path-to-opi-config-file>

  These integration tests depend on a running minikube and a OPI config file which must follow the template defined at 'eirini/integration/example_opi_config.yml'
  Also, a connection to the API endpoint is established as part of the test.
EOF

  exit 1
}


main "$@"
