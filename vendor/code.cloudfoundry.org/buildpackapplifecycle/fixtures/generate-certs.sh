#!/usr/bin/env bash

certstrap  init -cn CA --passphrase ''
certstrap  request-cert --cn server --ip 127.0.0.1 --passphrase ''
certstrap  sign  --CA CA server
certstrap  request-cert --cn client --ip 127.0.0.1 --passphrase ''
certstrap  sign  --CA CA client
rm -rf cacerts certs
mkdir cacerts certs
mv out/CA.{crt,key} cacerts/
mv out/client.{crt,key} out/server.{crt,key} certs/
rm -rf out/
