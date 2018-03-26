package st8ger_test

import (
	"encoding/json"

	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/lager/lagertest"
	"code.cloudfoundry.org/runtimeschema/cc_messages"
	"github.com/julz/cube"
	"github.com/julz/cube/opi"
	"github.com/julz/cube/st8ger"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Backend", func() {

	var (
		logger  lager.Logger
		backend cube.Backend
	)

	BeforeEach(func() {
		logger = lagertest.NewTestLogger("test")
		config := cube.BackendConfig{
			CfUsername: "admin",
			CfPassword: "admin",
			ApiAddress: "api.mycf.com",
		}

		backend = st8ger.NewBackend(config, logger)
	})

	Context("CreateStagingTask", func() {

		var (
			request    cc_messages.StagingRequestFromCC
			task       opi.Task
			backenderr error
		)

		BeforeEach(func() {
			buildpackStagingData := cc_messages.BuildpackStagingData{
				AppBitsDownloadUri:             "http://example-uri/download",
				BuildArtifactsCacheDownloadUri: "http://example-uri/shubidu",
				BuildArtifactsCacheUploadUri:   "http://example-uri.com/bunny-uppings",
				DropletUploadUri:               "http://example-uri.com/droplet-upload",
			}

			lifecycleDataJSON, err := json.Marshal(buildpackStagingData)
			Expect(err).ToNot(HaveOccurred())

			lifecycleData := json.RawMessage(lifecycleDataJSON)
			request = cc_messages.StagingRequestFromCC{
				AppId:              "appid",
				LogGuid:            "appid",
				LifecycleData:      &lifecycleData,
				CompletionCallback: "http://call-me.back",
			}

			task, backenderr = backend.CreateStagingTask("staging-guid", request)
			Expect(backenderr).ToNot(HaveOccurred())
		})

		It("should create a staging task with the required env vars", func() {
			Expect(task.Env["DOWNLOAD_URL"]).To(Equal("http://example-uri/download"))
			Expect(task.Env["UPLOAD_URL"]).To(Equal("http://example-uri.com/droplet-upload"))
			Expect(task.Env["APP_ID"]).To(Equal("appid"))
			Expect(task.Env["STAGING_GUID"]).To(Equal("staging-guid"))
			Expect(task.Env["COMPLETION_CALLBACK"]).To(Equal("http://call-me.back"))
			Expect(task.Env["CF_USERNAME"]).To(Equal("admin"))
			Expect(task.Env["CF_PASSWORD"]).To(Equal("admin"))
			Expect(task.Env["API_ADDRESS"]).To(Equal("api.mycf.com"))
		})

		It("should create a staging task with the right image name", func() {
			Expect(task.Image).To(Equal("diegoteam/recipe:build"))
		})
	})
})
