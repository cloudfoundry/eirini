#!/bin/bash

set -euo pipefail

BASEDIR="$(cd $(dirname $0)/.. && pwd)"

( cd $BASEDIR && GOOS=linux go build -a -o $BASEDIR/image/recipe )
( cd $BASEDIR/packs/cf/builder && GOOS=linux CGO_ENABLED=0 go build -a -installsuffix static -o $BASEDIR/image/builder )

pushd $BASEDIR/image
docker build --build-arg buildpacks="$(< "buildpacks.json")" -t "diegoteam/recipe:build" .
popd

