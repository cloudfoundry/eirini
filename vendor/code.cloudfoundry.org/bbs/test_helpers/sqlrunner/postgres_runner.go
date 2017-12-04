package sqlrunner

import (
	"database/sql"
	"fmt"
	"os"

	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/lager/lagertest"
	"github.com/lib/pq"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

// PostgresRunner is responsible for creating and tearing down a test database in
// a local Postgres instance. This runner assumes mysql is already running
// locally, and does not start or stop the mysql service.  mysql must be set up
// on localhost as described in the CONTRIBUTING.md doc in diego-release.
type PostgresRunner struct {
	logger    lager.Logger
	db        *sql.DB
	sqlDBName string
}

func NewPostgresRunner(sqlDBName string) *PostgresRunner {
	return &PostgresRunner{
		logger:    lagertest.NewTestLogger("postgres-runner"),
		sqlDBName: sqlDBName,
	}
}

func (p *PostgresRunner) Run(signals <-chan os.Signal, ready chan<- struct{}) error {
	defer GinkgoRecover()
	logger := p.logger.Session("run")
	logger.Info("starting")
	defer logger.Info("completed")

	var err error
	p.db, err = sql.Open("postgres", "postgres://diego:diego_pw@localhost")
	Expect(err).NotTo(HaveOccurred())
	Expect(p.db.Ping()).To(Succeed())

	_, err = p.db.Exec(fmt.Sprintf("DROP DATABASE IF EXISTS %s", p.sqlDBName))
	Expect(err).NotTo(HaveOccurred())

	_, err = p.db.Exec(fmt.Sprintf("CREATE DATABASE %s", p.sqlDBName))
	Expect(err).NotTo(HaveOccurred())

	Expect(p.db.Close()).To(Succeed())

	p.db, err = sql.Open("postgres", fmt.Sprintf("postgres://diego:diego_pw@localhost/%s", p.sqlDBName))
	Expect(err).NotTo(HaveOccurred())
	Expect(p.db.Ping()).To(Succeed())

	close(ready)

	<-signals

	// We need to close the connection to the database we want to drop before dropping it.
	Expect(p.db.Close()).To(Succeed())
	p.db, err = sql.Open("postgres", "postgres://diego:diego_pw@localhost")
	Expect(err).NotTo(HaveOccurred())

	_, err = p.db.Exec(fmt.Sprintf("DROP DATABASE %s", p.sqlDBName))
	Expect(err).NotTo(HaveOccurred())
	Expect(p.db.Close()).To(Succeed())

	return nil
}

func (p *PostgresRunner) ConnectionString() string {
	return fmt.Sprintf("postgres://diego:diego_pw@localhost/%s", p.sqlDBName)
}

func (p *PostgresRunner) Port() int {
	return 5432
}

func (p *PostgresRunner) DBName() string {
	return p.sqlDBName
}

func (p *PostgresRunner) DriverName() string {
	return "postgres"
}

func (p *PostgresRunner) Password() string {
	return "diego_pw"
}

func (p *PostgresRunner) Username() string {
	return "diego"
}

func (p *PostgresRunner) DB() *sql.DB {
	return p.db
}

func (p *PostgresRunner) ResetTables(tables []string) {
	logger := p.logger.Session("reset-tables")
	logger.Info("starting")
	defer logger.Info("completed")

	for _, name := range tables {
		query := fmt.Sprintf("TRUNCATE TABLE %s", name)
		result, err := p.db.Exec(query)

		switch err := err.(type) {
		case *pq.Error:
			if err.Code == "42P01" {
				// missing table error, it's fine because we're trying to truncate it
				continue
			}
		}

		Expect(err).NotTo(HaveOccurred())
		Expect(result.RowsAffected()).To(BeEquivalentTo(0))
	}
}

func (p *PostgresRunner) Reset() {
	p.ResetTables([]string{"domains", "configurations", "tasks", "desired_lrps", "actual_lrps", "locks"})
}
