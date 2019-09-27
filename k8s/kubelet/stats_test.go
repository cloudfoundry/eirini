package kubelet_test

import (
	"code.cloudfoundry.org/eirini/k8s/kubelet"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Stats", func() {
	var (
		diskMetricsClient kubelet.DiskMetricsClient
		//nodeClient        *kubeletfakes.FakeNodeAPI
	)
	BeforeEach(func() {
		diskMetricsClient = kubelet.DiskMetricsClient{}
	})

	It("shoud not return an error", func() {
		_, err := diskMetricsClient.GetPodMetrics()
		Expect(err).ToNot(HaveOccurred())
	})
})
