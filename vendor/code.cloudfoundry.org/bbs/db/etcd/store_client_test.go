package etcd_test

import (
	etcddb "code.cloudfoundry.org/bbs/db/etcd"
	"github.com/coreos/go-etcd/etcd"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("StoreClient", func() {
	BeforeEach(func() {
		_, err := storeClient.Create("a", []byte("thing"), 0)
		Expect(err).NotTo(HaveOccurred())
	})

	Describe("Create", func() {
		It("allows duplicate keys if contents are the same", func() {
			_, err := storeClient.Create("a", []byte("thing"), 0)
			Expect(err).NotTo(HaveOccurred())
		})

		It("does not allows duplicate keys if contents are different", func() {
			_, err := storeClient.Create("a", []byte("thing-2"), 0)
			Expect(err).To(HaveOccurred())
			Expect(err.(*etcd.EtcdError).ErrorCode).To(Equal(etcddb.ETCDErrKeyExists))
		})
	})

	Describe("CompareAndSwap", func() {
		Context("when an ETCDErrIndexComparisonFailed occurs", func() {
			var (
				node *etcd.Node
			)

			BeforeEach(func() {
				response, err := storeClient.Get("a", true, false)
				Expect(err).NotTo(HaveOccurred())

				node = response.Node
			})

			It("ignores the error if the key can be fetched and contains our payload", func() {
				_, err := storeClient.CompareAndSwap("a", []byte("thing"), 0, node.ModifiedIndex+1)
				Expect(err).ToNot(HaveOccurred())
			})

			It("returns an error if the fetched key does not contain our payload", func() {
				_, err := storeClient.CompareAndSwap("a", []byte("thing2"), 0, node.ModifiedIndex+1)
				Expect(err.(*etcd.EtcdError).ErrorCode).To(Equal(etcddb.ETCDErrIndexComparisonFailed))
			})
		})
	})
})
