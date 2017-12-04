package model_helpers

import (
	"encoding/json"
	"time"

	"code.cloudfoundry.org/bbs/models"
	. "github.com/onsi/gomega"
)

func NewValidActualLRP(guid string, index int32) *models.ActualLRP {
	actualLRP := &models.ActualLRP{
		ActualLRPKey:         models.NewActualLRPKey(guid, index, "some-domain"),
		ActualLRPInstanceKey: models.NewActualLRPInstanceKey("some-guid", "some-cell"),
		ActualLRPNetInfo:     models.NewActualLRPNetInfo("some-address", "container-address", models.NewPortMapping(2222, 4444)),
		CrashCount:           33,
		CrashReason:          "badness",
		State:                models.ActualLRPStateRunning,
		Since:                1138,
		ModificationTag: models.ModificationTag{
			Epoch: "some-epoch",
			Index: 999,
		},
	}
	err := actualLRP.Validate()
	Expect(err).NotTo(HaveOccurred())

	return actualLRP
}

func NewValidDesiredLRP(guid string) *models.DesiredLRP {
	myRouterJSON := json.RawMessage(`{"foo":"bar"}`)
	modTag := models.NewModificationTag("epoch", 0)
	desiredLRP := &models.DesiredLRP{
		ProcessGuid:          guid,
		Domain:               "some-domain",
		RootFs:               "some:rootfs",
		Instances:            1,
		EnvironmentVariables: []*models.EnvironmentVariable{{Name: "FOO", Value: "bar"}},
		CachedDependencies: []*models.CachedDependency{
			{Name: "app bits", From: "blobstore.com/bits/app-bits", To: "/usr/local/app", CacheKey: "cache-key", LogSource: "log-source"},
			{Name: "app bits with checksum", From: "blobstore.com/bits/app-bits-checksum", To: "/usr/local/app-checksum", CacheKey: "cache-key", LogSource: "log-source", ChecksumAlgorithm: "md5", ChecksumValue: "checksum-value"},
		},
		Setup:          models.WrapAction(&models.RunAction{Path: "ls", User: "name"}),
		Action:         models.WrapAction(&models.RunAction{Path: "ls", User: "name"}),
		StartTimeoutMs: 15000,
		Monitor: models.WrapAction(models.EmitProgressFor(
			models.Timeout(models.Try(models.Parallel(models.Serial(&models.RunAction{Path: "ls", User: "name"}))),
				10*time.Second,
			),
			"start-message",
			"success-message",
			"failure-message",
		)),
		CheckDefinition: &models.CheckDefinition{
			Checks: []*models.Check{
				&models.Check{
					HttpCheck: &models.HTTPCheck{
						Port:             8080,
						RequestTimeoutMs: 100,
						Path:             "",
					},
				},
			},
		},
		DiskMb:      512,
		MemoryMb:    1024,
		CpuWeight:   42,
		MaxPids:     1024,
		Routes:      &models.Routes{"my-router": &myRouterJSON},
		LogSource:   "some-log-source",
		LogGuid:     "some-log-guid",
		MetricsGuid: "some-metrics-guid",
		Annotation:  "some-annotation",
		Network: &models.Network{
			Properties: map[string]string{
				"some-key":       "some-value",
				"some-other-key": "some-other-value",
			},
		},
		EgressRules: []*models.SecurityGroupRule{{
			Protocol:     models.TCPProtocol,
			Destinations: []string{"1.1.1.1/32", "2.2.2.2/32"},
			PortRange:    &models.PortRange{Start: 10, End: 16000},
		}},
		ModificationTag:               &modTag,
		LegacyDownloadUser:            "legacy-dan",
		TrustedSystemCertificatesPath: "/etc/somepath",
		PlacementTags:                 []string{"red-tag", "blue-tag"},
		VolumeMounts: []*models.VolumeMount{
			{
				Driver:       "my-driver",
				ContainerDir: "/mnt/mypath",
				Mode:         "r",
				Shared: &models.SharedDevice{
					VolumeId:    "my-volume",
					MountConfig: `{"foo":"bar"}`,
				},
			},
		},
		CertificateProperties: &models.CertificateProperties{
			OrganizationalUnit: []string{"iamthelizardking", "iamthelizardqueen"},
		},
		ImageUsername: "image-username",
		ImagePassword: "image-password",
	}
	err := desiredLRP.Validate()
	Expect(err).NotTo(HaveOccurred())

	return desiredLRP
}

