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

// null migration to bump the database version
func init() {
	AppendMigration(NewAddCachedDependencies())
}

type AddCachedDependencies struct {
	serializer  format.Serializer
	storeClient etcd.StoreClient
}

func NewAddCachedDependencies() migration.Migration {
	return &AddCachedDependencies{}
}

func (a *AddCachedDependencies) Version() int64 {
	return 1450292094
}

func (a *AddCachedDependencies) SetStoreClient(storeClient etcd.StoreClient) {
	a.storeClient = storeClient
}

func (a *AddCachedDependencies) SetCryptor(cryptor encryption.Cryptor) {
	a.serializer = format.NewSerializer(cryptor)
}

func (a *AddCachedDependencies) RequiresSQL() bool {
	return false
}

func (a *AddCachedDependencies) SetRawSQLDB(rawSQLDB *sql.DB) {}
func (a *AddCachedDependencies) SetClock(clock.Clock)         {}
func (a *AddCachedDependencies) SetDBFlavor(string)           {}

func (a *AddCachedDependencies) Up(logger lager.Logger) error {
	return nil
}

func (a *AddCachedDependencies) Down(logger lager.Logger) error {
	return errors.New("not implemented")
}
