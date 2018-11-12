package mysqlshim

import (
	"crypto/tls"

	"github.com/go-sql-driver/mysql"
)

//go:generate counterfeiter -o mysql_fake/fake_mysql.go . MySQL
type MySQL interface {
	ParseDSN(dsn string) (*mysql.Config, error)
	RegisterTLSConfig(string, *tls.Config) error
}
