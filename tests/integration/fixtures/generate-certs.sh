#!/bin/bash

set -euo pipefail

openssl req -x509 \
  -newkey rsa:4096 \
  -keyout tls.key \
  -out tls.crt \
  -nodes \
  -subj '/CN=localhost' \
  -addext "subjectAltName = DNS:localhost, IP:127.0.0.1" \
  -days 3650

cp tls.crt tls.ca
