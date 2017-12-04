package migration

import (
	"database/sql"
	"errors"
	"fmt"
	"os"
	"sort"
	"time"

	"code.cloudfoundry.org/bbs/db"
	"code.cloudfoundry.org/bbs/db/etcd"
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
	etcdDB         db.DB
	storeClient    etcd.StoreClient
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
	etcdDB db.DB,
	etcdStoreClient etcd.StoreClient,
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
		etcdDB:         etcdDB,
		storeClient:    etcdStoreClient,
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

	lastETCDMigrationVersion := m.lastETCDMigrationVersion()

	var maxMigrationVersion int64
	if len(m.migrations) > 0 {
		maxMigrationVersion = m.migrations[len(m.migrations)-1].Version()
	}

	version, err := m.resolveStoredVersion(logger)
	if err != nil {
		return err
	}

	if !m.hasSQLConfigured() {
		logger.Info("no-sql-configuration")
		maxMigrationVersion = lastETCDMigrationVersion
	}

	if version == nil {
		if m.hasETCDConfigured() && !m.hasSQLConfigured() {
			logger.Info("fresh-etcd-skipping-migrations")
			err = m.writeVersion(lastETCDMigrationVersion, lastETCDMigrationVersion, lastETCDMigrationVersion)
			if err != nil {
				return err
			}

			m.finish(logger, ready)

			select {
			case <-signals:
				logger.Info("migration-interrupt")
				return nil
			}
		} else if m.hasSQLConfigured() {
			logger.Info("sql-is-configured")
			version = &models.Version{
				CurrentVersion: lastETCDMigrationVersion,
				TargetVersion:  maxMigrationVersion,
			}
			err = m.writeVersion(lastETCDMigrationVersion, maxMigrationVersion, lastETCDMigrationVersion)
			if err != nil {
				return err
			}
		} else {
			err := errors.New("no database configured")
			logger.Error("no-database-configured", err)
			return err
		}
	}

	if version.TargetVersion < version.CurrentVersion {
		return fmt.Errorf(
			"Existing DB target version (%d) exceeds current version (%d)",
			version.TargetVersion,
			version.CurrentVersion,
		)
	}

	if version.CurrentVersion > maxMigrationVersion {
		return fmt.Errorf(
			"Existing DB version (%d) exceeds bbs version (%d)",
			version.CurrentVersion,
			maxMigrationVersion,
		)
	}

	if version.TargetVersion != maxMigrationVersion {
		if version.TargetVersion > maxMigrationVersion {
			version.TargetVersion = maxMigrationVersion
		}

		logger.Info("running-migrations", lager.Data{
			"from_version": version.CurrentVersion,
			"to_version":   maxMigrationVersion,
		})

		m.writeVersion(version.CurrentVersion, maxMigrationVersion, lastETCDMigrationVersion)
	}

	errorChan := make(chan error)
	go m.performMigration(logger, version, maxMigrationVersion, lastETCDMigrationVersion, errorChan, ready)
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
	version *models.Version,
	maxMigrationVersion int64,
	lastETCDMigrationVersion int64,
	errorChan chan error,
	readyChan chan<- struct{},
) {
	migrateStart := m.clock.Now()
	if version.CurrentVersion != maxMigrationVersion {
		lastVersion := version.CurrentVersion
		nextVersion := version.CurrentVersion

		for _, currentMigration := range m.migrations {
			if maxMigrationVersion < currentMigration.Version() {
				break
			}

			if lastVersion < currentMigration.Version() {
				if nextVersion > currentMigration.Version() {
					errorChan <- fmt.Errorf(
						"Existing DB target version (%d) exceeds pending migration version (%d)",
						nextVersion,
						currentMigration.Version(),
					)
				}

				nextVersion = currentMigration.Version()
				logger.Info("running-migration", lager.Data{
					"current_version":   lastVersion,
					"migration_version": nextVersion,
				})

				currentMigration.SetCryptor(m.cryptor)
				if lastVersion <= lastETCDMigrationVersion {
					currentMigration.SetStoreClient(m.storeClient)
				}
				currentMigration.SetRawSQLDB(m.rawSQLDB)
				currentMigration.SetClock(m.clock)
				currentMigration.SetDBFlavor(m.databaseDriver)

				err := currentMigration.Up(m.logger.Session("migration"))
				if err != nil {
					errorChan <- err
					return
				}

				lastVersion = nextVersion
				err = m.writeVersion(lastVersion, maxMigrationVersion, lastETCDMigrationVersion)
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

func (m *Manager) findMaxTargetVersion() (int, int64) {
	if len(m.migrations) > 0 {
		if m.rawSQLDB == nil {
			for i, migration := range m.migrations {
				if migration.RequiresSQL() {
					return i, m.migrations[i-1].Version()
				}
			}
		}
		return len(m.migrations), m.migrations[len(m.migrations)-1].Version()
	}
	return 0, 0
}

func (m *Manager) lastETCDMigrationVersion() int64 {
	if len(m.migrations) > 0 {
		for i, migration := range m.migrations {
			if migration.RequiresSQL() {
				if i == 0 {
					return 0
				}
				return m.migrations[i-1].Version()
			}
		}
		return m.migrations[len(m.migrations)-1].Version()
	}
	return 0
}

// returns nil, nil if no version is found
func (m *Manager) resolveStoredVersion(logger lager.Logger) (*models.Version, error) {
	var (
		version *models.Version
		err     error
	)

	if m.hasSQLConfigured() {
		version, err = m.sqlDB.Version(logger)
		if err == nil {
			return version, nil
		} else if models.ConvertError(err) != models.ErrResourceNotFound {
			return nil, err
		}
	}

	if m.hasETCDConfigured() {
		version, err = m.etcdDB.Version(logger)
		if err != nil {
			if models.ConvertError(err) == models.ErrResourceNotFound {
				return nil, nil // totally fresh deploy
			}
			return nil, err
		}
		return version, nil
	}
	return nil, nil
}

func (m *Manager) writeVersion(currentVersion, targetVersion, lastETCDMigrationVersion int64) error {
	if m.hasSQLConfigured() {
		err := m.sqlDB.SetVersion(m.logger, &models.Version{
			CurrentVersion: currentVersion,
			TargetVersion:  targetVersion,
		})

		if err != nil {
			return err
		}
	}

	if m.hasETCDConfigured() {
		if currentVersion > lastETCDMigrationVersion {
			// make it lastETCDMigration plus 1 to indicate it's past ETCD to SQL
			currentVersion = lastETCDMigrationVersion + 1
		}
		err := m.etcdDB.SetVersion(m.logger, &models.Version{
			CurrentVersion: currentVersion,
			TargetVersion:  targetVersion,
		})
		if err != nil {
			return err
		}
	}

	return nil
}

func (m *Manager) hasETCDConfigured() bool {
	return m.storeClient != nil
}

func (m *Manager) hasSQLConfigured() bool {
	return m.rawSQLDB != nil
}

type Migrations []Migration

func (m Migrations) Len() int           { return len(m) }
func (m Migrations) Swap(i, j int)      { m[i], m[j] = m[j], m[i] }
func (m Migrations) Less(i, j int) bool { return m[i].Version() < m[j].Version() }
