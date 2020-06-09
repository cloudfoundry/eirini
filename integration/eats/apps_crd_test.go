package eats_test

import (
	"encoding/json"
	"os"

	"code.cloudfoundry.org/eirini"
	"code.cloudfoundry.org/eirini/integration/util"
	"code.cloudfoundry.org/eirini/k8s"
	eiriniv1 "code.cloudfoundry.org/eirini/pkg/apis/lrp/v1"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"
	. "github.com/onsi/gomega/gstruct"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = Describe("Apps CRDs", func() {

	const lrpName = "lrp-name-irrelevant"

	var (
		namespace      string
		lrpGUID        string
		lrpVersion     string
		lrpProcessGUID string

		controllerConfigFile string
		controllerSession    *gexec.Session
	)

	listStatefulSets := func() []appsv1.StatefulSet {
		stsList, err := fixture.Clientset.AppsV1().StatefulSets(fixture.Namespace).List(metav1.ListOptions{})
		Expect(err).NotTo(HaveOccurred())
		return stsList.Items
	}

	getLRP := func(name string) *eiriniv1.LRP {
		lrp, err := fixture.LRPClientset.EiriniV1().LRPs(namespace).Get(name, metav1.GetOptions{})
		Expect(err).NotTo(HaveOccurred())
		return lrp
	}

	BeforeEach(func() {
		namespace = fixture.Namespace
		lrpGUID = generateGUID("lrp")
		lrpVersion = generateGUID("version")
		lrpProcessGUID = processGUID(lrpGUID, lrpVersion)

		lrp := &eiriniv1.LRP{
			ObjectMeta: metav1.ObjectMeta{
				Name: lrpName,
			},
			Spec: eiriniv1.LRPSpec{
				GUID:             lrpGUID,
				Version:          lrpVersion,
				ProcessGUID:      lrpProcessGUID,
				AppGUID:          "the-app-guid",
				AppName:          "k-2so",
				SpaceName:        "s",
				OrganizationName: "o",
				Environment:      map[string]string{"FOO": "BAR"},
				NumInstances:     2,
				LastUpdated:      "a long time ago in a galaxy far, far away",
				Ports:            []int32{8080},
				Lifecycle: eiriniv1.Lifecycle{
					DockerLifecycle: &eiriniv1.DockerLifecycle{
						Image: "eirini/dorini",
					},
				},
			},
		}

		config := &eirini.LrpControllerConfig{
			KubeConfig: eirini.KubeConfig{
				Namespace:  fixture.DefaultNamespace,
				ConfigPath: fixture.KubeConfigPath,
			},
			EiriniURI: opiURL,

			EiriniCertPath: localhostCertPath,
			EiriniKeyPath:  localhostKeyPath,
			CAPath:         localhostCertPath,
		}
		controllerSession, controllerConfigFile = util.RunBinary(binPaths.LRPController, config)

		_, err := fixture.LRPClientset.EiriniV1().LRPs(namespace).Create(lrp)
		Expect(err).NotTo(HaveOccurred())
	})

	AfterEach(func() {
		if controllerSession != nil {
			controllerSession.Kill()
		}
		Expect(os.Remove(controllerConfigFile)).To(Succeed())
	})

	Describe("Desiring an app", func() {
		It("deploys the app to the same namespace as the CRD", func() {
			Eventually(listStatefulSets).Should(HaveLen(1))

			statefulsets := listStatefulSets()
			st := statefulsets[0]

			Expect(st.Labels).To(SatisfyAll(
				HaveKeyWithValue(k8s.LabelGUID, lrpGUID),
				HaveKeyWithValue(k8s.LabelVersion, lrpVersion),
				HaveKeyWithValue(k8s.LabelSourceType, "APP"),
				HaveKeyWithValue(k8s.LabelAppGUID, "the-app-guid"),
			))
			Expect(st.Spec.Replicas).To(PointTo(Equal(int32(2))))
			Expect(st.Spec.Template.Spec.Containers[0].Image).To(Equal("eirini/dorini"))
			Expect(st.Spec.Template.Spec.Containers[0].Env).To(ContainElement(corev1.EnvVar{Name: "FOO", Value: "BAR"}))
		})
	})

	Describe("Update an app", func() {
		When("routes are updated", func() {
			BeforeEach(func() {
				Eventually(listStatefulSets).Should(HaveLen(1))

				lrp := getLRP(lrpName)
				lrp.Spec.Routes = map[string]json.RawMessage{
					"cf-router": marshalRoutes([]routeInfo{
						{Hostname: "app-hostname-1", Port: 8080},
					}),
				}
				lrp.Spec.LastUpdated = "now"

				_, err := fixture.LRPClientset.EiriniV1().LRPs(namespace).Update(lrp)
				Expect(err).NotTo(HaveOccurred())
			})

			It("updates the underlying statefulset", func() {
				Eventually(func() string {
					statefulsets := listStatefulSets()
					st := statefulsets[0]
					return st.Annotations[k8s.AnnotationRegisteredRoutes]
				}).Should(MatchJSON(`[{"hostname": "app-hostname-1", "port": 8080}]`))
			})

		})

		When("instance count is updated", func() {
			BeforeEach(func() {
				Eventually(listStatefulSets).Should(HaveLen(1))

				lrp := getLRP(lrpName)
				lrp.Spec.NumInstances = 3
				lrp.Spec.LastUpdated = "now"

				_, err := fixture.LRPClientset.EiriniV1().LRPs(namespace).Update(lrp)
				Expect(err).NotTo(HaveOccurred())
			})

			It("updates the underlying statefulset", func() {
				Eventually(func() int32 {
					statefulsets := listStatefulSets()
					st := statefulsets[0]
					return *st.Spec.Replicas
				}).Should(Equal(int32(3)))
			})
		})

		When("lastUpdated timestamp is not updated", func() {
			BeforeEach(func() {
				Eventually(listStatefulSets).Should(HaveLen(1))

				lrp := getLRP(lrpName)
				lrp.Spec.NumInstances = 3

				_, err := fixture.LRPClientset.EiriniV1().LRPs(namespace).Update(lrp)
				Expect(err).NotTo(HaveOccurred())
			})

			It("does not update the underlying statefulset", func() {
				Consistently(func() int32 {
					statefulsets := listStatefulSets()
					st := statefulsets[0]
					return *st.Spec.Replicas
				}).Should(Equal(int32(2)))
			})
		})
	})

	Describe("Stop an app", func() {
		BeforeEach(func() {
			Eventually(listStatefulSets).Should(HaveLen(1))

			Expect(fixture.LRPClientset.EiriniV1().LRPs(namespace).Delete(lrpName, &metav1.DeleteOptions{})).To(Succeed())
		})

		It("deletes the undurlying statefulset", func() {
			Eventually(listStatefulSets).Should(BeEmpty())
		})
	})
})
