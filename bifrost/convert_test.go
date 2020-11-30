package bifrost_test

import (
	"encoding/json"
	"errors"

	"code.cloudfoundry.org/eirini"
	"code.cloudfoundry.org/eirini/bifrost"
	"code.cloudfoundry.org/eirini/bifrost/bifrostfakes"
	"code.cloudfoundry.org/eirini/models/cf"
	"code.cloudfoundry.org/eirini/opi"
	"code.cloudfoundry.org/lager/lagertest"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	v1 "github.com/opencontainers/image-spec/specs-go/v1"
)

var _ = Describe("OPI Converter", func() {
	var (
		logger              *lagertest.TestLogger
		err                 error
		converter           *bifrost.OPIConverter
		imgMetadataFetcher  *bifrostfakes.FakeImageMetadataFetcher
		imgRefParser        *bifrostfakes.FakeImageRefParser
		allowRunImageAsRoot bool
	)

	BeforeEach(func() {
		logger = lagertest.NewTestLogger("converter-test")
		imgMetadataFetcher = new(bifrostfakes.FakeImageMetadataFetcher)
		imgRefParser = new(bifrostfakes.FakeImageRefParser)
		allowRunImageAsRoot = false
	})

	JustBeforeEach(func() {
		converter = bifrost.NewOPIConverter(
			logger,
			imgMetadataFetcher.Spy,
			imgRefParser.Spy,
			allowRunImageAsRoot,
		)
	})

	Describe("Convert CC DesiredApp into an opi LRP", func() {
		var (
			lrp              opi.LRP
			desireLRPRequest cf.DesireLRPRequest
		)

		BeforeEach(func() {
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
				Namespace:        "namespace",
				Environment: map[string]string{
					"VAR_FROM_CC": "val from cc",
				},
				HealthCheckType:         "http",
				HealthCheckHTTPEndpoint: "/heat",
				HealthCheckTimeoutMs:    400,
				Ports:                   []int32{8000, 8888},
				Routes: map[string]json.RawMessage{
					"cf-router": rawJSON,
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
				UserDefinedAnnotations: map[string]string{
					"prometheus.io/scrape": "scrape",
				},
				Lifecycle: cf.Lifecycle{
					DockerLifecycle: &cf.DockerLifecycle{},
				},
			}
		})

		JustBeforeEach(func() {
			lrp, err = converter.ConvertLRP(desireLRPRequest)
		})

		It("should not error", func() {
			Expect(err).ToNot(HaveOccurred())
		})

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
			Expect(lrp.Env).To(HaveKeyWithValue(eirini.EnvCFInstanceAddr, "0.0.0.0:8000"))
			Expect(lrp.Env).To(HaveKeyWithValue(eirini.EnvCFInstancePort, "8000"))
			Expect(lrp.Env).To(HaveKeyWithValue(eirini.EnvCFInstancePorts, MatchJSON(`[{"external": 8000, "internal": 8000}]`)))
		})

		It("should set LANG env variable", func() {
			Expect(lrp.Env).To(HaveKeyWithValue("LANG", "en_US.UTF-8"))
		})

		It("sets the app routes", func() {
			Expect(lrp.AppURIs).To(ConsistOf(
				opi.Route{Hostname: "bumblebee.example.com", Port: 8000},
				opi.Route{Hostname: "transformers.example.com", Port: 7070},
			))
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

		It("should set user defined annotation", func() {
			Expect(lrp.UserDefinedAnnotations["prometheus.io/scrape"]).To(Equal("scrape"))
		})

		Context("when no ports are specified", func() {
			BeforeEach(func() {
				desireLRPRequest.Ports = []int32{}
			})

			It("should not error", func() {
				Expect(err).ToNot(HaveOccurred())
			})

			It("should not set the port-dependent env vars", func() {
				Expect(lrp.Env).NotTo(HaveKey(eirini.EnvCFInstanceAddr))
				Expect(lrp.Env).NotTo(HaveKey(eirini.EnvCFInstancePort))
				Expect(lrp.Env).NotTo(HaveKey(eirini.EnvCFInstancePorts))
			})

			It("defaults the healthcheck port to 0", func() {
				Expect(lrp.Health.Port).To(BeZero())
			})
		})

		Context("when the disk quota is not provided", func() {
			BeforeEach(func() {
				desireLRPRequest.DiskMB = 0
			})

			It("fails", func() {
				Expect(err).To(MatchError("DiskMB cannot be 0"))
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

			It("shouldn't set privateRegistry information", func() {
				Expect(lrp.PrivateRegistry).To(BeNil())
			})

			It("assumes that the pod should run as root", func() {
				Expect(lrp.RunsAsRoot).To(BeFalse())
			})

			Context("when the image lives in a private registry", func() {
				BeforeEach(func() {
					desireLRPRequest.Lifecycle = cf.Lifecycle{
						DockerLifecycle: &cf.DockerLifecycle{
							Image:            "my-secret-docker-registry.docker.io:5000/repo/the-mighty-image:not-latest",
							Command:          []string{"command-in-docker"},
							RegistryUsername: "super-user",
							RegistryPassword: "super-password",
						},
					}
				})

				It("should set the docker image url", func() {
					Expect(lrp.Image).To(Equal("my-secret-docker-registry.docker.io:5000/repo/the-mighty-image:not-latest"))
				})

				It("should provide information about the private regisry", func() {
					Expect(lrp.PrivateRegistry).ToNot(BeNil())
					Expect(lrp.PrivateRegistry.Username).To(Equal("super-user"))
					Expect(lrp.PrivateRegistry.Password).To(Equal("super-password"))
					Expect(lrp.PrivateRegistry.Server).To(Equal("my-secret-docker-registry.docker.io"))
				})

				Context("and the registry URL does not contain a host", func() {
					BeforeEach(func() {
						desireLRPRequest.Lifecycle = cf.Lifecycle{
							DockerLifecycle: &cf.DockerLifecycle{
								Image:            "repo/the-mighty-image:not-latest",
								Command:          []string{"command-in-docker"},
								RegistryUsername: "super-user",
								RegistryPassword: "super-password",
							},
						}
					})

					It("should default to the docker hub", func() {
						Expect(lrp.PrivateRegistry.Server).To(Equal("index.docker.io/v1/"))
					})
				})
			})

			Context("when running docker images with root user is allowed", func() {
				BeforeEach(func() {
					allowRunImageAsRoot = true
					imgMetadataFetcher.Returns(&v1.ImageConfig{}, nil)
					imgRefParser.Returns("//some-docker-image-ref", nil)
				})

				It("should parse the docker image ref", func() {
					Expect(imgRefParser.CallCount()).To(Equal(1))
					Expect(imgRefParser.ArgsForCall(0)).To(Equal(lrp.Image))
				})

				It("should fetch the image metadata", func() {
					Expect(imgMetadataFetcher.CallCount()).To(Equal(1))
					dockerRef, sysCtx := imgMetadataFetcher.ArgsForCall(0)

					Expect(dockerRef).To(Equal("//some-docker-image-ref"))
					Expect(sysCtx.DockerAuthConfig.Username).To(BeEmpty())
					Expect(sysCtx.DockerAuthConfig.Password).To(BeEmpty())
				})

				Context("and the image user is root", func() {
					BeforeEach(func() {
						imgMetadataFetcher.Returns(&v1.ImageConfig{
							User: "root",
						}, nil)
					})

					It("should be allowed to run as root", func() {
						Expect(lrp.RunsAsRoot).To(BeTrue())
					})
				})

				Context("and the image user is empty", func() {
					BeforeEach(func() {
						imgMetadataFetcher.Returns(&v1.ImageConfig{
							User: "",
						}, nil)
					})

					It("should be allowed to run as root", func() {
						Expect(lrp.RunsAsRoot).To(BeTrue())
					})
				})

				Context("and the image user is UID 0", func() {
					BeforeEach(func() {
						imgMetadataFetcher.Returns(&v1.ImageConfig{
							User: "0",
						}, nil)
					})

					It("should be allowed to run as root", func() {
						Expect(lrp.RunsAsRoot).To(BeTrue())
					})
				})

				Context("and the image user is not root", func() {
					BeforeEach(func() {
						imgMetadataFetcher.Returns(&v1.ImageConfig{
							User: "vcap",
						}, nil)
					})

					It("should not be allowed to run as root", func() {
						Expect(lrp.RunsAsRoot).To(BeFalse())
					})
				})

				Context("when the image lives in a private registry", func() {
					BeforeEach(func() {
						desireLRPRequest.Lifecycle = cf.Lifecycle{
							DockerLifecycle: &cf.DockerLifecycle{
								Image:            "my-secret-docker-registry.docker.io:5000/repo/the-mighty-image:not-latest",
								Command:          []string{"command-in-docker"},
								RegistryUsername: "super-user",
								RegistryPassword: "super-password",
							},
						}
					})

					It("should provide username & password", func() {
						Expect(imgMetadataFetcher.CallCount()).To(Equal(1))
						_, sysCtx := imgMetadataFetcher.ArgsForCall(0)

						Expect(sysCtx.DockerAuthConfig.Username).To(Equal("super-user"))
						Expect(sysCtx.DockerAuthConfig.Password).To(Equal("super-password"))
					})
				})

				Context("when image ref parsing fails", func() {
					BeforeEach(func() {
						imgRefParser.Returns("", errors.New("uh-oh-parsing-failed"))
					})

					It("should propagate the error", func() {
						Expect(err).To(MatchError(ContainSubstring("uh-oh-parsing-failed")))
					})
				})

				Context("when metadata fetching fails", func() {
					BeforeEach(func() {
						imgMetadataFetcher.Returns(nil, errors.New("uh-oh-fetching-failed"))
					})

					It("should propagate the error", func() {
						Expect(err).To(MatchError(ContainSubstring("uh-oh-fetching-failed")))
					})
				})
			})
		})
	})

	Describe("Convert Task", func() {
		var (
			taskRequest cf.TaskRequest
			task        opi.Task
		)

		JustBeforeEach(func() {
			task, err = converter.ConvertTask("guid_1234", taskRequest)
		})

		When("the task has a docker lifecycle", func() {
			BeforeEach(func() {
				taskRequest = cf.TaskRequest{
					AppGUID:            "our-app-id",
					Name:               "task-name",
					Environment:        []cf.EnvironmentVariable{{Name: "HOWARD", Value: "the alien"}},
					CompletionCallback: "example.com/call/me/maybe",
					Lifecycle: cf.Lifecycle{
						DockerLifecycle: &cf.DockerLifecycle{
							Image:   "some/image",
							Command: []string{"some", "command"},
						},
					},
				}
			})

			It("should convert the task request", func() {
				Expect(err).NotTo(HaveOccurred())
				Expect(task).To(Equal(opi.Task{
					GUID:               "guid_1234",
					AppGUID:            "our-app-id",
					Name:               "task-name",
					CompletionCallback: "example.com/call/me/maybe",
					Env: map[string]string{
						"HOWARD": "the alien",
						"HOME":   "/home/vcap/app",
						"PATH":   "/usr/local/bin:/usr/bin:/bin",
						"USER":   "vcap",
						"TMPDIR": "/home/vcap/tmp",
					},
					Command: []string{"some", "command"},
					Image:   "some/image",
				}))
			})

			When("the docker image is in a private registry", func() {
				BeforeEach(func() {
					taskRequest.Lifecycle.DockerLifecycle.Image = "private-registry/some/image"
					taskRequest.Lifecycle.DockerLifecycle.RegistryUsername = "bob"
					taskRequest.Lifecycle.DockerLifecycle.RegistryPassword = "12345"
				})

				It("includes the private registry details in the conversion", func() {
					Expect(err).NotTo(HaveOccurred())
					Expect(task.PrivateRegistry).ToNot(BeNil())
					Expect(task.PrivateRegistry.Username).To(Equal("bob"))
					Expect(task.PrivateRegistry.Password).To(Equal("12345"))
					Expect(task.PrivateRegistry.Server).To(Equal("private-registry"))
				})
			})
		})

		When("the task does not have any docker lifecycle information", func() {
			BeforeEach(func() {
				taskRequest = cf.TaskRequest{
					AppGUID:            "our-app-id",
					Name:               "task-name",
					Environment:        []cf.EnvironmentVariable{{Name: "HOWARD", Value: "the alien"}},
					CompletionCallback: "example.com/call/me/maybe",
					Lifecycle:          cf.Lifecycle{},
				}
			})

			It("fails with a useful message", func() {
				Expect(err).To(MatchError("docker is the only supported lifecycle"))
			})
		})
	})
})
