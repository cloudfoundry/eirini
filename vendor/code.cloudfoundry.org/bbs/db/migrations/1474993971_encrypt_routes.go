package migrations

import (
	"database/sql"
	"errors"
	"fmt"

	"code.cloudfoundry.org/bbs/db/etcd"
	"code.cloudfoundry.org/bbs/db/sqldb/helpers"
	"code.cloudfoundry.org/bbs/encryption"
	"code.cloudfoundry.org/bbs/format"
	"code.cloudfoundry.org/bbs/migration"
	"code.cloudfoundry.org/bbs/models"
	"code.cloudfoundry.org/clock"
	"code.cloudfoundry.org/lager"
)

func init() {
	AppendMigration(NewEncryptRoutes())
}

type EncryptRoutes struct {
	encoder     format.Encoder
	storeClient etcd.StoreClient
	clock       clock.Clock
	rawSQLDB    *sql.DB
	dbFlavor    string
}

func NewEncryptRoutes() migration.Migration {
	return &EncryptRoutes{}
}

func (e *EncryptRoutes) String() string {
	return "1474993971"
}

func (e *EncryptRoutes) Version() int64 {
	return 1474993971
}

func (e *EncryptRoutes) SetStoreClient(storeClient etcd.StoreClient) {
	e.storeClient = storeClient
}

func (e *EncryptRoutes) SetCryptor(cryptor encryption.Cryptor) {
	e.encoder = format.NewEncoder(cryptor)
}

func (e *EncryptRoutes) SetRawSQLDB(db *sql.DB) {
	e.rawSQLDB = db
}

func (e *EncryptRoutes) RequiresSQL() bool         { return true }
func (e *EncryptRoutes) SetClock(c clock.Clock)    { e.clock = c }
func (e *EncryptRoutes) SetDBFlavor(flavor string) { e.dbFlavor = flavor }

func (e *EncryptRoutes) Up(logger lager.Logger) error {
	logger = logger.Session("encrypt-route-column")
	logger.Info("starting")
	defer logger.Info("completed")

	query := fmt.Sprintf("SELECT process_guid, routes FROM desired_lrps")

	rows, err := e.rawSQLDB.Query(query)
	if err != nil {
		logger.Error("failed-query", err)
		return err
	}
	defer rows.Close()

	var processGuid string
	var routeData []byte

	for rows.Next() {
		err := rows.Scan(&processGuid, &routeData)
		if err != nil {
			logger.Error("failed-reading-row", err)
			continue
		}
		encodedData, err := e.encoder.Encode(format.BASE64_ENCRYPTED, routeData)
		if err != nil {
			logger.Error("failed-encrypting-routes", err)
			return models.ErrBadRequest
		}

		bindings := make([]interface{}, 0, 3)
		updateQuery := fmt.Sprintf("UPDATE desired_lrps SET routes = ? WHERE process_guid = ?")
		bindings = append(bindings, encodedData)
		bindings = append(bindings, processGuid)
		_, err = e.rawSQLDB.Exec(helpers.RebindForFlavor(updateQuery, e.dbFlavor), bindings...)
		if err != nil {
			logger.Error("failed-updating-desired-lrp-record", err)
			return models.ErrBadRequest
		}
	}

	if rows.Err() != nil {
		logger.Error("failed-fetching-row", rows.Err())
		return rows.Err()
	}
	return nil
}

func (e *EncryptRoutes) Down(logger lager.Logger) error {
	return errors.New("not implemented")
}
