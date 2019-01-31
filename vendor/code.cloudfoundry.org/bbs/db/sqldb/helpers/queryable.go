package helpers

import (
	"database/sql"

	"code.cloudfoundry.org/bbs/db/sqldb/helpers/monitor"
)

type RowScanner interface {
	Scan(dest ...interface{}) error
}

type Queryable interface {
	Exec(query string, args ...interface{}) (sql.Result, error)
	Prepare(query string) (*sql.Stmt, error)
	Query(query string, args ...interface{}) (*sql.Rows, error)
	QueryRow(query string, args ...interface{}) RowScanner
}

//go:generate counterfeiter . QueryableDB
type QueryableDB interface {
	Queryable
	Begin() (Tx, error)
	OpenConnections() int
}

type Tx interface {
	Queryable
	Commit() error
	Rollback() error
}

type monitoredTx struct {
	tx      *sql.Tx
	monitor monitor.Monitor
}

type monitoredDB struct {
	db      *sql.DB
	monitor monitor.Monitor
}

func NewMonitoredDB(db *sql.DB, monitor monitor.Monitor) QueryableDB {
	return &monitoredDB{
		db:      db,
		monitor: monitor,
	}
}

func (db *monitoredDB) OpenConnections() int {
	return db.db.Stats().OpenConnections
}

func (q *monitoredDB) Begin() (Tx, error) {
	var innerTx *sql.Tx
	err := q.monitor.Monitor(func() error {
		var err error
		innerTx, err = q.db.Begin()
		return err
	})

	tx := &monitoredTx{
		tx:      innerTx,
		monitor: q.monitor,
	}

	return tx, err
}

func (q *monitoredDB) Exec(query string, args ...interface{}) (sql.Result, error) {
	var result sql.Result
	err := q.monitor.Monitor(func() error {
		var err error
		result, err = q.db.Exec(query, args...)
		return err
	})
	return result, err
}

func (q *monitoredDB) Prepare(query string) (*sql.Stmt, error) {
	return q.db.Prepare(query)
}

func (q *monitoredDB) Query(query string, args ...interface{}) (*sql.Rows, error) {
	var result *sql.Rows
	err := q.monitor.Monitor(func() error {
		var err error
		result, err = q.db.Query(query, args...)
		return err
	})
	return result, err
}

func (q *monitoredDB) QueryRow(query string, args ...interface{}) RowScanner {
	return NewRowScanner(q.monitor, q.db.QueryRow(query, args...))
}

func (tx *monitoredTx) Exec(query string, args ...interface{}) (sql.Result, error) {
	var result sql.Result
	err := tx.monitor.Monitor(func() error {
		var err error
		result, err = tx.tx.Exec(query, args...)
		return err
	})
	return result, err
}

func (tx *monitoredTx) Prepare(query string) (*sql.Stmt, error) {
	return tx.tx.Prepare(query)
}

func (tx *monitoredTx) Query(query string, args ...interface{}) (*sql.Rows, error) {
	var result *sql.Rows
	err := tx.monitor.Monitor(func() error {
		var err error
		result, err = tx.tx.Query(query, args...)
		return err
	})
	return result, err
}

func (tx *monitoredTx) QueryRow(query string, args ...interface{}) RowScanner {
	return NewRowScanner(tx.monitor, tx.tx.QueryRow(query, args...))
}

func (tx *monitoredTx) Commit() error {
	return tx.monitor.Monitor(tx.tx.Commit)
}

func (tx *monitoredTx) Rollback() error {
	return tx.monitor.Monitor(tx.tx.Rollback)
}

type scannableRow struct {
	monitor monitor.Monitor
	scanner RowScanner
}

func NewRowScanner(monitor monitor.Monitor, scanner RowScanner) RowScanner {
	return &scannableRow{monitor: monitor, scanner: scanner}
}

func (r *scannableRow) Scan(dest ...interface{}) error {
	return r.monitor.Monitor(func() error {
		return r.scanner.Scan(dest...)
	})
}
