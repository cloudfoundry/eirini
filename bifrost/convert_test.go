package bifrost_test

import (
	"errors"
	"net/http"

	"code.cloudfoundry.org/eirini/bifrost"
	"code.cloudfoundry.org/eirini/eirinifakes"
	"code.cloudfoundry.org/eirini/models/cf"
	"code.cloudfoundry.org/eirini/opi"
	"code.cloudfoundry.org/lager/lagertest"
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
		err              error
		registryURL      string
		desireLRPRequest cf.DesireLRPRequest
		converter        bifrost.Converter
	)

	BeforeEach(func() {
		cfClient = new(eirinifakes.FakeCfClient)
		fakeServer = ghttp.NewServer()
		registryURL = fakeServer.URL()
		logger = lagertest.NewTestLogger("test")
		client = &http.Client{}
		fakeServer.AppendHandlers(
			ghttp.VerifyRequest("POST", "/v2/transformers/bumblebee/blobs/"),
		)
		desireLRPRequest = cf.DesireLRPRequest{
			ProcessGuid:    "b194809b-88c0-49af-b8aa-69da097fc360-2fdc448f-6bac-4085-9426-87d0124c433a",
			DropletHash:    "the-droplet-hash",
			DockerImageUrl: "the-image-url",
			LastUpdated:    "23534635232.3",
			NumInstances:   3,
			Environment: map[string]string{
				"VCAP_APPLICATION": `{"application_name":"bumblebee", "space_name":"transformers", "application_id":"b194809b-88c0-49af-b8aa-69da097fc360", "version": "something-something-uuid", "application_uris":["bumblebee.example.com", "transformers.example.com"]}`,
			},
		}
	})

	JustBeforeEach(func() {
		regIP := "eirini-registry.service.cf.internal"
		converter = bifrost.NewConverter(cfClient, client, logger, regIP, registryURL)
		lrp, err = converter.Convert(desireLRPRequest)
	})

	AfterEach(func() {
		fakeServer.Close()
	})

	Context("When request is converted successfully", func() {
		verifyLRPConvertedSuccessfully := func() {
			It("should not return an error", func() {
				Expect(err).ToNot(HaveOccurred())
			})

			It("should set the lrp.Name and TargetInstances", func() {
				Expect(lrp.Name).To(Equal("b194809b-88c0-49af-b8aa-69da097fc360"))
				Expect(lrp.TargetInstances).To(Equal(3))
			})

			It("should store the VCAP env variable as metadata", func() {
				Expect(lrp.Metadata[cf.VcapAppName]).To(Equal("bumblebee"))
				Expect(lrp.Metadata[cf.VcapAppId]).To(Equal("b194809b-88c0-49af-b8aa-69da097fc360"))
				Expect(lrp.Metadata[cf.VcapVersion]).To(Equal("something-something-uuid"))
			})

			It("should store the process guid in metadata", func() {
				Expect(lrp.Metadata[cf.ProcessGuid]).To(Equal("b194809b-88c0-49af-b8aa-69da097fc360-2fdc448f-6bac-4085-9426-87d0124c433a"))
			})

			It("should store the last updated timestamp in metadata", func() {
				Expect(lrp.Metadata[cf.LastUpdated]).To(Equal("23534635232.3"))
			})
		}

		Context("When the Docker image is provided", func() {
			It("should directly convert DockerImageURL", func() {
				Expect(lrp.Image).To(Equal("the-image-url"))
			})

			verifyLRPConvertedSuccessfully()
		})

		Context("When the Docker Image Url is not provided", func() {
			BeforeEach(func() {
				desireLRPRequest.DockerImageUrl = ""
			})

			Context("when the staging is successful", func() {
				BeforeEach(func() {
					fakeServer.SetHandler(0, ghttp.RespondWith(201, nil))
				})

				It("should convert droplet apps via the special registry URL", func() {
					Expect(lrp.Image).To(Equal("eirini-registry.service.cf.internal/cloudfoundry/app-name:the-droplet-hash"))
				})

				verifyLRPConvertedSuccessfully()
			})
		})

	})

	Context("When the request fails to be converted", func() {
		Context("when registry staging fails", func() {
			BeforeEach(func() {
				desireLRPRequest.DockerImageUrl = ""
			})

			Context("because of invalid url", func() {
				BeforeEach(func() {
					registryURL = "this_does_not_exist"
				})

				It("should return an error", func() {
					Expect(err).To(HaveOccurred())
				})
			})

			Context("because of bad response code", func() {
				BeforeEach(func() {
					fakeServer.SetHandler(0, ghttp.RespondWith(500, nil))
				})

				It("should return an error", func() {
					Expect(err).To(HaveOccurred())
				})
			})

			Context("when the app droplet is not available", func() {
				BeforeEach(func() {
					cfClient.GetDropletByAppGuidReturns([]byte{}, errors.New("droplet-not-found"))
				})

				It("should return an error", func() {
					Expect(err).To(HaveOccurred())
				})
			})
		})

		Context("When VCAP_APPLICATION env variable is invalid", func() {
			BeforeEach(func() {
				desireLRPRequest.Environment = map[string]string{
					"VCAP_APPLICATION": `{something is wrong`,
				}
			})

			It("should return an error", func() {
				Expect(err).To(HaveOccurred())
			})
		})
	})

})
