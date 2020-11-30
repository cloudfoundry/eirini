package eats_test

import (
	"context"
	"fmt"

	"code.cloudfoundry.org/eirini/k8s"
	"code.cloudfoundry.org/eirini/pkg/apis/eirini"
	eiriniv1 "code.cloudfoundry.org/eirini/pkg/apis/eirini/v1"
	"code.cloudfoundry.org/eirini/tests"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

var _ = Describe("Apps CRDs [needs-logs-for: eirini-api, eirini-controller]", func() {
	var (
		namespace   string
		lrpName     string
		lrpGUID     string
		lrpVersion  string
		lrp         *eiriniv1.LRP
		appListOpts metav1.ListOptions
	)

	getStatefulSet := func() *appsv1.StatefulSet {
		stsList, err := fixture.Clientset.
			AppsV1().
			StatefulSets(fixture.Namespace).
			List(context.Background(), appListOpts)

		Expect(err).NotTo(HaveOccurred())
		if len(stsList.Items) == 0 {
			return nil
		}
		Expect(stsList.Items).To(HaveLen(1))

		return &stsList.Items[0]
	}

	getLRP := func() *eiriniv1.LRP {
		l, err := fixture.EiriniClientset.
			EiriniV1().
			LRPs(namespace).
			Get(context.Background(), lrpName, metav1.GetOptions{})

		Expect(err).NotTo(HaveOccurred())

		return l
	}

	BeforeEach(func() {
		namespace = fixture.Namespace
		lrpName = tests.GenerateGUID()
		lrpGUID = tests.GenerateGUID()
		lrpVersion = tests.GenerateGUID()
		appListOpts = metav1.ListOptions{
			LabelSelector: fmt.Sprintf("%s=%s,%s=%s", k8s.LabelGUID, lrpGUID, k8s.LabelVersion, lrpVersion),
		}

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

	AfterEach(func() {
		bgDelete := metav1.DeletePropagationBackground
		err := fixture.EiriniClientset.
			EiriniV1().
			LRPs(namespace).
			DeleteCollection(context.Background(),
				metav1.DeleteOptions{PropagationPolicy: &bgDelete},
				metav1.ListOptions{FieldSelector: "metadata.name=" + lrpName},
			)
		Expect(err).NotTo(HaveOccurred())
	})

	Describe("Desiring an app", func() {
		var clientErr error

		JustBeforeEach(func() {
			_, clientErr = fixture.EiriniClientset.
				EiriniV1().
				LRPs(namespace).
				Create(context.Background(), lrp, metav1.CreateOptions{})
		})

		It("succeeds", func() {
			Expect(clientErr).NotTo(HaveOccurred())
		})

		It("deploys the app to the same namespace as the CRD", func() {
			Eventually(getStatefulSet).ShouldNot(BeNil())
			Eventually(func() bool {
				return getPodReadiness(lrpGUID, lrpVersion)
			}).Should(BeTrue(), "LRP Pod not ready")

			st := getStatefulSet()
			Expect(st.Labels).To(SatisfyAll(
				HaveKeyWithValue(k8s.LabelGUID, lrpGUID),
				HaveKeyWithValue(k8s.LabelVersion, lrpVersion),
				HaveKeyWithValue(k8s.LabelSourceType, "APP"),
				HaveKeyWithValue(k8s.LabelAppGUID, "the-app-guid"),
			))
			Expect(st.Spec.Replicas).To(PointTo(Equal(int32(1))))
			Expect(st.Spec.Template.Spec.Containers[0].Image).To(Equal("eirini/dorini"))
			Expect(st.Spec.Template.Spec.Containers[0].Env).To(ContainElement(corev1.EnvVar{Name: "FOO", Value: "BAR"}))
		})

		It("updates the CRD status", func() {
			Eventually(func() int32 {
				return getLRP().Status.Replicas
			}).Should(Equal(int32(1)))
		})

		When("the the app has sidecars", func() {
			BeforeEach(func() {
				lrp.Spec.Image = "eirini/busybox"
				lrp.Spec.Command = []string{"/bin/sh", "-c", "echo Hello from app; sleep 3600"}
				lrp.Spec.Sidecars = []eiriniv1.Sidecar{
					{
						Name:     "the-sidecar",
						Command:  []string{"/bin/sh", "-c", "echo Hello from sidecar; sleep 3600"},
						MemoryMB: 256,
					},
				}
			})

			It("deploys the app with the sidcar container", func() {
				Eventually(getStatefulSet).ShouldNot(BeNil())
				Eventually(func() bool {
					return getPodReadiness(lrpGUID, lrpVersion)
				}).Should(BeTrue(), "LRP Pod not ready")

				st := getStatefulSet()

				Expect(st.Spec.Template.Spec.Containers).To(HaveLen(2))
			})
		})

		When("the disk quota is not specified", func() {
			It("fails", func() {
				obj := &unstructured.Unstructured{
					Object: map[string]interface{}{
						"kind":       "LRP",
						"apiVersion": "eirini.cloudfoundry.org/v1",
						"metadata": map[string]interface{}{
							"name": "the-invalid-lrp",
						},
						"spec": map[string]interface{}{
							"guid":      lrpGUID,
							"version":   lrpVersion,
							"image":     "eirini/dorini",
							"appGUID":   "the-app-guid",
							"appName":   "k-2so",
							"spaceName": "s",
							"orgName":   "o",
							"env":       map[string]string{"FOO": "BAR"},
							"instances": 1,
							"appRoutes": []eiriniv1.Route{{Hostname: "app-hostname-1", Port: 8080}},
						},
					},
				}
				_, err := fixture.DynamicClientset.
					Resource(schema.GroupVersionResource{
						Group:    eirini.GroupName,
						Version:  "v1",
						Resource: "lrps",
					}).
					Namespace(namespace).
					Create(context.Background(), obj, metav1.CreateOptions{})
				Expect(err).To(MatchError(ContainSubstring("diskMB: Required value")))
			})
		})

		When("the disk quota is 0", func() {
			BeforeEach(func() {
				lrp.Spec.DiskMB = 0
			})

			It("fails", func() {
				Expect(clientErr).To(MatchError(ContainSubstring("spec.diskMB in body should be greater than or equal to 1")))
			})
		})
	})

	Describe("Update an app", func() {
		var clientErr error

		BeforeEach(func() {
			_, err := fixture.EiriniClientset.
				EiriniV1().
				LRPs(namespace).
				Create(context.Background(), lrp, metav1.CreateOptions{})
			Expect(err).NotTo(HaveOccurred())

			Eventually(func() int32 {
				lrp = getLRP()

				return lrp.Status.Replicas
			}).Should(Equal(int32(1)))
		})

		JustBeforeEach(func() {
			_, clientErr = fixture.EiriniClientset.
				EiriniV1().
				LRPs(namespace).
				Update(context.Background(), lrp, metav1.UpdateOptions{})
		})

		When("routes are updated", func() {
			BeforeEach(func() {
				lrp.Spec.AppRoutes = []eiriniv1.Route{{Hostname: "app-hostname-1", Port: 8080}}
			})

			It("succeeds", func() {
				Expect(clientErr).NotTo(HaveOccurred())
			})

			It("updates the underlying statefulset", func() {
				Eventually(func() string {
					return getStatefulSet().Annotations[k8s.AnnotationRegisteredRoutes]
				}).Should(MatchJSON(`[{"hostname": "app-hostname-1", "port": 8080}]`))
			})
		})

		When("instance count is updated", func() {
			BeforeEach(func() {
				lrp.Spec.Instances = 3
			})

			It("succeeds", func() {
				Expect(clientErr).NotTo(HaveOccurred())
			})

			It("updates the underlying statefulset", func() {
				Eventually(func() int32 {
					return *getStatefulSet().Spec.Replicas
				}).Should(Equal(int32(3)))

				Eventually(func() int32 {
					return getLRP().Status.Replicas
				}).Should(Equal(int32(3)))
			})
		})

		When("the image is updated", func() {
			BeforeEach(func() {
				lrp.Spec.Image = "new/image"
			})

			It("updates the underlying statefulset", func() {
				Eventually(func() string {
					return getStatefulSet().Spec.Template.Spec.Containers[0].Image
				}).Should(Equal("new/image"))
			})
		})
	})

	Describe("Stop an app", func() {
		BeforeEach(func() {
			_, err := fixture.EiriniClientset.
				EiriniV1().
				LRPs(namespace).
				Create(context.Background(), lrp, metav1.CreateOptions{})
			Expect(err).NotTo(HaveOccurred())

			Eventually(func() int32 {
				return getLRP().Status.Replicas
			}).Should(Equal(int32(1)))
		})

		JustBeforeEach(func() {
			Expect(fixture.EiriniClientset.
				EiriniV1().
				LRPs(namespace).
				Delete(context.Background(), lrpName, metav1.DeleteOptions{}),
			).To(Succeed())
		})

		It("deletes the underlying statefulset", func() {
			Eventually(getStatefulSet).Should(BeNil())
		})
	})

	Describe("App status", func() {
		When("an app instance becomes unready", func() {
			BeforeEach(func() {
				_, err := fixture.EiriniClientset.
					EiriniV1().
					LRPs(namespace).
					Create(context.Background(), lrp, metav1.CreateOptions{})
				Expect(err).NotTo(HaveOccurred())

				Eventually(func() int32 {
					return getLRP().Status.Replicas
				}).Should(Equal(int32(1)))
			})

			JustBeforeEach(func() {
				Expect(fixture.Clientset.
					CoreV1().
					Pods(fixture.Namespace).
					DeleteCollection(context.Background(), metav1.DeleteOptions{}, appListOpts),
				).To(Succeed())
			})

			It("is reflected in the LRP status", func() {
				Eventually(func() int32 {
					return getLRP().Status.Replicas
				}).Should(Equal(int32(0)))

				Eventually(func() int32 {
					return getLRP().Status.Replicas
				}).Should(Equal(int32(1)))
			})
		})
	})
})
