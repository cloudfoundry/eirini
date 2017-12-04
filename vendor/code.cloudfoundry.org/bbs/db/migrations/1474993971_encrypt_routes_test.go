package migrations_test

import (
	"crypto/rand"
	"time"

	"code.cloudfoundry.org/bbs/db/migrations"
	"code.cloudfoundry.org/bbs/db/sqldb/helpers"
	"code.cloudfoundry.org/bbs/encryption"
	"code.cloudfoundry.org/bbs/migration"
	"code.cloudfoundry.org/clock/fakeclock"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Encrypt Routes in Desired LRPs", func() {
	var (
		mig       migration.Migration
		migErr    error
		fakeClock *fakeclock.FakeClock
	)

	BeforeEach(func() {
		fakeClock = fakeclock.NewFakeClock(time.Now())
		rawSQLDB.Exec("DROP TABLE domains;")
		rawSQLDB.Exec("DROP TABLE tasks;")
		rawSQLDB.Exec("DROP TABLE desired_lrps;")
		rawSQLDB.Exec("DROP TABLE actual_lrps;")

		mig = migrations.NewEncryptRoutes()
	})

	It("appends itself to the migration list", func() {
		Expect(migrations.Migrations).To(ContainElement(mig))
	})

	Describe("Version", func() {
		It("returns the timestamp from which it was created", func() {
			Expect(mig.Version()).To(BeEquivalentTo(1474993971))
		})
	})

	Describe("Up", func() {
		var initialMigrations migration.Migrations
		var routes string

		BeforeEach(func() {
			initialMigrations = []migration.Migration{
				migrations.NewETCDToSQL(),
				migrations.NewIncreaseRunInfoColumnSize(),
				migrations.NewAddPlacementTagsToDesiredLRPs(),
				migrations.NewIncreaseErrorColumnsSize(),
			}

			for _, m := range initialMigrations {
				m.SetRawSQLDB(rawSQLDB)
				m.SetDBFlavor(flavor)
				m.SetClock(fakeClock)
				err := m.Up(logger)
				Expect(err).NotTo(HaveOccurred())
			}

			// Can't do this in the Describe BeforeEach
			// as the test on line 37 will cause ginkgo to panic
			mig.SetRawSQLDB(rawSQLDB)
			mig.SetDBFlavor(flavor)

			key, err := encryption.NewKey("a", "my key")
			Expect(err).NotTo(HaveOccurred())
			keys := []encryption.Key{key}

			keyManager, err := encryption.NewKeyManager(key, keys)
			Expect(err).NotTo(HaveOccurred())
			cryptor := encryption.NewCryptor(keyManager, rand.Reader)
			mig.SetCryptor(cryptor)

			routes = `{"cf-router":[{"hostnames":["dora.bosh-lite.com"],"port":8080}],"diego-ssh":{"container_port":2222,"host_fingerprint":"95:9d:7f:d7:cd:bc:d0:01:fa:8a:3a:a1:c6:ef:58:d7","private_key":"-----BEGIN RSA PRIVATE KEY-----\nMIICXAIBAAKBgQDR/LGweyezjduoCGqmp2AR+5ggWxAT8ofEGt+PFQYY4Un/+xJ7\naeiAkk7GhHhJdL7UjuFU45XROiiZxKZhHGD1jKyG7CvaV47NVLvgqvPiY5jNjR2M\nCfnjpQ98QJ2Bv7usVfBiQP0cWK1bScchwZ1Y5At9ipyIztMqlOshKLRJPQIDAQAB\nAoGAdVtHp3081AG9OGzzxg4XCBXXkIW0N6G9NOFb/ihezvriE5krXCP1mB2svw/7\n9fm0STFNR9clvNhHJqEb53wnxzCpHMV+oH5Zg+5suQ5UsX3nof/c5PI5PK0jvIRI\nFe83ty3cu9UzYEJFVDSqJjx6SFoKBLXnxCzbVSskpkTZvlUCQQDxRcIlGLOE1lEZ\nORZuTd3TI/lg8NssEDL801PGdOIxchkiAzZz1RZW3M3SjY/PswuwiV1s4qkeHIPh\nlVeg4kS3AkEA3s4OAEl+gUtYGtLw2lSmEhgxNjK1x5EHzhuIulEla9iftbSy9Jpa\nPtzfHZ5ZxFdCnCvyukVW3KGVww40w921qwJBAN7DFo6jsNP8AKK2J7SuJhoUw+Iy\nX1nelwUBpP692j3m57eUmcj2vAp1EX/OfjI5UJitK1omKBkKIOW9uktrvh8CQBlq\ngAZgW+H76k0FCxyc02T1BYgdOMdPMAi+81Xts8sdpvpfZpqokOri30DNs4fGPH78\nNHAzQLliZWce074UKIkCQDbumNywkGzajAu+fTk+/Hts/o0g+btFS1oBDF5ztpJE\nGr9v4KGkJ//Nam2GucW1OY/JpgvZ3ITqj340wSqyyu4=\n-----END RSA PRIVATE KEY-----\n"},"tcp-router":[]`

			_, err = rawSQLDB.Exec(
				helpers.RebindForFlavor(
					`INSERT INTO desired_lrps
						  (process_guid, domain, placement_tags, log_guid, instances, memory_mb,
							  disk_mb, rootfs, routes, volume_placement, modification_tag_epoch, run_info)
						  VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
					flavor,
				),
				"guid", "domain", "",
				"log guid", 2, 1, 1, "rootfs", routes, "volumes yo", 1, "run info",
			)
			Expect(err).NotTo(HaveOccurred())

		})

		JustBeforeEach(func() {
			migErr = mig.Up(logger)
		})

		It("does not error out", func() {
			Expect(migErr).NotTo(HaveOccurred())
		})

		It("should encrypt route column in desired lrps", func() {
			var fetchedJSONData string
			query := helpers.RebindForFlavor("select routes from desired_lrps limit 1", flavor)
			row := rawSQLDB.QueryRow(query)
			Expect(row.Scan(&fetchedJSONData)).NotTo(HaveOccurred())
			Expect(fetchedJSONData).ToNot(BeEquivalentTo(routes))
		})
	})

	Describe("Down", func() {
		It("returns a not implemented error", func() {
			Expect(mig.Down(logger)).To(HaveOccurred())
		})
	})
})
