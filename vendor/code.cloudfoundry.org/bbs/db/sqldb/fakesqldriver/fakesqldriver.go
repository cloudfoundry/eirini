package fakesqldriver

import "database/sql/driver"

//go:generate counterfeiter . Driver
type Driver interface {
	Open(name string) (driver.Conn, error)
}

//go:generate counterfeiter . Conn
type Conn interface {
	Prepare(query string) (driver.Stmt, error)
	Close() error
	Begin() (driver.Tx, error)
}

//go:generate counterfeiter . Tx
type Tx interface {
	Commit() error
	Rollback() error
}

//go:generate counterfeiter . Stmt
type Stmt interface {
	Close() error
	NumInput() int
	Exec(args []driver.Value) (driver.Result, error)
	Query(args []driver.Value) (driver.Rows, error)
}
