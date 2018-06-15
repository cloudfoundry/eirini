package bifrost_test

import (
	"net/http"

	"code.cloudfoundry.org/bbs/models"
	"code.cloudfoundry.org/eirini/bifrost"
	"code.cloudfoundry.org/eirini/eirinifakes"
	"code.cloudfoundry.org/eirini/opi"
	"code.cloudfoundry.org/lager/lagertest"
	"code.cloudfoundry.org/runtimeschema/cc_messages"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/ghttp"
)

var _ = Describe("Convert CC DesiredApp into an opi LRP", func() {
	var (
		cfClient         *eirinifakes.FakeCfClient
		fakeServer       *ghttp.Server
		logger           *lagertest.TestLogger
		client           *http.Client
		lrp              opi.LRP
		desireAppRequest cc_messages.DesireAppRequestFromCC
	)

	BeforeEach(func() {
		desireAppRequest = cc_messages.DesireAppRequestFromCC{
			ProcessGuid:    "b194809b-88c0-49af-b8aa-69da097fc360-2fdc448f-6bac-4085-9426-87d0124c433a",
			DropletHash:    "the-droplet-hash",
			DockerImageUrl: "the-image-url",
			NumInstances:   3,
			Environment: []*models.EnvironmentVariable{
				&models.EnvironmentVariable{
					Name:  "VCAP_APPLICATION",
					Value: `{"application_name":"bumblebee", "space_name":"transformers", "application_id":"b194809b-88c0-49af-b8aa-69da097fc360", "version": "something-something-uuid", "application_uris":["bumblebee.example.com", "transformers.example.com"]}`,
				},
			},
		}
	})

	JustBeforeEach(func() {
		cfClient = new(eirinifakes.FakeCfClient)
		fakeServer = ghttp.NewServer()
		logger = lagertest.NewTestLogger("test")
		client = &http.Client{}
		fakeServer.AppendHandlers(
			ghttp.VerifyRequest("POST", "/v2/transformers/bumblebee/blobs/"),
		)

		regIP := "eirini-registry.service.cf.internal"
		lrp = bifrost.Convert(desireAppRequest, fakeServer.URL(), regIP, cfClient, client, logger)
	})

	AfterSuite(func() {
		fakeServer.Close()
	})

	It("Directly converts DockerImageURL, Instances fields", func() {
		Expect(lrp.Image).To(Equal("the-image-url"))
		Expect(lrp.TargetInstances).To(Equal(3))
	})

	It("should set the lrp.Name equal to the app id", func() {
		Expect(lrp.Name).To(Equal("b194809b-88c0-49af-b8aa-69da097fc360"))
	})

	It("stores the VCAP env variable as metadata", func() {
		Expect(lrp.Metadata["application_name"]).To(Equal("bumblebee"))
		Expect(lrp.Metadata["application_id"]).To(Equal("b194809b-88c0-49af-b8aa-69da097fc360"))
		Expect(lrp.Metadata["version"]).To(Equal("something-something-uuid"))
	})

	It("stores the process guid in metadata", func() {
		Expect(lrp.Metadata["process_guid"]).To(Equal("b194809b-88c0-49af-b8aa-69da097fc360-2fdc448f-6bac-4085-9426-87d0124c433a"))
	})

	Context("When the Docker Image Url is not provided", func() {
		BeforeEach(func() {
			desireAppRequest.DockerImageUrl = ""
		})

		It("Converts droplet apps via the special registry URL", func() {
			Expect(lrp.Image).To(Equal("eirini-registry.service.cf.internal/cloudfoundry/app-name:the-droplet-hash"))
		})
	})
})
