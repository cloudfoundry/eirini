package stset_test

import (
	"context"
	"crypto/rand"
	"math/big"
	"testing"

	"code.cloudfoundry.org/eirini/api"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func TestStset(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Stset Suite")
}

var ctx context.Context

var _ = BeforeEach(func() {
	ctx = context.Background()
})

func createLRP(name string) *api.LRP {
	lastUpdated := randStringBytes()

	return &api.LRP{
		LRPIdentifier: api.LRPIdentifier{
			GUID:    "guid_1234",
			Version: "version_1234",
		},
		ProcessType:     "worker",
		AppName:         name,
		AppGUID:         "premium_app_guid_1234",
		SpaceName:       "space-foo",
		SpaceGUID:       "space-guid",
		TargetInstances: 1,
		OrgName:         "org-foo",
		OrgGUID:         "org-guid",
		Command: []string{
			"/bin/sh",
			"-c",
			"while true; do echo hello; sleep 10;done",
		},
		RunningInstances: 0,
		MemoryMB:         1024,
		DiskMB:           2048,
		CPUWeight:        2,
		Image:            "busybox",
		Ports:            []int32{8888, 9999},
		LastUpdated:      lastUpdated,
		VolumeMounts: []api.VolumeMount{
			{
				ClaimName: "some-claim",
				MountPath: "/some/path",
			},
		},
		LRP: "original request",
		UserDefinedAnnotations: map[string]string{
			"prometheus.io/scrape": "secret-value",
		},
	}
}

func randStringBytes() string {
	b := make([]byte, 10)
	for i := range b {
		randomNumber, err := rand.Int(rand.Reader, big.NewInt(int64(len(letterBytes))))
		Expect(err).NotTo(HaveOccurred())

		b[i] = letterBytes[randomNumber.Int64()]
	}

	return string(b)
}
