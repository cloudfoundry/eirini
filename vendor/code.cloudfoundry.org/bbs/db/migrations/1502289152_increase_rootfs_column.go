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
	appendMigration(NewIncreaseRootFSColumnSize())
}

type IncreaseRootFSColumnsSize struct {
	serializer format.Serializer
	clock      clock.Clock
	rawSQLDB   *sql.DB
	dbFlavor   string
}

func NewIncreaseRootFSColumnSize() migration.Migration {
	return new(IncreaseRootFSColumnsSize)
}

func (e *IncreaseRootFSColumnsSize) String() string {
	return migrationString(e)
}

func (e *IncreaseRootFSColumnsSize) Version() int64 {
	return 1502289152
}

func (e *IncreaseRootFSColumnsSize) SetCryptor(cryptor encryption.Cryptor) {
	e.serializer = format.NewSerializer(cryptor)
}

func (e *IncreaseRootFSColumnsSize) SetRawSQLDB(db *sql.DB) {
	e.rawSQLDB = db
}

func (e *IncreaseRootFSColumnsSize) SetClock(c clock.Clock)    { e.clock = c }
func (e *IncreaseRootFSColumnsSize) SetDBFlavor(flavor string) { e.dbFlavor = flavor }

func (e *IncreaseRootFSColumnsSize) Up(logger lager.Logger) error {
	logger = logger.Session("increase-rootfs-column")
	logger.Info("starting")
	defer logger.Info("completed")

	return e.alterTables(logger, e.rawSQLDB, e.dbFlavor)
}

func (e *IncreaseRootFSColumnsSize) alterTables(logger lager.Logger, db *sql.DB, flavor string) error {
	var alterActualLRPsSQL string

	if e.dbFlavor == "mysql" {
		alterActualLRPsSQL = `ALTER TABLE desired_lrps
	MODIFY rootfs VARCHAR(1024) NOT NULL DEFAULT ''`

	} else {
		alterActualLRPsSQL = `ALTER TABLE desired_lrps
	ALTER rootfs TYPE VARCHAR(1024)`
	}

	logger.Info("altering-tables")
	logger.Info("altering the table", lager.Data{"query": alterActualLRPsSQL})
	_, err := db.Exec(alterActualLRPsSQL)
	if err != nil {
		logger.Error("failed-altering-tables", err)
		return err
	}
	logger.Info("altered the table", lager.Data{"query": alterActualLRPsSQL})

	return nil
}
