#!/bin/bash

BASEDIR="$(cd $(dirname $0)/.. && pwd)"
EXCLUDE="cmd scripts blobondemand cubefakes launcher"

for d in $BASEDIR/*; do
  if [ -d "$d" ]; then
	  dirname=$(basename $d)
	  if [[ $EXCLUDE != *"$dirname"* ]]; then
	     pushd $d
             ginkgo -succinct
	     popd $d
	  fi
  fi
done
