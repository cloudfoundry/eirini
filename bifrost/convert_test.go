package bifrost_test

import (
	"encoding/json"

	"code.cloudfoundry.org/eirini"

	"code.cloudfoundry.org/eirini/bifrost"
	"code.cloudfoundry.org/eirini/models/cf"
	"code.cloudfoundry.org/eirini/opi"
	"code.cloudfoundry.org/lager/lagertest"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/ghttp"
)

var _ = Describe("Convert CC DesiredApp into an opi LRP", func() {
	var (
		fakeServer       *ghttp.Server
		logger           *lagertest.TestLogger
		lrp              opi.LRP
		err              error
		desireLRPRequest cf.DesireLRPRequest
		converter        bifrost.Converter
	)

	BeforeEach(func() {
		fakeServer = ghttp.NewServer()
		logger = lagertest.NewTestLogger("test")
		fakeServer.AppendHandlers(
			ghttp.VerifyRequest("POST", "/v2/transformers/bumblebee/blobs/"),
		)
		updatedRoutes := []map[string]interface{}{
			{
				"hostname": "bumblebee.example.com",
				"port":     8080,
			},
			{
				"hostname": "transformers.example.com",
				"port":     7070,
			},
		}

		routesJSON, marshalErr := json.Marshal(updatedRoutes)
		Expect(marshalErr).ToNot(HaveOccurred())

		rawJSON := json.RawMessage(routesJSON)
		desireLRPRequest = cf.DesireLRPRequest{
			GUID:           "b194809b-88c0-49af-b8aa-69da097fc360",
			Version:        "2fdc448f-6bac-4085-9426-87d0124c433a",
			ProcessGUID:    "b194809b-88c0-49af-b8aa-69da097fc360-2fdc448f-6bac-4085-9426-87d0124c433a",
			DropletHash:    "the-droplet-hash",
			DropletGUID:    "the-droplet-guid",
			DockerImageURL: "the-image-url",
			LastUpdated:    "23534635232.3",
			NumInstances:   3,
			MemoryMB:       456,
			CPUWeight:      50,
			Environment: map[string]string{
				"VCAP_APPLICATION": `{"application_name":"bumblebee", "space_name":"transformers", "application_id":"b194809b-88c0-49af-b8aa-69da097fc360", "version": "something-something-uuid", "application_uris":["bumblebee.example.com", "transformers.example.com"]}`,
				"VCAP_SERVICES":    `"user-provided": [{"binding_name": "bind-it-like-beckham","credentials": {"password": "notpassword1","username": "admin"},"instance_name": "dora","name": "serve"}]`,
				"PORT":             "8080",
			},
			StartCommand:            "start me",
			HealthCheckType:         "http",
			HealthCheckHTTPEndpoint: "/heat",
			HealthCheckTimeoutMs:    400,
			Ports:                   []int32{8080, 8888},
			Routes: map[string]*json.RawMessage{
				"cf-router": &rawJSON,
			},
			VolumeMounts: []cf.VolumeMount{
				{
					VolumeID: "claim-one",
					MountDir: "/path/one",
				},
				{
					VolumeID: "claim-two",
					MountDir: "/path/two",
				},
			},
		}
	})

	JustBeforeEach(func() {
		regIP := "eirini-registry.service.cf.internal"
		converter = bifrost.NewConverter(logger, regIP)
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

			It("should set the app name", func() {
				Expect(lrp.AppName).To(Equal("bumblebee"))
			})

			It("should set the space name", func() {
				Expect(lrp.SpaceName).To(Equal("transformers"))
			})

			It("should set the correct TargetInstances", func() {
				Expect(lrp.TargetInstances).To(Equal(3))
			})

			It("should set the correct identifier", func() {
				Expect(lrp.GUID).To(Equal("b194809b-88c0-49af-b8aa-69da097fc360"))
				Expect(lrp.Version).To(Equal("2fdc448f-6bac-4085-9426-87d0124c433a"))
			})

			It("should set the lrp memory", func() {
				Expect(lrp.CPUWeight).To(Equal(uint8(50)))
			})

			It("should set the lrp memory", func() {
				Expect(lrp.MemoryMB).To(Equal(int64(456)))
			})

			It("should store the VCAP env variable as metadata", func() {
				Expect(lrp.Metadata[cf.VcapAppName]).To(Equal("bumblebee"))
				Expect(lrp.Metadata[cf.VcapAppID]).To(Equal("b194809b-88c0-49af-b8aa-69da097fc360"))
				Expect(lrp.Metadata[cf.VcapVersion]).To(Equal("something-something-uuid"))
			})

			It("should store the process guid in metadata", func() {
				Expect(lrp.Metadata[cf.ProcessGUID]).To(Equal("b194809b-88c0-49af-b8aa-69da097fc360-2fdc448f-6bac-4085-9426-87d0124c433a"))
			})

			It("should store the last updated timestamp in metadata", func() {
				Expect(lrp.Metadata[cf.LastUpdated]).To(Equal("23534635232.3"))
			})

			It("should set the start command", func() {
				Expect(lrp.Command).To(Equal(append(eirini.InitProcess, eirini.Launch)))
			})

			It("should set the VCAP_APPLICATION environment variable", func() {
				val, ok := lrp.Env["VCAP_APPLICATION"]
				Expect(ok).To(BeTrue())
				Expect(val).To(Equal(desireLRPRequest.Environment["VCAP_APPLICATION"]))
			})

			It("should set the VCAP_SERVICES environment variable", func() {
				val, ok := lrp.Env["VCAP_SERVICES"]
				Expect(ok).To(BeTrue())
				Expect(val).To(Equal(desireLRPRequest.Environment["VCAP_SERVICES"]))
			})

			It("should set the launcher specific environment variables", func() {
				val, ok := lrp.Env["PORT"]
				Expect(ok).To(BeTrue())
				Expect(val).To(Equal("8080"))
			})

			It("should set the start command env variable", func() {
				val, ok := lrp.Env["START_COMMAND"]
				Expect(ok).To(BeTrue())
				Expect(val).To(Equal("start me"))
			})

			It("sets the healthcheck information", func() {
				health := lrp.Health
				Expect(health.Type).To(Equal("http"))
				Expect(health.Port).To(Equal(int32(8080)))
				Expect(health.Endpoint).To(Equal("/heat"))
				Expect(health.TimeoutMs).To(Equal(uint(400)))
			})

			It("sets the app routes", func() {
				Expect(lrp.Metadata[cf.VcapAppUris]).To(Equal(`[{"hostname":"bumblebee.example.com","port":8080},{"hostname":"transformers.example.com","port":7070}]`))
			})

			It("should set the ports", func() {
				Expect(lrp.Ports).To(Equal([]int32{8080, 8888}))
			})

			It("should set the volume mounts", func() {
				volumes := lrp.VolumeMounts
				Expect(len(volumes)).To(Equal(2))
				Expect(volumes).To(ContainElement(opi.VolumeMount{
					ClaimName: "claim-one",
					MountPath: "/path/one",
				}))
				Expect(volumes).To(ContainElement(opi.VolumeMount{
					ClaimName: "claim-two",
					MountPath: "/path/two",
				}))
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
				desireLRPRequest.DockerImageURL = ""
			})

			Context("when the staging is successful", func() {
				BeforeEach(func() {
					fakeServer.SetHandler(0, ghttp.RespondWith(201, nil))
				})

				It("should convert droplet apps via the special registry URL", func() {
					Expect(lrp.Image).To(Equal("eirini-registry.service.cf.internal/cloudfoundry/the-droplet-guid:the-droplet-hash"))
				})

				verifyLRPConvertedSuccessfully()
			})
		})

	})

	Context("When the request fails to be converted", func() {
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
