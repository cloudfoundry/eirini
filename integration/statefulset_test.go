// +build integration

package statefulsets_test

import (
	"fmt"

	"code.cloudfoundry.org/eirini/k8s"
	"code.cloudfoundry.org/eirini/models/cf"
	"code.cloudfoundry.org/eirini/opi"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"k8s.io/api/apps/v1beta2"
	v1 "k8s.io/api/core/v1"
)

var _ = Describe("StatefulSet Manager", func() {

	var (
		desirer opi.Desirer
		odinLRP *opi.LRP
		thorLRP *opi.LRP
		err     error
	)

	BeforeEach(func() {
		odinLRP = createLRP("odin")
		thorLRP = createLRP("thor")
	})

	AfterEach(func() {
		cleanupStatefulSet(odinLRP)
		cleanupStatefulSet(thorLRP)
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
			err = desirer.Desire(odinLRP)
			Expect(err).ToNot(HaveOccurred())
			err = desirer.Desire(thorLRP)
			Expect(err).ToNot(HaveOccurred())
		})

		It("should create a StatefulSet object", func() {
			statefulset := getStatefulSet(odinLRP)
			Expect(statefulset.Name).To(ContainSubstring(odinLRP.Name))
			Expect(statefulset.Spec.Template.Spec.Containers[0].Command).To(Equal(odinLRP.Command))
			Expect(statefulset.Spec.Template.Spec.Containers[0].Image).To(Equal(odinLRP.Image))
			Expect(statefulset.Spec.Replicas).To(Equal(int32ptr(odinLRP.TargetInstances)))
		})

		It("should create all associated pods", func() {
			var pods []string
			Eventually(func() []string {
				pods = podNamesFromPods(listPods(odinLRP.LRPIdentifier))
				return pods
			}, timeout).Should(HaveLen(2))
			Expect(pods[0]).To(ContainSubstring("odin"))
			Expect(pods[1]).To(ContainSubstring("odin"))
		})

		It("should be able to list pods by space name", func() {
			labelSelector := fmt.Sprintf("space_name=%s", "space-foo")
			var pods []string
			Eventually(func() []string {
				pods = podNamesFromPods(listPodsByLabel(labelSelector))
				return pods
			}, timeout).Should(HaveLen(4))
			Expect(pods[0]).To(SatisfyAny(ContainSubstring("odin"), ContainSubstring("thor")))
			Expect(pods[1]).To(SatisfyAny(ContainSubstring("odin"), ContainSubstring("thor")))
			Expect(pods[2]).To(SatisfyAny(ContainSubstring("odin"), ContainSubstring("thor")))
			Expect(pods[3]).To(SatisfyAny(ContainSubstring("odin"), ContainSubstring("thor")))
		})

		It("should be able to list pods by application name", func() {
			labelSelector := fmt.Sprintf("application_name=%s", odinLRP.AppName)
			var pods []string
			Eventually(func() []string {
				pods = podNamesFromPods(listPodsByLabel(labelSelector))
				return pods
			}, timeout).Should(HaveLen(2))
			Expect(pods[0]).To(ContainSubstring("odin"))
			Expect(pods[1]).To(ContainSubstring("odin"))

			labelSelector = fmt.Sprintf("application_name=%s", thorLRP.AppName)
			Eventually(func() []string {
				pods = podNamesFromPods(listPodsByLabel(labelSelector))
				return pods
			}, timeout).Should(HaveLen(2))
			Expect(pods[0]).To(ContainSubstring("thor"))
			Expect(pods[1]).To(ContainSubstring("thor"))
		})
	})

	Context("When deleting a LRP", func() {

		JustBeforeEach(func() {
			err = desirer.Desire(odinLRP)
			Expect(err).ToNot(HaveOccurred())
			err = desirer.Stop(odinLRP.LRPIdentifier)
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
				return listPods(odinLRP.LRPIdentifier)
			}, timeout).Should(BeEmpty())
		})
	})

	Context("When getting an app", func() {

		JustBeforeEach(func() {
			err = desirer.Desire(odinLRP)
			Expect(err).ToNot(HaveOccurred())
		})

		It("correctly reports the running instances", func() {
			Eventually(func() int {
				l, e := desirer.Get(odinLRP.LRPIdentifier)
				Expect(e).ToNot(HaveOccurred())
				return l.RunningInstances
			}, timeout).Should(Equal(2))
		})

		Context("When one of the instances if failing", func() {
			var failingLRP *opi.LRP
			BeforeEach(func() {
				failingLRP = createLRP("odin")
				failingLRP.Command = []string{
					"/bin/sh",
					"-c",
					"if [ $INSTANCE_INDEX -eq 1 ]; then exit; else  while true; do echo hello; sleep 10;done; fi;",
				}
			})

			It("correctly reports the running instances", func() {
				Eventually(func() int {
					odinLRP, err := desirer.Get(failingLRP.LRPIdentifier)
					Expect(err).ToNot(HaveOccurred())
					return odinLRP.RunningInstances
				}, timeout).Should(Equal(1))
			})
		})
	})

})

func int32ptr(i int) *int32 {
	i32 := int32(i)
	return &i32
}

func createLRP(name string) *opi.LRP {
	return &opi.LRP{
		Name: name,
		Command: []string{
			"/bin/sh",
			"-c",
			"while true; do echo hello; sleep 10;done",
		},
		AppName:         name,
		SpaceName:       "space-foo",
		TargetInstances: 2,
		Image:           "busybox",
		Metadata: map[string]string{
			cf.ProcessGUID: name,
		},
		LRPIdentifier: opi.LRPIdentifier{GUID: "guid_" + name, Version: "version_" + name},
	}

}
