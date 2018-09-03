#!/usr/bin/env bash

set -eu

# TODO allow buildpacks as args

BUILDPACKS_DIR=${1}
APP_DIR=${2}
CACHE_DIR=${3}
ENV_DIR=${4}
SLUG_FILE=${5}
CACHE_FILE=${6}

detect="$(/packs/cytokine detect-buildpack --verbose ${APP_DIR} ${BUILDPACKS_DIR} 2>&1)"

buildpack="$(echo "${detect}" | grep -e '"https://.*"' -oh | sed -e 's/"//g')"

rm -rf ${ENV_DIR}
mkdir -p ${ENV_DIR}

/packs/cytokine run-buildpacks \
  --buildpack=${buildpack} \
  ${APP_DIR} \
  ${CACHE_DIR} \
  ${ENV_DIR} \
  ${BUILDPACKS_DIR}

/packs/cytokine release-buildpacks \
  --buildpack=${buildpack} \
  ${APP_DIR} \
  ${BUILDPACKS_DIR} \
  ${APP_DIR}/release.yml

/packs/cytokine make-slug /tmp/slug.tgz ${APP_DIR}

mkdir -p $(dirname ${SLUG_FILE})
mv /tmp/slug.tgz ${SLUG_FILE}

mkdir -p $(dirname ${CACHE_FILE})
tar czf ${CACHE_FILE} -C ${CACHE_DIR} .
