package models_test

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"time"

	"code.cloudfoundry.org/bbs/format"
	"code.cloudfoundry.org/bbs/models"
	"code.cloudfoundry.org/bbs/models/test/model_helpers"
	"github.com/gogo/protobuf/proto"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
)

var _ = Describe("DesiredLRP", func() {
	var desiredLRP models.DesiredLRP

	jsonDesiredLRP := `{
    "setup": {
      "serial": {
        "actions": [
          {
            "download": {
              "from": "http://file-server.service.cf.internal:8080/v1/static/buildpack_app_lifecycle/buildpack_app_lifecycle.tgz",
              "to": "/tmp/lifecycle",
              "cache_key": "buildpack-cflinuxfs2-lifecycle",
							"user": "someone",
							"checksum_algorithm": "md5",
							"checksum_value": "some random value"
            }
          },
          {
            "download": {
              "from": "http://cloud-controller-ng.service.cf.internal:9022/internal/v2/droplets/some-guid/some-guid/download",
              "to": ".",
              "cache_key": "droplets-some-guid",
							"user": "someone"
            }
          }
        ]
      }
    },
    "action": {
      "codependent": {
        "actions": [
          {
            "run": {
              "path": "/tmp/lifecycle/launcher",
              "args": [
                "app",
                "",
                "{\"start_command\":\"bundle exec rackup config.ru -p $PORT\"}"
              ],
              "env": [
                {
                  "name": "VCAP_APPLICATION",
                  "value": "{\"limits\":{\"mem\":1024,\"disk\":1024,\"fds\":16384},\"application_id\":\"some-guid\",\"application_version\":\"some-guid\",\"application_name\":\"some-guid\",\"version\":\"some-guid\",\"name\":\"some-guid\",\"space_name\":\"CATS-SPACE-3-2015_07_01-11h28m01.515s\",\"space_id\":\"bc640806-ea03-40c6-8371-1c2b23fa4662\"}"
                },
                {
                  "name": "VCAP_SERVICES",
                  "value": "{}"
                },
                {
                  "name": "MEMORY_LIMIT",
                  "value": "1024m"
                },
                {
                  "name": "CF_STACK",
                  "value": "cflinuxfs2"
                },
                {
                  "name": "PORT",
                  "value": "8080"
                }
              ],
              "resource_limits": {
                "nofile": 16384
              },
              "user": "vcap",
              "log_source": "APP",
			  "suppress_log_output": false
            }
          },
          {
            "run": {
              "path": "/tmp/lifecycle/diego-sshd",
              "args": [
                "-address=0.0.0.0:2222",
                "-hostKey=-----BEGIN RSA PRIVATE KEY-----\nMIICWwIBAAKBgQCp72ylz6ow8P4km1Nzd2yyN9aiXAI8MHl6Crl6vjpBNQIhy+YH\nEf5fgAI/wHydaajSsk28Byf/hAm/Q/3EmT1bUmdCsVzzndzJvPNf5t11LGmPFcNV\nZ9vsfnFjMlsFM/ZHU60PT8POSoE8VnrplTLRhEtQFopdMcDN8nRl6imhUQIDAQAB\nAoGAWz8aQbZOFlVwwUs99gQsM03US/3HnXYR5DwZ+BRox1alPGx1qVo6EiF0E7NR\ntlxjsC7ZmprlGUhWy4LAom3+CUj712fI7Qnud9AH4GUHN4JrxytiDDLJJh/hRADB\niD/MKo9ih7c2bQvBU+FwLYlXyI/GViBMqIYzZ+6r7yVkp/kCQQDZIcMKzNwVV+LL\nnDXZg4nIyFgR3CGZb+cVrXnDaIEwmC5ABHlnhJJzI7FdsGuhwOJnKdMHQgI6+o+Z\nvmizsdyDAkEAyFrXDX+wRMPrEjmNga2TYaCIt6AWR3b4aLJskZQnf0iMI2DzL74e\na7Ibkxp+OxtSL2YIR7NCfDz/DiUtqvQKmwJAVRxX0K72geM+QiOMNCPMaYimhPGt\ntfBYO3YRaZhYM40ja/KVCA++PCW8i4Xw2qm51UhesNSd/TJkAZbSgcVxMwJAQSKX\nK4JJkfGHqKMhR/lgIqsIB3p6A72/wHnRJfreZFj3hkDsjqbmSOjcYhSI2Tpmm5Y2\nNukmQjGqUbTwhdVU5QJALpewrw7eiWAjnYxus6Fi0XiEduE91OEtuc3yHRrR0ubI\nCt2HP6jQ43siwcx+FAA8kBfvtQElIC2TF2qwjezEcA==\n-----END RSA PRIVATE KEY-----\n",
                "-authorizedKey=ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAAAgQDuOfcUnfiXE6g6Cvgur3Om6t8cEx27FAoVrDrxMzy+q2NTJaQFNYqG2DDDHZCLG2mJasryKZfDyK30c48ITpecBkCux429aZN2gEJCEsyYgsZheI+5eNYs1vzl68KQ1LdxlgNOqFZijyVjTOD60GMPCVlDICqGNUFH4aPTHA0fVw==\n",
                "-inheritDaemonEnv",
                "-logLevel=fatal"
              ],
              "env": [
                {
                  "name": "VCAP_APPLICATION",
                  "value": "{\"limits\":{\"mem\":1024,\"disk\":1024,\"fds\":16384},\"application_id\":\"some-guid\",\"application_version\":\"some-guid\",\"application_name\":\"some-guid\",\"version\":\"some-guid\",\"name\":\"some-guid\",\"space_name\":\"CATS-SPACE-3-2015_07_01-11h28m01.515s\",\"space_id\":\"some-guid\"}"
                },
                {
                  "name": "VCAP_SERVICES",
                  "value": "{}"
                },
                {
                  "name": "MEMORY_LIMIT",
                  "value": "1024m"
                },
                {
                  "name": "CF_STACK",
									"value": "cflinuxfs2"
                },
                {
                  "name": "PORT",
                  "value": "8080"
                }
              ],
              "resource_limits": {
                "nofile": 16384
              },
              "user": "vcap",
			  "suppress_log_output": false
            }
          }
        ]
      }
    },
    "monitor": {
      "timeout": {
        "action": {
          "run": {
            "path": "/tmp/lifecycle/healthcheck",
            "args": [
              "-port=8080"
            ],
            "resource_limits": {
              "nofile": 1024
            },
            "user": "vcap",
            "log_source": "HEALTH",
			"suppress_log_output": true
          }
        },
        "timeout_ms": 30000000
      }
    },
    "process_guid": "some-guid",
    "domain": "cf-apps",
    "rootfs": "preloaded:cflinuxfs2",
    "instances": 2,
    "env": [
      {
        "name": "LANG",
        "value": "en_US.UTF-8"
      }
    ],
    "start_timeout_ms": 60000,
    "disk_mb": 1024,
    "memory_mb": 1024,
    "cpu_weight": 10,
    "privileged": true,
    "ports": [
      8080,
      2222
    ],
    "routes": {
      "cf-router": [
        {
          "hostnames": [
            "some-route.example.com"
          ],
          "port": 8080
        }
      ],
      "diego-ssh": {
        "container_port": 2222,
        "host_fingerprint": "ac:99:67:20:7e:c2:7c:2c:d2:22:37:bc:9f:14:01:ec",
        "private_key": "-----BEGIN RSA PRIVATE KEY-----\nMIICXQIBAAKBgQDuOfcUnfiXE6g6Cvgur3Om6t8cEx27FAoVrDrxMzy+q2NTJaQF\nNYqG2DDDHZCLG2mJasryKZfDyK30c48ITpecBkCux429aZN2gEJCEsyYgsZheI+5\neNYs1vzl68KQ1LdxlgNOqFZijyVjTOD60GMPCVlDICqGNUFH4aPTHA0fVwIDAQAB\nAoGBAO1Ak19YGHy1mgP8asFsAT1KitrV+vUW9xgwiB8xjRzDac8kHJ8HfKfg5Wdc\nqViw+0FdNzNH0xqsYPqkn92BECDqdWOzhlEYNj/AFSHTdRPrs9w82b7h/LhrX0H/\nRUrU2QrcI2uSV/SQfQvFwC6YaYugCo35noljJEcD8EYQTcRxAkEA+jfjumM6da8O\n8u8Rc58Tih1C5mumeIfJMPKRz3FBLQEylyMWtGlr1XT6ppqiHkAAkQRUBgKi+Ffi\nYedQOvE0/wJBAPO7I+brmrknzOGtSK2tvVKnMqBY6F8cqmG4ZUm0W9tMLKiR7JWO\nAsjSlQfEEnpOr/AmuONwTsNg+g93IILv3akCQQDnrKfmA8o0/IlS1ZfK/hcRYlZ3\nEmVoZBEciPwInkxCZ0F4Prze/l0hntYVPEeuyoO7wc4qYnaSiozJKWtXp83xAkBo\nk+ubsYv51jH6wzdkDiAlzsfSNVO/O7V/qHcNYO3o8o5W5gX1RbG8KV74rhCfmhOz\nn2nFbPLeskWZTSwOAo3BAkBWHBjvCj1sBgsIG4v6Tn2ig21akbmssJezmZRjiqeh\nqt0sAzMVixAwIFM0GsW3vQ8Hr/eBTb5EBQVZ/doRqUzf\n-----END RSA PRIVATE KEY-----\n"
      }
    },
    "log_guid": "some-guid",
    "log_source": "CELL",
    "metrics_guid": "some-guid",
    "annotation": "1435775395.194748",
    "egress_rules": [
      {
        "protocol": "all",
        "destinations": [
          "0.0.0.0-9.255.255.255"
        ],
        "log": false
      },
      {
        "protocol": "all",
        "destinations": [
          "11.0.0.0-169.253.255.255"
        ],
        "log": false
      },
      {
        "protocol": "all",
        "destinations": [
          "169.255.0.0-172.15.255.255"
        ],
        "log": false
      },
      {
        "protocol": "all",
        "destinations": [
          "172.32.0.0-192.167.255.255"
        ],
        "log": false
      },
      {
        "protocol": "all",
        "destinations": [
          "192.169.0.0-255.255.255.255"
        ],
        "log": false
      },
      {
        "protocol": "tcp",
        "destinations": [
          "0.0.0.0/0"
        ],
        "ports": [
          53
        ],
        "log": false
      },
      {
        "protocol": "udp",
        "destinations": [
          "0.0.0.0/0"
        ],
        "ports": [
          53
        ],
        "log": false
      }
    ],
    "modification_tag": {
      "epoch": "some-guid",
      "index": 0
    },
		"placement_tags": ["red-tag", "blue-tag"],
    "trusted_system_certificates_path": "/etc/cf-system-certificates",
    "network": {
			"properties": {
				"key": "value",
				"another_key": "another_value"
			}
		},
		"max_pids": 256,
		"certificate_properties": {
			"organizational_unit": ["stuff"]
		},
		"check_definition": {
			"checks": [
				{
					"tcp_check": {
						"port": 12345,
						"connect_timeout_ms": 100
					}
				}
			],
			"log_source": "healthcheck_log_source"
		}
  }`

	BeforeEach(func() {
		desiredLRP = models.DesiredLRP{}
		err := json.Unmarshal([]byte(jsonDesiredLRP), &desiredLRP)
		Expect(err).NotTo(HaveOccurred())
	})

	Describe("CreateComponents", func() {
		It("decomposes the desired lrp into it's component parts", func() {
			schedInfo, runInfo := desiredLRP.CreateComponents(time.Unix(123, 456))
			newDesired := models.NewDesiredLRP(schedInfo, runInfo)
			Expect(newDesired).To(BeEquivalentTo(desiredLRP))
		})

		It("saves the created at time on the run info", func() {
			_, runInfo := desiredLRP.CreateComponents(time.Unix(123, 456))
			Expect(runInfo.CreatedAt).To(BeEquivalentTo((time.Unix(123, 456).UnixNano())))
		})
	})

	Describe("serialization", func() {
		It("successfully round trips through json and protobuf", func() {
			jsonSerialization, err := json.Marshal(desiredLRP)
			fmt.Sprintf("*******************\n%#v", desiredLRP)
			Expect(err).NotTo(HaveOccurred())
			Expect(jsonSerialization).To(MatchJSON(jsonDesiredLRP))

			protoSerialization, err := proto.Marshal(&desiredLRP)
			Expect(err).NotTo(HaveOccurred())

			var protoDeserialization models.DesiredLRP
			err = proto.Unmarshal(protoSerialization, &protoDeserialization)
			Expect(err).NotTo(HaveOccurred())

			desiredRoutes := *desiredLRP.Routes
			deserializedRoutes := *protoDeserialization.Routes

			Expect(deserializedRoutes).To(HaveLen(len(desiredRoutes)))
			for k := range desiredRoutes {
				Expect(string(*deserializedRoutes[k])).To(MatchJSON(string(*desiredRoutes[k])))
			}

			desiredLRP.Routes = nil
			protoDeserialization.Routes = nil
			Expect(protoDeserialization).To(Equal(desiredLRP))
		})
	})

	Describe("ApplyUpdate", func() {
		It("updates instances", func() {
			instances := int32(100)
			update := &models.DesiredLRPUpdate{Instances: &instances}
			schedulingInfo := desiredLRP.DesiredLRPSchedulingInfo()

			expectedSchedulingInfo := schedulingInfo
			expectedSchedulingInfo.Instances = instances
			expectedSchedulingInfo.ModificationTag.Increment()

			schedulingInfo.ApplyUpdate(update)
			Expect(schedulingInfo).To(Equal(expectedSchedulingInfo))
		})

		It("allows empty routes to be set", func() {
			update := &models.DesiredLRPUpdate{
				Routes: &models.Routes{},
			}

			schedulingInfo := desiredLRP.DesiredLRPSchedulingInfo()

			expectedSchedulingInfo := schedulingInfo
			expectedSchedulingInfo.Routes = models.Routes{}
			expectedSchedulingInfo.ModificationTag.Increment()

			schedulingInfo.ApplyUpdate(update)
			Expect(schedulingInfo).To(Equal(expectedSchedulingInfo))
		})

		It("allows annotation to be set", func() {
			annotation := "new-annotation"
			update := &models.DesiredLRPUpdate{
				Annotation: &annotation,
			}

			schedulingInfo := desiredLRP.DesiredLRPSchedulingInfo()

			expectedSchedulingInfo := schedulingInfo
			expectedSchedulingInfo.Annotation = annotation
			expectedSchedulingInfo.ModificationTag.Increment()

			schedulingInfo.ApplyUpdate(update)
			Expect(schedulingInfo).To(Equal(expectedSchedulingInfo))
		})

		It("allows empty annotation to be set", func() {
			emptyAnnotation := ""
			update := &models.DesiredLRPUpdate{
				Annotation: &emptyAnnotation,
			}

			schedulingInfo := desiredLRP.DesiredLRPSchedulingInfo()

			expectedSchedulingInfo := schedulingInfo
			expectedSchedulingInfo.Annotation = emptyAnnotation
			expectedSchedulingInfo.ModificationTag.Increment()

			schedulingInfo.ApplyUpdate(update)
			Expect(schedulingInfo).To(Equal(expectedSchedulingInfo))
		})

		It("updates routes", func() {
			rawMessage := json.RawMessage([]byte(`{"port": 8080,"hosts":["new-route-1","new-route-2"]}`))
			update := &models.DesiredLRPUpdate{
				Routes: &models.Routes{
					"router": &rawMessage,
				},
			}

			schedulingInfo := desiredLRP.DesiredLRPSchedulingInfo()

			expectedSchedulingInfo := schedulingInfo
			expectedSchedulingInfo.Routes = models.Routes{
				"router": &rawMessage,
			}
			expectedSchedulingInfo.ModificationTag.Increment()

			schedulingInfo.ApplyUpdate(update)
			Expect(schedulingInfo).To(Equal(expectedSchedulingInfo))
		})
	})

	Describe("Version Down To", func() {
		Context("V1", func() {
			BeforeEach(func() {
				desiredLRP.Setup = models.WrapAction(models.Timeout(
					&models.RunAction{
						Path: "/the/path",
						User: "the user",
					},
					10*time.Millisecond,
				))
				desiredLRP.Action = models.WrapAction(models.Timeout(
					&models.RunAction{
						Path: "/the/path",
						User: "the user",
					},
					20*time.Millisecond,
				))
				desiredLRP.Monitor = models.WrapAction(models.Timeout(
					&models.RunAction{
						Path: "/the/path",
						User: "the user",
					},
					30*time.Millisecond,
				))
				desiredLRP.StartTimeoutMs = 10000
			})

			It("converts TimeoutMs to Timeout in Nanoseconds", func() {
				convertedLRP := desiredLRP.VersionDownTo(format.V1)
				Expect(convertedLRP.GetSetup().GetTimeoutAction().DeprecatedTimeoutNs).To(BeEquivalentTo(10 * time.Millisecond))
				Expect(convertedLRP.GetAction().GetTimeoutAction().DeprecatedTimeoutNs).To(BeEquivalentTo(20 * time.Millisecond))
				Expect(convertedLRP.GetMonitor().GetTimeoutAction().DeprecatedTimeoutNs).To(BeEquivalentTo(30 * time.Millisecond))
			})

			It("converts StartTimeoutMs to StartTimeout in seconds", func() {
				convertedLRP := desiredLRP.VersionDownTo(format.V1)
				Expect(convertedLRP.GetDeprecatedStartTimeoutS()).To(BeEquivalentTo(10))
			})
		})

		Context("V0", func() {
			var (
				downloadAction1, downloadAction2 models.DownloadAction
			)

			BeforeEach(func() {
				desiredLRP.CachedDependencies = []*models.CachedDependency{
					{Name: "name-1", From: "from-1", To: "to-1", CacheKey: "cache-key-1", LogSource: "log-source-1"},
					{Name: "name-2", From: "from-2", To: "to-2", CacheKey: "cache-key-2", LogSource: "log-source-2"},
				}
				desiredLRP.LegacyDownloadUser = "joe-schmoe"

				downloadAction1 = models.DownloadAction{
					Artifact:  "name-1",
					From:      "from-1",
					To:        "to-1",
					CacheKey:  "cache-key-1",
					LogSource: "log-source-1",
					User:      "joe-schmoe",
				}

				downloadAction2 = models.DownloadAction{
					Artifact:  "name-2",
					From:      "from-2",
					To:        "to-2",
					CacheKey:  "cache-key-2",
					LogSource: "log-source-2",
					User:      "joe-schmoe",
				}

				desiredLRP.Action = models.WrapAction(models.Timeout(
					&models.RunAction{
						Path: "/the/path",
						User: "the user",
					},
					20*time.Millisecond,
				))
				desiredLRP.Monitor = models.WrapAction(models.Timeout(
					&models.RunAction{
						Path: "/the/path",
						User: "the user",
					},
					30*time.Millisecond,
				))
				desiredLRP.StartTimeoutMs = 10000
			})

			Context("when there is no existing setup action", func() {
				BeforeEach(func() {
					desiredLRP.Setup = nil
				})

				It("converts a cache dependency into download step action", func() {
					convertedLRP := desiredLRP.VersionDownTo(format.V0)
					Expect(convertedLRP.Setup.SerialAction.Actions).To(HaveLen(1))
					Expect(convertedLRP.Setup.SerialAction.Actions[0].ParallelAction.Actions).To(HaveLen(2))

					Expect(*convertedLRP.Setup.SerialAction.Actions[0].ParallelAction.Actions[0].DownloadAction).To(Equal(downloadAction1))
					Expect(*convertedLRP.Setup.SerialAction.Actions[0].ParallelAction.Actions[1].DownloadAction).To(Equal(downloadAction2))

					Expect(*convertedLRP.Setup).To(Equal(models.Action{
						SerialAction: &models.SerialAction{
							Actions: []*models.Action{
								{
									ParallelAction: &models.ParallelAction{
										Actions: []*models.Action{
											&models.Action{DownloadAction: &downloadAction1},
											&models.Action{DownloadAction: &downloadAction2},
										},
									},
								},
							},
						},
					}))
				})
			})

			It("converts TimeoutMs to Timeout in Nanoseconds", func() {
				convertedLRP := desiredLRP.VersionDownTo(format.V1)
				Expect(convertedLRP.GetAction().GetTimeoutAction().DeprecatedTimeoutNs).To(BeEquivalentTo(20 * time.Millisecond))
				Expect(convertedLRP.GetMonitor().GetTimeoutAction().DeprecatedTimeoutNs).To(BeEquivalentTo(30 * time.Millisecond))
			})

			It("converts StartTimeoutMs to StartTimeout in seconds", func() {
				convertedLRP := desiredLRP.VersionDownTo(format.V1)
				Expect(convertedLRP.GetDeprecatedStartTimeoutS()).To(BeEquivalentTo(10))
			})

			Context("when there is an existing setup action", func() {
				It("appends the new converted step action to the front", func() {
					convertedLRP := desiredLRP.VersionDownTo(format.V0)
					Expect(convertedLRP.Setup.SerialAction.Actions).To(HaveLen(2))
					Expect(convertedLRP.Setup.SerialAction.Actions[0].ParallelAction.Actions).To(HaveLen(2))

					Expect(*convertedLRP.Setup).To(Equal(models.Action{
						SerialAction: &models.SerialAction{
							Actions: []*models.Action{
								{
									ParallelAction: &models.ParallelAction{
										Actions: []*models.Action{
											&models.Action{DownloadAction: &downloadAction1},
											&models.Action{DownloadAction: &downloadAction2},
										},
									},
								},
								desiredLRP.Setup,
							},
						},
					}))
				})
			})

			Context("when there are no cache dependencies", func() {
				BeforeEach(func() {
					desiredLRP.CachedDependencies = nil
				})

				It("keeps the current setup", func() {
					convertedLRP := desiredLRP.VersionDownTo(format.V0)
					Expect(convertedLRP.Setup.SerialAction.Actions).To(HaveLen(2))

					Expect(*convertedLRP.Setup).To(Equal(*desiredLRP.Setup))
				})
			})
		})
	})

	Describe("Validate", func() {
		var assertDesiredLRPValidationFailsWithMessage = func(lrp models.DesiredLRP, substring string) {
			validationErr := lrp.Validate()
			Expect(validationErr).To(HaveOccurred())
			Expect(validationErr.Error()).To(ContainSubstring(substring))
		}

		Context("process_guid only contains `A-Z`, `a-z`, `0-9`, `-`, and `_`", func() {
			validGuids := []string{"a", "A", "0", "-", "_", "-aaaa", "_-aaa", "09a87aaa-_aASKDn"}
			for _, validGuid := range validGuids {
				func(validGuid string) {
					It(fmt.Sprintf("'%s' is a valid process_guid", validGuid), func() {
						desiredLRP.ProcessGuid = validGuid
						err := desiredLRP.Validate()
						Expect(err).NotTo(HaveOccurred())
					})
				}(validGuid)
			}

			invalidGuids := []string{"", "bang!", "!!!", "\\slash", "star*", "params()", "invalid/key", "with.dots"}
			for _, invalidGuid := range invalidGuids {
				func(invalidGuid string) {
					It(fmt.Sprintf("'%s' is an invalid process_guid", invalidGuid), func() {
						desiredLRP.ProcessGuid = invalidGuid
						assertDesiredLRPValidationFailsWithMessage(desiredLRP, "process_guid")
					})
				}(invalidGuid)
			}
		})

		It("requires a positive nonzero number of instances", func() {
			desiredLRP.Instances = -1
			assertDesiredLRPValidationFailsWithMessage(desiredLRP, "instances")

			desiredLRP.Instances = 0
			validationErr := desiredLRP.Validate()
			Expect(validationErr).NotTo(HaveOccurred())

			desiredLRP.Instances = 1
			validationErr = desiredLRP.Validate()
			Expect(validationErr).NotTo(HaveOccurred())
		})

		It("requires a domain", func() {
			desiredLRP.Domain = ""
			assertDesiredLRPValidationFailsWithMessage(desiredLRP, "domain")
		})

		It("requires a rootfs", func() {
			desiredLRP.RootFs = ""
			assertDesiredLRPValidationFailsWithMessage(desiredLRP, "rootfs")
		})

		It("requires a valid URL with a non-empty scheme for the rootfs", func() {
			desiredLRP.RootFs = ":not-a-url"
			assertDesiredLRPValidationFailsWithMessage(desiredLRP, "rootfs")
		})

		It("requires a valid absolute URL for the rootfs", func() {
			desiredLRP.RootFs = "not-an-absolute-url"
			assertDesiredLRPValidationFailsWithMessage(desiredLRP, "rootfs")
		})

		It("requires an action", func() {
			desiredLRP.Action = nil
			assertDesiredLRPValidationFailsWithMessage(desiredLRP, "action")
		})

		It("requires an action with an inner action", func() {
			desiredLRP.Action = &models.Action{}
			assertDesiredLRPValidationFailsWithMessage(desiredLRP, "action")
		})

		It("requires a valid action", func() {
			desiredLRP.Action = &models.Action{
				UploadAction: &models.UploadAction{
					From: "web_location",
				},
			}
			assertDesiredLRPValidationFailsWithMessage(desiredLRP, "to")
		})

		It("requires a valid setup action if specified", func() {
			desiredLRP.Setup = &models.Action{
				UploadAction: &models.UploadAction{
					From: "web_location",
				},
			}
			assertDesiredLRPValidationFailsWithMessage(desiredLRP, "to")
		})

		It("requires a setup action with an inner action", func() {
			desiredLRP.Setup = &models.Action{}
			assertDesiredLRPValidationFailsWithMessage(desiredLRP, "setup")
		})

		It("requires a valid monitor action if specified", func() {
			desiredLRP.Monitor = &models.Action{
				UploadAction: &models.UploadAction{
					From: "web_location",
				},
			}
			assertDesiredLRPValidationFailsWithMessage(desiredLRP, "to")
		})

		It("requires a monitor action with an inner action", func() {
			desiredLRP.Monitor = &models.Action{}
			assertDesiredLRPValidationFailsWithMessage(desiredLRP, "monitor")
		})

		It("requires a valid CPU weight", func() {
			desiredLRP.CpuWeight = 101
			assertDesiredLRPValidationFailsWithMessage(desiredLRP, "cpu_weight")
		})

		It("requires a valid MemoryMb", func() {
			desiredLRP.MemoryMb = -1
			assertDesiredLRPValidationFailsWithMessage(desiredLRP, "memory_mb")
		})

		It("requires a valid DiskMb", func() {
			desiredLRP.DiskMb = -1
			assertDesiredLRPValidationFailsWithMessage(desiredLRP, "disk_mb")
		})

		It("requires a valid MaxPids", func() {
			desiredLRP.MaxPids = -1
			assertDesiredLRPValidationFailsWithMessage(desiredLRP, "max_pids")
		})

		It("limits the annotation length", func() {
			desiredLRP.Annotation = randStringBytes(50000)
			assertDesiredLRPValidationFailsWithMessage(desiredLRP, "annotation")
		})

		Context("when security group is present", func() {
			It("must be valid", func() {
				desiredLRP.EgressRules = []*models.SecurityGroupRule{{
					Protocol: "foo",
				}}
				assertDesiredLRPValidationFailsWithMessage(desiredLRP, "egress_rules")
			})
		})

		Context("when security group is not present", func() {
			It("does not error", func() {
				desiredLRP.EgressRules = []*models.SecurityGroupRule{}

				validationErr := desiredLRP.Validate()
				Expect(validationErr).NotTo(HaveOccurred())
			})
		})

		Context("when cached dependencies are specified", func() {
			It("requires requires them to be valid", func() {
				desiredLRP.CachedDependencies = []*models.CachedDependency{
					{
						To:   "",
						From: "",
					},
				}
				desiredLRP.LegacyDownloadUser = "user"
				assertDesiredLRPValidationFailsWithMessage(desiredLRP, "cached_dependency")
			})

			It("requires a legacy download user", func() {
				desiredLRP.CachedDependencies = []*models.CachedDependency{
					{
						To:   "here",
						From: "there",
					},
				}
				assertDesiredLRPValidationFailsWithMessage(desiredLRP, "legacy_download_user")
			})

			It("requires a valid checksum algorithm", func() {
				desiredLRP.CachedDependencies = []*models.CachedDependency{
					{
						To:                "here",
						From:              "there",
						ChecksumAlgorithm: "wrong algorithm",
						ChecksumValue:     "sum value",
					},
				}
				desiredLRP.LegacyDownloadUser = "user"
				assertDesiredLRPValidationFailsWithMessage(desiredLRP, "invalid algorithm")
			})

			It("requires a valid checksum value", func() {
				desiredLRP.CachedDependencies = []*models.CachedDependency{
					{
						To:                "here",
						From:              "there",
						ChecksumAlgorithm: "md5",
					},
				}
				desiredLRP.LegacyDownloadUser = "user"
				assertDesiredLRPValidationFailsWithMessage(desiredLRP, "value")
			})
		})

		Context("when image credentials are specified", func() {
			It("is valid when both credentials are supplied", func() {
				desiredLRP.ImageUsername = "something"
				desiredLRP.ImagePassword = "something"
				Expect(desiredLRP.Validate()).To(Succeed())
			})

			It("is valid when no credentials are supplied", func() {
				desiredLRP.ImageUsername = ""
				desiredLRP.ImagePassword = ""
				Expect(desiredLRP.Validate()).To(Succeed())
			})

			It("is invalid when providing just a username", func() {
				desiredLRP.ImageUsername = "something"
				desiredLRP.ImagePassword = ""
				assertDesiredLRPValidationFailsWithMessage(desiredLRP, "image_password")
			})

			It("is invalid when providing just a password", func() {
				desiredLRP.ImageUsername = ""
				desiredLRP.ImagePassword = "something"
				assertDesiredLRPValidationFailsWithMessage(desiredLRP, "image_username")
			})
		})
	})
})

