package test_helpers

import (
	"fmt"
	"os"
	"strings"

	"code.cloudfoundry.org/bbs/test_helpers/sqlrunner"
)

const (
	mysqlFlavor    = "mysql"
	postgresFlavor = "postgres"
)

func UseSQL() bool {
	return true
}

func driver() string {
	flavor := os.Getenv("SQL_FLAVOR")
	if flavor == "" {
		flavor = postgresFlavor
	}
	return flavor
}

func UseMySQL() bool {
	return driver() == mysqlFlavor
}

func UsePostgres() bool {
	return driver() == postgresFlavor
}

func NewSQLRunner(dbName string) sqlrunner.SQLRunner {
	var sqlRunner sqlrunner.SQLRunner

	if UseMySQL() {
		sqlRunner = sqlrunner.NewMySQLRunner(dbName)
	} else if UsePostgres() {
		sqlRunner = sqlrunner.NewPostgresRunner(dbName)
	} else {
		panic(fmt.Sprintf("driver '%s' is not supported", driver()))
	}

	return sqlRunner
}

func ReplaceQuestionMarks(queryString string) string {
	strParts := strings.Split(queryString, "?")
	for i := 1; i < len(strParts); i++ {
		strParts[i-1] = fmt.Sprintf("%s$%d", strParts[i-1], i)
	}
	return strings.Join(strParts, "")
}
