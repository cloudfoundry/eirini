package integration_test

import (
	"code.cloudfoundry.org/eirini/integration/util"
	"code.cloudfoundry.org/eirini/k8s"
	"code.cloudfoundry.org/eirini/k8s/client"
	"code.cloudfoundry.org/eirini/opi"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	policyv1beta1 "k8s.io/api/policy/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = Describe("Pod", func() {
	var podClient *client.Pod

	BeforeEach(func() {
		podClient = client.NewPod(fixture.Clientset)
	})

	Describe("GetAll", func() {
		var extraNs string

		BeforeEach(func() {
			extraNs = fixture.CreateExtraNamespace()

			createPods(fixture.Namespace, "one", "two", "three")
			createPods(extraNs, "four", "five", "six")
		})

		It("lists all pods across all namespaces", func() {
			Eventually(func() []string {
				pods, err := podClient.GetAll()
				Expect(err).NotTo(HaveOccurred())

				return podNames(pods)
			}).Should(ContainElements("one", "two", "three", "four", "five", "six"))
		})
	})

	Describe("GetByLRPIdentifier", func() {
		var guid, extraNs string

		BeforeEach(func() {
			createPods(fixture.Namespace, "one", "two", "three")

			guid = util.GenerateGUID()

			createPod(fixture.Namespace, "four", map[string]string{
				k8s.LabelGUID:    guid,
				k8s.LabelVersion: "42",
			})
			createPod(fixture.Namespace, "five", map[string]string{
				k8s.LabelGUID:    guid,
				k8s.LabelVersion: "42",
			})

			extraNs = fixture.CreateExtraNamespace()

			createPod(extraNs, "six", map[string]string{
				k8s.LabelGUID:    guid,
				k8s.LabelVersion: "42",
			})
		})

		It("lists all pods matching the specified LRP identifier", func() {
			pods, err := podClient.GetByLRPIdentifier(opi.LRPIdentifier{GUID: guid, Version: "42"})

			Expect(err).NotTo(HaveOccurred())
			Eventually(func() []string { return podNames(pods) }).Should(ConsistOf("four", "five", "six"))
		})
	})

	Describe("Delete", func() {
		BeforeEach(func() {
			createPods(fixture.Namespace, "foo")
		})

		It("deletes a pod", func() {
			Eventually(func() []string { return podNames(listAllPods()) }).Should(ContainElement("foo"))

			err := podClient.Delete(fixture.Namespace, "foo")

			Expect(err).NotTo(HaveOccurred())
			Eventually(func() []string { return podNames(listAllPods()) }).ShouldNot(ContainElement("foo"))
		})

		Context("when it fails", func() {
			It("returns the error", func() {
				err := podClient.Delete(fixture.Namespace, "bar")

				Expect(err).To(MatchError(ContainSubstring(`"bar" not found`)))
			})
		})
	})
})

var _ = Describe("PodDisruptionBudgets", func() {
	var pdbClient *client.PodDisruptionBudget

	BeforeEach(func() {
		pdbClient = client.NewPodDisruptionBudget(fixture.Clientset)
	})

	Describe("Create", func() {
		It("creates a PDB", func() {
			_, err := pdbClient.Create(fixture.Namespace, &policyv1beta1.PodDisruptionBudget{
				ObjectMeta: metav1.ObjectMeta{
					Name: "foo",
				},
			})
			Expect(err).NotTo(HaveOccurred())

			pdbs := listPDBs(fixture.Namespace)

			Expect(pdbs).To(HaveLen(1))
			Expect(pdbs[0].Name).To(Equal("foo"))
		})
	})

	Describe("Delete", func() {
		BeforeEach(func() {
			createPDB(fixture.Namespace, "foo")
		})

		It("deletes a PDB", func() {
			Eventually(func() []policyv1beta1.PodDisruptionBudget { return listPDBs(fixture.Namespace) }).ShouldNot(BeEmpty())

			err := pdbClient.Delete(fixture.Namespace, "foo")

			Expect(err).NotTo(HaveOccurred())
			Eventually(func() []policyv1beta1.PodDisruptionBudget { return listPDBs(fixture.Namespace) }).Should(BeEmpty())
		})
	})
})

