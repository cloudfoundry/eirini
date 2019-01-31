#!/bin/bash

DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )"/.. && pwd )"

unset CREDHUB_DEBUG
pushd "$DIR" >/dev/null
  make test
popd >/dev/null
