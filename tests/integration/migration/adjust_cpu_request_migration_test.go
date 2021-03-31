package migration_test

import (
	"context"
	"fmt"
	"strconv"

	"code.cloudfoundry.org/eirini/k8s/stset"
	"code.cloudfoundry.org/eirini/migrations"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const originalRequestAnnotationValue = 12

var _ = Describe("Adjust CPU request migration", func() {
	BeforeEach(func() {
		stSet := &appsv1.StatefulSet{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "my-stset",
				Namespace: fixture.Namespace,
				Labels: map[string]string{
					stset.LabelSourceType: "APP",
				},
				Annotations: map[string]string{
					stset.AnnotationOriginalRequest: fmt.Sprintf(`{"cpu_weight":%d}`, originalRequestAnnotationValue),
				},
			},
			Spec: appsv1.StatefulSetSpec{
				Selector: &metav1.LabelSelector{
					MatchLabels: map[string]string{
						"app": "asdf",
					},
				},
				Template: corev1.PodTemplateSpec{
					ObjectMeta: metav1.ObjectMeta{
						Labels: map[string]string{
							"app": "asdf",
						},
					},
					Spec: corev1.PodSpec{
						Containers: []corev1.Container{{
							Name:  "opi",
							Image: "eirini/dorini",
							Resources: corev1.ResourceRequirements{
								Requests: corev1.ResourceList{
									corev1.ResourceCPU: resource.MustParse("120m"),
								},
							},
						}},
					},
				},
			},
		}

		_, err := fixture.Clientset.AppsV1().StatefulSets(fixture.Namespace).Create(context.Background(), stSet, metav1.CreateOptions{})
		Expect(err).NotTo(HaveOccurred())
	})

	It("bumps the latest migration annotation", func() {
		stSet, err := fixture.Clientset.AppsV1().StatefulSets(fixture.Namespace).Get(context.Background(), "my-stset", metav1.GetOptions{})
		Expect(err).NotTo(HaveOccurred())

		version, err := strconv.Atoi(stSet.Annotations[stset.AnnotationLatestMigration])
		Expect(err).NotTo(HaveOccurred())
		Expect(version).To(BeNumerically(">=", migrations.AdjustCPUResourceSequenceID))
	})

	It("sets the cpu resource limit of the opi container to the value of the original request annotation", func() {
		stSet, err := fixture.Clientset.AppsV1().StatefulSets(fixture.Namespace).Get(context.Background(), "my-stset", metav1.GetOptions{})
		Expect(err).NotTo(HaveOccurred())
		cpuRequest := stSet.Spec.Template.Spec.Containers[0].Resources.Requests.Cpu()

		Expect(cpuRequest.MilliValue()).To(Equal(int64(originalRequestAnnotationValue)))
	})
})