var _ = Describe("DesiredLRPUpdate", func() {
	var desiredLRPUpdate models.DesiredLRPUpdate

	BeforeEach(func() {
		two := int32(2)
		someText := "some-text"
		desiredLRPUpdate.Instances = &two
		desiredLRPUpdate.Annotation = &someText
	})

	Describe("Validate", func() {
		var assertDesiredLRPValidationFailsWithMessage = func(lrp models.DesiredLRPUpdate, substring string) {
			validationErr := lrp.Validate()
			Expect(validationErr).To(HaveOccurred())
			Expect(validationErr.Error()).To(ContainSubstring(substring))
		}

		It("requires a positive nonzero number of instances", func() {
			minusOne := int32(-1)
			desiredLRPUpdate.Instances = &minusOne
			assertDesiredLRPValidationFailsWithMessage(desiredLRPUpdate, "instances")

			zero := int32(0)
			desiredLRPUpdate.Instances = &zero
			validationErr := desiredLRPUpdate.Validate()
			Expect(validationErr).NotTo(HaveOccurred())

			one := int32(1)
			desiredLRPUpdate.Instances = &one
			validationErr = desiredLRPUpdate.Validate()
			Expect(validationErr).NotTo(HaveOccurred())
		})

		It("limits the annotation length", func() {
			largeString := randStringBytes(50000)
			desiredLRPUpdate.Annotation = &largeString
			assertDesiredLRPValidationFailsWithMessage(desiredLRPUpdate, "annotation")
		})
	})
})

