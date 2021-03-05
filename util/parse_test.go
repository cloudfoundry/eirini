package util_test

import (
	"code.cloudfoundry.org/eirini/util"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Parse", func() {
	Describe("AppIndex", func() {
		It("should index from pod name", func() {
			Expect(util.ParseAppIndex("some-name-1")).To(Equal(1))
		})

		Context("when the pod name does not contain dashes", func() {
			It("should return an error", func() {
				_, err := util.ParseAppIndex("somename1")

				Expect(err).To(HaveOccurred())
			})
		})

		Context("when the last part in pod name is not a number", func() {
			It("should return an error", func() {
				_, err := util.ParseAppIndex("somename-a")

				Expect(err).To(HaveOccurred())
			})
		})
	})

	Describe("ImageRegistryHost", func() {
		It("returns the registry host", func() {
			imageURL := "my-secret-docker-registry.docker.io:5000/repo/the-mighty-image:not-latest"
			Expect(util.ParseImageRegistryHost(imageURL)).To(Equal("my-secret-docker-registry.docker.io"))
		})

		It("should default to the docker hub", func() {
			imageURL := "repo/the-mighty-image:not-latest"
			Expect(util.ParseImageRegistryHost(imageURL)).To(Equal("index.docker.io/v1/"))
		})
	})
})
