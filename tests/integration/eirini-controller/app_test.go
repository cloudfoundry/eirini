package eirini_controller_test

import (
	"context"

	"code.cloudfoundry.org/eirini/k8s/stset"
	eiriniv1 "code.cloudfoundry.org/eirini/pkg/apis/eirini/v1"
	"code.cloudfoundry.org/eirini/tests"
	"code.cloudfoundry.org/eirini/tests/integration"
	"github.com/jinzhu/copier"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = Describe("App", func() {
	var (
		lrpName    string
		lrpGUID    string
		lrpVersion string
		lrp        *eiriniv1.LRP
	)

	BeforeEach(func() {
		lrpName = tests.GenerateGUID()
		lrpGUID = tests.GenerateGUID()
		lrpVersion = tests.GenerateGUID()

		lrp = &eiriniv1.LRP{
			ObjectMeta: metav1.ObjectMeta{
				Name: lrpName,
			},
			Spec: eiriniv1.LRPSpec{
				GUID:                   lrpGUID,
				Version:                lrpVersion,
				Image:                  "eirini/dorini",
				AppGUID:                "the-app-guid",
				AppName:                "k-2so",
				SpaceName:              "s",
				OrgName:                "o",
				Env:                    map[string]string{"FOO": "BAR"},
				MemoryMB:               256,
				DiskMB:                 256,
				CPUWeight:              10,
				Instances:              1,
				LastUpdated:            "a long time ago in a galaxy far, far away",
				Ports:                  []int32{8080},
				VolumeMounts:           []eiriniv1.VolumeMount{},
				UserDefinedAnnotations: map[string]string{},
				AppRoutes:              []eiriniv1.Route{{Hostname: "app-hostname-1", Port: 8080}},
			},
		}
	})

	Describe("desiring an app", func() {
		var st *appsv1.StatefulSet

		JustBeforeEach(func() {
			var err error
			lrp, err = fixture.EiriniClientset.
				EiriniV1().
				LRPs(fixture.Namespace).
				Create(context.Background(), lrp, metav1.CreateOptions{})
			Expect(err).NotTo(HaveOccurred())

			Eventually(func() *appsv1.StatefulSet {
				return integration.GetStatefulSet(fixture.Clientset, fixture.Namespace, lrpGUID, lrpVersion)
			}).ShouldNot(BeNil())

			Eventually(func() bool {
				return getPodReadiness(lrpGUID, lrpVersion)
			}).Should(BeTrue(), "LRP Pod not ready")

			st = integration.GetStatefulSet(fixture.Clientset, fixture.Namespace, lrpGUID, lrpVersion)
		})

		It("sets the runAsNonRoot in the PodSecurityContext", func() {
			Expect(st.Spec.Template.Spec.SecurityContext.RunAsNonRoot).To(PointTo(BeTrue()))
		})

		When("AllowRunImageAsRoot is true", func() {
			BeforeEach(func() {
				config.AllowRunImageAsRoot = true
			})

			It("doesn't set `runAsNonRoot` in the PodSecurityContext", func() {
				Expect(st.Spec.Template.Spec.SecurityContext.RunAsNonRoot).To(BeNil())
			})
		})
	})

	Describe("Update an app", func() {
		var updatedLRP *eiriniv1.LRP

		BeforeEach(func() {
			updatedLRP = &eiriniv1.LRP{}
			Expect(copier.Copy(updatedLRP, lrp)).To(Succeed())
		})

		JustBeforeEach(func() {
			_, err := fixture.EiriniClientset.
				EiriniV1().
				LRPs(fixture.Namespace).
				Create(context.Background(), lrp, metav1.CreateOptions{})
			Expect(err).NotTo(HaveOccurred())

			Eventually(func() int32 {
				lrp = integration.GetLRP(fixture.EiriniClientset, fixture.Namespace, lrpName)

				return lrp.Status.Replicas
			}).Should(Equal(int32(1)))
			updatedLRP.ResourceVersion = integration.GetLRP(fixture.EiriniClientset, fixture.Namespace, lrpName).ResourceVersion

			_, err = fixture.EiriniClientset.
				EiriniV1().
				LRPs(fixture.Namespace).
				Update(context.Background(), updatedLRP, metav1.UpdateOptions{})
			Expect(err).NotTo(HaveOccurred())
		})

		When("routes are updated", func() {
			BeforeEach(func() {
				updatedLRP.Spec.AppRoutes = []eiriniv1.Route{{Hostname: "another-hostname-1", Port: 8080}}
			})

			It("updates the underlying statefulset", func() {
				Eventually(func() string {
					return integration.GetStatefulSet(fixture.Clientset, fixture.Namespace, lrpGUID, lrpVersion).Annotations[stset.AnnotationRegisteredRoutes]
				}).Should(MatchJSON(`[{"hostname": "another-hostname-1", "port": 8080}]`))
			})
		})

		When("the image is updated", func() {
			BeforeEach(func() {
				updatedLRP.Spec.Image = "eirini/custom-port"
			})

			It("updates the underlying statefulset", func() {
				Eventually(func() string {
					return integration.GetStatefulSet(fixture.Clientset, fixture.Namespace, lrpGUID, lrpVersion).Spec.Template.Spec.Containers[0].Image
				}).Should(Equal("eirini/custom-port"))
			})
		})

		When("instance count is updated", func() {
			BeforeEach(func() {
				updatedLRP.Spec.Instances = 3
			})

			It("updates the underlying statefulset", func() {
				Eventually(func() int32 {
					return *integration.GetStatefulSet(fixture.Clientset, fixture.Namespace, lrpGUID, lrpVersion).Spec.Replicas
				}).Should(Equal(int32(3)))

				Eventually(func() int32 {
					return integration.GetLRP(fixture.EiriniClientset, fixture.Namespace, lrpName).Status.Replicas
				}).Should(Equal(int32(3)))
			})
		})
	})

	Describe("Stop an app", func() {
		JustBeforeEach(func() {
			_, err := fixture.EiriniClientset.
				EiriniV1().
				LRPs(fixture.Namespace).
				Create(context.Background(), lrp, metav1.CreateOptions{})
			Expect(err).NotTo(HaveOccurred())

			Eventually(func() int32 {
				return integration.GetLRP(fixture.EiriniClientset, fixture.Namespace, lrpName).Status.Replicas
			}).Should(Equal(int32(1)))

			Expect(fixture.EiriniClientset.
				EiriniV1().
				LRPs(fixture.Namespace).
				Delete(context.Background(), lrpName, metav1.DeleteOptions{}),
			).To(Succeed())
		})

		It("deletes the underlying statefulset", func() {
			Eventually(func() *appsv1.StatefulSet {
				return integration.GetStatefulSet(fixture.Clientset, fixture.Namespace, lrpGUID, lrpVersion)
			}).Should(BeNil())
		})
	})
})