func randStringBytes(n int) string {
	rb := make([]byte, n)
	rand.Read(rb)
	rs := base64.URLEncoding.EncodeToString(rb)
	return rs
}

var _ = Describe("DesiredLRPKey", func() {
	const guid = "valid-guid"
	const domain = "valid-domain"
	const log = "valid-log-guid"

	DescribeTable("Validation",
		func(key models.DesiredLRPKey, expectedErr string) {
			err := key.Validate()
			if expectedErr == "" {
				Expect(err).NotTo(HaveOccurred())
			} else {
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring(expectedErr))
			}
		},
		Entry("valid key", models.NewDesiredLRPKey(guid, domain, log), ""),
		Entry("blank process guid", models.NewDesiredLRPKey("", domain, log), "process_guid"),
		Entry("blank domain", models.NewDesiredLRPKey(guid, "", log), "domain"),
		Entry("blank log guid is valid", models.NewDesiredLRPKey(guid, domain, ""), ""),
	)
	Context("process_guid only contains `A-Z`, `a-z`, `0-9`, `-`, and `_`", func() {
		validGuids := []string{"a", "A", "0", "-", "_", "-aaaa", "_-aaa", "09a87aaa-_aASKDn"}
		for _, validGuid := range validGuids {
			func(validGuid string) {
				It(fmt.Sprintf("'%s' is a valid process_guid", validGuid), func() {
					key := models.NewDesiredLRPKey(validGuid, domain, log)
					err := key.Validate()
					Expect(err).NotTo(HaveOccurred())
				})
			}(validGuid)
		}

		invalidGuids := []string{"", "bang!", "!!!", "\\slash", "star*", "params()", "invalid/key", "with.dots"}
		for _, invalidGuid := range invalidGuids {
			func(invalidGuid string) {
				It(fmt.Sprintf("'%s' is an invalid process_guid", invalidGuid), func() {
					key := models.NewDesiredLRPKey(invalidGuid, domain, log)
					err := key.Validate()
					Expect(err).To(HaveOccurred())
					Expect(err.Error()).To(ContainSubstring("process_guid"))
				})
			}(invalidGuid)
		}
	})
})

