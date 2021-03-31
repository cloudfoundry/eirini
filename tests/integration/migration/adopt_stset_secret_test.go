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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = Describe("Adopt StatefulSet Secret Migration", func() {
	BeforeEach(func() {
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
					stset.AnnotationLatestMigration: strconv.Itoa(migrations.AdoptPDBSequenceID),
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
						ImagePullSecrets: []corev1.LocalObjectReference{
							{Name: "common-secret"},
							{Name: "my-stset-registry-credentials"},
						},
						Containers: []corev1.Container{{
							Name:  "opi",
							Image: "eirini/dorini",
						}},
					},
				},
			},
		}

		commonSecret := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{Name: "common-secret"},
		}

		privateRegistrySecret := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{Name: "my-stset-registry-credentials"},
		}

		_, err := fixture.Clientset.CoreV1().Secrets(fixture.Namespace).Create(context.Background(), commonSecret, metav1.CreateOptions{})
		Expect(err).NotTo(HaveOccurred())

		_, err = fixture.Clientset.CoreV1().Secrets(fixture.Namespace).Create(context.Background(), privateRegistrySecret, metav1.CreateOptions{})
		Expect(err).NotTo(HaveOccurred())

		_, err = fixture.Clientset.AppsV1().StatefulSets(fixture.Namespace).Create(context.Background(), stSet, metav1.CreateOptions{})
		Expect(err).NotTo(HaveOccurred())
	})

	It("sets the owner reference on the secret", func() {
		stSet, err := fixture.Clientset.AppsV1().StatefulSets(fixture.Namespace).Get(context.Background(), "my-stset", metav1.GetOptions{})
		Expect(err).NotTo(HaveOccurred())

		secret, err := fixture.Clientset.CoreV1().Secrets(fixture.Namespace).Get(context.Background(), "my-stset-registry-credentials", metav1.GetOptions{})
		Expect(err).NotTo(HaveOccurred())

		Expect(secret.OwnerReferences).To(HaveLen(1))
		Expect(secret.OwnerReferences[0].UID).To(Equal(stSet.UID))
		Expect(secret.OwnerReferences[0].Name).To(Equal(stSet.Name))
	})

	It("does not set the owner reference on the common secret", func() {
		secret, err := fixture.Clientset.CoreV1().Secrets(fixture.Namespace).Get(context.Background(), "common-secret", metav1.GetOptions{})
		Expect(err).NotTo(HaveOccurred())

		Expect(secret.OwnerReferences).To(HaveLen(0))
	})

	It("bumps the latest migration annotation", func() {
		stSet, err := fixture.Clientset.AppsV1().StatefulSets(fixture.Namespace).Get(context.Background(), "my-stset", metav1.GetOptions{})
		Expect(err).NotTo(HaveOccurred())

		version, err := strconv.Atoi(stSet.Annotations[stset.AnnotationLatestMigration])
		Expect(err).NotTo(HaveOccurred())
		Expect(version).To(BeNumerically(">=", migrations.AdoptStSetSecretSequenceID))
	})
})
