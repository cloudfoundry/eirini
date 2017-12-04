package etcd_test

import (
	"errors"

	"code.cloudfoundry.org/bbs/db/etcd"
	"code.cloudfoundry.org/bbs/models"
	"code.cloudfoundry.org/bbs/models/test/model_helpers"
	etcderrors "github.com/coreos/go-etcd/etcd"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Evacuation", func() {
	Describe("EvacuateActualLRP", func() {
		var (
			actualLRP *models.ActualLRP
			index     int32
			guid      string
			ttl       uint64
		)

		BeforeEach(func() {
			guid = "the-guid"
			index = 1
			ttl = 60
			actualLRP = model_helpers.NewValidActualLRP(guid, index)

			etcdHelper.SetRawEvacuatingActualLRP(actualLRP, ttl)

			node, err := storeClient.Get(etcd.EvacuatingActualLRPSchemaPath(guid, index), false, false)
			fakeStoreClient.GetReturns(node, err)
		})

		Context("when the something about the actual LRP has changed", func() {
			BeforeEach(func() {
				clock.IncrementBySeconds(5)
				actualLRP.Since = clock.Now().UnixNano()
			})

			Context("when the lrp key changes", func() {
				BeforeEach(func() {
					actualLRP.Domain = "some-other-domain"
				})

				It("persists the evacuating lrp in etcd", func() {
					group, err := etcdDB.EvacuateActualLRP(logger, &actualLRP.ActualLRPKey, &actualLRP.ActualLRPInstanceKey, &actualLRP.ActualLRPNetInfo, ttl)
					Expect(err).NotTo(HaveOccurred())

					actualLRP.ModificationTag.Increment()
					actualLRPGroup, err := etcdDB.ActualLRPGroupByProcessGuidAndIndex(logger, guid, index)
					Expect(err).NotTo(HaveOccurred())
					Expect(actualLRPGroup.Evacuating).To(BeEquivalentTo(actualLRP))
					Expect(group).To(Equal(actualLRPGroup))
				})
			})

			Context("when the instance key changes", func() {
				BeforeEach(func() {
					actualLRP.ActualLRPInstanceKey.InstanceGuid = "i am different here me roar"
				})

				It("persists the evacuating lrp in etcd", func() {
					group, err := etcdDB.EvacuateActualLRP(logger, &actualLRP.ActualLRPKey, &actualLRP.ActualLRPInstanceKey, &actualLRP.ActualLRPNetInfo, ttl)
					Expect(err).NotTo(HaveOccurred())

					actualLRP.ModificationTag.Increment()
					actualLRPGroup, err := etcdDB.ActualLRPGroupByProcessGuidAndIndex(logger, guid, index)
					Expect(err).NotTo(HaveOccurred())
					Expect(actualLRPGroup.Evacuating).To(BeEquivalentTo(actualLRP))
					Expect(group).To(Equal(actualLRPGroup))
				})
			})

			Context("when the netinfo changes", func() {
				BeforeEach(func() {
					actualLRP.ActualLRPNetInfo.Ports = []*models.PortMapping{
						models.NewPortMapping(6666, 7777),
					}
				})

				It("persists the evacuating lrp in etcd", func() {
					group, err := etcdDB.EvacuateActualLRP(logger, &actualLRP.ActualLRPKey, &actualLRP.ActualLRPInstanceKey, &actualLRP.ActualLRPNetInfo, ttl)
					Expect(err).NotTo(HaveOccurred())

					actualLRP.ModificationTag.Increment()
					actualLRPGroup, err := etcdDB.ActualLRPGroupByProcessGuidAndIndex(logger, guid, index)
					Expect(err).NotTo(HaveOccurred())
					Expect(actualLRPGroup.Evacuating).To(BeEquivalentTo(actualLRP))
					Expect(group).To(Equal(actualLRPGroup))
				})
			})

			Context("when compare and swap fails", func() {
				BeforeEach(func() {
					actualLRP.Domain = "some-other-domain"
					fakeStoreClient.CompareAndSwapReturns(nil, errors.New("compare and swap failed"))
				})

				It("returns an error", func() {
					_, err := etcdDBWithFakeStore.EvacuateActualLRP(logger, &actualLRP.ActualLRPKey, &actualLRP.ActualLRPInstanceKey, &actualLRP.ActualLRPNetInfo, ttl)
					Expect(err).To(HaveOccurred())
				})
			})
		})

		Context("when the actual lrp data is the same", func() {
			It("does nothing", func() {
				_, err := etcdDBWithFakeStore.EvacuateActualLRP(logger, &actualLRP.ActualLRPKey, &actualLRP.ActualLRPInstanceKey, &actualLRP.ActualLRPNetInfo, ttl)
				Expect(err).NotTo(HaveOccurred())

				Expect(fakeStoreClient.CompareAndSwapCallCount()).To(Equal(0))
			})
		})

		Context("when the evacuating actual lrp does not exist", func() {
			BeforeEach(func() {
				_, err := storeClient.Delete(etcd.EvacuatingActualLRPSchemaPath(guid, index), false)
				Expect(err).NotTo(HaveOccurred())

				actualLRP.CrashCount = 0
				actualLRP.CrashReason = ""
				actualLRP.Since = clock.Now().UnixNano()
			})

			It("creates the evacuating actual lrp", func() {
				group, err := etcdDB.EvacuateActualLRP(logger, &actualLRP.ActualLRPKey, &actualLRP.ActualLRPInstanceKey, &actualLRP.ActualLRPNetInfo, ttl)
				Expect(err).NotTo(HaveOccurred())

				actualLRPGroup, err := etcdDB.ActualLRPGroupByProcessGuidAndIndex(logger, guid, index)
				Expect(err).NotTo(HaveOccurred())
				Expect(group).To(Equal(actualLRPGroup))

				Expect(actualLRPGroup.Evacuating.ModificationTag.Epoch).NotTo(BeNil())
				Expect(actualLRPGroup.Evacuating.ModificationTag.Index).To(BeEquivalentTo((1)))
				actualLRPGroup.Evacuating.ModificationTag = actualLRP.ModificationTag
				Expect(actualLRPGroup.Evacuating).To(BeEquivalentTo(actualLRP))
			})

			Context("when create fails", func() {
				BeforeEach(func() {
					fakeStoreClient.GetReturns(nil, etcderrors.EtcdError{ErrorCode: etcd.ETCDErrKeyNotFound})
					fakeStoreClient.CreateReturns(nil, errors.New("ohhhh noooo mr billlll"))
				})

				It("returns an error", func() {
					_, err := etcdDBWithFakeStore.EvacuateActualLRP(logger, &actualLRP.ActualLRPKey, &actualLRP.ActualLRPInstanceKey, &actualLRP.ActualLRPNetInfo, ttl)
					Expect(err).To(HaveOccurred())
				})
			})
		})

		Context("when fetching the evacuating actual lrp fails", func() {
			BeforeEach(func() {
				fakeStoreClient.GetReturns(nil, errors.New("i failed"))
			})

			It("returns an error", func() {
				_, err := etcdDBWithFakeStore.EvacuateActualLRP(logger, &actualLRP.ActualLRPKey, &actualLRP.ActualLRPInstanceKey, &actualLRP.ActualLRPNetInfo, ttl)
				Expect(err).To(HaveOccurred())
			})
		})

		Context("when deserializing the data fails", func() {
			BeforeEach(func() {
				_, err := storeClient.Set(etcd.EvacuatingActualLRPSchemaPath(guid, index), []byte("{{"), ttl)
				Expect(err).NotTo(HaveOccurred())
			})

			It("returns an error", func() {
				_, err := etcdDB.EvacuateActualLRP(logger, &actualLRP.ActualLRPKey, &actualLRP.ActualLRPInstanceKey, &actualLRP.ActualLRPNetInfo, ttl)
				Expect(err).To(HaveOccurred())
			})
		})
	})

	Describe("RemoveEvacuatingActualLRP", func() {
		var (
			actualLRP *models.ActualLRP
			guid      string
			index     int32
		)

		BeforeEach(func() {
			guid = "the-guid"
			index = 1

			actualLRP = model_helpers.NewValidActualLRP(guid, index)
			etcdHelper.SetRawEvacuatingActualLRP(actualLRP, 0)
		})

		It("removes the evacuating actual lrp", func() {
			err := etcdDB.RemoveEvacuatingActualLRP(logger, &actualLRP.ActualLRPKey, &actualLRP.ActualLRPInstanceKey)
			Expect(err).NotTo(HaveOccurred())

			_, err = etcdDB.ActualLRPGroupByProcessGuidAndIndex(logger, guid, index)
			Expect(err).To(Equal(models.ErrResourceNotFound))

			key := etcd.EvacuatingActualLRPSchemaPath(guid, index)
			_, err = storeClient.Get(key, false, false)
			Expect(err).To(HaveOccurred())
		})

		Context("when the evacuating actual lrp does not exist", func() {
			BeforeEach(func() {
				_, err := storeClient.Delete(etcd.EvacuatingActualLRPSchemaPath(guid, index), false)
				Expect(err).NotTo(HaveOccurred())
			})

			It("does not return an error", func() {
				err := etcdDB.RemoveEvacuatingActualLRP(logger, &actualLRP.ActualLRPKey, &actualLRP.ActualLRPInstanceKey)
				Expect(err).NotTo(HaveOccurred())
			})
		})

		Context("fetching the evacuating actual lrp fails", func() {
			BeforeEach(func() {
				fakeStoreClient.GetReturns(nil, errors.New("get failed"))
			})

			It("returns the error", func() {
				err := etcdDBWithFakeStore.RemoveEvacuatingActualLRP(logger, &actualLRP.ActualLRPKey, &actualLRP.ActualLRPInstanceKey)
				Expect(err).To(HaveOccurred())
				Expect(fakeStoreClient.CompareAndDeleteCallCount()).To(Equal(0))
			})
		})

		Context("when deserializing the actual lrp fails", func() {
			BeforeEach(func() {
				_, err := storeClient.Set(etcd.EvacuatingActualLRPSchemaPath(guid, index), []byte("{{"), 0)
				Expect(err).NotTo(HaveOccurred())
			})

			It("returns the deserializaiton error", func() {
				err := etcdDB.RemoveEvacuatingActualLRP(logger, &actualLRP.ActualLRPKey, &actualLRP.ActualLRPInstanceKey)
				Expect(err).To(HaveOccurred())
				bbsErr := models.ConvertError(err)
				Expect(bbsErr.Type).To(Equal(models.Error_InvalidRecord))
			})
		})

		Context("when the actual lrp key does not match", func() {
			BeforeEach(func() {
				actualLRP.Domain = "a different domain"
			})

			It("returns a ErrActualLRPCannotBeRemoved", func() {
				err := etcdDB.RemoveEvacuatingActualLRP(logger, &actualLRP.ActualLRPKey, &actualLRP.ActualLRPInstanceKey)
				Expect(err).To(HaveOccurred())
				Expect(err).To(Equal(models.ErrActualLRPCannotBeRemoved))
			})
		})

		Context("when the actual lrp instance key does not match", func() {
			BeforeEach(func() {
				actualLRP.CellId = "a different cell"
			})

			It("returns a ErrActualLRPCannotBeRemoved", func() {
				err := etcdDB.RemoveEvacuatingActualLRP(logger, &actualLRP.ActualLRPKey, &actualLRP.ActualLRPInstanceKey)
				Expect(err).To(HaveOccurred())
				Expect(err).To(Equal(models.ErrActualLRPCannotBeRemoved))
			})
		})

		Context("when compare and delete fails", func() {
			BeforeEach(func() {
				resp, err := storeClient.Get(etcd.EvacuatingActualLRPSchemaPath(guid, index), false, false)
				fakeStoreClient.GetReturns(resp, err)
				fakeStoreClient.CompareAndDeleteReturns(nil, errors.New("compare and delete failed"))
			})

			It("returns a ErrActualLRPCannotBeRemoved", func() {
				err := etcdDBWithFakeStore.RemoveEvacuatingActualLRP(logger, &actualLRP.ActualLRPKey, &actualLRP.ActualLRPInstanceKey)
				Expect(err).To(HaveOccurred())
				Expect(err).To(Equal(models.ErrActualLRPCannotBeRemoved))
			})
		})
	})
})
