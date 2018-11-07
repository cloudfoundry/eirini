// +build integration

package integration_test

import (
	"code.cloudfoundry.org/eirini"
	"code.cloudfoundry.org/eirini/k8s"
	"code.cloudfoundry.org/eirini/models/cf"
	"code.cloudfoundry.org/eirini/opi"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"k8s.io/api/apps/v1beta2"
	"k8s.io/api/core/v1"
	meta "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = Describe("StatefulSet Manager", func() {

	var (
		desirer opi.Desirer
		lrp     *opi.LRP
		err     error
	)

	BeforeEach(func() {
		lrp = &opi.LRP{
			Name: "odin",
			Command: []string{
				"/bin/sh",
				"-c",
				"while true; do echo hello; sleep 10;done",
			},
			TargetInstances: 2,
			Image:           "busybox",
			Metadata: map[string]string{
				cf.ProcessGUID: "odin",
			},
		}
	})

	AfterEach(func() {
		cleanupStatefulSet(lrp.Name)
		cleanupHeadlessService(lrp.Name)
		Eventually(listAllStatefulSets, timeout).Should(BeEmpty())
	})

	JustBeforeEach(func() {
		desirer = k8s.NewStatefulSetDesirer(
			clientset,
			namespace,
		)
	})

	Context("When creating a StatefulSet", func() {

		JustBeforeEach(func() {
			err = desirer.Desire(lrp)
		})

		It("should not fail", func() {
			Expect(err).ToNot(HaveOccurred())
		})

		It("should create a StatefulSet object", func() {
			statefulset, getErr := clientset.AppsV1beta2().StatefulSets(namespace).Get(lrp.Name, meta.GetOptions{})
			Expect(getErr).ToNot(HaveOccurred())

			Expect(statefulset.Name).To(Equal(lrp.Name))
			Expect(statefulset.Spec.Template.Spec.Containers[0].Command).To(Equal(lrp.Command))
			Expect(statefulset.Spec.Template.Spec.Containers[0].Image).To(Equal(lrp.Image))
			Expect(statefulset.Spec.Replicas).To(Equal(int32ptr(lrp.TargetInstances)))
		})

		It("should create all associated pods", func() {
			Eventually(func() []string {
				return getPodNames("odin")
			}, timeout).Should(ConsistOf("odin-0", "odin-1"))
		})

		It("should create a headless service", func() {
			service, getErr := clientset.CoreV1().Services(namespace).Get(eirini.GetInternalHeadlessServiceName("odin"), meta.GetOptions{})
			Expect(getErr).ToNot(HaveOccurred())
			Expect(service.Spec.ClusterIP).To(Equal("None"))
		})

	})

	Context("When deleting a LRP", func() {

		JustBeforeEach(func() {
			err = desirer.Desire(lrp)
			Expect(err).ToNot(HaveOccurred())
			err = desirer.Stop("odin")
		})

		It("should not fail", func() {
			Expect(err).ToNot(HaveOccurred())
		})

		It("should delete the StatefulSet object", func() {
			Eventually(func() []v1beta2.StatefulSet {
				return listStatefulSets("odin")
			}, timeout).Should(BeEmpty())
		})

		It("should delete the associated pods", func() {
			Eventually(func() []v1.Pod {
				return listPods("odin")
			}, timeout).Should(BeEmpty())
		})
	})

	Context("When getting an app", func() {

		JustBeforeEach(func() {
			err = desirer.Desire(lrp)
			Expect(err).ToNot(HaveOccurred())
		})

		It("correctly reports the running instances", func() {
			Eventually(func() int {
				l, e := desirer.Get("odin")
				Expect(e).ToNot(HaveOccurred())
				return l.RunningInstances
			}, timeout).Should(Equal(2))
		})

		Context("When one of the instances if failing", func() {
			BeforeEach(func() {
				lrp = &opi.LRP{
					Name: "odin",
					Command: []string{
						"/bin/sh",
						"-c",
						"if [ $INSTANCE_INDEX -eq 1 ]; then exit; else  while true; do echo hello; sleep 10;done; fi;",
					},
					TargetInstances: 2,
					Image:           "busybox",
					Metadata: map[string]string{
						cf.ProcessGUID: "odin",
					},
				}
			})

			It("correctly reports the running instances", func() {
				Eventually(func() int {
					lrp, err := desirer.Get("odin")
					Expect(err).ToNot(HaveOccurred())
					return lrp.RunningInstances
				}, timeout).Should(Equal(1))
			})
		})
	})

})

func int32ptr(i int) *int32 {
	i32 := int32(i)
	return &i32
}
