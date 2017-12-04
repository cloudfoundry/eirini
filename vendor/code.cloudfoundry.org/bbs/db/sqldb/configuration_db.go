package sqldb

import (
	"database/sql"

	"code.cloudfoundry.org/bbs/db/sqldb/helpers"
	"code.cloudfoundry.org/lager"
)

const configurationsTable = "configurations"

func (db *SQLDB) setConfigurationValue(logger lager.Logger, key, value string) error {
	return db.transact(logger, func(logger lager.Logger, tx *sql.Tx) error {
		_, err := db.upsert(
			logger,
			tx,
			configurationsTable,
			helpers.SQLAttributes{"value": value, "id": key},
			"id = ?", key,
		)
		if err != nil {
			logger.Error("failed-setting-config-value", err, lager.Data{"key": key})
			return err
		}

		return nil
	})
}

func (db *SQLDB) getConfigurationValue(logger lager.Logger, key string) (string, error) {
	var value string
	err := db.transact(logger, func(logger lager.Logger, tx *sql.Tx) error {
		return db.one(logger, tx, "configurations",
			helpers.ColumnList{"value"}, helpers.NoLockRow,
			"id = ?", key,
		).Scan(&value)
	})

	if err != nil {
		logger.Error("failed-fetching-configuration-value", err, lager.Data{"key": key})
		return "", err
	}

	return value, nil
}
