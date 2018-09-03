#!/bin/bash

set -euo pipefail

readonly BASEDIR="$(cd $(dirname $0)/.. && pwd)"

main() {
    build-recipe
    build-packs-builder
    build-image
}

build-recipe() {
    pushd "$BASEDIR/cmd"
        GOOS=linux go build -a -o "$BASEDIR"/image/recipe
    popd
}

build-packs-builder() {
    pushd "$BASEDIR"/packs/cf/cmd/builder
        GOOS=linux CGO_ENABLED=0 go build -a -installsuffix static -o "$BASEDIR"/image/builder
    popd

}

build-image() {
    pushd "$BASEDIR"/image
        docker build --add-host="cc-uploader.service.cf.internal:10.45.94.125" --build-arg buildpacks="$(< "buildpacks.json")" -t "eirini/recipe" .
    popd
}

main
