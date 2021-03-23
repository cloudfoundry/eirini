package integration_test

import (
	"code.cloudfoundry.org/eirini/k8s/client"
	"code.cloudfoundry.org/eirini/k8s/stset"
	"code.cloudfoundry.org/eirini/opi"
	"code.cloudfoundry.org/eirini/tests"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = Describe("StatefulSets", func() {
	var statefulSetClient *client.StatefulSet

	BeforeEach(func() {
		statefulSetClient = client.NewStatefulSet(fixture.Clientset, "")
	})

	Describe("Create", func() {
		It("creates a StatefulSet", func() {
			_, err := statefulSetClient.Create(ctx, fixture.Namespace, &appsv1.StatefulSet{
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
			guid = tests.GenerateGUID()

			createStatefulSet(fixture.Namespace, "foo", map[string]string{
				stset.LabelGUID: guid,
			})

			extraNs = fixture.CreateExtraNamespace()

			createStatefulSet(extraNs, "foo", nil)
		})

		It("retrieves a StatefulSet by namespace and name", func() {
			statefulSet, err := statefulSetClient.Get(ctx, fixture.Namespace, "foo")
			Expect(err).NotTo(HaveOccurred())

			Expect(statefulSet.Name).To(Equal("foo"))
			Expect(statefulSet.Labels[stset.LabelGUID]).To(Equal(guid))
		})
	})

	Describe("GetBySourceType", func() {
		var extraNs string

		BeforeEach(func() {
			createStatefulSet(fixture.Namespace, "one", map[string]string{
				stset.LabelSourceType: "FOO",
			})
			createStatefulSet(fixture.Namespace, "two", map[string]string{
				stset.LabelSourceType: "BAR",
			})

			extraNs = fixture.CreateExtraNamespace()

			createStatefulSet(extraNs, "three", map[string]string{
				stset.LabelSourceType: "FOO",
			})
		})

		It("lists all StatefulSets with the specified source type", func() {
			Eventually(func() []string {
				statefulSets, err := statefulSetClient.GetBySourceType(ctx, "FOO")
				Expect(err).NotTo(HaveOccurred())

				return statefulSetNames(statefulSets)
			}).Should(ContainElements("one", "three"))

			Consistently(func() []string {
				statefulSets, err := statefulSetClient.GetBySourceType(ctx, "FOO")
				Expect(err).NotTo(HaveOccurred())

				return statefulSetNames(statefulSets)
			}).ShouldNot(ContainElements("two"))
		})
	})

	Describe("GetByLRPIdentifier", func() {
		var guid, extraNs string

		BeforeEach(func() {
			guid = tests.GenerateGUID()

			createStatefulSet(fixture.Namespace, "one", map[string]string{
				stset.LabelGUID:    guid,
				stset.LabelVersion: "42",
			})

			extraNs = fixture.CreateExtraNamespace()

			createStatefulSet(extraNs, "two", map[string]string{
				stset.LabelGUID:    guid,
				stset.LabelVersion: "42",
			})
		})

		It("lists all StatefulSets matching the specified LRP identifier", func() {
			statefulSets, err := statefulSetClient.GetByLRPIdentifier(ctx, opi.LRPIdentifier{GUID: guid, Version: "42"})

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

			newStatefulSet, err := statefulSetClient.Update(ctx, fixture.Namespace, statefulSet)
			Expect(err).NotTo(HaveOccurred())
			Expect(newStatefulSet.Labels["label"]).To(Equal("new-value"))

			Eventually(func() string {
				return getStatefulSet(fixture.Namespace, "foo").Labels["label"]
			}).Should(Equal("new-value"))
		})
	})

	Describe("SetCPURequest", func() {
		var (
			statefulSet    *appsv1.StatefulSet
			containers     []corev1.Container
			cpuRequest     resource.Quantity
			newStatefulSet *appsv1.StatefulSet
		)

		BeforeEach(func() {
			cpuRequest = resource.MustParse("321m")

			containers = []corev1.Container{
				{
					Name:    "not-opi",
					Image:   "eirini/busybox",
					Command: []string{"echo", "hi"},
					Resources: corev1.ResourceRequirements{
						Requests: corev1.ResourceList{
							corev1.ResourceCPU: resource.MustParse("120m"),
						},
					},
				},
				{
					Name:    stset.OPIContainerName,
					Image:   "eirini/busybox",
					Command: []string{"echo", "hi"},
					Resources: corev1.ResourceRequirements{
						Requests: corev1.ResourceList{
							corev1.ResourceCPU: resource.MustParse("120m"),
						},
					},
				},
			}
		})

		JustBeforeEach(func() {
			statefulSet = createStatefulSetWithContainers(fixture.Namespace, "foo", containers)

			var err error
			newStatefulSet, err = statefulSetClient.SetCPURequest(ctx, statefulSet, &cpuRequest)
			Expect(err).NotTo(HaveOccurred())
		})

		getCPURequests := func(stSet *appsv1.StatefulSet) []int64 {
			millis := []int64{}

			for _, c := range stSet.Spec.Template.Spec.Containers {
				q := c.Resources.Requests[corev1.ResourceCPU]
				millis = append(millis, (&q).MilliValue())
			}

			return millis
		}

		It("patches CPU request onto an OPI container only on a StatefulSet", func() {
			Expect(getCPURequests(newStatefulSet)).To(Equal([]int64{120, 321}))

			Eventually(func() []int64 {
				stSet := getStatefulSet(fixture.Namespace, "foo")

				return getCPURequests(stSet)
			}).Should(Equal([]int64{120, 321}))
		})

		When("the stateful set doesn't have an opi container", func() {
			BeforeEach(func() {
				containers = []corev1.Container{
					{
						Name:    "not-opi",
						Image:   "eirini/busybox",
						Command: []string{"echo", "hi"},
						Resources: corev1.ResourceRequirements{
							Requests: corev1.ResourceList{
								corev1.ResourceCPU: resource.MustParse("120m"),
							},
						},
					},
				}
			})

			It("does not modify cpu requests", func() {
				Expect(getCPURequests(newStatefulSet)).To(Equal([]int64{120}))
			})
		})
	})

	Describe("Delete", func() {
		BeforeEach(func() {
			createStatefulSet(fixture.Namespace, "foo", nil)
		})

		It("deletes a StatefulSet", func() {
			Eventually(func() []appsv1.StatefulSet { return listStatefulSets(fixture.Namespace) }).ShouldNot(BeEmpty())

			err := statefulSetClient.Delete(ctx, fixture.Namespace, "foo")

			Expect(err).NotTo(HaveOccurred())
			Eventually(func() []appsv1.StatefulSet { return listStatefulSets(fixture.Namespace) }).Should(BeEmpty())
		})
	})
})