var _ = Describe("StatefulSets", func() {
	var statefulSetClient *client.StatefulSet

	BeforeEach(func() {
		statefulSetClient = client.NewStatefulSet(fixture.Clientset)
	})

	Describe("Create", func() {
		It("creates a StatefulSet", func() {
			_, err := statefulSetClient.Create(fixture.Namespace, &appsv1.StatefulSet{
				ObjectMeta: metav1.ObjectMeta{
					Name: "foo",
				},
				Spec: appsv1.StatefulSetSpec{
					Selector: &metav1.LabelSelector{
						MatchLabels: map[string]string{
							"app": "foo",
						},
					},
					Template: corev1.PodTemplateSpec{
						ObjectMeta: metav1.ObjectMeta{
							Labels: map[string]string{
								"app": "foo",
							},
						},
					},
				},
			})
			Expect(err).NotTo(HaveOccurred())

			statefulSets := listStatefulSets(fixture.Namespace)

			Expect(statefulSets).To(HaveLen(1))
			Expect(statefulSets[0].Name).To(Equal("foo"))
		})
	})

	Describe("Get", func() {
		var guid, extraNs string

		BeforeEach(func() {
			guid = util.GenerateGUID()

			createStatefulSet(fixture.Namespace, "foo", map[string]string{
				k8s.LabelGUID: guid,
			})

			extraNs = fixture.CreateExtraNamespace()

			createStatefulSet(extraNs, "foo", nil)
		})

		It("retrieves a StatefulSet by namespace and name", func() {
			statefulSet, err := statefulSetClient.Get(fixture.Namespace, "foo")
			Expect(err).NotTo(HaveOccurred())

			Expect(statefulSet.Name).To(Equal("foo"))
			Expect(statefulSet.Labels[k8s.LabelGUID]).To(Equal(guid))
		})
	})

	Describe("GetBySourceType", func() {
		var extraNs string

		BeforeEach(func() {
			createStatefulSet(fixture.Namespace, "one", map[string]string{
				k8s.LabelSourceType: "FOO",
			})
			createStatefulSet(fixture.Namespace, "two", map[string]string{
				k8s.LabelSourceType: "BAR",
			})

			extraNs = fixture.CreateExtraNamespace()

			createStatefulSet(extraNs, "three", map[string]string{
				k8s.LabelSourceType: "FOO",
			})
		})

		It("lists all StatefulSets with the specified source type", func() {
			Eventually(func() []string {
				statefulSets, err := statefulSetClient.GetBySourceType("FOO")
				Expect(err).NotTo(HaveOccurred())

				return statefulSetNames(statefulSets)
			}).Should(ContainElements("one", "three"))

			Consistently(func() []string {
				statefulSets, err := statefulSetClient.GetBySourceType("FOO")
				Expect(err).NotTo(HaveOccurred())

				return statefulSetNames(statefulSets)
			}).ShouldNot(ContainElements("two"))
		})
	})

	Describe("GetByLRPIdentifier", func() {
		var guid, extraNs string

		BeforeEach(func() {
			guid = util.GenerateGUID()

			createStatefulSet(fixture.Namespace, "one", map[string]string{
				k8s.LabelGUID:    guid,
				k8s.LabelVersion: "42",
			})

			extraNs = fixture.CreateExtraNamespace()

			createStatefulSet(extraNs, "two", map[string]string{
				k8s.LabelGUID:    guid,
				k8s.LabelVersion: "42",
			})
		})

		It("lists all StatefulSets matching the specified LRP identifier", func() {
			statefulSets, err := statefulSetClient.GetByLRPIdentifier(opi.LRPIdentifier{GUID: guid, Version: "42"})

			Expect(err).NotTo(HaveOccurred())
			Eventually(func() []string { return statefulSetNames(statefulSets) }).Should(ConsistOf("one", "two"))
		})
	})

	Describe("Update", func() {
		var statefulSet *appsv1.StatefulSet

		BeforeEach(func() {
			statefulSet = createStatefulSet(fixture.Namespace, "foo", map[string]string{
				"label": "old-value",
			})
		})

		It("updates a StatefulSet", func() {
			statefulSet.Labels["label"] = "new-value"

			newStatefulSet, err := statefulSetClient.Update(fixture.Namespace, statefulSet)
			Expect(err).NotTo(HaveOccurred())
			Expect(newStatefulSet.Labels["label"]).To(Equal("new-value"))

			Eventually(func() string {
				return getStatefulSet(fixture.Namespace, "foo").Labels["label"]
			}).Should(Equal("new-value"))
		})
	})

	Describe("Delete", func() {
		BeforeEach(func() {
			createStatefulSet(fixture.Namespace, "foo", nil)
		})

		It("deletes a StatefulSet", func() {
			Eventually(func() []appsv1.StatefulSet { return listStatefulSets(fixture.Namespace) }).ShouldNot(BeEmpty())

			err := statefulSetClient.Delete(fixture.Namespace, "foo")

			Expect(err).NotTo(HaveOccurred())
			Eventually(func() []appsv1.StatefulSet { return listStatefulSets(fixture.Namespace) }).Should(BeEmpty())
		})
	})
})

func podNames(pods []corev1.Pod) []string {
	names := make([]string, 0, len(pods))
	for _, pod := range pods {
		names = append(names, pod.Name)
	}

	return names
}

func statefulSetNames(statefulSets []appsv1.StatefulSet) []string {
	names := make([]string, 0, len(statefulSets))
	for _, statefulSet := range statefulSets {
		names = append(names, statefulSet.Name)
	}

	return names
}
