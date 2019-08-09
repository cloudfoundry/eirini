#!/usr/bin/env sh

set -eux

goml set -f /configs/opi.yml -p opi.nats_password -v "$NATS_PASSWORD" -d >/output/opi.yml