var _ = Describe("DesiredLRPResource", func() {
	const rootFs = "preloaded://linux64"
	const memoryMb = 256
	const diskMb = 256
	const maxPids = 256

	DescribeTable("Validation",
		func(key models.DesiredLRPResource, expectedErr string) {
			err := key.Validate()
			if expectedErr == "" {
				Expect(err).NotTo(HaveOccurred())
			} else {
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring(expectedErr))
			}
		},
		Entry("valid resource", models.NewDesiredLRPResource(memoryMb, diskMb, maxPids, rootFs), ""),
		Entry("invalid rootFs", models.NewDesiredLRPResource(memoryMb, diskMb, maxPids, "BAD URL"), "rootfs"),
		Entry("invalid memoryMb", models.NewDesiredLRPResource(-1, diskMb, maxPids, rootFs), "memory_mb"),
		Entry("invalid diskMb", models.NewDesiredLRPResource(memoryMb, -1, maxPids, rootFs), "disk_mb"),
		Entry("invalid maxPids", models.NewDesiredLRPResource(memoryMb, diskMb, -1, rootFs), "max_pids"),
	)
})

var _ = Describe("DesiredLRPSchedulingInfo", func() {
	const annotation = "the annotation"
	const instances = 2
	var (
		largeString = randStringBytes(50000)
		rawMessage  = json.RawMessage([]byte(`{"port": 8080,"hosts":["new-route-1","new-route-2"]}`))
		routes      = models.Routes{
			"router": &rawMessage,
		}
		largeRoutingString = randStringBytes(129 * 1024)
		largeRoute         = json.RawMessage([]byte(largeRoutingString))
		largeRoutes        = models.Routes{
			"router": &largeRoute,
		}
		tag = models.ModificationTag{}
	)

	DescribeTable("Validation",
		func(key models.DesiredLRPSchedulingInfo, expectedErr string) {
			err := key.Validate()
			if expectedErr == "" {
				Expect(err).NotTo(HaveOccurred())
			} else {
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring(expectedErr))
			}
		},
		Entry("valid scheduling info", models.NewDesiredLRPSchedulingInfo(newValidLRPKey(), annotation, instances, newValidResource(), routes, tag, nil, nil), ""),
		Entry("invalid annotation", models.NewDesiredLRPSchedulingInfo(newValidLRPKey(), largeString, instances, newValidResource(), routes, tag, nil, nil), "annotation"),
		Entry("invalid instances", models.NewDesiredLRPSchedulingInfo(newValidLRPKey(), annotation, -2, newValidResource(), routes, tag, nil, nil), "instances"),
		Entry("invalid key", models.NewDesiredLRPSchedulingInfo(models.DesiredLRPKey{}, annotation, instances, newValidResource(), routes, tag, nil, nil), "process_guid"),
		Entry("invalid resource", models.NewDesiredLRPSchedulingInfo(newValidLRPKey(), annotation, instances, models.DesiredLRPResource{}, routes, tag, nil, nil), "rootfs"),
		Entry("invalid routes", models.NewDesiredLRPSchedulingInfo(newValidLRPKey(), annotation, instances, newValidResource(), largeRoutes, tag, nil, nil), "routes"),
	)
})

