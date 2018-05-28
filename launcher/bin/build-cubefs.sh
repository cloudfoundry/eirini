#!/bin/bash

set -euo pipefail

echo "building eirinifs..."

BASEDIR="$(cd $(dirname $0)/.. && pwd)"

( cd $BASEDIR/launchcmd && GOOS=linux go build -a -o $BASEDIR/image/launch )
( cd $BASEDIR/buildpackapplifecycle/launcher && GOOS=linux CGO_ENABLED=0 go build -a -installsuffix static -o $BASEDIR/image/launcher )

pushd $BASEDIR/image
docker build -t "eirini/launch" .
popd

rm $BASEDIR/image/launch
rm $BASEDIR/image/launcher

docker run -it -d --name="eirini-launch" eirini/launch /bin/bash
docker export eirini-launch -o $BASEDIR/image/eirinifs.tar
docker stop eirini-launch
docker rm eirini-launch
echo "successfully created eirinifs.tar"
