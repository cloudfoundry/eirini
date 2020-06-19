package config_test

import (
	"errors"

	"code.cloudfoundry.org/eirini/k8s/informers/config"
	"code.cloudfoundry.org/eirini/k8s/informers/config/configfakes"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	k8s_errors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

var _ = Describe("SecretReplicator", func() {
	var (
		secretsClient    *configfakes.FakeSecretsClient
		secretReplicator config.SecretReplicator
		replicationErr   error

		sourceSecret      *corev1.Secret
		destinationSecret *corev1.Secret
	)

	BeforeEach(func() {
		sourceSecret = &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "srcSecret",
				Namespace: "srcNs",
			},
			Type: corev1.SecretTypeDockerConfigJson,
			Data: map[string][]byte{"FOO": []byte("BAR")},
		}
		destinationSecret = &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "dstSecret",
				Namespace: "dstNs",
			},
			Type: corev1.SecretTypeDockerConfigJson,
			Data: map[string][]byte{"FOO": []byte("BAZ")},
		}

		secretsClient = new(configfakes.FakeSecretsClient)
		secretReplicator = config.NewSecretReplicator(secretsClient)

		secretsClient.GetReturnsOnCall(0, sourceSecret, nil)
		secretsClient.GetReturnsOnCall(1, destinationSecret, nil)
	})

	JustBeforeEach(func() {
		replicationErr = secretReplicator.ReplicateSecret("srcNs", "srcSecret", "dstNs", "dstSecret")
	})

	It("replicates the specified secret", func() {
		Expect(replicationErr).NotTo(HaveOccurred())

		By("creating a secret with the correct name in the destination namespace")
		Expect(secretsClient.CreateCallCount()).To(Equal(1))
		creationNs, createdSecret := secretsClient.CreateArgsForCall(0)
		Expect(creationNs).To(Equal("dstNs"))
		Expect(createdSecret.Name).To(Equal("dstSecret"))

		By("creating a secret with the same data and type")
		Expect(createdSecret.Type).To(Equal(sourceSecret.Type))
		Expect(createdSecret.Data).To(Equal(sourceSecret.Data))
	})

	When("getting the source secret fails", func() {
		BeforeEach(func() {
			secretsClient.GetReturnsOnCall(0, nil, errors.New("failed to get secret"))
		})

		It("erorrs appropriately", func() {
			Expect(replicationErr).To(MatchError(ContainSubstring("failed to get secret")))
		})
	})

	When("creating the source secret fails", func() {
		BeforeEach(func() {
			secretsClient.CreateReturns(nil, errors.New("failed to create secret"))
		})

		It("erorrs appropriately", func() {
			Expect(replicationErr).To(MatchError(ContainSubstring("failed to create secret")))
		})
	})

	When("the secret being replicated is already there", func() {
		BeforeEach(func() {
			secretsClient.CreateReturns(nil, k8s_errors.NewAlreadyExists(schema.GroupResource{}, "dstSecret"))
		})

		It("updates it with the new secret data", func() {
			Expect(replicationErr).NotTo(HaveOccurred())

			Expect(secretsClient.UpdateCallCount()).To(Equal(1))
			updatedNs, updatedSecret := secretsClient.UpdateArgsForCall(0)
			Expect(updatedNs).To(Equal("dstNs"))
			Expect(updatedSecret.Data).To(Equal(sourceSecret.Data))
		})

		When("getting the old destination secret fails", func() {
			BeforeEach(func() {
				secretsClient.GetReturnsOnCall(1, nil, errors.New("failed to get destination secret"))
			})

			It("erorrs appropriately", func() {
				Expect(replicationErr).To(MatchError(ContainSubstring("failed to get destination secret")))
			})
		})

		When("updating the old destination secret fails", func() {
			BeforeEach(func() {
				secretsClient.UpdateReturns(nil, errors.New("failed to update destination secret"))
			})

			It("erorrs appropriately", func() {
				Expect(replicationErr).To(MatchError(ContainSubstring("failed to update destination secret")))
			})
		})
	})
})
