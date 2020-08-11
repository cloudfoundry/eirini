package docker_test

import (
	"code.cloudfoundry.org/eirini/stager/docker"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("ImageRefParser", func() {
	It("should create image ref for an image in dockerhub", func() {
		ref, err := docker.Parse("eirini/some-app:some-tag")

		Expect(err).ToNot(HaveOccurred())
		Expect(ref).To(Equal("//docker.io/eirini/some-app:some-tag"))
	})

	It("should create image ref for an image from the standard library", func() {
		ref, err := docker.Parse("ubuntu")

		Expect(err).ToNot(HaveOccurred())
		Expect(ref).To(Equal("//docker.io/library/ubuntu"))
	})

	It("should create image ref for an image in a private registry", func() {
		ref, err := docker.Parse("private-registry.io/user/repo")

		Expect(err).ToNot(HaveOccurred())
		Expect(ref).To(Equal("//private-registry.io/user/repo"))
	})

	It("should error if the image is invalid", func() {
		_, err := docker.Parse("this is invalid")

		Expect(err).To(HaveOccurred())
	})
})
