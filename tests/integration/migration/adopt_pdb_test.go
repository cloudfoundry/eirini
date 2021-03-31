package migration_test

import (
	"context"
	"strconv"

	"code.cloudfoundry.org/eirini/k8s/stset"
	"code.cloudfoundry.org/eirini/migrations"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	policyv1beta1 "k8s.io/api/policy/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

var _ = Describe("Adopt PDB Migration", func() {
	BeforeEach(func() {
		two := int32(2)
		stSet := &appsv1.StatefulSet{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "my-stset",
				Namespace: fixture.Namespace,
				Labels: map[string]string{
					stset.LabelSourceType: stset.AppSourceType,
					stset.LabelGUID:       "stset-guid",
					stset.LabelVersion:    "stset-version",
				},
				Annotations: map[string]string{
					stset.AnnotationLatestMigration: strconv.Itoa(migrations.AdjustCPUResourceSequenceID),
				},
			},
			Spec: appsv1.StatefulSetSpec{
				Selector: &metav1.LabelSelector{
					MatchLabels: map[string]string{
						"app": "asdf",
					},
				},
				Replicas: &two,
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
						}},
					},
				},
			},
		}

		half := intstr.FromString("50%")
		pdb := &policyv1beta1.PodDisruptionBudget{
			ObjectMeta: metav1.ObjectMeta{
				Name: "my-stset",
			},
			Spec: policyv1beta1.PodDisruptionBudgetSpec{
				Selector: &metav1.LabelSelector{
					MatchLabels: map[string]string{
						stset.LabelGUID:       "stset-guid",
						stset.LabelVersion:    "stset-version",
						stset.LabelSourceType: stset.AppSourceType,
					},
				},

				MaxUnavailable: &half,
			},
		}

		_, err := fixture.Clientset.AppsV1().StatefulSets(fixture.Namespace).Create(context.Background(), stSet, metav1.CreateOptions{})
		Expect(err).NotTo(HaveOccurred())
		_, err = fixture.Clientset.PolicyV1beta1().PodDisruptionBudgets(fixture.Namespace).Create(context.Background(), pdb, metav1.CreateOptions{})
		Expect(err).NotTo(HaveOccurred())
	})

	It("sets the owner reference on the pdb", func() {
		stSet, err := fixture.Clientset.AppsV1().StatefulSets(fixture.Namespace).Get(context.Background(), "my-stset", metav1.GetOptions{})
		Expect(err).NotTo(HaveOccurred())
		pdb, err := fixture.Clientset.PolicyV1beta1().PodDisruptionBudgets(fixture.Namespace).Get(context.Background(), "my-stset", metav1.GetOptions{})
		Expect(err).NotTo(HaveOccurred())
		Expect(pdb.OwnerReferences).To(HaveLen(1))
		Expect(pdb.OwnerReferences[0].UID).To(Equal(stSet.UID))
		Expect(pdb.OwnerReferences[0].Name).To(Equal(stSet.Name))
	})

	It("bumps the latest migration annotation", func() {
		stSet, err := fixture.Clientset.AppsV1().StatefulSets(fixture.Namespace).Get(context.Background(), "my-stset", metav1.GetOptions{})
		Expect(err).NotTo(HaveOccurred())

		version, err := strconv.Atoi(stSet.Annotations[stset.AnnotationLatestMigration])
		Expect(err).NotTo(HaveOccurred())
		Expect(version).To(BeNumerically(">=", migrations.AdoptPDBSequenceID))
	})
})
