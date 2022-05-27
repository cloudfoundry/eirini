package docker_test

import (
	"code.cloudfoundry.org/eirini/stager/docker"
	"code.cloudfoundry.org/eirini/tests"
	"github.com/containers/image/types"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Fetch Docker Image Metadata", func() {
	Context("public image from DockerHub", func() {
		It("should return the correct exposed ports", func() {
			imgConfig, err := docker.Fetch("//docker.io/eirini/custom-port:latest", types.SystemContext{})

			Expect(err).To(BeNil())
			Expect(imgConfig).ToNot(BeNil())
			Expect(imgConfig.ExposedPorts).To(HaveLen(1))
			Expect(imgConfig.ExposedPorts).To(HaveKey("8888/tcp"))
		})

		Context("when repo is invalid", func() {
			It("should return an error", func() {
				imgConfig, err := docker.Fetch("//docker.io/eirini/no_such_image:latest", types.SystemContext{})

				Expect(err).To(MatchError(ContainSubstring("failed to get image source")))
				Expect(imgConfig).To(BeNil())
			})
		})

		Context("private image from DockerHub", func() {
			It("should return the correct exposed ports", func() {
				imgConfig, err := docker.Fetch("//docker.io/eiriniuser/notdora:custom-port", types.SystemContext{
					DockerAuthConfig: &types.DockerAuthConfig{
						Username: "eiriniuser",
						Password: tests.GetEiriniDockerHubPassword(),
					},
				})

				Expect(err).To(BeNil())
				Expect(imgConfig).ToNot(BeNil())
				Expect(imgConfig.ExposedPorts).To(HaveLen(1))
				Expect(imgConfig.ExposedPorts).To(HaveKey("8888/tcp"))
			})
		})
	})
})
