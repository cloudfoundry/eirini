package models_test

import (
	"encoding/json"
	"strings"
	"time"

	"code.cloudfoundry.org/bbs/format"
	"code.cloudfoundry.org/bbs/models"
	"github.com/gogo/protobuf/proto"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Task", func() {
	var taskPayload string
	var task models.Task

	BeforeEach(func() {
		taskPayload = `{
		"task_guid":"some-guid",
		"domain":"some-domain",
		"rootfs": "docker:///docker.com/docker",
		"env":[
			{
				"name":"ENV_VAR_NAME",
				"value":"an environmment value"
			}
		],
		"cell_id":"cell",
		"action": {
			"download":{
				"from":"old_location",
				"to":"new_location",
				"cache_key":"the-cache-key",
				"user":"someone",
				"checksum_algorithm": "md5",
				"checksum_value": "some value"
			}
		},
		"result_file":"some-file.txt",
		"result": "turboencabulated",
		"failed":true,
		"failure_reason":"because i said so",
		"memory_mb":256,
		"disk_mb":1024,
		"cpu_weight": 42,
		"privileged": true,
		"log_guid": "123",
		"log_source": "APP",
		"metrics_guid": "456",
		"created_at": 1393371971000000000,
		"updated_at": 1393371971000000010,
		"first_completed_at": 1393371971000000030,
		"state": "Pending",
		"annotation": "[{\"anything\": \"you want!\"}]... dude",
		"network": {
			"properties": {
				"some-key": "some-value",
				"some-other-key": "some-other-value"
			}
		},
		"egress_rules": [
			{
				"protocol": "tcp",
				"destinations": ["0.0.0.0/0"],
				"port_range": {
					"start": 1,
					"end": 1024
				},
				"log": true
			},
			{
				"protocol": "udp",
				"destinations": ["8.8.0.0/16"],
				"ports": [53],
				"log": false
			}
		],
		"completion_callback_url":"http://user:password@a.b.c/d/e/f",
		"max_pids": 256,
		"certificate_properties": {
			"organizational_unit": ["stuff"]
		},
		"image_username": "jake",
		"image_password": "thedog"
	}`

		task = models.Task{
			TaskDefinition: &models.TaskDefinition{
				RootFs: "docker:///docker.com/docker",
				EnvironmentVariables: []*models.EnvironmentVariable{
					{
						Name:  "ENV_VAR_NAME",
						Value: "an environmment value",
					},
				},
				Action: models.WrapAction(&models.DownloadAction{
					From:              "old_location",
					To:                "new_location",
					CacheKey:          "the-cache-key",
					User:              "someone",
					ChecksumAlgorithm: "md5",
					ChecksumValue:     "some value",
				}),
				MemoryMb:    256,
				DiskMb:      1024,
				MaxPids:     256,
				CpuWeight:   42,
				Privileged:  true,
				LogGuid:     "123",
				LogSource:   "APP",
				MetricsGuid: "456",
				ResultFile:  "some-file.txt",

				EgressRules: []*models.SecurityGroupRule{
					{
						Protocol:     "tcp",
						Destinations: []string{"0.0.0.0/0"},
						PortRange: &models.PortRange{
							Start: 1,
							End:   1024,
						},
						Log: true,
					},
					{
						Protocol:     "udp",
						Destinations: []string{"8.8.0.0/16"},
						Ports:        []uint32{53},
					},
				},

				Annotation: `[{"anything": "you want!"}]... dude`,
				Network: &models.Network{
					Properties: map[string]string{
						"some-key":       "some-value",
						"some-other-key": "some-other-value",
					},
				},
				CompletionCallbackUrl: "http://user:password@a.b.c/d/e/f",
				CertificateProperties: &models.CertificateProperties{
					OrganizationalUnit: []string{"stuff"},
				},
				ImageUsername: "jake",
				ImagePassword: "thedog",
			},
			TaskGuid:         "some-guid",
			Domain:           "some-domain",
			CreatedAt:        time.Date(2014, time.February, 25, 23, 46, 11, 00, time.UTC).UnixNano(),
			UpdatedAt:        time.Date(2014, time.February, 25, 23, 46, 11, 10, time.UTC).UnixNano(),
			FirstCompletedAt: time.Date(2014, time.February, 25, 23, 46, 11, 30, time.UTC).UnixNano(),
			State:            models.Task_Pending,
			CellId:           "cell",
			Result:           "turboencabulated",
			Failed:           true,
			FailureReason:    "because i said so",
		}
	})

	Describe("serialization", func() {
		It("successfully round trips through json and protobuf", func() {
			jsonSerialization, err := json.Marshal(task)
			Expect(err).NotTo(HaveOccurred())
			Expect(jsonSerialization).To(MatchJSON(taskPayload))

			protoSerialization, err := proto.Marshal(&task)
			Expect(err).NotTo(HaveOccurred())

			var protoDeserialization models.Task
			err = proto.Unmarshal(protoSerialization, &protoDeserialization)
			Expect(err).NotTo(HaveOccurred())

			Expect(protoDeserialization).To(Equal(task))
		})
	})

	Describe("VersionDownTo", func() {
		Context("V1", func() {
			BeforeEach(func() {
				task.Action = models.WrapAction(models.Timeout(
					&models.RunAction{
						Path: "/the/path",
						User: "the user",
					},
					10*time.Millisecond,
				))
			})

			It("converts TimeoutMs to Timeout in Nanoseconds", func() {
				task.VersionDownTo(format.V1)
				Expect(task.GetAction().GetTimeoutAction().DeprecatedTimeoutNs).To(BeEquivalentTo(10 * time.Millisecond))
			})
		})

		Context("V0", func() {
			var (
				downloadAction1, downloadAction2 *models.DownloadAction
			)

			Context("timeouts", func() {
				BeforeEach(func() {
					task.Action = models.WrapAction(models.Timeout(
						&models.RunAction{
							Path: "/the/path",
							User: "the user",
						},
						10*time.Millisecond,
					))
				})

				It("converts TimeoutMs to Timeout in Nanoseconds", func() {
					task.VersionDownTo(format.V0)
					Expect(task.GetAction().GetTimeoutAction().DeprecatedTimeoutNs).To(BeEquivalentTo(10 * time.Millisecond))
				})
			})

			Describe("downloads", func() {
				BeforeEach(func() {
					task.CachedDependencies = []*models.CachedDependency{
						{Name: "name-1", From: "from-1", To: "to-1", CacheKey: "cache-key-1", LogSource: "log-source-1"},
						{Name: "name-2", From: "from-2", To: "to-2", CacheKey: "cache-key-2", LogSource: "log-source-2"},
					}
					task.LegacyDownloadUser = "bob"

					downloadAction1 = &models.DownloadAction{
						Artifact:  "name-1",
						From:      "from-1",
						To:        "to-1",
						CacheKey:  "cache-key-1",
						LogSource: "log-source-1",
						User:      "bob",
					}

					downloadAction2 = &models.DownloadAction{
						Artifact:  "name-2",
						From:      "from-2",
						To:        "to-2",
						CacheKey:  "cache-key-2",
						LogSource: "log-source-2",
						User:      "bob",
					}
				})

				Context("when there is no existing setup action", func() {
					BeforeEach(func() {
						task.Action = nil
					})

					It("converts a cache dependency into download action", func() {
						newTask := task.VersionDownTo(format.V0)
						Expect(newTask.Action.SerialAction.Actions).To(HaveLen(1))
						Expect(newTask.Action.SerialAction.Actions[0].ParallelAction.Actions).To(HaveLen(2))

						Expect(*newTask.Action.SerialAction.Actions[0].ParallelAction.Actions[0].DownloadAction).To(Equal(*downloadAction1))
						Expect(*newTask.Action.SerialAction.Actions[0].ParallelAction.Actions[1].DownloadAction).To(Equal(*downloadAction2))

						Expect(*newTask.Action).To(Equal(models.Action{
							SerialAction: &models.SerialAction{
								Actions: []*models.Action{
									{
										ParallelAction: &models.ParallelAction{
											Actions: []*models.Action{
												&models.Action{DownloadAction: downloadAction1},
												&models.Action{DownloadAction: downloadAction2},
											},
										},
									},
								},
							},
						}))
					})
				})

				Context("when there is an existing action", func() {
					It("appends the new converted step action to the front", func() {
						newTask := task.VersionDownTo(format.V0)
						Expect(newTask.Action.SerialAction.Actions).To(HaveLen(2))
						Expect(newTask.Action.SerialAction.Actions[0].ParallelAction.Actions).To(HaveLen(2))

						Expect(*newTask.Action).To(Equal(models.Action{
							SerialAction: &models.SerialAction{
								Actions: []*models.Action{
									{
										ParallelAction: &models.ParallelAction{
											Actions: []*models.Action{
												&models.Action{DownloadAction: downloadAction1},
												&models.Action{DownloadAction: downloadAction2},
											},
										},
									},
									task.Action,
								},
							},
						}))
					})
				})

				Context("when there are no cache dependencies", func() {
					BeforeEach(func() {
						task.CachedDependencies = nil
					})

					It("keeps the current action", func() {
						newTask := task.VersionDownTo(format.V0)
						Expect(*newTask.Action).To(Equal(*task.Action))
					})
				})
			})
		})
	})

	Describe("Validate", func() {
		Context("when the task has a domain, valid guid, stack, and valid action", func() {
			It("is valid", func() {
				task = models.Task{
					Domain:   "some-domain",
					TaskGuid: "some-task-guid",
					TaskDefinition: &models.TaskDefinition{
						RootFs: "some:rootfs",
						Action: models.WrapAction(&models.RunAction{
							Path: "ls",
							User: "me",
						}),
					},
				}

				err := task.Validate()
				Expect(err).NotTo(HaveOccurred())
			})
		})

		Context("when the task GUID is present but invalid", func() {
			It("returns an error indicating so", func() {
				task = models.Task{
					Domain:   "some-domain",
					TaskGuid: "invalid/guid",
					TaskDefinition: &models.TaskDefinition{
						RootFs: "some:rootfs",
						Action: models.WrapAction(&models.RunAction{
							Path: "ls",
							User: "me",
						}),
					},
				}

				err := task.Validate()
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("task_guid"))
			})
		})

		for _, testCase := range []ValidatorErrorCase{
			{
				"task_guid",
				&models.Task{
					Domain: "some-domain",
					TaskDefinition: &models.TaskDefinition{
						RootFs: "some:rootfs",
						Action: models.WrapAction(&models.RunAction{
							Path: "ls",
							User: "me",
						}),
					},
				},
			},
			{
				"rootfs",
				&models.Task{
					Domain:   "some-domain",
					TaskGuid: "task-guid",
					TaskDefinition: &models.TaskDefinition{
						Action: models.WrapAction(&models.RunAction{
							Path: "ls",
							User: "me",
						}),
					},
				},
			},
			{
				"rootfs",
				&models.Task{
					Domain:   "some-domain",
					TaskGuid: "task-guid",
					TaskDefinition: &models.TaskDefinition{
						RootFs: ":invalid-url",
						Action: models.WrapAction(&models.RunAction{
							Path: "ls",
							User: "me",
						}),
					},
				},
			},
			{
				"rootfs",
				&models.Task{
					Domain:   "some-domain",
					TaskGuid: "task-guid",
					TaskDefinition: &models.TaskDefinition{
						RootFs: "invalid-absolute-url",
						Action: models.WrapAction(&models.RunAction{
							Path: "ls",
							User: "me",
						}),
					},
				},
			},
			{
				"domain",
				&models.Task{
					TaskGuid: "task-guid",
					TaskDefinition: &models.TaskDefinition{
						RootFs: "some:rootfs",
						Action: models.WrapAction(&models.RunAction{
							Path: "ls",
							User: "me",
						}),
					},
				},
			},
			{
				"action",
				&models.Task{
					Domain:   "some-domain",
					TaskGuid: "task-guid",
					TaskDefinition: &models.TaskDefinition{
						RootFs: "some:rootfs",
						Action: nil,
					},
				}},
			{
				"path",
				&models.Task{
					Domain:   "some-domain",
					TaskGuid: "task-guid",
					TaskDefinition: &models.TaskDefinition{
						RootFs: "some:rootfs",
						Action: models.WrapAction(&models.RunAction{User: "me"}),
					},
				},
			},
			{
				"annotation",
				&models.Task{
					Domain:   "some-domain",
					TaskGuid: "task-guid",
					TaskDefinition: &models.TaskDefinition{
						RootFs: "some:rootfs",
						Action: models.WrapAction(&models.RunAction{
							Path: "ls",
							User: "me",
						}),
						Annotation: strings.Repeat("a", 10*1024+1),
					},
				},
			},
			{
				"cpu_weight",
				&models.Task{
					Domain:   "some-domain",
					TaskGuid: "task-guid",
					TaskDefinition: &models.TaskDefinition{
						RootFs: "some:rootfs",
						Action: models.WrapAction(&models.RunAction{
							Path: "ls",
							User: "me",
						}),
						CpuWeight: 101,
					},
				},
			},
			{
				"memory_mb",
				&models.Task{
					Domain:   "some-domain",
					TaskGuid: "task-guid",
					TaskDefinition: &models.TaskDefinition{
						RootFs: "some:rootfs",
						Action: models.WrapAction(&models.RunAction{
							Path: "ls",
							User: "me",
						}),
						MemoryMb: -1,
					},
				},
			},
			{
				"disk_mb",
				&models.Task{
					Domain:   "some-domain",
					TaskGuid: "task-guid",
					TaskDefinition: &models.TaskDefinition{
						RootFs: "some:rootfs",
						Action: models.WrapAction(&models.RunAction{
							Path: "ls",
							User: "me",
						}),
						DiskMb: -1,
					},
				},
			},
			{
				"max_pids",
				&models.Task{
					Domain:   "some-domain",
					TaskGuid: "task-guid",
					TaskDefinition: &models.TaskDefinition{
						RootFs: "some:rootfs",
						Action: models.WrapAction(&models.RunAction{
							Path: "ls",
							User: "me",
						}),
						MaxPids: -1,
					},
				},
			},
			{
				"egress_rules",
				&models.Task{
					Domain:   "some-domain",
					TaskGuid: "task-guid",
					TaskDefinition: &models.TaskDefinition{
						RootFs: "some:rootfs",
						Action: models.WrapAction(&models.RunAction{
							Path: "ls",
							User: "me",
						}),
						EgressRules: []*models.SecurityGroupRule{
							{Protocol: "invalid"},
						},
					},
				},
			},
			{
				"legacy_download_user",
				&models.Task{
					TaskGuid: "guid-1",
					Domain:   "some-domain",
					TaskDefinition: &models.TaskDefinition{
						RootFs: "some-rootfs",
						CachedDependencies: []*models.CachedDependency{
							{
								To:   "here",
								From: "there",
							},
						},
					},
				},
			},
			{
				"cached_dependency",
				&models.Task{
					TaskGuid: "guid-1",
					Domain:   "some-domain",
					TaskDefinition: &models.TaskDefinition{
						RootFs: "some-rootfs",
						CachedDependencies: []*models.CachedDependency{
							{
								To: "here",
							},
						},
					},
				},
			},
			{
				"invalid algorithm",
				&models.Task{
					TaskGuid: "guid-1",
					Domain:   "some-domain",
					TaskDefinition: &models.TaskDefinition{
						RootFs: "some-rootfs",
						CachedDependencies: []*models.CachedDependency{
							{
								To:                "here",
								From:              "there",
								ChecksumAlgorithm: "wrong algorithm",
								ChecksumValue:     "some value",
							},
						},
					},
				},
			},
			{
				"image_username",
				&models.Task{
					Domain:   "some-domain",
					TaskGuid: "task-guid",
					TaskDefinition: &models.TaskDefinition{
						RootFs: "some:rootfs",
						Action: models.WrapAction(&models.RunAction{
							Path: "ls",
							User: "me",
						}),
						ImageUsername: "",
						ImagePassword: "thedog",
					},
				},
			},
			{
				"image_password",
				&models.Task{
					Domain:   "some-domain",
					TaskGuid: "task-guid",
					TaskDefinition: &models.TaskDefinition{
						RootFs: "some:rootfs",
						Action: models.WrapAction(&models.RunAction{
							Path: "ls",
							User: "me",
						}),
						ImageUsername: "jake",
						ImagePassword: "",
					},
				},
			},
		} {
			testValidatorErrorCase(testCase)
		}
	})
})
