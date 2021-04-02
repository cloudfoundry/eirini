package migration_test

import (
	"context"
	"strconv"

	"code.cloudfoundry.org/eirini/k8s/jobs"
	"code.cloudfoundry.org/eirini/k8s/shared"
	"code.cloudfoundry.org/eirini/migrations"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = Describe("Adopt Job Secret Migration", func() {
	var privateRegistrySecret *corev1.Secret
	BeforeEach(func() {
		privateRegistrySecretPrefix := "my-app-my-space-registry-secret-"

		privateRegistrySecret = &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{GenerateName: privateRegistrySecretPrefix},
		}

		var err error
		privateRegistrySecret, err = fixture.Clientset.CoreV1().Secrets(fixture.Namespace).Create(context.Background(), privateRegistrySecret, metav1.CreateOptions{})
		Expect(err).NotTo(HaveOccurred())

		commonSecret := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{Name: "common-secret"},
		}
		_, err = fixture.Clientset.CoreV1().Secrets(fixture.Namespace).Create(context.Background(), commonSecret, metav1.CreateOptions{})
		Expect(err).NotTo(HaveOccurred())

		job := &batchv1.Job{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "my-task",
				Namespace: fixture.Namespace,
				Labels: map[string]string{
					jobs.LabelGUID:       "my-task-guid",
					jobs.LabelSourceType: "TASK",
				},
				Annotations: map[string]string{
					jobs.AnnotationAppName:   "my-app",
					jobs.AnnotationSpaceName: "my-space",
				},
			},
			Spec: batchv1.JobSpec{
				Template: corev1.PodTemplateSpec{
					ObjectMeta: metav1.ObjectMeta{
						Labels: map[string]string{
							"app": "asdf",
						},
					},
					Spec: corev1.PodSpec{
						RestartPolicy: corev1.RestartPolicyNever,
						ImagePullSecrets: []corev1.LocalObjectReference{
							{Name: "common-secret"},
							{Name: privateRegistrySecret.Name},
						},
						Containers: []corev1.Container{{
							Name:  "opi",
							Image: "eirini/dorini",
						}},
					},
				},
			},
		}

		_, err = fixture.Clientset.BatchV1().Jobs(fixture.Namespace).Create(context.Background(), job, metav1.CreateOptions{})
		Expect(err).NotTo(HaveOccurred())
	})

	It("sets the owner reference on the secret", func() {
		job, err := fixture.Clientset.BatchV1().Jobs(fixture.Namespace).Get(context.Background(), "my-task", metav1.GetOptions{})
		Expect(err).NotTo(HaveOccurred())

		secret, err := fixture.Clientset.CoreV1().Secrets(fixture.Namespace).Get(context.Background(), privateRegistrySecret.Name, metav1.GetOptions{})
		Expect(err).NotTo(HaveOccurred())

		Expect(secret.OwnerReferences).To(HaveLen(1))
		Expect(secret.OwnerReferences[0].UID).To(Equal(job.UID))
		Expect(secret.OwnerReferences[0].Name).To(Equal(job.Name))
	})

	It("does not set the owner reference on the common secret", func() {
		secret, err := fixture.Clientset.CoreV1().Secrets(fixture.Namespace).Get(context.Background(), "common-secret", metav1.GetOptions{})
		Expect(err).NotTo(HaveOccurred())

		Expect(secret.OwnerReferences).To(HaveLen(0))
	})

	It("bumps the latest migration annotation", func() {
		job, err := fixture.Clientset.BatchV1().Jobs(fixture.Namespace).Get(context.Background(), "my-task", metav1.GetOptions{})
		Expect(err).NotTo(HaveOccurred())

		version, err := strconv.Atoi(job.Annotations[shared.AnnotationLatestMigration])
		Expect(err).NotTo(HaveOccurred())
		Expect(version).To(BeNumerically(">=", migrations.AdoptJobSecretSequenceID))
	})
})
