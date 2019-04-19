package statefulsets_test

import (
	"code.cloudfoundry.org/eirini/k8s"
	"code.cloudfoundry.org/eirini/models/cf"
	"code.cloudfoundry.org/eirini/opi"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
)

var _ = Describe("StatefulSet Manager", func() {

	var (
		desirer opi.Desirer
		odinLRP *opi.LRP
		thorLRP *opi.LRP
	)

	BeforeEach(func() {
		odinLRP = createLRP("ödin", "guid-odin")
		thorLRP = createLRP("thor", "guid-thor")
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
			err := desirer.Desire(odinLRP)
			Expect(err).ToNot(HaveOccurred())
			err = desirer.Desire(thorLRP)
			Expect(err).ToNot(HaveOccurred())
		})

		It("should create a StatefulSet object", func() {
			statefulset := getStatefulSet(odinLRP)
			Expect(statefulset.Name).To(ContainSubstring(odinLRP.GUID))
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

		Context("when we create the same StatefulSet again", func() {
			It("should error", func() {
				err := desirer.Desire(odinLRP)
				Expect(err).To(HaveOccurred())
			})
		})
	})

	Context("When deleting a LRP", func() {

		JustBeforeEach(func() {
			err := desirer.Desire(odinLRP)
			Expect(err).ToNot(HaveOccurred())
			err = desirer.Stop(odinLRP.LRPIdentifier)
			Expect(err).ToNot(HaveOccurred())
		})

		It("should delete the StatefulSet object", func() {
			Eventually(func() []appsv1.StatefulSet {
				return listStatefulSets("odin")
			}, timeout).Should(BeEmpty())
		})

		It("should delete the associated pods", func() {
			Eventually(func() []corev1.Pod {
				return listPods(odinLRP.LRPIdentifier)
			}, timeout).Should(BeEmpty())
		})
	})

	Context("When getting an app", func() {
		numberOfInstancesFn := func() int {
			lrp, err := desirer.Get(odinLRP.LRPIdentifier)
			Expect(err).ToNot(HaveOccurred())
			return lrp.RunningInstances
		}

		JustBeforeEach(func() {
			err := desirer.Desire(odinLRP)
			Expect(err).ToNot(HaveOccurred())
		})

		It("correctly reports the running instances", func() {
			Eventually(numberOfInstancesFn, timeout).Should(Equal(2))
			Consistently(numberOfInstancesFn, "10s").Should(Equal(2))
		})

		Context("When one of the instances if failing", func() {
			BeforeEach(func() {
				odinLRP = createLRP("odin", "guid-odin")
				odinLRP.Health = opi.Healtcheck{
					Type: "port",
					Port: 3000,
				}
				odinLRP.Command = []string{
					"/bin/sh",
					"-c",
					`if [ $(echo $HOSTNAME | sed 's|.*-\(.*\)|\1|') -eq 1 ]; then exit; else  while true; do echo hello; nc -l -p 3000 ;done; fi;`,
				}
			})

			It("correctly reports the running instances", func() {
				Eventually(numberOfInstancesFn, timeout).Should(Equal(1))
				Consistently(numberOfInstancesFn, "10s").Should(Equal(1))
			})
		})
	})

})

func int32ptr(i int) *int32 {
	i32 := int32(i)
	return &i32
}

func createLRP(name string, guid string) *opi.LRP {
	return &opi.LRP{
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
		LRPIdentifier: opi.LRPIdentifier{GUID: guid, Version: "version_" + guid},
	}
}
