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
	appendMigration(NewAddTaskRejectionCount())
}

type AddTaskRejectionCount struct {
	serializer format.Serializer
	clock      clock.Clock
	rawSQLDB   *sql.DB
	dbFlavor   string
}

func NewAddTaskRejectionCount() migration.Migration {
	return new(AddTaskRejectionCount)
}

func (e *AddTaskRejectionCount) String() string {
	return migrationString(e)
}

func (e *AddTaskRejectionCount) Version() int64 {
	return 1522700600
}

func (e *AddTaskRejectionCount) SetCryptor(cryptor encryption.Cryptor) {
	e.serializer = format.NewSerializer(cryptor)
}

func (e *AddTaskRejectionCount) SetRawSQLDB(db *sql.DB)    { e.rawSQLDB = db }
func (e *AddTaskRejectionCount) SetClock(c clock.Clock)    { e.clock = c }
func (e *AddTaskRejectionCount) SetDBFlavor(flavor string) { e.dbFlavor = flavor }

func (e *AddTaskRejectionCount) Up(logger lager.Logger) error {
	logger = logger.Session("add-task-rejection-count")
	logger.Info("starting")
	defer logger.Info("completed")

	const stmt = "ALTER TABLE tasks ADD COLUMN rejection_count INTEGER NOT NULL DEFAULT 0;"
	_, err := e.rawSQLDB.Exec(stmt)
	if err != nil {
		logger.Error("failed-altering-table", err)
		return err
	}
	return nil
}
