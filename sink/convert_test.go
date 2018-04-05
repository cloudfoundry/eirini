package sink_test

import (
	"net/http"

	"code.cloudfoundry.org/bbs/models"
	"code.cloudfoundry.org/lager/lagertest"
	"code.cloudfoundry.org/runtimeschema/cc_messages"
	"github.com/julz/cube/cubefakes"
	"github.com/julz/cube/opi"
	"github.com/julz/cube/sink"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/ghttp"
)

var _ = Describe("Convert CC DesiredApp into an opi LRP", func() {
	var (
		cfClient   *cubefakes.FakeCfClient
		fakeServer *ghttp.Server
		logger     *lagertest.TestLogger
		client     *http.Client
		lrp        opi.LRP
		regIP      string
	)

	BeforeEach(func() {
		cfClient = new(cubefakes.FakeCfClient)
		fakeServer = ghttp.NewServer()
		logger = lagertest.NewTestLogger("test")
		client = &http.Client{}
		regIP = "cube-registry.service.cf.internal"
		fakeServer.AppendHandlers(
			ghttp.VerifyRequest("POST", "/v2/transformers/bumblebee/blobs/"),
		)
	})

	AfterEach(func() {
		fakeServer.Close()
	})

	It("Directly converts DockerImageURL, Instances fields", func() {
		lrp := sink.Convert(cc_messages.DesireAppRequestFromCC{
			DockerImageUrl: "the-image-url",
			NumInstances:   3,
		}, fakeServer.URL(), regIP, cfClient, client, logger)

		Expect(lrp.Image).To(Equal("the-image-url"))
		Expect(lrp.TargetInstances).To(Equal(3))
	})

	BeforeEach(func() {
		lrp = sink.Convert(cc_messages.DesireAppRequestFromCC{
			ProcessGuid: "b194809b-88c0-49af-b8aa-69da097fc360-2fdc448f-6bac-4085-9426-87d0124c433a",
			DropletHash: "the-droplet-hash",
			Environment: []*models.EnvironmentVariable{
				&models.EnvironmentVariable{
					Name:  "VCAP_APPLICATION",
					Value: `{"name":"bumblebee", "space_name":"transformers", "application_id":"1234"}`,
				},
			},
		}, fakeServer.URL(), regIP, cfClient, client, logger)
	})

	// a processGuid is <app-guid>-<version-guid>
	It("truncates the ProcessGuid so it only contain the app guid", func() {
		Expect(lrp.Name).To(Equal("b194809b-88c0-49af-b8aa-69da097fc360"))
	})

	It("Converts droplet apps via the special registry URL", func() {
		Expect(lrp.Image).To(Equal("cube-registry.service.cf.internal:8080/cloudfoundry/app-name:the-droplet-hash"))
	})
})
