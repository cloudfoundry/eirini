package bifrost_test

import (
	"context"
	"encoding/json"
	"errors"

	"code.cloudfoundry.org/eirini/bifrost"
	"code.cloudfoundry.org/eirini/bifrost/bifrostfakes"
	"code.cloudfoundry.org/eirini/models/cf"
	"code.cloudfoundry.org/lager/lagertest"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	v1 "github.com/opencontainers/image-spec/specs-go/v1"
)

var _ = Describe("DockerStager", func() {
	var (
		stager           bifrost.DockerStaging
		fetcher          *bifrostfakes.FakeImageMetadataFetcher
		parser           *bifrostfakes.FakeImageRefParser
		stagingCompleter *bifrostfakes.FakeStagingCompleter
	)

	Context("Stage a docker image", func() {
		var (
			stagingErr     error
			stagingRequest cf.StagingRequest
		)

		BeforeEach(func() {
			fetcher = new(bifrostfakes.FakeImageMetadataFetcher)
			parser = new(bifrostfakes.FakeImageRefParser)
			stagingCompleter = new(bifrostfakes.FakeStagingCompleter)
			stagingRequest = cf.StagingRequest{
				CompletionCallback: "the-completion-callback/call/me",
				Lifecycle: cf.StagingLifecycle{
					DockerLifecycle: &cf.StagingDockerLifecycle{
						Image: "eirini/some-app:some-tag",
					},
				},
			}

			fetcher.Returns(&v1.ImageConfig{
				ExposedPorts: map[string]struct{}{
					"8888/tcp": {},
				},
			}, nil)

			parser.Returns("//some-valid-docker-ref", nil)
		})

		JustBeforeEach(func() {
			stager = bifrost.DockerStaging{
				Logger:               lagertest.NewTestLogger(""),
				ImageMetadataFetcher: fetcher.Spy,
				ImageRefParser:       parser.Spy,
				StagingCompleter:     stagingCompleter,
			}

			stagingErr = stager.TransferStaging(context.Background(), "stg-guid", stagingRequest)
		})

		It("should succeed", func() {
			Expect(stagingErr).ToNot(HaveOccurred())
		})

		It("should parse the docker image ref", func() {
			Expect(parser.CallCount()).To(Equal(1))
			img := parser.ArgsForCall(0)
			Expect(img).To(Equal("eirini/some-app:some-tag"))
		})

		It("should use the parsed docker image ref", func() {
			Expect(fetcher.CallCount()).To(Equal(1))
			ref, _ := fetcher.ArgsForCall(0)
			Expect(ref).To(Equal("//some-valid-docker-ref"))
		})

		It("should complete staging with correct parameters", func() {
			Expect(stagingCompleter.CompleteStagingCallCount()).To(Equal(1))
			_, taskCompletedRequest := stagingCompleter.CompleteStagingArgsForCall(0)

			Expect(taskCompletedRequest.TaskGUID).To(Equal("stg-guid"))
			Expect(taskCompletedRequest.Failed).To(BeFalse())
			Expect(taskCompletedRequest.Annotation).To(Equal(`{"completion_callback": "the-completion-callback/call/me"}`))

			var payload bifrost.StagingResult
			Expect(json.Unmarshal([]byte(taskCompletedRequest.Result), &payload)).To(Succeed())

			Expect(payload.LifecycleType).To(Equal("docker"))
			Expect(payload.LifecycleMetadata.DockerImage).To(Equal("eirini/some-app:some-tag"))
			Expect(payload.ProcessTypes.Web).To(BeEmpty())
			Expect(payload.ExecutionMetadata).To(Equal(`{"cmd":[],"ports":[{"Port":8888,"Protocol":"tcp"}]}`))
		})

		Context("when the image is from a private registry", func() {
			BeforeEach(func() {
				stagingRequest.Lifecycle.DockerLifecycle.Image = "private-registry.io/user/repo"
				stagingRequest.Lifecycle.DockerLifecycle.RegistryUsername = "some-user"
				stagingRequest.Lifecycle.DockerLifecycle.RegistryPassword = "thepasswrd"
			})

			It("should succeed", func() {
				Expect(stagingErr).ToNot(HaveOccurred())
			})

			It("should provide the correct credentials", func() {
				Expect(fetcher.CallCount()).To(Equal(1))
				_, ctx := fetcher.ArgsForCall(0)
				Expect(ctx.DockerAuthConfig.Username).To(Equal("some-user"))
				Expect(ctx.DockerAuthConfig.Password).To(Equal("thepasswrd"))
			})
		})

		Context("when the staging completion callback fails", func() {
			BeforeEach(func() {
				stagingCompleter.CompleteStagingReturns(errors.New("callback failed"))
			})

			It("should fail with the right error", func() {
				Expect(stagingErr).To(MatchError("callback failed"))
			})
		})

		Context("when the image is invalid", func() {
			BeforeEach(func() {
				parser.Returns("", errors.New("failed to create an image ref because of reasons"))
			})

			It("should fail with the right error", func() {
				Expect(stagingErr).ToNot(HaveOccurred())
				Expect(stagingCompleter.CompleteStagingCallCount()).To(Equal(1))

				_, taskCallbackResponse := stagingCompleter.CompleteStagingArgsForCall(0)
				Expect(taskCallbackResponse.Failed).To(BeTrue())
				Expect(taskCallbackResponse.FailureReason).To(ContainSubstring("failed to create an image ref because of reasons"))
			})

			Context("when the staging completion callback fails", func() {
				BeforeEach(func() {
					stagingCompleter.CompleteStagingReturns(errors.New("callback failed"))
				})

				It("should fail with the right error", func() {
					Expect(stagingErr).To(MatchError("callback failed"))
				})
			})
		})

		Context("when metadata fetching fails", func() {
			BeforeEach(func() {
				fetcher.Returns(nil, errors.New("boom"))
			})

			It("should fail with the right error", func() {
				Expect(stagingErr).ToNot(HaveOccurred())
				Expect(stagingCompleter.CompleteStagingCallCount()).To(Equal(1))

				_, taskCallbackResponse := stagingCompleter.CompleteStagingArgsForCall(0)
				Expect(taskCallbackResponse.Failed).To(BeTrue())
				Expect(taskCallbackResponse.FailureReason).To(ContainSubstring("failed to fetch image metadata"))
			})

			Context("when the staging completion callback fails", func() {
				BeforeEach(func() {
					stagingCompleter.CompleteStagingReturns(errors.New("callback failed"))
				})

				It("should fail with the right error", func() {
					Expect(stagingErr).To(MatchError("callback failed"))
				})
			})
		})

		Context("when exposed ports are wrongly formatted in the image metadata", func() {
			BeforeEach(func() {
				fetcher.Returns(&v1.ImageConfig{
					ExposedPorts: map[string]struct{}{
						"invalid-port-spec": {},
					},
				}, nil)
			})

			It("should respond to the callback url with failure", func() {
				Expect(stagingErr).ToNot(HaveOccurred())
				Expect(stagingCompleter.CompleteStagingCallCount()).To(Equal(1))

				_, taskCallbackResponse := stagingCompleter.CompleteStagingArgsForCall(0)
				Expect(taskCallbackResponse.Failed).To(BeTrue())
				Expect(taskCallbackResponse.FailureReason).To(ContainSubstring("failed to parse exposed ports"))
			})
		})

		Context("when the staging completion callback fails", func() {
			BeforeEach(func() {
				stagingCompleter.CompleteStagingReturns(errors.New("callback failed"))
			})

			It("should fail with the right error", func() {
				Expect(stagingErr).To(MatchError("callback failed"))
			})
		})
	})
})
