#!/bin/bash

usage() {
  >&2 cat <<EOF
SYNOPSIS:
    Add a migration to db/migrations

USAGE:
    $0 MIGRATION_NAME

EXAMPLE:
    $0 add_column_to_table
EOF
  exit 1
}

name=$1
id=`date +%s`

if [ -z ${name} ]; then
  >&2 echo "ERROR: Name is missing."
  usage
fi

touch db/migrations/${id}_${name}.go
touch db/migrations/${id}_${name}_test.go
