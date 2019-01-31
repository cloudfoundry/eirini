package migration

import (
	"database/sql"
	"errors"
	"fmt"
	"os"
	"sort"
	"time"

	"code.cloudfoundry.org/bbs/db"
	"code.cloudfoundry.org/bbs/encryption"
	"code.cloudfoundry.org/bbs/models"
	"code.cloudfoundry.org/clock"
	loggingclient "code.cloudfoundry.org/diego-logging-client"
	"code.cloudfoundry.org/lager"
)

const (
	migrationDuration = "MigrationDuration"
)

type Manager struct {
	logger         lager.Logger
	sqlDB          db.DB
	rawSQLDB       *sql.DB
	cryptor        encryption.Cryptor
	migrations     []Migration
	migrationsDone chan<- struct{}
	clock          clock.Clock
	databaseDriver string
	metronClient   loggingclient.IngressClient
}

func NewManager(
	logger lager.Logger,
	sqlDB db.DB,
	rawSQLDB *sql.DB,
	cryptor encryption.Cryptor,
	migrations Migrations,
	migrationsDone chan<- struct{},
	clock clock.Clock,
	databaseDriver string,
	metronClient loggingclient.IngressClient,
) Manager {
	sort.Sort(migrations)

	return Manager{
		logger:         logger,
		sqlDB:          sqlDB,
		rawSQLDB:       rawSQLDB,
		cryptor:        cryptor,
		migrations:     migrations,
		migrationsDone: migrationsDone,
		clock:          clock,
		databaseDriver: databaseDriver,
		metronClient:   metronClient,
	}
}

func (m Manager) Run(signals <-chan os.Signal, ready chan<- struct{}) error {
	logger := m.logger.Session("migration-manager")
	logger.Info("starting")

	if m.rawSQLDB == nil {
		err := errors.New("no database configured")
		logger.Error("no-database-configured", err)
		return err
	}

	var maxMigrationVersion int64
	if len(m.migrations) > 0 {
		maxMigrationVersion = m.migrations[len(m.migrations)-1].Version()
	}

	version, err := m.resolveStoredVersion(logger)
	if err == models.ErrResourceNotFound {
		err = m.writeVersion(0)
		if err != nil {
			return err
		}
	} else if err != nil {
		return err
	}

	if version > maxMigrationVersion {
		return fmt.Errorf(
			"Existing DB version (%d) exceeds bbs version (%d)",
			version,
			maxMigrationVersion,
		)
	}

	errorChan := make(chan error)
	go m.performMigration(logger, version, maxMigrationVersion, errorChan, ready)
	defer logger.Info("exited")

	select {
	case err := <-errorChan:
		logger.Error("migration-failed", err)
		return err
	case <-signals:
		logger.Info("migration-interrupt")
		return nil
	}
}

func (m *Manager) performMigration(
	logger lager.Logger,
	version int64,
	maxMigrationVersion int64,
	errorChan chan error,
	readyChan chan<- struct{},
) {
	migrateStart := m.clock.Now()
	if version != maxMigrationVersion {
		lastVersion := version

		for _, currentMigration := range m.migrations {
			if maxMigrationVersion < currentMigration.Version() {
				break
			}

			if lastVersion < currentMigration.Version() {
				nextVersion := currentMigration.Version()
				logger.Info("running-migration", lager.Data{
					"current_version":   lastVersion,
					"migration_version": nextVersion,
				})

				currentMigration.SetCryptor(m.cryptor)
				currentMigration.SetRawSQLDB(m.rawSQLDB)
				currentMigration.SetClock(m.clock)
				currentMigration.SetDBFlavor(m.databaseDriver)

				err := currentMigration.Up(m.logger.Session("migration"))
				if err != nil {
					errorChan <- err
					return
				}

				lastVersion = nextVersion
				err = m.writeVersion(lastVersion)
				if err != nil {
					errorChan <- err
					return
				}
				logger.Info("completed-migration", lager.Data{
					"current_version": lastVersion,
					"target_version":  maxMigrationVersion,
				})
			}
		}
	}

	logger.Debug("migrations-finished")

	err := m.metronClient.SendDuration(migrationDuration, time.Since(migrateStart))
	if err != nil {
		logger.Error("failed-to-send-migration-duration-metric", err)
	}

	m.finish(logger, readyChan)
}

func (m *Manager) finish(logger lager.Logger, ready chan<- struct{}) {
	close(ready)
	close(m.migrationsDone)
	logger.Info("finished-migrations")
}

func (m *Manager) resolveStoredVersion(logger lager.Logger) (int64, error) {
	version, err := m.sqlDB.Version(logger)
	if err != nil {
		return -1, err
	}
	return version.CurrentVersion, nil
}

func (m *Manager) writeVersion(currentVersion int64) error {
	return m.sqlDB.SetVersion(m.logger, &models.Version{
		CurrentVersion: currentVersion,
	})
}

type Migrations []Migration

func (m Migrations) Len() int           { return len(m) }
func (m Migrations) Swap(i, j int)      { m[i], m[j] = m[j], m[i] }
func (m Migrations) Less(i, j int) bool { return m[i].Version() < m[j].Version() }
