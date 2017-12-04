package models_test

import (
	"code.cloudfoundry.org/bbs/models"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("CellPresence", func() {
	var (
		cellPresence         models.CellPresence
		capacity             models.CellCapacity
		expectedProviderList []*models.Provider
	)

	BeforeEach(func() {
		capacity = models.NewCellCapacity(128, 1024, 3)
		rootfsProviders := []string{"provider-1"}
		preloadedRootFSes := []string{"provider-2", "provider-3"}
		placementTags := []string{"tag-1", "tag-2"}
		optionalPlacementTags := []string{"optional-tag-1", "optional-tag-2"}
		cellPresence = models.NewCellPresence("some-id", "some-address", "http://some-url", "some-zone", capacity, rootfsProviders, preloadedRootFSes, placementTags, optionalPlacementTags)
		expectedProviderList = []*models.Provider{
			&models.Provider{"preloaded", []string{"provider-2", "provider-3"}},
			&models.Provider{"preloaded+layer", []string{"provider-2", "provider-3"}},
			&models.Provider{"provider-1", []string{}},
		}
	})

	Describe("Validate", func() {
		Context("when cell presence is valid", func() {
			It("does not return an error", func() {
				Expect(cellPresence.Validate()).NotTo(HaveOccurred())
				Expect(cellPresence.GetRootfsProviders()).To(Equal(expectedProviderList))
			})

			Context("when the RepUrl is empty", func() {
				BeforeEach(func() {
					cellPresence.RepUrl = ""
				})

				It("does not return an error", func() {
					err := cellPresence.Validate()
					Expect(err).NotTo(HaveOccurred())
				})
			})
		})

		Context("when cell presence is invalid", func() {
			Context("when cell id is invalid", func() {
				BeforeEach(func() {
					cellPresence.CellId = ""
				})

				It("returns an error", func() {
					err := cellPresence.Validate()
					Expect(err).To(HaveOccurred())
					Expect(err.Error()).To(ContainSubstring("cell_id"))
				})
			})

			Context("when rep address is invalid", func() {
				BeforeEach(func() {
					cellPresence.RepAddress = ""
				})

				It("returns an error", func() {
					err := cellPresence.Validate()
					Expect(err).To(HaveOccurred())
					Expect(err.Error()).To(ContainSubstring("rep_address"))
				})
			})

			Context("when rep url is invalid", func() {
				Context("when RepUrl is not configured with HTTP or HTTPS", func() {
					BeforeEach(func() {
						cellPresence.RepUrl = "some-rep-url"
					})

					It("returns an error", func() {
						err := cellPresence.Validate()
						Expect(err).To(HaveOccurred())
						Expect(err.Error()).To(ContainSubstring("rep_url"))
					})
				})
			})

			Context("when cell capacity is invalid", func() {
				Context("when memory is zero", func() {
					BeforeEach(func() {
						cellPresence.Capacity.MemoryMb = 0
					})
					It("returns an error", func() {
						err := cellPresence.Validate()
						Expect(err).To(HaveOccurred())
						Expect(err.Error()).To(ContainSubstring("memory_mb"))
					})
				})

				Context("when memory is negative", func() {
					BeforeEach(func() {
						cellPresence.Capacity.MemoryMb = -1
					})
					It("returns an error", func() {
						err := cellPresence.Validate()
						Expect(err).To(HaveOccurred())
						Expect(err.Error()).To(ContainSubstring("memory_mb"))
					})
				})

				Context("when containers are zero", func() {
					BeforeEach(func() {
						cellPresence.Capacity.Containers = 0
					})
					It("returns an error", func() {
						err := cellPresence.Validate()
						Expect(err).To(HaveOccurred())
						Expect(err.Error()).To(ContainSubstring("containers"))
					})
				})

				Context("when containers are negative", func() {
					BeforeEach(func() {
						cellPresence.Capacity.Containers = -1
					})
					It("returns an error", func() {
						err := cellPresence.Validate()
						Expect(err).To(HaveOccurred())
						Expect(err.Error()).To(ContainSubstring("containers"))
					})
				})

				Context("when disk is negative", func() {
					BeforeEach(func() {
						cellPresence.Capacity.DiskMb = -1
					})
					It("returns an error", func() {
						err := cellPresence.Validate()
						Expect(err).To(HaveOccurred())
						Expect(err.Error()).To(ContainSubstring("disk_mb"))
					})
				})
			})
		})
	})
})
