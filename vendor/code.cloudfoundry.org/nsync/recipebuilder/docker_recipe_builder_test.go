package recipebuilder_test

import (
	"encoding/json"
	"errors"
	"time"

	"code.cloudfoundry.org/bbs/models"
	"code.cloudfoundry.org/diego-ssh/keys"
	"code.cloudfoundry.org/diego-ssh/keys/fake_keys"
	"code.cloudfoundry.org/diego-ssh/routes"
	"code.cloudfoundry.org/lager/lagertest"
	"code.cloudfoundry.org/nsync/recipebuilder"
	"code.cloudfoundry.org/nsync/test_helpers"
	"code.cloudfoundry.org/runtimeschema/cc_messages"
	"github.com/cloudfoundry-incubator/routing-info/cfroutes"
	"github.com/cloudfoundry-incubator/routing-info/tcp_routes"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
)

var _ = Describe("Docker Recipe Builder", func() {
	var (
		builder        *recipebuilder.DockerRecipeBuilder
		lifecycles     map[string]string
		egressRules    []*models.SecurityGroupRule
		networkInfo    *models.Network
		fakeKeyFactory *fake_keys.FakeSSHKeyFactory
		logger         *lagertest.TestLogger

		desiredCCVolumeMounts   []*cc_messages.VolumeMount
		expectedBBSVolumeMounts []*models.VolumeMount
	)

	defaultNofile := recipebuilder.DefaultFileDescriptorLimit

	BeforeEach(func() {
		logger = lagertest.NewTestLogger("test")
		lifecycles = map[string]string{
			"buildpack/some-stack": "some-lifecycle.tgz",
			"docker":               "the/docker/lifecycle/path.tgz",
		}

		egressRules = []*models.SecurityGroupRule{
			{
				Protocol:     "TCP",
				Destinations: []string{"0.0.0.0/0"},
				PortRange:    &models.PortRange{Start: 80, End: 443},
			},
		}

		networkInfo = &models.Network{
			Properties: map[string]string{
				"app_id":   "some-app-guid",
				"some_key": "some-value",
			},
		}

		fakeKeyFactory = &fake_keys.FakeSSHKeyFactory{}
		config := recipebuilder.Config{
			Lifecycles:    lifecycles,
			FileServerURL: "http://file-server.com",
			KeyFactory:    fakeKeyFactory,
		}

		builder = recipebuilder.NewDockerRecipeBuilder(logger, config)

		desiredCCVolumeMounts = []*cc_messages.VolumeMount{{
			Driver:       "testdriver",
			ContainerDir: "/Volumes/myvol",
			Mode:         "rw",
			DeviceType:   "shared",
			Device:       cc_messages.SharedDevice{VolumeId: "volumeId", MountConfig: map[string]interface{}{"key": "value"}},
		}}

		expectedBBSVolumeMounts = []*models.VolumeMount{{
			Driver:       "testdriver",
			ContainerDir: "/Volumes/myvol",
			Mode:         "rw",
			Shared: &models.SharedDevice{
				VolumeId:    "volumeId",
				MountConfig: `{"key":"value"}`,
			}},
		}
	})

	Describe("Build LRPs", func() {
		var (
			err            error
			desiredAppReq  cc_messages.DesireAppRequestFromCC
			desiredLRP     *models.DesiredLRP
			expectedRoutes models.Routes
		)

		BeforeEach(func() {
			routingInfo, err := cc_messages.CCHTTPRoutes{
				{Hostname: "route1"},
				{Hostname: "route2"},
			}.CCRouteInfo()
			Expect(err).NotTo(HaveOccurred())

			desiredAppReq = cc_messages.DesireAppRequestFromCC{
				ProcessGuid:       "the-app-guid-the-app-version",
				Stack:             "some-stack",
				StartCommand:      "the-start-command with-arguments",
				DockerImageUrl:    "user/repo:tag",
				ExecutionMetadata: "{}",
				Environment: []*models.EnvironmentVariable{
					{Name: "foo", Value: "bar"},
				},
				MemoryMB:        128,
				DiskMB:          512,
				FileDescriptors: 32,
				NumInstances:    23,
				RoutingInfo:     routingInfo,
				LogGuid:         "the-log-id",
				LogSource:       "MYSOURCE",

				HealthCheckType:             cc_messages.PortHealthCheckType,
				HealthCheckTimeoutInSeconds: 123456,

				EgressRules: egressRules,
				Network:     networkInfo,

				ETag: "etag-updated-at",
			}

			cfRoutes := json.RawMessage([]byte(`[{"hostnames":["route1","route2"],"port":8080}]`))
			tcpRoutes := json.RawMessage([]byte("[]"))
			expectedRoutes = models.Routes{
				cfroutes.CF_ROUTER:    &cfRoutes,
				tcp_routes.TCP_ROUTER: &tcpRoutes,
			}
		})

		JustBeforeEach(func() {
			desiredLRP, err = builder.Build(&desiredAppReq)
		})

		Describe("CPU weight calculation", func() {
			Context("when the memory limit is below the minimum value", func() {
				BeforeEach(func() {
					desiredAppReq.MemoryMB = recipebuilder.MinCpuProxy - 1
				})

				It("returns 100*MIN/MAX", func() {
					expectedWeight := (100 * recipebuilder.MinCpuProxy) / recipebuilder.MaxCpuProxy
					Expect(desiredLRP.CpuWeight).To(BeEquivalentTo(expectedWeight))
				})
			})

			Context("when the memory limit is above the maximum value", func() {
				BeforeEach(func() {
					desiredAppReq.MemoryMB = recipebuilder.MaxCpuProxy + 1
				})

				It("returns 100", func() {
					Expect(desiredLRP.CpuWeight).To(BeEquivalentTo(100))
				})
			})

			Context("when the memory limit is in between the minimum and maximum value", func() {
				BeforeEach(func() {
					desiredAppReq.MemoryMB = (recipebuilder.MinCpuProxy + recipebuilder.MaxCpuProxy) / 2
				})

				It("returns 100*M/MAX", func() {
					expectedWeight := (100 * desiredAppReq.MemoryMB) / recipebuilder.MaxCpuProxy
					Expect(desiredLRP.CpuWeight).To(BeEquivalentTo(expectedWeight))
				})
			})
		})

		Context("when ports is an empty array", func() {
			BeforeEach(func() {
				desiredAppReq.Ports = []uint32{}
			})

			It("does not include a PORT environment variable", func() {
				varNames := []string{}
				for _, envVar := range desiredLRP.EnvironmentVariables {
					varNames = append(varNames, envVar.Name)
				}
				Expect(varNames).NotTo(ContainElement("PORT"))
			})
		})

		Context("when everything is correct", func() {
			It("does not error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("builds a valid DesiredLRP", func() {
				Expect(desiredLRP.ProcessGuid).To(Equal("the-app-guid-the-app-version"))
				Expect(desiredLRP.Instances).To(BeEquivalentTo(23))
				Expect(desiredLRP.Routes).To(Equal(&expectedRoutes))

				Expect(desiredLRP.Annotation).To(Equal("etag-updated-at"))
				Expect(desiredLRP.RootFs).To(Equal("docker:///user/repo#tag"))
				Expect(desiredLRP.MemoryMb).To(BeEquivalentTo(128))
				Expect(desiredLRP.DiskMb).To(BeEquivalentTo(512))
				Expect(desiredLRP.Ports).To(Equal([]uint32{8080}))
				Expect(desiredLRP.Privileged).To(BeFalse())
				Expect(desiredLRP.StartTimeoutMs).To(BeEquivalentTo(123456000))

				Expect(desiredLRP.LogGuid).To(Equal("the-log-id"))
				Expect(desiredLRP.LogSource).To(Equal("CELL"))

				Expect(desiredLRP.EnvironmentVariables).To(BeEmpty())

				Expect(desiredLRP.MetricsGuid).To(Equal("the-log-id"))

				Expect(desiredLRP.Network).To(Equal(networkInfo))

				Expect(desiredLRP.PlacementTags).To(BeEmpty())

				expectedCachedDependencies := []*models.CachedDependency{}
				expectedCachedDependencies = append(expectedCachedDependencies, &models.CachedDependency{
					From:     "http://file-server.com/v1/static/the/docker/lifecycle/path.tgz",
					To:       "/tmp/lifecycle",
					CacheKey: "docker-lifecycle",
				})
				Expect(desiredLRP.CachedDependencies).To(BeEquivalentTo(expectedCachedDependencies))
				Expect(desiredLRP.LegacyDownloadUser).To(Equal("root"))

				parallelRunAction := desiredLRP.Action.CodependentAction
				Expect(parallelRunAction.Actions).To(HaveLen(1))

				runAction := parallelRunAction.Actions[0].RunAction

				Expect(desiredLRP.Monitor.GetValue()).To(Equal(models.Timeout(
					&models.ParallelAction{
						Actions: []*models.Action{
							&models.Action{
								RunAction: &models.RunAction{
									User:      "root",
									Path:      "/tmp/lifecycle/healthcheck",
									Args:      []string{"-port=8080"},
									LogSource: "HEALTH",
									ResourceLimits: &models.ResourceLimits{
										Nofile: &defaultNofile,
									},
									SuppressLogOutput: true,
								},
							},
						},
					},
					10*time.Minute,
				)))

				Expect(runAction.Path).To(Equal("/tmp/lifecycle/launcher"))
				Expect(runAction.Args).To(Equal([]string{
					"app",
					"the-start-command with-arguments",
					"{}",
				}))

				Expect(runAction.LogSource).To(Equal("MYSOURCE"))

				numFiles := uint64(32)
				Expect(runAction.ResourceLimits).To(Equal(&models.ResourceLimits{
					Nofile: &numFiles,
				}))

				Expect(runAction.Env).To(ContainElement(&models.EnvironmentVariable{
					Name:  "foo",
					Value: "bar",
				}))

				Expect(runAction.Env).To(ContainElement(&models.EnvironmentVariable{
					Name:  "PORT",
					Value: "8080",
				}))

				Expect(runAction.Env).NotTo(ContainElement(&models.EnvironmentVariable{
					Name:  "VCAP_APP_PORT",
					Value: "8080",
				}))

				Expect(runAction.Env).NotTo(ContainElement(&models.EnvironmentVariable{
					Name:  "VCAP_APP_HOST",
					Value: "0.0.0.0",
				}))

				Expect(desiredLRP.EgressRules).To(ConsistOf(egressRules))

				Expect(desiredLRP.TrustedSystemCertificatesPath).To(Equal(recipebuilder.TrustedSystemCertificatesPath))
			})

			Context("when route service url is specified in RoutingInfo", func() {
				BeforeEach(func() {
					routingInfo, err := cc_messages.CCHTTPRoutes{
						{Hostname: "route1"},
						{Hostname: "route2", RouteServiceUrl: "https://rs.example.com"},
					}.CCRouteInfo()
					Expect(err).NotTo(HaveOccurred())
					desiredAppReq.RoutingInfo = routingInfo
				})

				It("sets up routes with the route service url", func() {
					routes := *desiredLRP.Routes
					cfRoutesJson := routes[cfroutes.CF_ROUTER]
					cfRoutes := cfroutes.CFRoutes{}

					err := json.Unmarshal(*cfRoutesJson, &cfRoutes)
					Expect(err).ToNot(HaveOccurred())

					Expect(cfRoutes).To(ConsistOf([]cfroutes.CFRoute{
						{Hostnames: []string{"route1"}, Port: 8080},
						{Hostnames: []string{"route2"}, Port: 8080, RouteServiceUrl: "https://rs.example.com"},
					}))
				})
			})

			Context("when no health check is specified", func() {
				BeforeEach(func() {
					desiredAppReq.HealthCheckType = cc_messages.UnspecifiedHealthCheckType
				})

				It("sets up the port check for backwards compatibility", func() {
					downloadDestinations := []string{}
					for _, dep := range desiredLRP.CachedDependencies {
						if dep != nil {
							downloadDestinations = append(downloadDestinations, dep.To)
						}
					}

					Expect(downloadDestinations).To(ContainElement("/tmp/lifecycle"))

					Expect(desiredLRP.Monitor.GetValue()).To(Equal(models.Timeout(
						&models.ParallelAction{
							Actions: []*models.Action{
								&models.Action{
									RunAction: &models.RunAction{
										User:      "root",
										Path:      "/tmp/lifecycle/healthcheck",
										Args:      []string{"-port=8080"},
										LogSource: "HEALTH",
										ResourceLimits: &models.ResourceLimits{
											Nofile: &defaultNofile,
										},
										SuppressLogOutput: true,
									},
								},
							},
						},
						10*time.Minute,
					)))
				})
			})

			Context("when the 'none' health check is specified", func() {
				BeforeEach(func() {
					desiredAppReq.HealthCheckType = cc_messages.NoneHealthCheckType
				})

				It("does not populate the monitor action", func() {
					Expect(desiredLRP.Monitor).To(BeNil())
				})

				It("still downloads the lifecycle, since we need it for the launcher", func() {
					downloadDestinations := []string{}
					for _, dep := range desiredLRP.CachedDependencies {
						if dep != nil {
							downloadDestinations = append(downloadDestinations, dep.To)
						}
					}

					Expect(downloadDestinations).To(ContainElement("/tmp/lifecycle"))
				})
			})

			Context("when the 'http' health check is specified", func() {
				BeforeEach(func() {
					desiredAppReq.HealthCheckType = cc_messages.HTTPHealthCheckType
					desiredAppReq.HealthCheckHTTPEndpoint = "/healthz"
				})

				It("builds a valid monitor value", func() {
					Expect(desiredLRP.Monitor.GetValue()).To(Equal(models.Timeout(
						&models.ParallelAction{
							Actions: []*models.Action{
								&models.Action{
									RunAction: &models.RunAction{
										User:      "root",
										Path:      "/tmp/lifecycle/healthcheck",
										Args:      []string{"-port=8080", "-uri=/healthz"},
										LogSource: "HEALTH",
										ResourceLimits: &models.ResourceLimits{
											Nofile: &defaultNofile,
										},
										SuppressLogOutput: true,
									},
								},
							},
						},
						10*time.Minute,
					)))

				})
			})

			Context("when allow ssh is true", func() {
				BeforeEach(func() {
					desiredAppReq.AllowSSH = true

					keyPairChan := make(chan keys.KeyPair, 2)

					fakeHostKeyPair := &fake_keys.FakeKeyPair{}
					fakeHostKeyPair.PEMEncodedPrivateKeyReturns("pem-host-private-key")
					fakeHostKeyPair.FingerprintReturns("host-fingerprint")

					fakeUserKeyPair := &fake_keys.FakeKeyPair{}
					fakeUserKeyPair.AuthorizedKeyReturns("authorized-user-key")
					fakeUserKeyPair.PEMEncodedPrivateKeyReturns("pem-user-private-key")

					keyPairChan <- fakeHostKeyPair
					keyPairChan <- fakeUserKeyPair

					fakeKeyFactory.NewKeyPairStub = func(bits int) (keys.KeyPair, error) {
						return <-keyPairChan, nil
					}
				})

				It("setup should download the ssh daemon", func() {
					expectedCacheDependencies := []*models.CachedDependency{
						{
							From:     "http://file-server.com/v1/static/the/docker/lifecycle/path.tgz",
							To:       "/tmp/lifecycle",
							CacheKey: "docker-lifecycle",
						},
					}

					Expect(desiredLRP.CachedDependencies).To(BeEquivalentTo(expectedCacheDependencies))
				})

				It("runs the ssh daemon in the container", func() {
					expectedNumFiles := uint64(32)

					expectedAction := models.Codependent(
						&models.RunAction{
							User: "root",
							Path: "/tmp/lifecycle/launcher",
							Args: []string{
								"app",
								"the-start-command with-arguments",
								"{}",
							},
							Env: []*models.EnvironmentVariable{
								{Name: "foo", Value: "bar"},
								{Name: "PORT", Value: "8080"},
							},
							ResourceLimits: &models.ResourceLimits{
								Nofile: &expectedNumFiles,
							},
							LogSource: "MYSOURCE",
						},
						&models.RunAction{
							User: "root",
							Path: "/tmp/lifecycle/diego-sshd",
							Args: []string{
								"-address=0.0.0.0:2222",
								"-hostKey=pem-host-private-key",
								"-authorizedKey=authorized-user-key",
								"-inheritDaemonEnv",
								"-logLevel=fatal",
							},
							Env: []*models.EnvironmentVariable{
								{Name: "foo", Value: "bar"},
								{Name: "PORT", Value: "8080"},
							},
							ResourceLimits: &models.ResourceLimits{
								Nofile: &expectedNumFiles,
							},
						},
					)

					Expect(desiredLRP.Action.GetValue()).To(Equal(expectedAction))
				})

				It("opens up the default ssh port", func() {
					Expect(desiredLRP.Ports).To(Equal([]uint32{
						8080,
						2222,
					}))
				})

				It("declares ssh routing information in the LRP", func() {
					cfRoutePayload, err := json.Marshal(cfroutes.CFRoutes{
						{Hostnames: []string{"route1", "route2"}, Port: 8080},
					})
					Expect(err).NotTo(HaveOccurred())

					sshRoutePayload, err := json.Marshal(routes.SSHRoute{
						ContainerPort:   2222,
						PrivateKey:      "pem-user-private-key",
						HostFingerprint: "host-fingerprint",
					})
					Expect(err).NotTo(HaveOccurred())

					cfRouteMessage := json.RawMessage(cfRoutePayload)
					tcpRouteMessage := json.RawMessage([]byte("[]"))
					sshRouteMessage := json.RawMessage(sshRoutePayload)

					Expect(desiredLRP.Routes).To(Equal(&models.Routes{
						cfroutes.CF_ROUTER:    &cfRouteMessage,
						tcp_routes.TCP_ROUTER: &tcpRouteMessage,
						routes.DIEGO_SSH:      &sshRouteMessage,
					}))
				})

				Context("when generating the host key fails", func() {
					BeforeEach(func() {
						fakeKeyFactory.NewKeyPairReturns(nil, errors.New("boom"))
					})

					It("should return an error", func() {
						Expect(err).To(HaveOccurred())
					})
				})

				Context("when generating the user key fails", func() {
					BeforeEach(func() {
						errorCh := make(chan error, 2)
						errorCh <- nil
						errorCh <- errors.New("woops")

						fakeKeyFactory.NewKeyPairStub = func(bits int) (keys.KeyPair, error) {
							return nil, <-errorCh
						}
					})

					It("should return an error", func() {
						Expect(err).To(HaveOccurred())
					})
				})
			})

			Context("when an Isolation segment is specified", func() {
				BeforeEach(func() {
					desiredAppReq.IsolationSegment = "foo"
				})

				It("includes the correct segment in the desiredLRP", func() {
					Expect(desiredLRP.PlacementTags).To(ContainElement("foo"))
				})
			})
		})

		Context("when there is a docker image url instead of a droplet uri", func() {
			BeforeEach(func() {
				desiredAppReq.DockerImageUrl = "user/repo:tag"
				desiredAppReq.ExecutionMetadata = "{}"
			})

			It("does not error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("uses an unprivileged container", func() {
				Expect(desiredLRP.Privileged).To(BeFalse())
			})

			It("converts the docker image url into a root fs path", func() {
				Expect(desiredLRP.RootFs).To(Equal("docker:///user/repo#tag"))
			})

			It("exposes the default port", func() {
				parallelRunAction := desiredLRP.Action.CodependentAction
				Expect(parallelRunAction.Actions).To(HaveLen(1))

				runAction := parallelRunAction.Actions[0].RunAction

				Expect(desiredLRP.Routes).To(Equal(&expectedRoutes))

				Expect(desiredLRP.Ports).To(Equal([]uint32{8080}))

				Expect(desiredLRP.Monitor.GetValue()).To(Equal(models.Timeout(
					&models.ParallelAction{
						Actions: []*models.Action{
							&models.Action{
								RunAction: &models.RunAction{
									User:      "root",
									Path:      "/tmp/lifecycle/healthcheck",
									Args:      []string{"-port=8080"},
									LogSource: "HEALTH",
									ResourceLimits: &models.ResourceLimits{
										Nofile: &defaultNofile,
									},
									SuppressLogOutput: true,
								},
							},
						},
					},
					10*time.Minute,
				)))

				Expect(runAction.Env).To(ContainElement(&models.EnvironmentVariable{
					Name:  "PORT",
					Value: "8080",
				}))
			})

			Context("when ports are passed in desired app request", func() {
				BeforeEach(func() {
					desiredAppReq.Ports = []uint32{1456, 2345, 3456}
				})

				Context("when allow ssh is false", func() {
					BeforeEach(func() {
						desiredAppReq.AllowSSH = false
					})

					Context("with ports specified in execution metadata", func() {
						BeforeEach(func() {
							desiredAppReq.ExecutionMetadata = `{"ports":[
							{"Port":320, "Protocol": "udp"},
							{"Port":8081, "Protocol": "tcp"},
							{"Port":8082, "Protocol": "tcp"}
							]}`
						})
						It("builds the desiredLRP with the ports specified in the desireAppRequest", func() {
							Expect(desiredLRP.Ports).To(Equal([]uint32{1456, 2345, 3456}))
						})
					})

					Context("with ports not specified in execution metadata", func() {
						It("builds the desiredLRP with the ports specified in the desireAppRequest", func() {
							Expect(desiredLRP.Ports).To(Equal([]uint32{1456, 2345, 3456}))
						})
					})

				})

				Context("when allow ssh is true", func() {
					BeforeEach(func() {
						desiredAppReq.AllowSSH = true
						keyPairChan := make(chan keys.KeyPair, 2)

						fakeHostKeyPair := &fake_keys.FakeKeyPair{}
						fakeHostKeyPair.PEMEncodedPrivateKeyReturns("pem-host-private-key")
						fakeHostKeyPair.FingerprintReturns("host-fingerprint")

						fakeUserKeyPair := &fake_keys.FakeKeyPair{}
						fakeUserKeyPair.AuthorizedKeyReturns("authorized-user-key")
						fakeUserKeyPair.PEMEncodedPrivateKeyReturns("pem-user-private-key")

						keyPairChan <- fakeHostKeyPair
						keyPairChan <- fakeUserKeyPair

						fakeKeyFactory.NewKeyPairStub = func(bits int) (keys.KeyPair, error) {
							return <-keyPairChan, nil
						}
					})

					Context("with ports specified in execution metadata", func() {
						BeforeEach(func() {
							desiredAppReq.ExecutionMetadata = `{"ports":[
							{"Port":320, "Protocol": "udp"},
							{"Port":8081, "Protocol": "tcp"},
							{"Port":8082, "Protocol": "tcp"}
							]}`
						})

						It("builds the desiredLRP with the ports specified in the desireAppRequest", func() {
							Expect(desiredLRP.Ports).To(Equal([]uint32{1456, 2345, 3456, 2222}))
						})
					})

					Context("with ports not specified in execution metadata", func() {
						It("builds the desiredLRP with the ports specified in the desireAppRequest", func() {
							Expect(desiredLRP.Ports).To(Equal([]uint32{1456, 2345, 3456, 2222}))
						})
					})
				})
			})

			Context("when the docker image exposes several ports in its metadata", func() {
				BeforeEach(func() {
					desiredAppReq.ExecutionMetadata = `{"ports":[
					{"Port":320, "Protocol": "udp"},
					{"Port":8081, "Protocol": "tcp"},
					{"Port":8082, "Protocol": "tcp"}
					]}`
					routingInfo, err := cc_messages.CCHTTPRoutes{
						{Hostname: "route1", Port: 8081},
						{Hostname: "route2", Port: 8082},
					}.CCRouteInfo()
					Expect(err).NotTo(HaveOccurred())
					desiredAppReq.RoutingInfo = routingInfo
				})

				It("exposes all encountered tcp ports", func() {
					parallelRunAction := desiredLRP.Action.CodependentAction
					Expect(parallelRunAction.Actions).To(HaveLen(1))

					runAction := parallelRunAction.Actions[0].RunAction

					httpRoutes := cfroutes.CFRoutes{
						{Hostnames: []string{"route1"}, Port: 8081},
						{Hostnames: []string{"route2"}, Port: 8082},
					}
					test_helpers.VerifyHttpRoutes(*desiredLRP.Routes, httpRoutes)

					Expect(desiredLRP.Ports).To(Equal([]uint32{8081, 8082}))

					Expect(desiredLRP.Monitor.GetValue()).To(Equal(models.Timeout(
						&models.ParallelAction{
							Actions: []*models.Action{
								&models.Action{
									RunAction: &models.RunAction{
										User:      "root",
										Path:      "/tmp/lifecycle/healthcheck",
										Args:      []string{"-port=8081"},
										LogSource: "HEALTH",
										ResourceLimits: &models.ResourceLimits{
											Nofile: &defaultNofile,
										},
										SuppressLogOutput: true,
									},
								},
								&models.Action{
									RunAction: &models.RunAction{
										User:      "root",
										Path:      "/tmp/lifecycle/healthcheck",
										Args:      []string{"-port=8082"},
										LogSource: "HEALTH",
										ResourceLimits: &models.ResourceLimits{
											Nofile: &defaultNofile,
										},
										SuppressLogOutput: true,
									},
								},
							},
						},
						10*time.Minute,
					)))

					Expect(runAction.Env).To(ContainElement(&models.EnvironmentVariable{
						Name:  "PORT",
						Value: "8081",
					}))
				})

				Context("when ports in desired app request is empty slice", func() {
					BeforeEach(func() {
						desiredAppReq.Ports = []uint32{}
					})
					It("exposes all encountered tcp ports", func() {
						Expect(desiredLRP.Ports).To(Equal([]uint32{8081, 8082}))
					})
				})
			})

			Context("when the docker image exposes several non-tcp ports in its metadata", func() {
				BeforeEach(func() {
					desiredAppReq.ExecutionMetadata = `{"ports":[
					{"Port":319, "Protocol": "udp"},
					{"Port":320, "Protocol": "udp"}
					]}`
				})

				It("errors", func() {
					Expect(err).To(HaveOccurred())
					Expect(logger.TestSink.Buffer()).To(gbytes.Say("parsing-exposed-ports-failed"))
					Expect(logger.TestSink.Buffer()).To(gbytes.Say("No tcp ports found in image metadata"))
				})
			})

			Context("when the docker image execution metadata is not valid json", func() {
				BeforeEach(func() {
					desiredAppReq.ExecutionMetadata = "invalid-json"
				})

				It("errors", func() {
					Expect(err).To(HaveOccurred())
					Expect(logger.TestSink.Buffer()).To(gbytes.Say("parsing-execution-metadata-failed"))
				})
			})

			testLegacySetupUser := func(user string) func() {
				return func() {
					Expect(desiredLRP.LegacyDownloadUser).To(Equal(user))
				}
			}

			testRunActionUser := func(user string) func() {
				return func() {
					parallelRunAction := desiredLRP.Action.CodependentAction
					Expect(parallelRunAction.Actions).To(HaveLen(1))

					runAction := parallelRunAction.Actions[0].RunAction
					Expect(runAction.User).To(Equal(user))
				}
			}

			testHealthcheckActionUser := func(user string) func() {
				return func() {
					timeoutAction := desiredLRP.Monitor.TimeoutAction

					healthcheckRunAction := timeoutAction.Action.ParallelAction.Actions[0].RunAction
					Expect(healthcheckRunAction.User).To(Equal(user))
				}
			}

			Context("when the docker image exposes a user in its metadata", func() {
				BeforeEach(func() {
					desiredAppReq.ExecutionMetadata = `{"user":"custom"}`
				})

				It("builds a setup action with the correct user", testLegacySetupUser("custom"))
				It("builds a run action with the correct user", testRunActionUser("custom"))
				It("builds a healthcheck action with the correct user", testHealthcheckActionUser("custom"))
			})

			Context("when the docker image does not exposes a user in its metadata", func() {
				It("builds a setup action with the default user", testLegacySetupUser("root"))
				It("builds a run action with the default user", testRunActionUser("root"))
				It("builds a healthcheck action with the default user", testHealthcheckActionUser("root"))
			})

			testRootFSPath := func(imageUrl string, expectedRootFSPath string) func() {
				return func() {
					BeforeEach(func() {
						desiredAppReq.DockerImageUrl = imageUrl
					})

					It("builds correct rootFS path", func() {
						Expect(desiredLRP.RootFs).To(Equal(expectedRootFSPath))
					})
				}
			}

			Context("and the docker image url has no host", func() {
				Context("and image only", testRootFSPath("image", "docker:///library/image"))
				//does not specify a url fragment for the tag, assumes garden-linux sets a default
				Context("and user/image", testRootFSPath("user/image", "docker:///user/image"))
				Context("and a image with tag", testRootFSPath("image:tag", "docker:///library/image#tag"))
				Context("and a user/image with tag", testRootFSPath("user/image:tag", "docker:///user/image#tag"))
			})

			Context("and the docker image url has host:port", func() {
				Context("and image only", testRootFSPath("10.244.2.6:8080/image", "docker://10.244.2.6:8080/image"))
				Context("and user/image", testRootFSPath("10.244.2.6:8080/user/image", "docker://10.244.2.6:8080/user/image"))
				Context("and a image with tag", testRootFSPath("10.244.2.6:8080/image:tag", "docker://10.244.2.6:8080/image#tag"))
				Context("and a user/image with tag", testRootFSPath("10.244.2.6:8080/user/image:tag", "docker://10.244.2.6:8080/user/image#tag"))
			})

			Context("and the docker image url has host docker.io", func() {
				Context("and image only", testRootFSPath("docker.io/image", "docker://docker.io/library/image"))
				Context("and user/image", testRootFSPath("docker.io/user/image", "docker://docker.io/user/image"))
				Context("and image with tag", testRootFSPath("docker.io/image:tag", "docker://docker.io/library/image#tag"))
				Context("and a user/image with tag", testRootFSPath("docker.io/user/image:tag", "docker://docker.io/user/image#tag"))
			})

			Context("and the docker image url has scheme", func() {
				BeforeEach(func() {
					desiredAppReq.DockerImageUrl = "https://docker.io/repo"
				})

				It("errors", func() {
					Expect(err).To(HaveOccurred())
				})
			})

			It("does not set the container's LANG", func() {
				Expect(desiredLRP.EnvironmentVariables).To(BeEmpty())
			})
		})

		Context("when there is a docker username", func() {
			BeforeEach(func() {
				desiredAppReq.DockerUser = "someuser"
			})

			It("includes the docker username in the desiredLRP", func() {
				Expect(desiredLRP.ImageUsername).To(Equal("someuser"))
			})
		})

		Context("when there is a docker password", func() {
			BeforeEach(func() {
				desiredAppReq.DockerPassword = "apassword"
			})

			It("includes the docker password in the desiredLRP", func() {
				Expect(desiredLRP.ImagePassword).To(Equal("apassword"))
			})
		})

		Context("when there is a docker image url AND a droplet uri", func() {
			BeforeEach(func() {
				desiredAppReq.DockerImageUrl = "user/repo:tag"
				desiredAppReq.DropletUri = "http://the-droplet.uri.com"
			})

			It("should error", func() {
				Expect(err).To(MatchError(recipebuilder.ErrMultipleAppSources))
			})
		})

		Context("when there is NEITHER a docker image url NOR a droplet uri", func() {
			BeforeEach(func() {
				desiredAppReq.DockerImageUrl = ""
				desiredAppReq.DropletUri = ""
			})

			It("should error", func() {
				Expect(err).To(MatchError(recipebuilder.ErrDockerImageMissing))
			})
		})

		Context("when there is no file descriptor limit", func() {
			BeforeEach(func() {
				desiredAppReq.FileDescriptors = 0
			})

			It("does not error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("sets a default FD limit on the run action", func() {
				parallelRunAction := desiredLRP.Action.CodependentAction
				Expect(parallelRunAction.Actions).To(HaveLen(1))

				runAction := parallelRunAction.Actions[0].RunAction

				Expect(runAction.ResourceLimits.Nofile).NotTo(BeNil())
				Expect(*runAction.ResourceLimits.Nofile).To(Equal(recipebuilder.DefaultFileDescriptorLimit))
			})
		})

		Context("when log source is empty", func() {
			BeforeEach(func() {
				desiredAppReq.LogSource = ""
			})

			It("uses APP", func() {
				parallelRunAction := desiredLRP.Action.CodependentAction
				runAction := parallelRunAction.Actions[0].RunAction
				Expect(runAction.LogSource).To(Equal("APP"))
			})
		})

		Context("When the recipeBuilder Config has Privileged set to true", func() {
			BeforeEach(func() {
				config := recipebuilder.Config{
					Lifecycles:           lifecycles,
					FileServerURL:        "http://file-server.com",
					KeyFactory:           fakeKeyFactory,
					PrivilegedContainers: true,
				}
				builder = recipebuilder.NewDockerRecipeBuilder(logger, config)
			})

			It("sets Priviledged to false", func() {
				Expect(desiredLRP.Privileged).To(BeFalse())
			})

		})

		Describe("volume mounts", func() {
			Context("when none are provided", func() {
				It("is empty", func() {
					Expect(len(desiredLRP.VolumeMounts)).To(Equal(0))
				})
			})

			Context("when some are provided", func() {
				BeforeEach(func() {
					desiredAppReq.VolumeMounts = desiredCCVolumeMounts
				})

				It("desires the mounts", func() {
					Expect(desiredLRP.VolumeMounts).To(Equal(expectedBBSVolumeMounts))
				})
			})
		})

	})

	Describe("BuildTask", func() {
		var (
			newTaskReq     *cc_messages.TaskRequestFromCC
			taskDefinition *models.TaskDefinition
			err            error
		)

		BeforeEach(func() {
			newTaskReq = &cc_messages.TaskRequestFromCC{
				LogGuid:     "the-log-guid",
				DiskMb:      128,
				MemoryMb:    512,
				EgressRules: egressRules,
				EnvironmentVariables: []*models.EnvironmentVariable{
					{Name: "foo", Value: "bar"},
				},
				CompletionCallbackUrl: "http://google.com",
				Command:               "docker run fast",
				DockerPath:            "cloudfoundry/diego-docker-app",
				LogSource:             "APP/TASK/my-task",
			}
		})

		JustBeforeEach(func() {
			taskDefinition, err = builder.BuildTask(newTaskReq)
		})

		It("does not error", func() {
			Expect(err).NotTo(HaveOccurred())
		})

		It("builds a task definition", func() {
			Expect(taskDefinition.DiskMb).To(BeEquivalentTo(128))
			Expect(taskDefinition.MemoryMb).To(BeEquivalentTo(512))
			Expect(taskDefinition.LogGuid).To(Equal("the-log-guid"))
			Expect(taskDefinition.Privileged).To(BeFalse())

			Expect(taskDefinition.EgressRules).To(Equal(egressRules))
			Expect(taskDefinition.EnvironmentVariables).To(Equal([]*models.EnvironmentVariable{
				{Name: "foo", Value: "bar"},
			}))
			Expect(taskDefinition.CompletionCallbackUrl).To(Equal("http://google.com"))
			Expect(taskDefinition.RootFs).To(Equal("docker:///cloudfoundry/diego-docker-app"))
			Expect(taskDefinition.LogSource).To(Equal("APP/TASK/my-task"))

			expectedCacheDependencies := []*models.CachedDependency{
				&models.CachedDependency{
					From:     "http://file-server.com/v1/static/the/docker/lifecycle/path.tgz",
					To:       "/tmp/lifecycle",
					CacheKey: "docker-lifecycle",
				},
			}

			Expect(taskDefinition.LegacyDownloadUser).To(Equal("vcap"))
			Expect(taskDefinition.CachedDependencies).To(BeEquivalentTo(expectedCacheDependencies))

			expectedAction := models.WrapAction(&models.RunAction{
				User: "root",
				Path: "/tmp/lifecycle/launcher",
				Args: append(
					[]string{"app"},
					"docker run fast",
					"{}",
				),
				Env:            newTaskReq.EnvironmentVariables,
				ResourceLimits: &models.ResourceLimits{},
				LogSource:      "APP/TASK/my-task",
			})

			Expect(taskDefinition.Action).To(BeEquivalentTo(expectedAction))

			Expect(taskDefinition.TrustedSystemCertificatesPath).To(Equal(recipebuilder.TrustedSystemCertificatesPath))
			Expect(taskDefinition.PlacementTags).To(BeEmpty())
		})

		Context("when an Isolation segment is specified", func() {
			BeforeEach(func() {
				newTaskReq.IsolationSegment = "foo"
			})

			It("includes the correct segment in the desiredLRP", func() {
				Expect(taskDefinition.PlacementTags).To(ContainElement("foo"))
			})
		})

		Context("When the recipeBuilder Config has Privileged set to true", func() {
			BeforeEach(func() {
				config := recipebuilder.Config{
					Lifecycles:           lifecycles,
					FileServerURL:        "http://file-server.com",
					KeyFactory:           fakeKeyFactory,
					PrivilegedContainers: true,
				}
				builder = recipebuilder.NewDockerRecipeBuilder(logger, config)
			})

			It("sets Priviledged to false", func() {
				Expect(taskDefinition.Privileged).To(BeFalse())
			})

		})
		Context("when the docker path is not specified", func() {
			BeforeEach(func() {
				newTaskReq.DockerPath = ""
			})

			It("returns an error", func() {
				Expect(err).To(Equal(recipebuilder.ErrDockerImageMissing))
			})
		})

		Context("with an invalid docker path url", func() {
			BeforeEach(func() {
				newTaskReq.DockerPath = "docker://jim/jim"
			})

			It("returns an error", func() {
				Expect(err).To(HaveOccurred())
			})
		})

		Context("when a droplet uri is specified", func() {
			BeforeEach(func() {
				newTaskReq.DropletUri = "https://utako.utako.com"
			})

			It("returns an error", func() {
				Expect(err).To(Equal(recipebuilder.ErrMultipleAppSources))
			})
		})

		Context("when there is a docker username", func() {
			BeforeEach(func() {
				newTaskReq.DockerUser = "someuser"
			})

			It("includes the docker username in the task", func() {
				Expect(taskDefinition.ImageUsername).To(Equal("someuser"))
			})
		})

		Context("when there is a docker password", func() {
			BeforeEach(func() {
				newTaskReq.DockerPassword = "apassword"
			})

			It("includes the docker password in the task", func() {
				Expect(taskDefinition.ImagePassword).To(Equal("apassword"))
			})
		})

		Describe("volume mounts", func() {
			Context("when none are provided", func() {
				It("is empty", func() {
					Expect(len(taskDefinition.VolumeMounts)).To(Equal(0))
				})
			})

			Context("when some are provided", func() {
				BeforeEach(func() {
					newTaskReq.VolumeMounts = desiredCCVolumeMounts
				})

				It("desires the mounts", func() {
					Expect(err).NotTo(HaveOccurred())
					Expect(taskDefinition.VolumeMounts).To(Equal(expectedBBSVolumeMounts))
				})
			})
		})
	})
})
