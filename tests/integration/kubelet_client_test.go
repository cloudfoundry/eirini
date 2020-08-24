package integration_test

import (
	"context"

	"code.cloudfoundry.org/eirini/k8s/kubelet"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = Describe("Kubelet Client", func() {
	var client kubelet.Client

	BeforeEach(func() {
		client = kubelet.NewClient(fixture.Clientset.CoreV1().RESTClient())
	})

	It("should return the stats summary for a node", func() {
		nodes, err := fixture.Clientset.CoreV1().Nodes().List(context.Background(), metav1.ListOptions{})
		Expect(err).ToNot(HaveOccurred())
		Expect(nodes.Items).ToNot(BeEmpty())

		name := nodes.Items[0].Name
		stats, err := client.StatsSummary(name)
		Expect(err).ToNot(HaveOccurred())
		Expect(stats.Pods).ToNot(BeEmpty())
		Expect(stats.Pods[0].PodRef.Name).ToNot(BeEmpty())
		Expect(stats.Pods[0].PodRef.Namespace).ToNot(BeEmpty())
		Expect(stats.Pods[0].Containers).ToNot(BeEmpty())
	})

	When("the node name is not correct", func() {
		It("should retrun an error", func() {
			name := "does-not-exist"
			_, err := client.StatsSummary(name)
			Expect(err).To(HaveOccurred())
		})
	})
})
