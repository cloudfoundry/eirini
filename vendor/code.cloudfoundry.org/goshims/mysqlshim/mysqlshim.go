package mysqlshim

import (
	"crypto/tls"

	"github.com/go-sql-driver/mysql"
)

type MySQLShim struct{}

func (sh *MySQLShim) ParseDSN(dsn string) (cfg *mysql.Config, err error) {
	return mysql.ParseDSN(dsn)
}

func (sh *MySQLShim) RegisterTLSConfig(key string, config *tls.Config) error {
	return mysql.RegisterTLSConfig(key, config)
}
