package integration_test

import (
	"code.cloudfoundry.org/eirini/k8s"
	"code.cloudfoundry.org/eirini/models/cf"
	"code.cloudfoundry.org/eirini/opi"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"k8s.io/api/apps/v1beta2"
	"k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = Describe("Instance {SYSTEM}", func() {

	var (
		instanceManager k8s.InstanceManager
		lrp             *opi.LRP
		err             error
	)

	cleanupStatefulSet := func(appName string) {
		backgroundPropagation := metav1.DeletePropagationBackground
		clientset.AppsV1beta2().StatefulSets(namespace).Delete(appName, &metav1.DeleteOptions{PropagationPolicy: &backgroundPropagation})
	}

	listStatefulSets := func() []v1beta2.StatefulSet {
		list, err := clientset.AppsV1beta2().StatefulSets(namespace).List(metav1.ListOptions{})
		Expect(err).NotTo(HaveOccurred())
		return list.Items
	}

	BeforeEach(func() {
		lrp = createLRP("odin")
	})

	AfterEach(func() {
		cleanupStatefulSet(lrp.Name)
		Eventually(listStatefulSets, timeout).Should(BeEmpty())
	})

	JustBeforeEach(func() {
		instanceManager = k8s.NewInstanceManager(
			clientset,
			namespace,
			k8s.UseStatefulSets,
		)
	})

	Context("When creating an LRP", func() {
		JustBeforeEach(func() {
			err = instanceManager.Create(lrp)
		})

		It("should not fail", func() {
			Expect(err).ToNot(HaveOccurred())
		})

		It("should create an StatefulSet with an associated pod", func() {
			var pod *v1.Pod
			Eventually(func() error {
				pod, err = clientset.CoreV1().Pods(namespace).Get(
					"odin-0",
					metav1.GetOptions{},
				)
				return err
			}, timeout).ShouldNot(HaveOccurred())
			Expect(pod.Name).To(Equal("odin-0"))
		})
	})

	Context("When deleting an LRP", func() {

		JustBeforeEach(func() {
			err = instanceManager.Create(lrp)
		})

		It("should not fail", func() {
			Expect(err).ToNot(HaveOccurred())
		})

		It("should delete the StatefulSet and the associated pod", func() {
			Eventually(func() []v1.Pod {
				list, err := clientset.CoreV1().Pods(namespace).List(metav1.ListOptions{})
				Expect(err).ToNot(HaveOccurred())
				return list.Items
			}, timeout).Should(BeEmpty())
		})
	})
})

func createLRP(name string) *opi.LRP {
	return &opi.LRP{
		Name: name,
		Command: []string{
			"/bin/sh",
			"-c",
			"while true; do echo hello; sleep 10;done",
		},
		TargetInstances: 1,
		Image:           "busybox",
		Metadata: map[string]string{
			cf.ProcessGUID: name,
		},
	}
}
