package integration_test

import (
	"code.cloudfoundry.org/eirini/k8s/client"
	"code.cloudfoundry.org/eirini/k8s/stset"
	"code.cloudfoundry.org/eirini/tests"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = Describe("Secrets", func() {
	var secretClient *client.Secret

	BeforeEach(func() {
		secretClient = client.NewSecret(fixture.Clientset)
	})

	Describe("Get", func() {
		var guid, extraNs string

		BeforeEach(func() {
			guid = tests.GenerateGUID()

			createSecret(fixture.Namespace, "foo", map[string]string{
				stset.LabelGUID: guid,
			})

			extraNs = fixture.CreateExtraNamespace()

			createSecret(extraNs, "foo", nil)
		})

		It("retrieves a Secret by namespace and name", func() {
			secret, err := secretClient.Get(ctx, fixture.Namespace, "foo")
			Expect(err).NotTo(HaveOccurred())

			Expect(secret.Name).To(Equal("foo"))
			Expect(secret.Labels[stset.LabelGUID]).To(Equal(guid))
		})
	})

	Describe("Create", func() {
		It("creates the secret in the namespace", func() {
			_, createErr := secretClient.Create(ctx, fixture.Namespace, &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name: "very-secret",
				},
			})
			Expect(createErr).NotTo(HaveOccurred())

			secrets := listSecrets(fixture.Namespace)
			Expect(secretNames(secrets)).To(ContainElement("very-secret"))
		})
	})

	Describe("Update", func() {
		BeforeEach(func() {
			createSecret(fixture.Namespace, "top-secret", map[string]string{"worst-year-ever": "2016"})
		})

		It("updates the existing secret", func() {
			_, err := secretClient.Update(ctx, fixture.Namespace, &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name: "top-secret",
					Labels: map[string]string{
						"worst-year-ever": "2020",
					},
				},
			})
			Expect(err).NotTo(HaveOccurred())

			secret, err := getSecret(fixture.Namespace, "top-secret")
			Expect(err).NotTo(HaveOccurred())
			Expect(secret.Labels).To(HaveKeyWithValue("worst-year-ever", "2020"))
		})
	})

	Describe("Delete", func() {
		BeforeEach(func() {
			createSecret(fixture.Namespace, "open-secret", nil)
		})

		It("deletes a Secret", func() {
			Eventually(func() []string {
				return secretNames(listSecrets(fixture.Namespace))
			}).Should(ContainElement("open-secret"))

			err := secretClient.Delete(ctx, fixture.Namespace, "open-secret")

			Expect(err).NotTo(HaveOccurred())
			Eventually(func() []string {
				return secretNames(listSecrets(fixture.Namespace))
			}).ShouldNot(ContainElement("open-secret"))
		})
	})

	Describe("set owner", func() {
		var (
			secret *corev1.Secret
			stSet  *appsv1.StatefulSet
		)

		BeforeEach(func() {
			secret = createSecret(fixture.Namespace, "open-secret", nil)
			stSet = createStatefulSetSpec(fixture.Namespace, "foo", nil, nil)
			stSet.UID = "my-uid"
			stSet.OwnerReferences = []metav1.OwnerReference{}
		})

		It("updates owner info", func() {
			updatedSecret, err := secretClient.SetOwner(ctx, secret, stSet)
			Expect(err).NotTo(HaveOccurred())

			Expect(updatedSecret.OwnerReferences).To(HaveLen(1))
			Expect(updatedSecret.OwnerReferences[0].Name).To(Equal("foo"))
			Expect(updatedSecret.OwnerReferences[0].Kind).To(Equal("StatefulSet"))
			Expect(string(updatedSecret.OwnerReferences[0].UID)).To(Equal("my-uid"))
		})
	})
})

func secretNames(secrets []corev1.Secret) []string {
	names := make([]string, 0, len(secrets))
	for _, s := range secrets {
		names = append(names, s.Name)
	}

	return names
}
