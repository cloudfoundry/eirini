package cc_messages_test

import (
	"encoding/json"

	"code.cloudfoundry.org/bbs/models"
	"code.cloudfoundry.org/runtimeschema/cc_messages"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("StagingMessages", func() {
	Describe("StagingRequestFromCC", func() {
		ccJSON := `{
           "app_id" : "fake-app_id",
           "memory_mb" : 1024,
           "disk_mb" : 10000,
           "file_descriptors" : 3,
           "environment" : [{"name": "FOO", "value":"BAR"}],
           "timeout" : 900,
           "lifecycle": "buildpack",
					 "lifecycle_data": {"foo": "bar"},
					 "completion_callback": "https://api.cc.com/staging/complete"
        }`

		It("should be mapped to the CC's staging request JSON", func() {
			var stagingRequest cc_messages.StagingRequestFromCC
			err := json.Unmarshal([]byte(ccJSON), &stagingRequest)
			Expect(err).NotTo(HaveOccurred())

			lifecycle_data := json.RawMessage([]byte(`{"foo": "bar"}`))
			Expect(stagingRequest).To(Equal(cc_messages.StagingRequestFromCC{
				AppId:           "fake-app_id",
				MemoryMB:        1024,
				DiskMB:          10000,
				FileDescriptors: 3,
				Environment: []*models.EnvironmentVariable{
					{Name: "FOO", Value: "BAR"},
				},
				Timeout:            900,
				Lifecycle:          "buildpack",
				LifecycleData:      &lifecycle_data,
				CompletionCallback: "https://api.cc.com/staging/complete",
			}))
		})
	})

	Describe("BuildpackLifecycleData", func() {
		lifecycleDataJSON := `{
				"app_bits_download_uri" : "http://fake-download_uri",
				"build_artifacts_cache_download_uri" : "http://a-nice-place-to-get-valuable-artifacts.com",
				"build_artifacts_cache_upload_uri" : "http://a-nice-place-to-upload-valuable-artifacts.com",
				"buildpacks" : [{"name":"fake-buildpack-name", "key":"fake-buildpack-key" ,"url":"fake-buildpack-url", "skip_detect":true}],
				"droplet_upload_uri" : "http://droplet-upload-uri",
				"stack": "pancakes"
			}`

		It("unmarshals correctly", func() {
			var lifecycleData cc_messages.BuildpackStagingData
			err := json.Unmarshal([]byte(lifecycleDataJSON), &lifecycleData)
			Expect(err).NotTo(HaveOccurred())

			Expect(lifecycleData).To(Equal(cc_messages.BuildpackStagingData{
				AppBitsDownloadUri:             "http://fake-download_uri",
				BuildArtifactsCacheDownloadUri: "http://a-nice-place-to-get-valuable-artifacts.com",
				BuildArtifactsCacheUploadUri:   "http://a-nice-place-to-upload-valuable-artifacts.com",
				Buildpacks: []cc_messages.Buildpack{
					{
						Name:       "fake-buildpack-name",
						Key:        "fake-buildpack-key",
						Url:        "fake-buildpack-url",
						SkipDetect: true,
					},
				},
				DropletUploadUri: "http://droplet-upload-uri",
				Stack:            "pancakes",
			}))

		})
	})

	Describe("DockerStagingData", func() {
		lifecycleDataJSON := `{
      "docker_image" : "docker:///diego/image"
    }`

		It("should be mapped to the CC's staging request JSON", func() {
			var stagingData cc_messages.DockerStagingData
			err := json.Unmarshal([]byte(lifecycleDataJSON), &stagingData)
			Expect(err).NotTo(HaveOccurred())

			Expect(stagingData).To(Equal(cc_messages.DockerStagingData{
				DockerImageUrl: "docker:///diego/image",
			}))

		})
	})

	Describe("Buildpack", func() {
		Context("when skipping the detect phase is not specified", func() {
			ccJSONFragment := `{
       "name": "ocaml-buildpack",
       "key": "ocaml-buildpack-guid",
       "url": "http://ocaml.org/buildpack.zip"
      }`

			It("extracts the name, key, and url values", func() {
				var buildpack cc_messages.Buildpack

				err := json.Unmarshal([]byte(ccJSONFragment), &buildpack)
				Expect(err).NotTo(HaveOccurred())

				Expect(buildpack).To(Equal(cc_messages.Buildpack{
					Name: "ocaml-buildpack",
					Key:  "ocaml-buildpack-guid",
					Url:  "http://ocaml.org/buildpack.zip",
				}))

			})
		})

		Context("when skipping the detect phase is specified", func() {
			ccJSONFragment := `{
        "name": "ocaml-buildpack",
        "key": "ocaml-buildpack-guid",
        "url": "http://ocaml.org/buildpack.zip",
        "skip_detect": true
      }`

			It("extracts the name, key, url, and skip_detect values", func() {
				var buildpack cc_messages.Buildpack

				err := json.Unmarshal([]byte(ccJSONFragment), &buildpack)
				Expect(err).NotTo(HaveOccurred())

				Expect(buildpack).To(Equal(cc_messages.Buildpack{
					Name:       "ocaml-buildpack",
					Key:        "ocaml-buildpack-guid",
					Url:        "http://ocaml.org/buildpack.zip",
					SkipDetect: true,
				}))

			})
		})
	})

	Describe("StagingResponseForCC", func() {
		Context("without an error", func() {
			It("generates valid JSON", func() {
				result := json.RawMessage(`{"foo":"bar"}`)
				stagingResponseForCC := cc_messages.StagingResponseForCC{
					Result: &result,
				}
				Expect(json.Marshal(stagingResponseForCC)).To(MatchJSON(`{
					"result": {"foo":"bar"}
				}`))
			})
		})

		Context("with an error", func() {
			It("generates valid JSON with the error", func() {
				err := &cc_messages.StagingError{
					Id:      "StagingError",
					Message: "FAIL, missing camels!",
				}
				stagingResponseForCC := cc_messages.StagingResponseForCC{
					Error: err,
				}
				Expect(json.Marshal(stagingResponseForCC)).To(MatchJSON(`{
					"error": { "id": "StagingError", "message": "FAIL, missing camels!" }
				}`))
			})
		})
	})
})
