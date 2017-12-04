package migrations

import (
	"database/sql"
	"errors"

	"code.cloudfoundry.org/bbs/db/etcd"
	"code.cloudfoundry.org/bbs/encryption"
	"code.cloudfoundry.org/bbs/format"
	"code.cloudfoundry.org/bbs/migration"
	"code.cloudfoundry.org/clock"
	"code.cloudfoundry.org/lager"
)

func init() {
	AppendMigration(NewIncreaseRunInfoColumnSize())
}

type IncreaseRunInfoColumnSize struct {
	serializer  format.Serializer
	storeClient etcd.StoreClient
	clock       clock.Clock
	rawSQLDB    *sql.DB
	dbFlavor    string
}

func NewIncreaseRunInfoColumnSize() migration.Migration {
	return &IncreaseRunInfoColumnSize{}
}

func (e *IncreaseRunInfoColumnSize) String() string {
	return "1471030898"
}

func (e *IncreaseRunInfoColumnSize) Version() int64 {
	return 1471030898
}

func (e *IncreaseRunInfoColumnSize) SetStoreClient(storeClient etcd.StoreClient) {
	e.storeClient = storeClient
}

func (e *IncreaseRunInfoColumnSize) SetCryptor(cryptor encryption.Cryptor) {
	e.serializer = format.NewSerializer(cryptor)
}

func (e *IncreaseRunInfoColumnSize) SetRawSQLDB(db *sql.DB) {
	e.rawSQLDB = db
}

func (e *IncreaseRunInfoColumnSize) RequiresSQL() bool         { return true }
func (e *IncreaseRunInfoColumnSize) SetClock(c clock.Clock)    { e.clock = c }
func (e *IncreaseRunInfoColumnSize) SetDBFlavor(flavor string) { e.dbFlavor = flavor }

func (e *IncreaseRunInfoColumnSize) Up(logger lager.Logger) error {
	logger = logger.Session("increase-run-info-column")
	logger.Info("starting")
	defer logger.Info("completed")

	return alterTables(logger, e.rawSQLDB, e.dbFlavor)
}

func (e *IncreaseRunInfoColumnSize) Down(logger lager.Logger) error {
	return errors.New("not implemented")
}

func alterTables(logger lager.Logger, db *sql.DB, flavor string) error {
	if flavor != "mysql" {
		return nil
	}

	var alterTablesSQL = []string{
		alterDesiredLRPsSQL,
		alterActualLRPsSQL,
		alterTasksSQL,
	}

	logger.Info("altering-tables")
	for _, query := range alterTablesSQL {
		logger.Info("altering the table", lager.Data{"query": query})
		_, err := db.Exec(query)
		if err != nil {
			logger.Error("failed-altering-tables", err)
			return err
		}
		logger.Info("altered the table", lager.Data{"query": query})
	}

	return nil
}

const alterDesiredLRPsSQL = `ALTER TABLE desired_lrps
	MODIFY annotation MEDIUMTEXT,
	MODIFY routes MEDIUMTEXT NOT NULL,
	MODIFY volume_placement MEDIUMTEXT NOT NULL,
	MODIFY run_info MEDIUMTEXT NOT NULL;`

const alterActualLRPsSQL = `ALTER TABLE actual_lrps
	MODIFY net_info MEDIUMTEXT NOT NULL;`

const alterTasksSQL = `ALTER TABLE tasks
	MODIFY result MEDIUMTEXT,
	MODIFY task_definition MEDIUMTEXT NOT NULL;`
