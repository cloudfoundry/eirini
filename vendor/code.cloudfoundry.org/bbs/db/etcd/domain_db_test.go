package etcd_test

import (
	. "code.cloudfoundry.org/bbs/db/etcd"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("DomainDB", func() {
	Describe("UpsertDomain", func() {
		Context("when the domain is not present in the DB", func() {
			It("inserts a new domain with the requested TTL", func() {
				domain := "my-awesome-domain"
				bbsErr := etcdDB.UpsertDomain(logger, domain, 5432)
				Expect(bbsErr).NotTo(HaveOccurred())

				etcdEntry, err := storeClient.Get(DomainSchemaPath(domain), false, false)
				Expect(err).ToNot(HaveOccurred())
				Expect(etcdEntry.Node.TTL).To(BeNumerically("<=", 5432))
			})
		})

		Context("when the domain is already present in the DB", func() {
			var existingDomain = "the-domain-that-was-already-there"

			BeforeEach(func() {
				var err error
				_, err = storeClient.Set(DomainSchemaPath(existingDomain), []byte(""), 100)
				Expect(err).NotTo(HaveOccurred())
			})

			It("updates the TTL on the existing record", func() {
				bbsErr := etcdDB.UpsertDomain(logger, existingDomain, 1337)
				Expect(bbsErr).NotTo(HaveOccurred())

				etcdEntry, err := storeClient.Get(DomainSchemaPath(existingDomain), false, false)
				Expect(err).ToNot(HaveOccurred())
				Expect(etcdEntry.Node.TTL).To(BeNumerically("<=", 1337))
				Expect(etcdEntry.Node.TTL).To(BeNumerically(">", 100))
			})
		})
	})

	Describe("Domains", func() {
		Context("when there are domains in the DB", func() {
			BeforeEach(func() {
				var err error
				_, err = storeClient.Set(DomainSchemaPath("domain-1"), []byte(""), 100)
				Expect(err).NotTo(HaveOccurred())
				_, err = storeClient.Set(DomainSchemaPath("domain-2"), []byte(""), 100)
				Expect(err).NotTo(HaveOccurred())
			})

			It("returns all the existing domains in the DB", func() {
				domains, err := etcdDB.Domains(logger)
				Expect(err).NotTo(HaveOccurred())

				Expect(domains).To(HaveLen(2))
				Expect(domains).To(ConsistOf([]string{"domain-1", "domain-2"}))
			})
		})

		Context("when there are no domains in the DB", func() {
			It("returns no domains", func() {
				domains, err := etcdDB.Domains(logger)
				Expect(err).NotTo(HaveOccurred())
				Expect(domains).To(HaveLen(0))
			})
		})
	})
})
