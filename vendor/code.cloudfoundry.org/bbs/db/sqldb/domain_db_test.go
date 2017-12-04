package sqldb_test

import (
	"math"
	"time"

	"code.cloudfoundry.org/bbs/test_helpers"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("DomainDB", func() {
	Describe("Domains", func() {
		Context("when there are domains in the DB", func() {
			BeforeEach(func() {
				futureTime := fakeClock.Now().Add(5 * time.Second).UnixNano()

				queryStr := "INSERT INTO domains VALUES (?, ?)"
				if test_helpers.UsePostgres() {
					queryStr = test_helpers.ReplaceQuestionMarks(queryStr)
				}
				_, err := db.Exec(queryStr, "jims-domain", futureTime)
				Expect(err).NotTo(HaveOccurred())

				_, err = db.Exec(queryStr, "amelias-domain", futureTime)
				Expect(err).NotTo(HaveOccurred())

				pastTime := fakeClock.Now().Add(-5 * time.Second).UnixNano()
				_, err = db.Exec(queryStr, "past-domain", pastTime)
				Expect(err).NotTo(HaveOccurred())

				_, err = db.Exec(queryStr, "current-domain", fakeClock.Now().Round(time.Second).UnixNano())
				Expect(err).NotTo(HaveOccurred())
			})

			It("returns all the non-expired domains in the DB", func() {
				domains, err := sqlDB.Domains(logger)
				Expect(err).NotTo(HaveOccurred())

				Expect(domains).To(HaveLen(2))
				Expect(domains).To(ConsistOf([]string{"jims-domain", "amelias-domain"}))
			})
		})

		Context("when there are no domains in the DB", func() {
			It("returns no domains", func() {
				domains, err := sqlDB.Domains(logger)
				Expect(err).NotTo(HaveOccurred())
				Expect(domains).To(HaveLen(0))
			})
		})
	})

	Describe("UpsertDomain", func() {
		Context("when the domain is not present in the DB", func() {
			It("inserts a new domain with the requested TTL", func() {
				domain := "my-awesome-domain"

				bbsErr := sqlDB.UpsertDomain(logger, domain, 5432)
				Expect(bbsErr).NotTo(HaveOccurred())

				rows, err := db.Query("SELECT * FROM domains;")
				Expect(err).NotTo(HaveOccurred())
				defer rows.Close()

				var domainName string
				var expireTime int64

				Expect(rows.Next()).To(BeTrue())
				err = rows.Scan(&domainName, &expireTime)
				Expect(err).NotTo(HaveOccurred())
				Expect(domainName).To(Equal(domain))
				expectedExpireTime := fakeClock.Now().UTC().Add(time.Duration(5432) * time.Second).UnixNano()
				Expect(expireTime).To(BeEquivalentTo(expectedExpireTime))
			})

			It("never expires when the ttl is Zero", func() {
				domain := "my-awesome-domain"

				bbsErr := sqlDB.UpsertDomain(logger, domain, 0)
				Expect(bbsErr).NotTo(HaveOccurred())

				rows, err := db.Query("SELECT * FROM domains")
				Expect(err).NotTo(HaveOccurred())
				defer rows.Close()

				var domainName string
				var expireTime int64

				Expect(rows.Next()).To(BeTrue())
				err = rows.Scan(&domainName, &expireTime)
				Expect(err).NotTo(HaveOccurred())
				Expect(domainName).To(Equal(domain))
				Expect(expireTime).To(BeNumerically("==", math.MaxInt64))
			})

			Context("when the domain is too long", func() {
				It("returns an error", func() {
					domain := randStr(256)
					bbsErr := sqlDB.UpsertDomain(logger, domain, 5432)
					Expect(bbsErr).To(HaveOccurred())
				})
			})
		})

		Context("when the domain is already present in the DB", func() {
			var existingDomain = "the-domain-that-was-already-there"

			BeforeEach(func() {
				bbsErr := sqlDB.UpsertDomain(logger, existingDomain, 1)
				Expect(bbsErr).NotTo(HaveOccurred())
			})

			It("updates the TTL on the existing record", func() {
				fakeClock.Increment(10 * time.Second)

				bbsErr := sqlDB.UpsertDomain(logger, existingDomain, 1)
				Expect(bbsErr).NotTo(HaveOccurred())

				rowsCount, err := db.Query("SELECT COUNT(*) FROM domains;")
				Expect(err).NotTo(HaveOccurred())
				defer rowsCount.Close()

				Expect(rowsCount.Next()).To(BeTrue())
				var domainCount int
				err = rowsCount.Scan(&domainCount)
				Expect(err).NotTo(HaveOccurred())
				Expect(domainCount).To(Equal(1))
				Expect(rowsCount.Close()).To(Succeed())

				rows, err := db.Query("SELECT * FROM domains;")
				Expect(err).NotTo(HaveOccurred())
				defer rows.Close()

				var domainName string
				var expireTime int64

				Expect(rows.Next()).To(BeTrue())
				err = rows.Scan(&domainName, &expireTime)
				Expect(err).NotTo(HaveOccurred())
				Expect(domainName).To(Equal(existingDomain))
				expectedExpireTime := fakeClock.Now().UTC().Add(time.Duration(1) * time.Second).UnixNano()
				Expect(expireTime).To(BeEquivalentTo(expectedExpireTime))
			})
		})
	})
})
