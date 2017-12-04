package sqldb

import (
	"database/sql"

	"code.cloudfoundry.org/bbs/db/sqldb/helpers"
	"code.cloudfoundry.org/bbs/encryption"
	"code.cloudfoundry.org/bbs/format"
	"code.cloudfoundry.org/bbs/guidprovider"
	"code.cloudfoundry.org/bbs/models"
	"code.cloudfoundry.org/clock"
	loggingclient "code.cloudfoundry.org/diego-logging-client"
	"code.cloudfoundry.org/lager"
)

type SQLDB struct {
	db                     *sql.DB
	convergenceWorkersSize int
	updateWorkersSize      int
	clock                  clock.Clock
	format                 *format.Format
	guidProvider           guidprovider.GUIDProvider
	serializer             format.Serializer
	cryptor                encryption.Cryptor
	encoder                format.Encoder
	flavor                 string
	helper                 helpers.SQLHelper
	metronClient           loggingclient.IngressClient
}

type RowScanner interface {
	Scan(dest ...interface{}) error
}

type Queryable interface {
	Exec(query string, args ...interface{}) (sql.Result, error)
	Prepare(query string) (*sql.Stmt, error)
	Query(query string, args ...interface{}) (*sql.Rows, error)
	QueryRow(query string, args ...interface{}) *sql.Row
}

func NewSQLDB(
	db *sql.DB,
	convergenceWorkersSize int,
	updateWorkersSize int,
	serializationFormat *format.Format,
	cryptor encryption.Cryptor,
	guidProvider guidprovider.GUIDProvider,
	clock clock.Clock,
	flavor string,
	metronClient loggingclient.IngressClient,
) *SQLDB {
	helper := helpers.NewSQLHelper(flavor)
	return &SQLDB{
		db: db,
		convergenceWorkersSize: convergenceWorkersSize,
		updateWorkersSize:      updateWorkersSize,
		clock:                  clock,
		format:                 serializationFormat,
		guidProvider:           guidProvider,
		serializer:             format.NewSerializer(cryptor),
		cryptor:                cryptor,
		encoder:                format.NewEncoder(cryptor),
		flavor:                 flavor,
		helper:                 helper,
		metronClient:           metronClient,
	}
}

func (db *SQLDB) transact(logger lager.Logger, f func(logger lager.Logger, tx *sql.Tx) error) error {
	err := db.helper.Transact(logger, db.db, f)
	if err != nil {
		return db.convertSQLError(err)
	}
	return nil
}

func (db *SQLDB) serializeModel(logger lager.Logger, model format.Versioner) ([]byte, error) {
	encodedPayload, err := db.serializer.Marshal(logger, db.format, model)
	if err != nil {
		logger.Error("failed-to-serialize-model", err)
		return nil, models.NewError(models.Error_InvalidRecord, err.Error())
	}
	return encodedPayload, nil
}

func (db *SQLDB) deserializeModel(logger lager.Logger, data []byte, model format.Versioner) error {
	err := db.serializer.Unmarshal(logger, data, model)
	if err != nil {
		logger.Error("failed-to-deserialize-model", err)
		return models.NewError(models.Error_InvalidRecord, err.Error())
	}
	return nil
}

func (db *SQLDB) convertSQLError(err error) *models.Error {
	converted := db.helper.ConvertSQLError(err)
	switch converted {
	case helpers.ErrResourceExists:
		return models.ErrResourceExists
	case helpers.ErrDeadlock:
		return models.ErrDeadlock
	case helpers.ErrBadRequest:
		return models.ErrBadRequest
	case helpers.ErrUnrecoverableError:
		return models.NewUnrecoverableError(err)
	case helpers.ErrUnknownError:
		return models.ErrUnknownError
	case helpers.ErrResourceNotFound:
		return models.ErrResourceNotFound
	default:
		return models.ConvertError(err)
	}
}
