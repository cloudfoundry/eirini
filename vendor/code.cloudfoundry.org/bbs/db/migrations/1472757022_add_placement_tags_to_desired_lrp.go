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
	AppendMigration(NewAddPlacementTagsToDesiredLRPs())
}

type AddPlacementTagsToDesiredLRPs struct {
	serializer  format.Serializer
	storeClient etcd.StoreClient
	clock       clock.Clock
	rawSQLDB    *sql.DB
	dbFlavor    string
}

func NewAddPlacementTagsToDesiredLRPs() migration.Migration {
	return &AddPlacementTagsToDesiredLRPs{}
}

func (e *AddPlacementTagsToDesiredLRPs) String() string {
	return "1472757022"
}

func (e *AddPlacementTagsToDesiredLRPs) Version() int64 {
	return 1472757022
}

func (e *AddPlacementTagsToDesiredLRPs) SetStoreClient(storeClient etcd.StoreClient) {
	e.storeClient = storeClient
}

func (e *AddPlacementTagsToDesiredLRPs) SetCryptor(cryptor encryption.Cryptor) {
	e.serializer = format.NewSerializer(cryptor)
}

func (e *AddPlacementTagsToDesiredLRPs) SetRawSQLDB(db *sql.DB) {
	e.rawSQLDB = db
}

func (e *AddPlacementTagsToDesiredLRPs) RequiresSQL() bool         { return true }
func (e *AddPlacementTagsToDesiredLRPs) SetClock(c clock.Clock)    { e.clock = c }
func (e *AddPlacementTagsToDesiredLRPs) SetDBFlavor(flavor string) { e.dbFlavor = flavor }

func (e *AddPlacementTagsToDesiredLRPs) Up(logger lager.Logger) error {
	logger.Info("altering the table", lager.Data{"query": alterDesiredLRPAddPlacementTagSQL})
	_, err := e.rawSQLDB.Exec(alterDesiredLRPAddPlacementTagSQL)
	if err != nil {
		logger.Error("failed-altering-tables", err)
		return err
	}
	logger.Info("altered the table", lager.Data{"query": alterDesiredLRPAddPlacementTagSQL})

	return nil
}

const alterDesiredLRPAddPlacementTagSQL = `ALTER TABLE desired_lrps
	ADD COLUMN placement_tags TEXT;`

func (e *AddPlacementTagsToDesiredLRPs) Down(logger lager.Logger) error {
	return errors.New("not implemented")
}
