package eirini_controller_test

import (
	"context"

	"code.cloudfoundry.org/eirini/k8s/stset"
	eiriniv1 "code.cloudfoundry.org/eirini/pkg/apis/eirini/v1"
	"code.cloudfoundry.org/eirini/tests"
	"code.cloudfoundry.org/eirini/tests/integration"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
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
		JustBeforeEach(func() {
			var err error
			lrp, err = fixture.EiriniClientset.
				EiriniV1().
				LRPs(fixture.Namespace).
				Create(context.Background(), lrp, metav1.CreateOptions{})
			Expect(err).NotTo(HaveOccurred())
		})

		It("deploys the app as a stateful set with correct properties", func() {
			Eventually(func() *appsv1.StatefulSet {
				return integration.GetStatefulSet(fixture.Clientset, fixture.Namespace, lrpGUID, lrpVersion)
			}).ShouldNot(BeNil())

			Eventually(func() bool {
				return getPodReadiness(lrpGUID, lrpVersion)
			}).Should(BeTrue(), "LRP Pod not ready")

			st := integration.GetStatefulSet(fixture.Clientset, fixture.Namespace, lrpGUID, lrpVersion)
			Expect(st.Labels).To(SatisfyAll(
				HaveKeyWithValue(stset.LabelGUID, lrpGUID),
				HaveKeyWithValue(stset.LabelVersion, lrpVersion),
				HaveKeyWithValue(stset.LabelSourceType, "APP"),
				HaveKeyWithValue(stset.LabelAppGUID, "the-app-guid"),
			))
			Expect(st.Spec.Replicas).To(PointTo(Equal(int32(1))))
			Expect(st.Spec.Template.Spec.Containers[0].Image).To(Equal("eirini/dorini"))
			Expect(st.Spec.Template.Spec.Containers[0].Env).To(ContainElement(corev1.EnvVar{Name: "FOO", Value: "BAR"}))
		})

		When("the the app has sidecars", func() {
			assertEqualValues := func(actual, expected *resource.Quantity) {
				Expect(actual.Value()).To(Equal(expected.Value()))
			}

			BeforeEach(func() {
				lrp.Spec.Image = "eirini/busybox"
				lrp.Spec.Command = []string{"/bin/sh", "-c", "echo Hello from app; sleep 3600"}
				lrp.Spec.Sidecars = []eiriniv1.Sidecar{
					{
						Name:     "the-sidecar",
						Command:  []string{"/bin/sh", "-c", "echo Hello from sidecar; sleep 3600"},
						MemoryMB: 101,
					},
				}
			})

			It("deploys the app with the sidcar container", func() {
				Eventually(func() *appsv1.StatefulSet {
					return integration.GetStatefulSet(fixture.Clientset, fixture.Namespace, lrpGUID, lrpVersion)
				}).ShouldNot(BeNil())

				Eventually(func() bool {
					return getPodReadiness(lrpGUID, lrpVersion)
				}).Should(BeTrue(), "LRP Pod not ready")

				st := integration.GetStatefulSet(fixture.Clientset, fixture.Namespace, lrpGUID, lrpVersion)

				Expect(st.Spec.Template.Spec.Containers).To(HaveLen(2))
			})

			It("sets resource limits on the sidecar container", func() {
				Eventually(func() *appsv1.StatefulSet {
					return integration.GetStatefulSet(fixture.Clientset, fixture.Namespace, lrpGUID, lrpVersion)
				}).ShouldNot(BeNil())

				Eventually(func() bool {
					return getPodReadiness(lrpGUID, lrpVersion)
				}).Should(BeTrue(), "LRP Pod not ready")

				st := integration.GetStatefulSet(fixture.Clientset, fixture.Namespace, lrpGUID, lrpVersion)

				containers := st.Spec.Template.Spec.Containers
				for _, container := range containers {
					if container.Name == "the-sidecar" {
						limits := container.Resources.Limits
						requests := container.Resources.Requests

						expectedMemory := resource.NewScaledQuantity(101, resource.Mega)
						expectedDisk := resource.NewScaledQuantity(lrp.Spec.DiskMB, resource.Mega)
						expectedCPU := resource.NewScaledQuantity(int64(lrp.Spec.CPUWeight*10), resource.Milli)

						assertEqualValues(limits.Memory(), expectedMemory)
						assertEqualValues(limits.StorageEphemeral(), expectedDisk)
						assertEqualValues(requests.Memory(), expectedMemory)
						assertEqualValues(requests.Cpu(), expectedCPU)
					}
				}
			})
		})
	})

	Describe("Update an app", func() {
		var clientErr error

		BeforeEach(func() {
			_, err := fixture.EiriniClientset.
				EiriniV1().
				LRPs(fixture.Namespace).
				Create(context.Background(), lrp, metav1.CreateOptions{})
			Expect(err).NotTo(HaveOccurred())

			Eventually(func() int32 {
				lrp = integration.GetLRP(fixture.EiriniClientset, fixture.Namespace, lrpName)

				return lrp.Status.Replicas
			}).Should(Equal(int32(1)))
		})

		JustBeforeEach(func() {
			_, clientErr = fixture.EiriniClientset.
				EiriniV1().
				LRPs(fixture.Namespace).
				Update(context.Background(), lrp, metav1.UpdateOptions{})
		})

		When("routes are updated", func() {
			BeforeEach(func() {
				lrp.Spec.AppRoutes = []eiriniv1.Route{{Hostname: "another-hostname-1", Port: 8080}}
			})

			It("succeeds", func() {
				Expect(clientErr).NotTo(HaveOccurred())
			})

			It("updates the underlying statefulset", func() {
				Eventually(func() string {
					return integration.GetStatefulSet(fixture.Clientset, fixture.Namespace, lrpGUID, lrpVersion).Annotations[stset.AnnotationRegisteredRoutes]
				}).Should(MatchJSON(`[{"hostname": "another-hostname-1", "port": 8080}]`))
			})
		})

		When("the image is updated", func() {
			BeforeEach(func() {
				lrp.Spec.Image = "eirini/custom-port"
			})

			It("updates the underlying statefulset", func() {
				Eventually(func() string {
					return integration.GetStatefulSet(fixture.Clientset, fixture.Namespace, lrpGUID, lrpVersion).Spec.Template.Spec.Containers[0].Image
				}).Should(Equal("eirini/custom-port"))
			})
		})

		When("instance count is updated", func() {
			BeforeEach(func() {
				lrp.Spec.Instances = 3
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
		BeforeEach(func() {
			_, err := fixture.EiriniClientset.
				EiriniV1().
				LRPs(fixture.Namespace).
				Create(context.Background(), lrp, metav1.CreateOptions{})
			Expect(err).NotTo(HaveOccurred())

			Eventually(func() int32 {
				return integration.GetLRP(fixture.EiriniClientset, fixture.Namespace, lrpName).Status.Replicas
			}).Should(Equal(int32(1)))
		})

		JustBeforeEach(func() {
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
