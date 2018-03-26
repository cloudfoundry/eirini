package main_test

import (
	"errors"

	"github.com/julz/cube/cubefakes"
	. "github.com/julz/cube/recipe"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Uploader", func() {

	var (
		cfclient *cubefakes.FakeCfClient
		uploader Uploader
	)

	BeforeEach(func() {
		cfclient = new(cubefakes.FakeCfClient)
		uploader = Uploader{cfclient}
	})

	Context("UploadWithCfClient", func() {
		It("should return an error if an empty guid parameter is passed", func() {
			err := uploader.Upload("", "")
			Expect(err).To(HaveOccurred())
			Expect(err).To(MatchError(ContainSubstring("empty guid parameter")))
		})

		It("should return an error if an empty path parameter is passed", func() {
			err := uploader.Upload("guid", "")
			Expect(err).To(HaveOccurred())
			Expect(err).To(MatchError(ContainSubstring("empty path parameter")))
		})

		Context("Cfclient", func() {
			Context("When PushDroplet is called", func() {
				It("passes the right parameters", func() {
					err := uploader.Upload("my-guid", "path")
					Expect(err).ToNot(HaveOccurred())
					name, guid := cfclient.PushDropletArgsForCall(0)
					Expect(name).To(Equal("path"))
					Expect(guid).To(Equal("my-guid"))
				})
			})

			Context("When a push is successful", func() {
				It("should not return any error", func() {
					err := uploader.Upload("my-guid", "path")
					Expect(err).ToNot(HaveOccurred())
				})
			})

			Context("When a push fails", func() {
				BeforeEach(func() {
					cfclient.PushDropletReturns(errors.New("aargh"))
				})

				It("should return an meaningful error message", func() {
					err := uploader.Upload("guid", "path")
					Expect(err).To(HaveOccurred())

					Expect(err).To(MatchError(ContainSubstring("perform request failed")))
				})
			})
		})
	})
})
