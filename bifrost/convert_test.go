package bifrost_test

import (
	"encoding/json"

	"code.cloudfoundry.org/eirini/bifrost"
	"code.cloudfoundry.org/eirini/models/cf"
	"code.cloudfoundry.org/eirini/opi"
	"code.cloudfoundry.org/lager/lagertest"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Convert CC DesiredApp into an opi LRP", func() {
	const defaultDiskQuota = int64(2058)
	var (
		logger           *lagertest.TestLogger
		lrp              opi.LRP
		err              error
		desireLRPRequest cf.DesireLRPRequest
		converter        bifrost.Converter
	)

	BeforeEach(func() {
		logger = lagertest.NewTestLogger("converter-test")
		updatedRoutes := []map[string]interface{}{
			{
				"hostname": "bumblebee.example.com",
				"port":     8000,
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
			GUID:             "capi-process-guid-01cba02034f1",
			Version:          "capi-process-version-87d0124c433a",
			ProcessGUID:      "capi-process-guid-69da097fc360-capi-process-version-87d0124c433a",
			ProcessType:      "web",
			LastUpdated:      "23534635232.3",
			NumInstances:     3,
			MemoryMB:         456,
			DiskMB:           256,
			CPUWeight:        50,
			AppGUID:          "app-guid-69da097fc360",
			AppName:          "bumblebee",
			SpaceName:        "transformers",
			OrganizationName: "marvel",
			Environment: map[string]string{
				"VAR_FROM_CC": "val from cc",
			},
			HealthCheckType:         "http",
			HealthCheckHTTPEndpoint: "/heat",
			HealthCheckTimeoutMs:    400,
			Ports:                   []int32{8000, 8888},
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
			LRP: "full LRP request",
		}
	})

	JustBeforeEach(func() {
		regIP := "eirini-registry.service.cf.internal"
		converter = bifrost.NewConverter(logger, regIP, defaultDiskQuota)
		lrp, err = converter.Convert(desireLRPRequest)
		Expect(err).ToNot(HaveOccurred())
	})

	Context("When request is converted successfully", func() {
		verifyLRPConvertedSuccessfully := func() {
			It("should set the app name", func() {
				Expect(lrp.AppName).To(Equal("bumblebee"))
			})

			It("should set the app guid", func() {
				Expect(lrp.AppGUID).To(Equal("app-guid-69da097fc360"))
			})

			It("should set the space name", func() {
				Expect(lrp.SpaceName).To(Equal("transformers"))
			})

			It("should set the org name", func() {
				Expect(lrp.OrgName).To(Equal("marvel"))
			})

			It("should set the correct TargetInstances", func() {
				Expect(lrp.TargetInstances).To(Equal(3))
			})

			It("should set the correct identifier", func() {
				Expect(lrp.GUID).To(Equal("capi-process-guid-01cba02034f1"))
				Expect(lrp.Version).To(Equal("capi-process-version-87d0124c433a"))
			})

			It("should set the process type", func() {
				Expect(lrp.ProcessType).To(Equal("web"))
			})

			It("should set the lrp memory", func() {
				Expect(lrp.CPUWeight).To(Equal(uint8(50)))
			})

			It("should set the lrp memory", func() {
				Expect(lrp.MemoryMB).To(Equal(int64(456)))
			})

			It("should set the lrp disk", func() {
				Expect(lrp.DiskMB).To(Equal(int64(256)))
			})

			It("should set the app name", func() {
				Expect(lrp.AppName).To(Equal("bumblebee"))
			})

			It("should set the app guid", func() {
				Expect(lrp.AppGUID).To(Equal("app-guid-69da097fc360"))
			})

			It("should store the last updated timestamp in metadata", func() {
				Expect(lrp.LastUpdated).To(Equal("23534635232.3"))
			})

			It("should set the environment variables provided by cloud controller", func() {
				Expect(lrp.Env).To(HaveKeyWithValue("VAR_FROM_CC", desireLRPRequest.Environment["VAR_FROM_CC"]))
			})

			It("should set CF_INSTANCE_* env variables", func() {
				Expect(lrp.Env).To(HaveKeyWithValue("CF_INSTANCE_ADDR", "0.0.0.0:8080"))
				Expect(lrp.Env).To(HaveKeyWithValue("CF_INSTANCE_PORT", "8080"))
				Expect(lrp.Env).To(HaveKeyWithValue("CF_INSTANCE_PORTS", MatchJSON(`[{"external": 8080, "internal": 8080}]`)))
			})

			It("should set LANG env variable", func() {
				Expect(lrp.Env).To(HaveKeyWithValue("LANG", "en_US.UTF-8"))
			})

			It("sets the app routes", func() {
				Expect(lrp.AppURIs).To(Equal(`[{"hostname":"bumblebee.example.com","port":8000},{"hostname":"transformers.example.com","port":7070}]`))
			})

			It("should set the ports", func() {
				Expect(lrp.Ports).To(Equal([]int32{8000, 8888}))
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

			It("should set the LRP request", func() {
				Expect(lrp.LRP).To(Equal("full LRP request"))
			})
		}

		Context("when the disk quota is not provided", func() {
			BeforeEach(func() {
				desireLRPRequest.Lifecycle = cf.Lifecycle{
					BuildpackLifecycle: &cf.BuildpackLifecycle{},
				}
				desireLRPRequest.DiskMB = 0
			})

			It("should use the default disk quota", func() {
				Expect(lrp.DiskMB).To(Equal(defaultDiskQuota))
			})

		})

		Context("When the app is using docker lifecycle", func() {
			BeforeEach(func() {
				desireLRPRequest.Lifecycle = cf.Lifecycle{
					DockerLifecycle: &cf.DockerLifecycle{
						Image:   "the-image-url",
						Command: []string{"command-in-docker"},
					},
				}
			})
			It("should directly convert DockerImageURL", func() {
				Expect(lrp.Image).To(Equal("the-image-url"))
			})

			It("should set command from docker lifecycle", func() {
				Expect(lrp.Command).To(Equal([]string{"command-in-docker"}))
			})

			It("sets the healthcheck information", func() {
				health := lrp.Health
				Expect(health.Type).To(Equal("http"))
				Expect(health.Port).To(Equal(int32(8000)))
				Expect(health.Endpoint).To(Equal("/heat"))
				Expect(health.TimeoutMs).To(Equal(uint(400)))
			})

			verifyLRPConvertedSuccessfully()
		})

		Context("When the app is using buildpack lifecycle", func() {
			BeforeEach(func() {
				desireLRPRequest.Lifecycle = cf.Lifecycle{
					BuildpackLifecycle: &cf.BuildpackLifecycle{
						DropletHash:  "the-droplet-hash",
						DropletGUID:  "the-droplet-guid",
						StartCommand: "start me",
					}}
			})

			It("sets the healthcheck information", func() {
				health := lrp.Health
				Expect(health.Type).To(Equal("http"))
				Expect(health.Port).To(Equal(int32(8080)))
				Expect(health.Endpoint).To(Equal("/heat"))
				Expect(health.TimeoutMs).To(Equal(uint(400)))
			})

			It("should convert droplet apps via the special registry URL", func() {
				Expect(lrp.Image).To(Equal("eirini-registry.service.cf.internal/cloudfoundry/the-droplet-guid:the-droplet-hash"))
			})

			It("should set buildpack specific env variables", func() {
				Expect(lrp.Env).To(HaveKeyWithValue("HOME", "/home/vcap/app"))
				Expect(lrp.Env).To(HaveKeyWithValue("PATH", "/usr/local/bin:/usr/bin:/bin"))
				Expect(lrp.Env).To(HaveKeyWithValue("USER", "vcap"))
				Expect(lrp.Env).To(HaveKeyWithValue("TMPDIR", "/home/vcap/tmp"))
				Expect(lrp.Env).To(HaveKeyWithValue("START_COMMAND", "start me"))
			})

			verifyLRPConvertedSuccessfully()
		})
	})
})
