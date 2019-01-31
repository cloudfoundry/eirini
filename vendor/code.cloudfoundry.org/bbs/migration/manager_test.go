package migration_test

import (
	"database/sql"
	"errors"

	"code.cloudfoundry.org/bbs/db/dbfakes"
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

		logger *lagertest.TestLogger

		fakeSQLDB *dbfakes.FakeDB
		rawSQLDB  *sql.DB

		migrations []migration.Migration

		migrationsDone chan struct{}

		fakeMigration *migrationfakes.FakeMigration

		cryptor encryption.Cryptor

		fakeMetronClient *mfakes.FakeIngressClient
	)

	BeforeEach(func() {
		migrationsDone = make(chan struct{})

		fakeMetronClient = new(mfakes.FakeIngressClient)

		logger = lagertest.NewTestLogger("test")

		fakeSQLDB = &dbfakes.FakeDB{}

		cryptor = &encryptionfakes.FakeCryptor{}

		fakeMigration = &migrationfakes.FakeMigration{}
		migrations = []migration.Migration{fakeMigration}
	})

	JustBeforeEach(func() {
		manager = migration.NewManager(logger, fakeSQLDB, rawSQLDB, cryptor, migrations, migrationsDone, clock.NewClock(), "db-driver", fakeMetronClient)
		migrationProcess = ifrit.Background(manager)
	})

	AfterEach(func() {
		ginkgomon.Kill(migrationProcess)
	})

	Context("when configured with a SQL database", func() {
		BeforeEach(func() {
			rawSQLDB = &sql.DB{}
			fakeSQLDB.VersionReturns(&models.Version{}, nil)
		})

		It("fetches the stored version from sql", func() {
			Eventually(fakeSQLDB.VersionCallCount).Should(Equal(1))
			Consistently(fakeSQLDB.VersionCallCount).Should(Equal(1))

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

				fakeMigrationToSQL = &migrationfakes.FakeMigration{}
				fakeMigrationToSQL.VersionReturns(100)

				fakeSQLOnlyMigration = &migrationfakes.FakeMigration{}
				fakeSQLOnlyMigration.VersionReturns(101)

				migrations = []migration.Migration{fakeSQLOnlyMigration, fakeMigrationToSQL}
			})

			It("runs all the migrations in the correct order and sets the version to the latest migration version", func() {
				Eventually(fakeSQLDB.SetVersionCallCount).Should(Equal(3))

				_, version := fakeSQLDB.SetVersionArgsForCall(0)
				Expect(version.CurrentVersion).To(BeEquivalentTo(0))

				_, version = fakeSQLDB.SetVersionArgsForCall(1)
				Expect(version.CurrentVersion).To(BeEquivalentTo(100))

				_, version = fakeSQLDB.SetVersionArgsForCall(2)
				Expect(version.CurrentVersion).To(BeEquivalentTo(101))

				Expect(fakeMigrationToSQL.UpCallCount()).To(Equal(1))
				Expect(fakeSQLOnlyMigration.UpCallCount()).To(Equal(1))
			})
		})

		Context("when fetching the version fails", func() {
			BeforeEach(func() {
				fakeSQLDB.VersionReturns(nil, errors.New("kablamo"))
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
				fakeSQLDB.VersionReturns(&models.Version{CurrentVersion: 100}, nil)
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
				fakeSQLDB.VersionReturns(&models.Version{CurrentVersion: 100}, nil)
				fakeMigration.VersionReturns(100)
			})

			It("signals ready and does not change the version", func() {
				Eventually(migrationProcess.Ready()).Should(BeClosed())
				Expect(migrationsDone).To(BeClosed())
				Consistently(fakeSQLDB.SetVersionCallCount).Should(Equal(0))
			})
		})

		Context("when the current version is older than the maximum migration version", func() {
			var fakeMigration102 *migrationfakes.FakeMigration

			BeforeEach(func() {
				fakeMigration102 = &migrationfakes.FakeMigration{}
				fakeMigration102.VersionReturns(102)

				fakeSQLDB.VersionReturns(&models.Version{CurrentVersion: 99}, nil)
				fakeMigration.VersionReturns(100)

				migrations = []migration.Migration{fakeMigration102, fakeMigration}
			})

			Describe("reporting", func() {
				It("reports the duration that it took to migrate", func() {
					Eventually(migrationProcess.Ready()).Should(BeClosed())
					Expect(migrationsDone).To(BeClosed())

					Expect(fakeMetronClient.SendDurationCallCount()).To(Equal(1))
					name, value, _ := fakeMetronClient.SendDurationArgsForCall(0)
					Expect(name).To(Equal("MigrationDuration"))
					Expect(value).NotTo(BeZero())
				})
			})

			It("sorts the migrations and runs them sequentially", func() {
				Eventually(migrationProcess.Ready()).Should(BeClosed())
				Expect(migrationsDone).To(BeClosed())
				Consistently(fakeSQLDB.SetVersionCallCount).Should(Equal(2))

				_, version := fakeSQLDB.SetVersionArgsForCall(0)
				Expect(version).To(Equal(&models.Version{CurrentVersion: 100}))

				_, version = fakeSQLDB.SetVersionArgsForCall(1)
				Expect(version).To(Equal(&models.Version{CurrentVersion: 102}))

				Expect(fakeMigration.UpCallCount()).To(Equal(1))
				Expect(fakeMigration102.UpCallCount()).To(Equal(1))
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

			It("sets the cryptor on the migration", func() {
				Eventually(migrationProcess.Ready()).Should(BeClosed())
				Expect(migrationsDone).To(BeClosed())
				Expect(fakeMigration.SetCryptorCallCount()).To(Equal(1))
				actualCryptor := fakeMigration.SetCryptorArgsForCall(0)
				Expect(actualCryptor).To(Equal(cryptor))
			})
		})

		Context("when there are no migrations", func() {
			BeforeEach(func() {
				migrations = []migration.Migration{}
			})

			Context("and there is an existing version", func() {
				BeforeEach(func() {
					fakeSQLDB.VersionReturns(&models.Version{CurrentVersion: 100}, nil)
				})

				It("treats the bbs migration version as 0", func() {
					var err error
					Eventually(migrationProcess.Wait()).Should(Receive(&err))
					Expect(err).To(MatchError("Existing DB version (100) exceeds bbs version (0)"))
					Expect(migrationProcess.Ready()).ToNot(BeClosed())
				})
			})

			Context("and there is an existing version 0", func() {
				BeforeEach(func() {
					fakeSQLDB.VersionReturns(&models.Version{CurrentVersion: 0}, nil)
				})

				It("it skips writing the version into the db", func() {
					Consistently(fakeSQLDB.SetVersionCallCount).Should(Equal(0))
				})
			})

			Context("and there is no existing version", func() {
				BeforeEach(func() {
					fakeSQLDB.VersionReturns(nil, models.ErrResourceNotFound)
				})

				It("writes a zero version into the db", func() {
					Eventually(fakeSQLDB.SetVersionCallCount).Should(Equal(1))

					_, version := fakeSQLDB.SetVersionArgsForCall(0)
					Expect(version.CurrentVersion).To(BeEquivalentTo(0))
					Expect(version.CurrentVersion).To(BeEquivalentTo(0))
				})
			})
		})
	})

	Context("when not configured with a database", func() {
		BeforeEach(func() {
			rawSQLDB = nil
		})

		It("fails early", func() {
			var err error
			Eventually(migrationProcess.Wait()).Should(Receive(&err))
			Expect(err).To(MatchError("no database configured"))
			Expect(migrationProcess.Ready()).ToNot(BeClosed())
			Expect(migrationsDone).NotTo(BeClosed())
		})
	})
})
