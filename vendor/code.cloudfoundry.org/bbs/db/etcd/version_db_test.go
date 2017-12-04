package etcd_test

import (
	"encoding/json"

	"code.cloudfoundry.org/bbs/db/etcd"
	"code.cloudfoundry.org/bbs/models"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Version", func() {
	Describe("SetVersion", func() {
		It("sets the version into the database", func() {
			expectedVersion := &models.Version{CurrentVersion: 99, TargetVersion: 100}
			err := etcdDB.SetVersion(logger, expectedVersion)
			Expect(err).NotTo(HaveOccurred())

			response, err := storeClient.Get(etcd.VersionKey, false, false)
			Expect(err).NotTo(HaveOccurred())

			var actualVersion models.Version
			err = json.Unmarshal([]byte(response.Node.Value), &actualVersion)
			Expect(err).NotTo(HaveOccurred())

			Expect(actualVersion).To(Equal(*expectedVersion))
		})
	})

	Describe("Version", func() {
		Context("when the version key exists", func() {
			It("retrieves the version from the database", func() {
				expectedVersion := &models.Version{CurrentVersion: 199, TargetVersion: 200}
				value, err := json.Marshal(expectedVersion)
				Expect(err).NotTo(HaveOccurred())

				_, err = storeClient.Set(etcd.VersionKey, value, etcd.NO_TTL)
				Expect(err).NotTo(HaveOccurred())

				version, err := etcdDB.Version(logger)
				Expect(err).NotTo(HaveOccurred())

				Expect(*version).To(Equal(*expectedVersion))
			})
		})

		Context("when the version key does not exist", func() {
			It("returns a ErrResourceNotFound", func() {
				version, err := etcdDB.Version(logger)
				Expect(err).To(MatchError(models.ErrResourceNotFound))
				Expect(version).To(BeNil())
			})
		})

		Context("when the version key is not valid json", func() {
			It("returns a ErrDeserialize", func() {
				_, err := storeClient.Set(etcd.VersionKey, []byte(`{{`), etcd.NO_TTL)
				Expect(err).NotTo(HaveOccurred())

				_, err = etcdDB.Version(logger)
				Expect(err).To(MatchError(models.ErrDeserialize))
			})
		})
	})
})
