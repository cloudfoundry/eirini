package migration_test

import (
	"database/sql"
	"errors"

	"code.cloudfoundry.org/bbs/db/dbfakes"
	"code.cloudfoundry.org/bbs/db/etcd"
	"code.cloudfoundry.org/bbs/encryption"
	"code.cloudfoundry.org/bbs/encryption/encryptionfakes"
	"code.cloudfoundry.org/bbs/migration"
	"code.cloudfoundry.org/bbs/migration/migrationfakes"
	"code.cloudfoundry.org/bbs/models"
	"code.cloudfoundry.org/clock"
	mfakes "code.cloudfoundry.org/diego-logging-client/testhelpers"
	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/lager/lagertest"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/tedsuo/ifrit"
	"github.com/tedsuo/ifrit/ginkgomon"
)

var _ = Describe("Migration Manager", func() {
	var (
		manager          ifrit.Runner
		migrationProcess ifrit.Process

		logger          *lagertest.TestLogger
		fakeETCDDB      *dbfakes.FakeDB
		etcdStoreClient etcd.StoreClient

		fakeSQLDB *dbfakes.FakeDB
		rawSQLDB  *sql.DB

		migrations []migration.Migration

		migrationsDone chan struct{}

		dbVersion     *models.Version
		fakeMigration *migrationfakes.FakeMigration

		cryptor encryption.Cryptor

		fakeMetronClient *mfakes.FakeIngressClient
	)

	BeforeEach(func() {
		migrationsDone = make(chan struct{})

		fakeMetronClient = new(mfakes.FakeIngressClient)

		dbVersion = &models.Version{}

		logger = lagertest.NewTestLogger("test")
		fakeETCDDB = &dbfakes.FakeDB{}
		fakeETCDDB.VersionReturns(dbVersion, nil)

		fakeSQLDB = &dbfakes.FakeDB{}

		cryptor = &encryptionfakes.FakeCryptor{}

		fakeMigration = &migrationfakes.FakeMigration{}
		fakeMigration.RequiresSQLReturns(false)
		migrations = []migration.Migration{fakeMigration}
	})

	JustBeforeEach(func() {
		manager = migration.NewManager(logger, fakeETCDDB, etcdStoreClient, fakeSQLDB, rawSQLDB, cryptor, migrations, migrationsDone, clock.NewClock(), "db-driver", fakeMetronClient)
		migrationProcess = ifrit.Background(manager)
	})

	AfterEach(func() {
		ginkgomon.Kill(migrationProcess)
	})

	Context("when both a etcd and sql configurations are present", func() {
		BeforeEach(func() {
			rawSQLDB = &sql.DB{}
			etcdStoreClient = etcd.NewStoreClient(nil)
		})

		Context("but SQL does not have a version", func() {
			BeforeEach(func() {
				fakeSQLDB.VersionReturns(nil, models.ErrResourceNotFound)
			})

			It("fetches the version from etcd", func() {
				Eventually(fakeSQLDB.VersionCallCount).Should(Equal(1))
				Consistently(fakeSQLDB.VersionCallCount).Should(Equal(1))

				Eventually(fakeETCDDB.VersionCallCount).Should(Equal(1))
				Consistently(fakeETCDDB.VersionCallCount).Should(Equal(1))

				ginkgomon.Interrupt(migrationProcess)
				Eventually(migrationProcess.Wait()).Should(Receive(BeNil()))
			})

			// cross-db migration
			Context("but etcd does", func() {
				var fakeMigrationToSQL *migrationfakes.FakeMigration

				BeforeEach(func() {
					fakeMigrationToSQL = &migrationfakes.FakeMigration{}
					fakeMigrationToSQL.VersionReturns(102)
					fakeMigrationToSQL.RequiresSQLReturns(true)

					dbVersion.CurrentVersion = 99
					dbVersion.TargetVersion = 99
					fakeMigration.VersionReturns(100)

					migrations = []migration.Migration{fakeMigrationToSQL, fakeMigration}
				})

				It("sorts all the migrations and runs them", func() {
					Eventually(migrationProcess.Ready()).Should(BeClosed())
					Expect(migrationsDone).To(BeClosed())
					Expect(fakeETCDDB.SetVersionCallCount()).To(Equal(3))

					_, version := fakeETCDDB.SetVersionArgsForCall(0)
					Expect(version).To(Equal(&models.Version{CurrentVersion: 99, TargetVersion: 102}))

					_, version = fakeETCDDB.SetVersionArgsForCall(1)
					Expect(version).To(Equal(&models.Version{CurrentVersion: 100, TargetVersion: 102}))

					_, version = fakeETCDDB.SetVersionArgsForCall(2)
					// Current Version set to last ETCD migration plus 1
					Expect(version).To(Equal(&models.Version{CurrentVersion: 101, TargetVersion: 102}))

					Expect(fakeSQLDB.SetVersionCallCount()).To(Equal(3))
					_, version = fakeSQLDB.SetVersionArgsForCall(0)
					Expect(version).To(Equal(&models.Version{CurrentVersion: 99, TargetVersion: 102}))

					_, version = fakeSQLDB.SetVersionArgsForCall(1)
					Expect(version).To(Equal(&models.Version{CurrentVersion: 100, TargetVersion: 102}))

					_, version = fakeSQLDB.SetVersionArgsForCall(2)
					Expect(version).To(Equal(&models.Version{CurrentVersion: 102, TargetVersion: 102}))

					Expect(fakeMigration.UpCallCount()).To(Equal(1))
					Expect(fakeMigrationToSQL.UpCallCount()).To(Equal(1))

					Expect(fakeMigration.DownCallCount()).To(Equal(0))
					Expect(fakeMigrationToSQL.DownCallCount()).To(Equal(0))
				})

				It("sets the raw SQL db and the storeClient on the migration to SQL", func() {
					Eventually(migrationProcess.Ready()).Should(BeClosed())
					Expect(migrationsDone).To(BeClosed())
					Expect(fakeMigrationToSQL.SetRawSQLDBCallCount()).To(Equal(1))
					Expect(fakeMigrationToSQL.SetStoreClientCallCount()).To(Equal(1))
				})
			})

			Context("etcd to sql has already been run", func() {
				var fakeMigrationToSQL *migrationfakes.FakeMigration

				BeforeEach(func() {
					fakeMigrationToSQL = &migrationfakes.FakeMigration{}
					fakeMigrationToSQL.VersionReturns(103)
					fakeMigrationToSQL.RequiresSQLReturns(true)

					// Current Version is 1 more than the last ETCD Migration (99)
					dbVersion.CurrentVersion = 100
					dbVersion.TargetVersion = 103
					fakeMigration.VersionReturns(99)

					migrations = []migration.Migration{fakeMigrationToSQL, fakeMigration}
				})

				It("sorts all the migrations and runs them", func() {
					Eventually(migrationProcess.Ready()).Should(BeClosed())
					Expect(migrationsDone).To(BeClosed())
					Expect(fakeETCDDB.SetVersionCallCount()).To(Equal(1))

					Expect(fakeMigration.UpCallCount()).To(Equal(0))
					Expect(fakeMigrationToSQL.UpCallCount()).To(Equal(1))

					Expect(fakeMigration.DownCallCount()).To(Equal(0))
					Expect(fakeMigrationToSQL.DownCallCount()).To(Equal(0))
				})

				It("sets the raw SQL db and the storeClient on the migration to SQL", func() {
					Eventually(migrationProcess.Ready()).Should(BeClosed())
					Expect(migrationsDone).To(BeClosed())
					Expect(fakeMigrationToSQL.SetRawSQLDBCallCount()).To(Equal(1))
					Expect(fakeMigrationToSQL.SetStoreClientCallCount()).To(Equal(0))
				})
			})

			// fresh sql bbs
			Context("and neither does etcd", func() {
				BeforeEach(func() {
					fakeETCDDB.VersionReturns(nil, models.ErrResourceNotFound)
					fakeMigration.VersionReturns(101)
				})

				It("creates versions in both backends and doesn't run any migrations", func() {
					Eventually(migrationProcess.Ready()).Should(BeClosed())
					Expect(migrationsDone).To(BeClosed())

					Expect(fakeETCDDB.SetVersionCallCount()).To(Equal(1))
					_, version := fakeETCDDB.SetVersionArgsForCall(0)
					Expect(version.CurrentVersion).To(BeEquivalentTo(101))
					Expect(version.TargetVersion).To(BeEquivalentTo(101))

					Expect(fakeSQLDB.SetVersionCallCount()).To(Equal(1))
					_, version = fakeSQLDB.SetVersionArgsForCall(0)
					Expect(version.CurrentVersion).To(BeEquivalentTo(101))
					Expect(version.TargetVersion).To(BeEquivalentTo(101))

					Expect(fakeMigration.UpCallCount()).To(Equal(0))
				})
			})
		})

		// already on sql
		Context("and SQL has a version", func() {
			BeforeEach(func() {
				fakeSQLDB.VersionReturns(dbVersion, nil)

				dbVersion.CurrentVersion = 99
				dbVersion.TargetVersion = 99
				fakeMigration.VersionReturns(100)
				fakeMigration.RequiresSQLReturns(true)
			})

			It("ignores etcd entirely and uses SQL's stored version", func() {
				Eventually(fakeSQLDB.VersionCallCount).Should(Equal(1))
				Consistently(fakeSQLDB.VersionCallCount).Should(Equal(1))

				Eventually(fakeETCDDB.VersionCallCount).Should(Equal(0))
				Consistently(fakeETCDDB.VersionCallCount).Should(Equal(0))
			})

			It("runs migrations", func() {
				Eventually(migrationProcess.Ready()).Should(BeClosed())
				Expect(migrationsDone).To(BeClosed())

				Expect(fakeSQLDB.SetVersionCallCount()).To(Equal(2))
				Expect(fakeMigration.UpCallCount()).To(Equal(1))
				Expect(fakeMigration.SetRawSQLDBCallCount()).To(Equal(1))
			})
		})
	})

	Context("when there's only sql configuration present", func() {
		BeforeEach(func() {
			etcdStoreClient = nil
			rawSQLDB = &sql.DB{}
		})

		It("fetches the stored version from sql", func() {
			Eventually(fakeSQLDB.VersionCallCount).Should(Equal(1))
			Consistently(fakeSQLDB.VersionCallCount).Should(Equal(1))

			Eventually(fakeETCDDB.VersionCallCount).Should(Equal(0))
			Consistently(fakeETCDDB.VersionCallCount).Should(Equal(0))

			ginkgomon.Interrupt(migrationProcess)
			Eventually(migrationProcess.Wait()).Should(Receive(BeNil()))
		})

		Context("when there is no version", func() {
			var (
				fakeMigrationToSQL   *migrationfakes.FakeMigration
				fakeSQLOnlyMigration *migrationfakes.FakeMigration
			)

			BeforeEach(func() {
				fakeSQLDB.VersionReturns(nil, models.ErrResourceNotFound)
				fakeMigration.VersionReturns(99)

				fakeMigrationToSQL = &migrationfakes.FakeMigration{}
				fakeMigrationToSQL.VersionReturns(100)
				fakeMigrationToSQL.RequiresSQLReturns(true)

				fakeSQLOnlyMigration = &migrationfakes.FakeMigration{}
				fakeSQLOnlyMigration.VersionReturns(101)
				fakeSQLOnlyMigration.RequiresSQLReturns(true)

				migrations = []migration.Migration{fakeMigrationToSQL, fakeMigration, fakeSQLOnlyMigration}
			})

			It("creates a version table and seeds it with the lowest sql-requiring version", func() {
				Eventually(fakeSQLDB.SetVersionCallCount).Should(Equal(3))
				Consistently(fakeETCDDB.SetVersionCallCount).Should(Equal(0))

				_, version := fakeSQLDB.SetVersionArgsForCall(0)
				Expect(version.CurrentVersion).To(BeEquivalentTo(99))
				Expect(version.TargetVersion).To(BeEquivalentTo(101))

				_, version = fakeSQLDB.SetVersionArgsForCall(1)
				Expect(version.CurrentVersion).To(BeEquivalentTo(100))
				Expect(version.TargetVersion).To(BeEquivalentTo(101))

				_, version = fakeSQLDB.SetVersionArgsForCall(2)
				Expect(version.CurrentVersion).To(BeEquivalentTo(101))
				Expect(version.TargetVersion).To(BeEquivalentTo(101))

				Expect(fakeMigration.UpCallCount()).To(Equal(0))
				Expect(fakeMigrationToSQL.UpCallCount()).To(Equal(1))
				Expect(fakeSQLOnlyMigration.UpCallCount()).To(Equal(1))
			})
		})

		Context("and SQL has a version", func() {
			BeforeEach(func() {
				fakeSQLDB.VersionReturns(dbVersion, nil)

				dbVersion.CurrentVersion = 99
				dbVersion.TargetVersion = 99
				fakeMigration.VersionReturns(100)
				fakeMigration.RequiresSQLReturns(true)
			})

			It("runs migrations", func() {
				Eventually(migrationProcess.Ready()).Should(BeClosed())
				Expect(migrationsDone).To(BeClosed())

				Expect(fakeSQLDB.SetVersionCallCount()).To(Equal(2))
				Expect(fakeMigration.UpCallCount()).To(Equal(1))
				Expect(fakeMigration.SetRawSQLDBCallCount()).To(Equal(1))
			})

			Context("and there are more than one migrations", func() {
				var fakeMigration2 *migrationfakes.FakeMigration

				BeforeEach(func() {
					fakeMigration2 = &migrationfakes.FakeMigration{}
					migrations = append(migrations, fakeMigration2)

					fakeMigration2.VersionReturns(101)
					fakeMigration2.RequiresSQLReturns(true)
				})

				It("runs migrations", func() {
					Eventually(migrationProcess.Ready()).Should(BeClosed())
					Expect(migrationsDone).To(BeClosed())

					Expect(fakeSQLDB.SetVersionCallCount()).To(Equal(3))
					_, v1 := fakeSQLDB.SetVersionArgsForCall(0)
					_, v2 := fakeSQLDB.SetVersionArgsForCall(1)
					_, v3 := fakeSQLDB.SetVersionArgsForCall(2)
					Expect(v1).To(Equal(&models.Version{
						CurrentVersion: 99,
						TargetVersion:  101,
					}))
					Expect(v2).To(Equal(&models.Version{
						CurrentVersion: 100,
						TargetVersion:  101,
					}))
					Expect(v3).To(Equal(&models.Version{
						CurrentVersion: 101,
						TargetVersion:  101,
					}))

					Expect(fakeMigration.UpCallCount()).To(Equal(1))
					Expect(fakeMigration2.UpCallCount()).To(Equal(1))
					Expect(fakeMigration.SetRawSQLDBCallCount()).To(Equal(1))
					Expect(fakeMigration2.SetRawSQLDBCallCount()).To(Equal(1))
				})
			})
		})
	})

	Context("when there's only etcd configuration present", func() {
		BeforeEach(func() {
			rawSQLDB = nil
			etcdStoreClient = etcd.NewStoreClient(nil)
		})

		It("fetches the stored version from etcd", func() {
			Eventually(fakeETCDDB.VersionCallCount).Should(Equal(1))
			Consistently(fakeETCDDB.VersionCallCount).Should(Equal(1))

			Eventually(fakeSQLDB.VersionCallCount).Should(Equal(0))
			Consistently(fakeSQLDB.VersionCallCount).Should(Equal(0))

			ginkgomon.Interrupt(migrationProcess)
			Eventually(migrationProcess.Wait()).Should(Receive(BeNil()))
		})

		Context("when there is no version", func() {
			BeforeEach(func() {
				fakeETCDDB.VersionReturns(nil, models.ErrResourceNotFound)
				fakeMigration.VersionReturns(9)
			})

			It("creates a version with the correct target version and does not run any migrations", func() {
				Eventually(fakeETCDDB.SetVersionCallCount).Should(Equal(1))

				_, version := fakeETCDDB.SetVersionArgsForCall(0)
				Expect(version.CurrentVersion).To(BeEquivalentTo(9))
				Expect(version.TargetVersion).To(BeEquivalentTo(9))

				Expect(fakeMigration.UpCallCount()).To(Equal(0))
			})
		})

		Context("when fetching the version fails", func() {
			BeforeEach(func() {
				fakeETCDDB.VersionReturns(nil, errors.New("kablamo"))
			})

			It("fails early", func() {
				var err error
				Eventually(migrationProcess.Wait()).Should(Receive(&err))
				Expect(err).To(MatchError("kablamo"))
				Expect(migrationProcess.Ready()).ToNot(BeClosed())
				Expect(migrationsDone).NotTo(BeClosed())
			})
		})

		Context("when the current version is newer than bbs migration version", func() {
			BeforeEach(func() {
				dbVersion.CurrentVersion = 100
				dbVersion.TargetVersion = 100
				fakeMigration.VersionReturns(99)
			})

			It("shuts down without signalling ready", func() {
				var err error
				Eventually(migrationProcess.Wait()).Should(Receive(&err))
				Expect(err).To(MatchError("Existing DB version (100) exceeds bbs version (99)"))
				Expect(migrationProcess.Ready()).ToNot(BeClosed())
				Expect(migrationsDone).NotTo(BeClosed())
			})
		})

		Context("when the current version is the same as the bbs migration version", func() {
			BeforeEach(func() {
				dbVersion.CurrentVersion = 100
				dbVersion.TargetVersion = 100
				fakeMigration.VersionReturns(100)
			})

			It("signals ready and does not change the version", func() {
				Eventually(migrationProcess.Ready()).Should(BeClosed())
				Expect(migrationsDone).To(BeClosed())
				Consistently(fakeETCDDB.SetVersionCallCount).Should(Equal(0))
			})

			Context("and the target version is greater than the bbs migration version", func() {
				BeforeEach(func() {
					dbVersion.TargetVersion = 101
				})

				It("sets the target version to the current version and signals ready", func() {
					Eventually(migrationProcess.Ready()).Should(BeClosed())
					Expect(migrationsDone).To(BeClosed())

					Eventually(fakeETCDDB.SetVersionCallCount).Should(Equal(1))

					_, version := fakeETCDDB.SetVersionArgsForCall(0)
					Expect(version.CurrentVersion).To(BeEquivalentTo(100))
					Expect(version.TargetVersion).To(BeEquivalentTo(100))
				})
			})

			Context("and the target version is less than the bbs migration version", func() {
				BeforeEach(func() {
					dbVersion.TargetVersion = 99
				})

				It("shuts down without signalling ready", func() {
					var err error
					Eventually(migrationProcess.Wait()).Should(Receive(&err))
					Expect(err).To(MatchError("Existing DB target version (99) exceeds current version (100)"))
					Expect(migrationProcess.Ready()).ToNot(BeClosed())
					Expect(migrationsDone).ToNot(BeClosed())
				})
			})
		})

		Context("when the current version is older than the maximum migration version", func() {
			var fakeMigration102 *migrationfakes.FakeMigration

			BeforeEach(func() {
				fakeMigration102 = &migrationfakes.FakeMigration{}
				fakeMigration102.VersionReturns(102)

				dbVersion.CurrentVersion = 99
				dbVersion.TargetVersion = 99
				fakeMigration.VersionReturns(100)

				migrations = []migration.Migration{fakeMigration102, fakeMigration}
			})

			Describe("reporting", func() {
				It("reports the duration that it took to migrate", func() {
					Eventually(migrationProcess.Ready()).Should(BeClosed())
					Expect(migrationsDone).To(BeClosed())

					Expect(fakeMetronClient.SendDurationCallCount()).To(Equal(1))
					name, value := fakeMetronClient.SendDurationArgsForCall(0)
					Expect(name).To(Equal("MigrationDuration"))
					Expect(value).NotTo(BeZero())
				})
			})

			It("it sorts the migrations and runs them sequentially", func() {
				Eventually(migrationProcess.Ready()).Should(BeClosed())
				Expect(migrationsDone).To(BeClosed())
				Consistently(fakeETCDDB.SetVersionCallCount).Should(Equal(3))

				_, version := fakeETCDDB.SetVersionArgsForCall(0)
				Expect(version).To(Equal(&models.Version{CurrentVersion: 99, TargetVersion: 102}))

				_, version = fakeETCDDB.SetVersionArgsForCall(1)
				Expect(version).To(Equal(&models.Version{CurrentVersion: 100, TargetVersion: 102}))

				_, version = fakeETCDDB.SetVersionArgsForCall(2)
				Expect(version).To(Equal(&models.Version{CurrentVersion: 102, TargetVersion: 102}))

				Expect(fakeMigration.UpCallCount()).To(Equal(1))
				Expect(fakeMigration102.UpCallCount()).To(Equal(1))

				Expect(fakeMigration.DownCallCount()).To(Equal(0))
				Expect(fakeMigration102.DownCallCount()).To(Equal(0))
			})

			Describe("and one of the migrations takes a long time", func() {
				var longMigrationExitChan chan struct{}

				BeforeEach(func() {
					longMigrationExitChan = make(chan struct{}, 1)
					longMigration := &migrationfakes.FakeMigration{}
					longMigration.UpStub = func(logger lager.Logger) error {
						<-longMigrationExitChan
						return nil
					}
					longMigration.VersionReturns(103)
					migrations = []migration.Migration{longMigration}
				})

				AfterEach(func() {
					longMigrationExitChan <- struct{}{}
				})

				It("should not close the channel until the migration finishes", func() {
					Consistently(migrationProcess.Ready()).ShouldNot(BeClosed())
				})

				Context("when the migration finishes", func() {
					JustBeforeEach(func() {
						Eventually(longMigrationExitChan).Should(BeSent(struct{}{}))
					})

					It("should close the ready channel", func() {
						Eventually(migrationProcess.Ready()).Should(BeClosed())
					})
				})

				Context("when interrupted", func() {
					JustBeforeEach(func() {
						ginkgomon.Interrupt(migrationProcess)
					})

					It("exits and does not wait for the migration to finish", func() {
						Eventually(migrationProcess.Wait()).Should(Receive())
					})
				})
			})

			It("sets the store client on the migration", func() {
				Eventually(migrationProcess.Ready()).Should(BeClosed())
				Expect(migrationsDone).To(BeClosed())
				Expect(fakeMigration.SetStoreClientCallCount()).To(Equal(1))
				actualStoreClient := fakeMigration.SetStoreClientArgsForCall(0)
				Expect(actualStoreClient).To(Equal(etcdStoreClient))
			})

			It("sets the cryptor on the migration", func() {
				Eventually(migrationProcess.Ready()).Should(BeClosed())
				Expect(migrationsDone).To(BeClosed())
				Expect(fakeMigration.SetCryptorCallCount()).To(Equal(1))
				actualCryptor := fakeMigration.SetCryptorArgsForCall(0)
				Expect(actualCryptor).To(Equal(cryptor))
			})

			Context("when there's a migration that requires SQL", func() {
				var fakeMigrationToSQL *migrationfakes.FakeMigration

				BeforeEach(func() {
					fakeMigrationToSQL = &migrationfakes.FakeMigration{}
					fakeMigrationToSQL.VersionReturns(102)
					fakeMigrationToSQL.RequiresSQLReturns(true)

					dbVersion.CurrentVersion = 99
					dbVersion.TargetVersion = 99
					fakeMigration.VersionReturns(100)

					migrations = []migration.Migration{fakeMigrationToSQL, fakeMigration}
				})

				It("Does not attempt to run that migration, and only upgrades to the max etcd version", func() {
					Eventually(migrationProcess.Ready()).Should(BeClosed())
					Expect(migrationsDone).To(BeClosed())
					Consistently(fakeETCDDB.SetVersionCallCount).Should(Equal(2))

					_, version := fakeETCDDB.SetVersionArgsForCall(0)
					Expect(version).To(Equal(&models.Version{CurrentVersion: 99, TargetVersion: 100}))

					_, version = fakeETCDDB.SetVersionArgsForCall(1)
					Expect(version).To(Equal(&models.Version{CurrentVersion: 100, TargetVersion: 100}))

					Consistently(fakeSQLDB.SetVersionCallCount).Should(Equal(0))
					Expect(fakeMigrationToSQL.UpCallCount()).To(Equal(0))
				})
			})

			Context("when the db's target version is greater than the maximum known migration version", func() {
				BeforeEach(func() {
					dbVersion.TargetVersion = 103
				})

				It("runs the migrations up to the maximum known migration version", func() {
					Eventually(migrationProcess.Ready()).Should(BeClosed())
					Expect(migrationsDone).To(BeClosed())
					Consistently(fakeETCDDB.SetVersionCallCount).Should(Equal(3))

					_, version := fakeETCDDB.SetVersionArgsForCall(0)
					Expect(version).To(Equal(&models.Version{CurrentVersion: 99, TargetVersion: 102}))

					_, version = fakeETCDDB.SetVersionArgsForCall(1)
					Expect(version).To(Equal(&models.Version{CurrentVersion: 100, TargetVersion: 102}))

					_, version = fakeETCDDB.SetVersionArgsForCall(2)
					Expect(version).To(Equal(&models.Version{CurrentVersion: 102, TargetVersion: 102}))

					Expect(fakeMigration.UpCallCount()).To(Equal(1))
					Expect(fakeMigration102.UpCallCount()).To(Equal(1))

					Expect(fakeMigration.DownCallCount()).To(Equal(0))
					Expect(fakeMigration102.DownCallCount()).To(Equal(0))
				})
			})
		})

		Context("when there are no migrations", func() {
			BeforeEach(func() {
				migrations = []migration.Migration{}
			})

			Context("and there is an existing version", func() {
				BeforeEach(func() {
					dbVersion.CurrentVersion = 100
					dbVersion.TargetVersion = 100
				})

				It("treats the bbs migration version as 0", func() {
					var err error
					Eventually(migrationProcess.Wait()).Should(Receive(&err))
					Expect(err).To(MatchError("Existing DB version (100) exceeds bbs version (0)"))
					Expect(migrationProcess.Ready()).ToNot(BeClosed())
				})
			})

			Context("and there is no existing version", func() {
				BeforeEach(func() {
					fakeETCDDB.VersionReturns(nil, models.ErrResourceNotFound)
				})

				It("writes a zero version into the db", func() {
					Eventually(fakeETCDDB.SetVersionCallCount).Should(Equal(1))

					_, version := fakeETCDDB.SetVersionArgsForCall(0)
					Expect(version.CurrentVersion).To(BeEquivalentTo(0))
					Expect(version.CurrentVersion).To(BeEquivalentTo(0))
					Expect(version.TargetVersion).To(BeEquivalentTo(0))
				})
			})
		})
	})
})
