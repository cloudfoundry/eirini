#!/bin/bash

set -euo pipefail

echo "building cubefs..."

BASEDIR="$(cd $(dirname $0)/.. && pwd)"

( cd $BASEDIR/launchcmd && GOOS=linux go build -a -o $BASEDIR/image/launch )
( cd $BASEDIR/buildpackapplifecycle/launcher && GOOS=linux CGO_ENABLED=0 go build -a -installsuffix static -o $BASEDIR/image/launcher )

pushd $BASEDIR/image
docker build -t "cube/launch" .
popd

rm $BASEDIR/image/launch
rm $BASEDIR/image/launcher

docker run -it -d --name="cube-launch" cube/launch /bin/bash
docker export cube-launch -o $BASEDIR/image/cubefs.tar
docker stop cube-launch
docker rm cube-launch
echo "successfully created cubefs.tar"
