package sqldb

import (
	"database/sql"
	"fmt"

	"code.cloudfoundry.org/bbs/db/sqldb/helpers"
	"code.cloudfoundry.org/bbs/format"
	"code.cloudfoundry.org/lager"
)

const EncryptionKeyID = "encryption_key_label"

func (db *SQLDB) SetEncryptionKeyLabel(logger lager.Logger, label string) error {
	logger = logger.Session("set-encrption-key-label", lager.Data{"label": label})
	logger.Debug("starting")
	defer logger.Debug("complete")

	return db.setConfigurationValue(logger, EncryptionKeyID, label)
}

func (db *SQLDB) EncryptionKeyLabel(logger lager.Logger) (string, error) {
	logger = logger.Session("encrption-key-label")
	logger.Debug("starting")
	defer logger.Debug("complete")

	return db.getConfigurationValue(logger, EncryptionKeyID)
}

func (db *SQLDB) PerformEncryption(logger lager.Logger) error {
	errCh := make(chan error)

	funcs := []func(){
		func() {
			errCh <- db.reEncrypt(logger, tasksTable, "guid", true, "task_definition")
		},
		func() {
			errCh <- db.reEncrypt(logger, desiredLRPsTable, "process_guid", true, "run_info", "volume_placement", "routes")
		},
		func() {
			errCh <- db.reEncrypt(logger, actualLRPsTable, "process_guid", false, "net_info")
		},
	}

	for _, f := range funcs {
		go f()
	}

	for range funcs {
		err := <-errCh
		if err != nil {
			return err
		}
	}
	return nil
}

func (db *SQLDB) reEncrypt(logger lager.Logger, tableName, primaryKey string, encryptIfEmpty bool, blobColumns ...string) error {
	logger = logger.WithData(
		lager.Data{"table_name": tableName, "primary_key": primaryKey, "blob_columns": blobColumns},
	)
	rows, err := db.db.Query(fmt.Sprintf("SELECT %s FROM %s", primaryKey, tableName))
	if err != nil {
		return err
	}
	defer rows.Close()

	guids := []string{}
	for rows.Next() {
		var guid string
		err := rows.Scan(&guid)
		if err != nil {
			logger.Error("failed-to-scan-primary-key", err)
			continue
		}
		guids = append(guids, guid)
	}

	where := fmt.Sprintf("%s = ?", primaryKey)
	for _, guid := range guids {
		err = db.transact(logger, func(logger lager.Logger, tx *sql.Tx) error {
			blobs := make([]interface{}, len(blobColumns))

			row := db.one(logger, tx, tableName, blobColumns, helpers.LockRow, where, guid)
			for i := range blobColumns {
				var blob []byte
				blobs[i] = &blob
			}

			err := row.Scan(blobs...)
			if err != nil {
				logger.Error("failed-to-scan-blob", err)
				return nil
			}

			updatedColumnValues := map[string]interface{}{}

			for columnIdx := range blobs {
				// This type assertion should not fail because we set the value to be a pointer to a byte array above
				blobPtr := blobs[columnIdx].(*[]byte)
				blob := *blobPtr

				// don't encrypt column if it doesn't contain any data, see #132626553 for more info
				if !encryptIfEmpty && len(blob) == 0 {
					return nil
				}

				encoder := format.NewEncoder(db.cryptor)
				payload, err := encoder.Decode(blob)
				if err != nil {
					logger.Error("failed-to-decode-blob", err)
					return nil
				}
				encryptedPayload, err := encoder.Encode(format.BASE64_ENCRYPTED, payload)
				if err != nil {
					logger.Error("failed-to-encode-blob", err)
					return err
				}

				columnName := blobColumns[columnIdx]
				updatedColumnValues[columnName] = encryptedPayload
			}
			_, err = db.update(logger, tx, tableName,
				updatedColumnValues,
				where, guid,
			)
			if err != nil {
				logger.Error("failed-to-update-blob", err)
				return err
			}
			return nil
		})

		if err != nil {
			return err
		}
	}
	return nil
}
