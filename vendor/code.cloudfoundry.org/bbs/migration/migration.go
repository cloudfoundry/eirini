package migration

import (
	"database/sql"

	"code.cloudfoundry.org/bbs/encryption"
	"code.cloudfoundry.org/clock"
	"code.cloudfoundry.org/lager"
)

//go:generate counterfeiter -o migrationfakes/fake_migration.go . Migration

type Migration interface {
	String() string
	Version() int64
	Up(logger lager.Logger) error
	SetCryptor(cryptor encryption.Cryptor)
	SetClock(c clock.Clock)
	SetRawSQLDB(rawSQLDB *sql.DB)
	SetDBFlavor(flavor string)
}
