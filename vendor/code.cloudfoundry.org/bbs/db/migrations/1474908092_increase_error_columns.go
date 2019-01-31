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
	appendMigration(NewIncreaseErrorColumnsSize())
}

type IncreaseErrorColumnsSize struct {
	serializer format.Serializer
	clock      clock.Clock
	rawSQLDB   *sql.DB
	dbFlavor   string
}

func NewIncreaseErrorColumnsSize() migration.Migration {
	return &IncreaseErrorColumnsSize{}
}

func (e *IncreaseErrorColumnsSize) String() string {
	return migrationString(e)
}

func (e *IncreaseErrorColumnsSize) Version() int64 {
	return 1474908092
}

func (e *IncreaseErrorColumnsSize) SetCryptor(cryptor encryption.Cryptor) {
	e.serializer = format.NewSerializer(cryptor)
}

func (e *IncreaseErrorColumnsSize) SetRawSQLDB(db *sql.DB) {
	e.rawSQLDB = db
}

func (e *IncreaseErrorColumnsSize) SetClock(c clock.Clock)    { e.clock = c }
func (e *IncreaseErrorColumnsSize) SetDBFlavor(flavor string) { e.dbFlavor = flavor }

func (e *IncreaseErrorColumnsSize) Up(logger lager.Logger) error {
	logger = logger.Session("increase-run-info-column")
	logger.Info("starting")
	defer logger.Info("completed")

	return e.alterTables(logger, e.rawSQLDB, e.dbFlavor)
}

func (e *IncreaseErrorColumnsSize) alterTables(logger lager.Logger, db *sql.DB, flavor string) error {
	var alterActualLRPsSQL string

	if e.dbFlavor == "mysql" {
		alterActualLRPsSQL = `ALTER TABLE actual_lrps
	MODIFY crash_reason VARCHAR(1024) NOT NULL DEFAULT '',
	MODIFY placement_error VARCHAR(1024) NOT NULL DEFAULT ''`

	} else {
		alterActualLRPsSQL = `ALTER TABLE actual_lrps
	ALTER crash_reason TYPE VARCHAR(1024),
	ALTER placement_error TYPE VARCHAR(1024)`
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