var _ = Describe("DesiredLRPRunInfo", func() {
	var envVars = []models.EnvironmentVariable{{"FOO", "bar"}}
	var action = model_helpers.NewValidAction()
	const startTimeoutMs int64 = 12
	const privileged = true
	var ports = []uint32{80, 443}
	var egressRules = model_helpers.NewValidEgressRules()
	const logSource = "log-source"
	const metricsGuid = "metrics-guid"
	const cpuWeight = 50
	var createdAt = time.Unix(123, 456)
	var trustedSystemCertificatesPath = "/etc/cf-system-certificates"
	var httpCheckDef = model_helpers.NewValidHTTPCheckDefinition()

	DescribeTable("Validation",
		func(key models.DesiredLRPRunInfo, expectedErr string) {
			err := key.Validate()
			if expectedErr == "" {
				Expect(err).NotTo(HaveOccurred())
			} else {
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring(expectedErr))
			}
		},
		Entry("valid run info", models.NewDesiredLRPRunInfo(newValidLRPKey(), createdAt, envVars, nil, action, action, action, startTimeoutMs, privileged, cpuWeight, ports, egressRules, logSource, metricsGuid, "legacy-jim", trustedSystemCertificatesPath, []*models.VolumeMount{}, nil, nil, "", "", httpCheckDef), ""),
		Entry("invalid key", models.NewDesiredLRPRunInfo(models.DesiredLRPKey{}, createdAt, envVars, nil, action, action, action, startTimeoutMs, privileged, cpuWeight, ports, egressRules, logSource, metricsGuid, "legacy-jim", trustedSystemCertificatesPath, []*models.VolumeMount{}, nil, nil, "", "", httpCheckDef), "process_guid"),
		Entry("invalid env vars", models.NewDesiredLRPRunInfo(newValidLRPKey(), createdAt, append(envVars, models.EnvironmentVariable{}), nil, action, action, action, startTimeoutMs, privileged, cpuWeight, ports, egressRules, logSource, metricsGuid, "legacy-jim", trustedSystemCertificatesPath, []*models.VolumeMount{}, nil, nil, "", "", httpCheckDef), "name"),
		Entry("invalid setup action", models.NewDesiredLRPRunInfo(newValidLRPKey(), createdAt, envVars, nil, &models.Action{}, action, action, startTimeoutMs, privileged, cpuWeight, ports, egressRules, logSource, metricsGuid, "legacy-jim", trustedSystemCertificatesPath, []*models.VolumeMount{}, nil, nil, "", "", httpCheckDef), "inner-action"),
		Entry("invalid run action", models.NewDesiredLRPRunInfo(newValidLRPKey(), createdAt, envVars, nil, action, &models.Action{}, action, startTimeoutMs, privileged, cpuWeight, ports, egressRules, logSource, metricsGuid, "legacy-jim", trustedSystemCertificatesPath, []*models.VolumeMount{}, nil, nil, "", "", httpCheckDef), "inner-action"),
		Entry("invalid monitor action", models.NewDesiredLRPRunInfo(newValidLRPKey(), createdAt, envVars, nil, action, action, &models.Action{}, startTimeoutMs, privileged, cpuWeight, ports, egressRules, logSource, metricsGuid, "legacy-jim", trustedSystemCertificatesPath, []*models.VolumeMount{}, nil, nil, "", "", httpCheckDef), "inner-action"),
		Entry("invalid http check definition", models.NewDesiredLRPRunInfo(newValidLRPKey(), createdAt, envVars, nil, action, action, action, startTimeoutMs, privileged, cpuWeight, ports, egressRules, logSource, metricsGuid, "legacy-jim", trustedSystemCertificatesPath, []*models.VolumeMount{}, nil, nil, "", "", &models.CheckDefinition{[]*models.Check{&models.Check{HttpCheck: &models.HTTPCheck{Port: 65536}}}, "healthcheck_log_source"}), "port"),
		Entry("invalid tcp check definition", models.NewDesiredLRPRunInfo(newValidLRPKey(), createdAt, envVars, nil, action, action, action, startTimeoutMs, privileged, cpuWeight, ports, egressRules, logSource, metricsGuid, "legacy-jim", trustedSystemCertificatesPath, []*models.VolumeMount{}, nil, nil, "", "", &models.CheckDefinition{[]*models.Check{&models.Check{TcpCheck: &models.TCPCheck{}}}, "healthcheck_log_source"}), "port"),
		Entry("invalid check in check definition", models.NewDesiredLRPRunInfo(newValidLRPKey(), createdAt, envVars, nil, action, action, action, startTimeoutMs, privileged, cpuWeight, ports, egressRules, logSource, metricsGuid, "legacy-jim", trustedSystemCertificatesPath, []*models.VolumeMount{}, nil, nil, "", "", &models.CheckDefinition{[]*models.Check{&models.Check{HttpCheck: &models.HTTPCheck{}, TcpCheck: &models.TCPCheck{}}}, "healthcheck_log_source"}), "check"),
		Entry("invalid cpu weight", models.NewDesiredLRPRunInfo(newValidLRPKey(), createdAt, envVars, nil, action, action, action, startTimeoutMs, privileged, 150, ports, egressRules, logSource, metricsGuid, "legacy-jim", trustedSystemCertificatesPath, []*models.VolumeMount{}, nil, nil, "", "", httpCheckDef), "cpu_weight"),
		Entry("invalid legacy download user", models.NewDesiredLRPRunInfo(newValidLRPKey(), createdAt, envVars, []*models.CachedDependency{{To: "here", From: "there"}}, action, action, action, startTimeoutMs, privileged, cpuWeight, ports, egressRules, logSource, metricsGuid, "", trustedSystemCertificatesPath, []*models.VolumeMount{}, nil, nil, "", "", httpCheckDef), "legacy_download_user"),
		Entry("invalid cached dependency", models.NewDesiredLRPRunInfo(newValidLRPKey(), createdAt, envVars, []*models.CachedDependency{{To: "here"}}, action, action, action, startTimeoutMs, privileged, cpuWeight, ports, egressRules, logSource, metricsGuid, "user", trustedSystemCertificatesPath, []*models.VolumeMount{}, nil, nil, "", "", httpCheckDef), "cached_dependency"),
		Entry("invalid volume mount", models.NewDesiredLRPRunInfo(newValidLRPKey(), createdAt, envVars, nil, action, action, action, startTimeoutMs, privileged, cpuWeight, ports, egressRules, logSource, metricsGuid, "user", trustedSystemCertificatesPath, []*models.VolumeMount{{DeprecatedConfig: []byte(`lol`)}}, nil, nil, "", "", httpCheckDef), "volume_mount"),
		Entry("invalid image username", models.NewDesiredLRPRunInfo(newValidLRPKey(), createdAt, envVars, nil, action, action, action, startTimeoutMs, privileged, cpuWeight, ports, egressRules, logSource, metricsGuid, "user", trustedSystemCertificatesPath, []*models.VolumeMount{}, nil, nil, "", "password", httpCheckDef), "image_username"),
		Entry("invalid image password", models.NewDesiredLRPRunInfo(newValidLRPKey(), createdAt, envVars, nil, action, action, action, startTimeoutMs, privileged, cpuWeight, ports, egressRules, logSource, metricsGuid, "user", trustedSystemCertificatesPath, []*models.VolumeMount{}, nil, nil, "username", "", httpCheckDef), "image_password"),
	)
})

func newValidLRPKey() models.DesiredLRPKey {
	return models.NewDesiredLRPKey("some-guid", "domain", "log-guid")
}

func newValidResource() models.DesiredLRPResource {
	return models.NewDesiredLRPResource(256, 256, 256, "preloaded://linux64")
}
