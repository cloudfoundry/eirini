package helpers

import (
	"database/sql"
	"errors"

	"github.com/go-sql-driver/mysql"
	"github.com/lib/pq"
)

var (
	ErrResourceExists     = errors.New("sql-resource-exists")
	ErrDeadlock           = errors.New("sql-deadlock")
	ErrBadRequest         = errors.New("sql-bad-request")
	ErrUnrecoverableError = errors.New("sql-unrecoverable")
	ErrUnknownError       = errors.New("sql-unknown")
	ErrResourceNotFound   = errors.New("sql-resource-not-found")
)

func (h *sqlHelper) ConvertSQLError(err error) error {
	if err != nil {
		switch err.(type) {
		case *mysql.MySQLError:
			return h.convertMySQLError(err.(*mysql.MySQLError))
		case *pq.Error:
			return h.convertPostgresError(err.(*pq.Error))
		}

		if err == sql.ErrNoRows {
			return ErrResourceNotFound
		}
	}

	return err
}

func (h *sqlHelper) convertMySQLError(err *mysql.MySQLError) error {
	switch err.Number {
	case 1062:
		return ErrResourceExists
	case 1213:
		return ErrDeadlock
	case 1406:
		return ErrBadRequest
	case 1146:
		return ErrUnrecoverableError
	default:
		return ErrUnknownError
	}
}

func (h *sqlHelper) convertPostgresError(err *pq.Error) error {
	switch err.Code {
	case "22001":
		return ErrBadRequest
	case "23505":
		return ErrResourceExists
	case "42P01":
		return ErrUnrecoverableError
	default:
		return ErrUnknownError
	}
}
