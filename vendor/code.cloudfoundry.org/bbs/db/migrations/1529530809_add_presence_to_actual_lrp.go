package migrations

import (
	"database/sql"
	"fmt"

	"code.cloudfoundry.org/bbs/db/sqldb/helpers"
	"code.cloudfoundry.org/bbs/encryption"
	"code.cloudfoundry.org/bbs/format"
	"code.cloudfoundry.org/bbs/migration"
	"code.cloudfoundry.org/bbs/models"
	"code.cloudfoundry.org/clock"
	"code.cloudfoundry.org/lager"
)

func init() {
	appendMigration(NewAddPresenceToActualLrp())
}

type AddPresenceToActualLrp struct {
	serializer format.Serializer
	clock      clock.Clock
	rawSQLDB   *sql.DB
	dbFlavor   string
}

func NewAddPresenceToActualLrp() migration.Migration {
	return new(AddPresenceToActualLrp)
}

func (e *AddPresenceToActualLrp) String() string {
	return migrationString(e)
}

func (e *AddPresenceToActualLrp) Version() int64 {
	return 1529530809
}

func (e *AddPresenceToActualLrp) SetCryptor(cryptor encryption.Cryptor) {
	e.serializer = format.NewSerializer(cryptor)
}

func (e *AddPresenceToActualLrp) SetRawSQLDB(db *sql.DB)    { e.rawSQLDB = db }
func (e *AddPresenceToActualLrp) SetClock(c clock.Clock)    { e.clock = c }
func (e *AddPresenceToActualLrp) SetDBFlavor(flavor string) { e.dbFlavor = flavor }

func (e *AddPresenceToActualLrp) Up(logger lager.Logger) error {
	logger = logger.Session("add-presence")
	logger.Info("starting")
	defer logger.Info("completed")

	return e.alterTable(logger)
}

func (e *AddPresenceToActualLrp) alterTable(logger lager.Logger) error {
	alterTablesSQL := []string{
		"ALTER TABLE actual_lrps ADD COLUMN presence INT NOT NULL DEFAULT 0;",
	}

	alterTablesSQL = append(alterTablesSQL, fmt.Sprintf("UPDATE actual_lrps SET presence = %d WHERE evacuating = true;", models.ActualLRP_Evacuating))

	if e.dbFlavor == "mysql" {
		alterTablesSQL = append(alterTablesSQL,
			"ALTER TABLE actual_lrps DROP primary key, ADD PRIMARY KEY (process_guid, instance_index, presence);",
		)
	} else {
		alterTablesSQL = append(alterTablesSQL,
			"ALTER TABLE actual_lrps DROP CONSTRAINT actual_lrps_pkey, ADD PRIMARY KEY (process_guid, instance_index, presence);",
		)
	}

	logger.Info("altering-table")
	for _, query := range alterTablesSQL {
		logger.Info("altering the table", lager.Data{"query": query})
		_, err := e.rawSQLDB.Exec(helpers.RebindForFlavor(query, e.dbFlavor))
		if err != nil {
			logger.Error("failed-altering-table", err)
			return err
		}
		logger.Info("altered the table", lager.Data{"query": query})
	}

	return nil
}
