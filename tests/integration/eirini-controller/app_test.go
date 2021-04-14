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
	corev1 "k8s.io/api/core/v1"
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
		var createErr error

		getRunningStatefulset := func() *appsv1.StatefulSet {
			Eventually(func() *appsv1.StatefulSet {
				return integration.GetStatefulSet(fixture.Clientset, fixture.Namespace, lrpGUID, lrpVersion)
			}).ShouldNot(BeNil())

			Eventually(func() bool {
				return getPodReadiness(lrpGUID, lrpVersion)
			}).Should(BeTrue(), "LRP Pod not ready")

			return integration.GetStatefulSet(fixture.Clientset, fixture.Namespace, lrpGUID, lrpVersion)
		}

		JustBeforeEach(func() {
			lrp, createErr = fixture.EiriniClientset.
				EiriniV1().
				LRPs(fixture.Namespace).
				Create(context.Background(), lrp, metav1.CreateOptions{})
		})

		It("creates a statefulset for the app", func() {
			Expect(createErr).NotTo(HaveOccurred())
			st := getRunningStatefulset()
			Expect(st.Labels[stset.LabelGUID]).To(Equal(lrp.Spec.GUID))
		})

		It("sets the runAsNonRoot in the PodSecurityContext", func() {
			st := getRunningStatefulset()
			Expect(st.Spec.Template.Spec.SecurityContext.RunAsNonRoot).To(PointTo(BeTrue()))
		})

		When("DiskMB is not set", func() {
			BeforeEach(func() {
				lrp.Spec.DiskMB = 0
			})

			It("errors", func() {
				Expect(createErr).To(MatchError(ContainSubstring("Invalid value")))
			})
		})

		When("AllowRunImageAsRoot is true", func() {
			BeforeEach(func() {
				config.AllowRunImageAsRoot = true
			})

			It("doesn't set `runAsNonRoot` in the PodSecurityContext", func() {
				st := getRunningStatefulset()
				Expect(st.Spec.Template.Spec.SecurityContext.RunAsNonRoot).To(BeNil())
			})
		})

		Describe("automounting serviceacccount token", func() {
			const serviceAccountTokenMountPath = "/var/run/secrets/kubernetes.io/serviceaccount"
			var podMountPaths []string

			JustBeforeEach(func() {
				Eventually(func() ([]corev1.Pod, error) {
					pods, err := fixture.Clientset.CoreV1().Pods(fixture.Namespace).List(context.Background(), metav1.ListOptions{})
					if err != nil {
						return nil, err
					}

					return pods.Items, nil
				}).ShouldNot(BeEmpty())

				pods, err := fixture.Clientset.CoreV1().Pods(fixture.Namespace).List(context.Background(), metav1.ListOptions{})
				Expect(err).NotTo(HaveOccurred())
				Expect(pods.Items).To(HaveLen(1))

				podMountPaths = []string{}
				for _, podMount := range pods.Items[0].Spec.Containers[0].VolumeMounts {
					podMountPaths = append(podMountPaths, podMount.MountPath)
				}
			})

			It("does not mount the service account token", func() {
				Expect(podMountPaths).NotTo(ContainElement(serviceAccountTokenMountPath))
			})

			When("unsafe_allow_automount_service_account_token is set", func() {
				BeforeEach(func() {
					config.UnsafeAllowAutomountServiceAccountToken = true
				})

				It("mounts the service account token (because this is how K8S works by default)", func() {
					Expect(podMountPaths).To(ContainElement(serviceAccountTokenMountPath))
				})

				When("the app service account has its automountServiceAccountToken set to false", func() {
					updateServiceaccount := func() error {
						appServiceAccount, err := fixture.Clientset.CoreV1().ServiceAccounts(fixture.Namespace).Get(context.Background(), tests.GetApplicationServiceAccount(), metav1.GetOptions{})
						Expect(err).NotTo(HaveOccurred())
						automountServiceAccountToken := false
						appServiceAccount.AutomountServiceAccountToken = &automountServiceAccountToken
						_, err = fixture.Clientset.CoreV1().ServiceAccounts(fixture.Namespace).Update(context.Background(), appServiceAccount, metav1.UpdateOptions{})

						return err
					}

					BeforeEach(func() {
						Eventually(updateServiceaccount, "5s").Should(Succeed())
					})

					It("does not mount the service account token", func() {
						Expect(podMountPaths).NotTo(ContainElement(serviceAccountTokenMountPath))
					})
				})
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
