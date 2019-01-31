#!/bin/bash

usage() {
  >&2 cat <<EOF
SYNOPSIS:
    Add a migration to db/migrations

USAGE:
    $0 MIGRATION_NAME (please use snake_case)

EXAMPLE:
    $0 add_column_to_table
EOF
  exit 1
}

NAME=$1
if [ -z ${NAME} ]; then
  >&2 echo "ERROR: Name is missing."
  usage
fi

VERSION=`date +%s`

CAMEL_CASE_NAME=$(echo ${NAME^} | sed 's/\(_\)\([a-z]\)/\u\2/g')

> db/migrations/${VERSION}_${NAME}.go cat <<EOF
package migrations

import (
	"database/sql"

	"code.cloudfoundry.org/bbs/encryption"
	"code.cloudfoundry.org/bbs/format"
	"code.cloudfoundry.org/bbs/migration"
	"code.cloudfoundry.org/clock"
	"code.cloudfoundry.org/lager"
)

func init() {
	appendMigration(New${CAMEL_CASE_NAME}())
}

type ${CAMEL_CASE_NAME} struct {
	serializer  format.Serializer
	clock	    clock.Clock
	rawSQLDB    *sql.DB
	dbFlavor    string
}

func New${CAMEL_CASE_NAME}() migration.Migration {
	return new(${CAMEL_CASE_NAME})
}

func (e *${CAMEL_CASE_NAME}) String() string {
	return migrationString(e)
}

func (e *${CAMEL_CASE_NAME}) Version() int64 {
	return ${VERSION}
}

func (e *${CAMEL_CASE_NAME}) SetCryptor(cryptor encryption.Cryptor) {
	e.serializer = format.NewSerializer(cryptor)
}

func (e *${CAMEL_CASE_NAME}) SetRawSQLDB(db *sql.DB)	{ e.rawSQLDB = db }
func (e *${CAMEL_CASE_NAME}) SetClock(c clock.Clock)	{ e.clock = c }
func (e *${CAMEL_CASE_NAME}) SetDBFlavor(flavor string) { e.dbFlavor = flavor }

func (e *${CAMEL_CASE_NAME}) Up(logger lager.Logger) error {
	// TODO: add migration code here

	return nil
}
EOF

> db/migrations/${VERSION}_${NAME}_test.go cat <<EOF
package migrations_test

import (
	"code.cloudfoundry.org/bbs/db/migrations"
	"code.cloudfoundry.org/bbs/migration"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("${CAMEL_CASE_NAME}", func() {
	var (
		migration migration.Migration
	)

	BeforeEach(func() {
		// TODO: db cleanup here

		migration = migrations.New${CAMEL_CASE_NAME}()
	})

	It("appends itself to the migration list", func() {
		Expect(migrations.AllMigrations()).To(ContainElement(migration))
	})

	Describe("Version", func() {
		It("returns the timestamp from which it was created", func() {
			Expect(migration.Version()).To(BeEquivalentTo(${VERSION}))
		})
	})

	Describe("Up", func() {
		BeforeEach(func() {
			migration.SetRawSQLDB(rawSQLDB)
			migration.SetDBFlavor(flavor)

			// TODO: db setup here
		})

		It("TODO: CHANGE ME", func() {
			// TODO: test here
		})

		It("is idempotent", func() {
			testIdempotency(rawSQLDB, migration, logger)
		})
	})
})
EOF