func NewValidTaskDefinition() *models.TaskDefinition {
	return &models.TaskDefinition{
		RootFs: "docker:///docker.com/docker",
		EnvironmentVariables: []*models.EnvironmentVariable{
			{
				Name:  "FOO",
				Value: "BAR",
			},
		},
		CachedDependencies: []*models.CachedDependency{
			{Name: "app bits", From: "blobstore.com/bits/app-bits", To: "/usr/local/app", CacheKey: "cache-key", LogSource: "log-source"},
			{Name: "app bits with checksum", From: "blobstore.com/bits/app-bits-checksum", To: "/usr/local/app-checksum", CacheKey: "cache-key", LogSource: "log-source", ChecksumAlgorithm: "md5", ChecksumValue: "checksum-value"},
		},
		Action: models.WrapAction(&models.RunAction{
			User:           "user",
			Path:           "echo",
			Args:           []string{"hello world"},
			ResourceLimits: &models.ResourceLimits{},
		}),
		MemoryMb:    256,
		DiskMb:      1024,
		MaxPids:     1024,
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
		LegacyDownloadUser:            "legacy-jim",
		TrustedSystemCertificatesPath: "/etc/somepath",
		VolumeMounts: []*models.VolumeMount{
			{
				Driver:       "my-driver",
				ContainerDir: "/mnt/mypath",
				Mode:         "r",
				Shared: &models.SharedDevice{
					VolumeId:    "my-volume",
					MountConfig: `{"foo":"bar"}`,
				},
			},
		},
		PlacementTags: []string{"red-tag", "blue-tag", "one-tag", "two-tag"},
		CertificateProperties: &models.CertificateProperties{
			OrganizationalUnit: []string{"iamthelizardking", "iamthelizardqueen"},
		},
		ImageUsername: "image-username",
		ImagePassword: "image-password",
	}
}

func NewValidEgressRules() []models.SecurityGroupRule {
	return []models.SecurityGroupRule{
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
	}
}

func NewValidTask(guid string) *models.Task {
	task := &models.Task{
		TaskGuid:       guid,
		Domain:         "some-domain",
		TaskDefinition: NewValidTaskDefinition(),

		CreatedAt:        time.Date(2014, time.February, 25, 23, 46, 11, 00, time.UTC).UnixNano(),
		UpdatedAt:        time.Date(2014, time.February, 25, 23, 46, 11, 10, time.UTC).UnixNano(),
		FirstCompletedAt: time.Date(2014, time.February, 25, 23, 46, 11, 30, time.UTC).UnixNano(),

		CellId:        "cell",
		State:         models.Task_Pending,
		Result:        "turboencabulated",
		Failed:        true,
		FailureReason: "because i said so",
	}

	err := task.Validate()
	if err != nil {
		panic(err)
	}
	return task
}

func NewValidAction() *models.Action {
	return models.WrapAction(&models.RunAction{Path: "ls", User: "name"})
}

func NewValidHTTPCheckDefinition() *models.CheckDefinition {
	checkDefinition := &models.CheckDefinition{
		Checks: []*models.Check{
			{
				HttpCheck: &models.HTTPCheck{
					Port:             12345,
					RequestTimeoutMs: 100,
					Path:             "/some/path",
				},
			},
		},
	}

	err := checkDefinition.Validate()
	if err != nil {
		panic(err)
	}
	return checkDefinition
}
