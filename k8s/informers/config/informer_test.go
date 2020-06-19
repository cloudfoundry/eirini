package config_test

import (
	"errors"
	"fmt"

	"code.cloudfoundry.org/eirini"
	"code.cloudfoundry.org/eirini/k8s/informers/config"
	"code.cloudfoundry.org/eirini/k8s/informers/config/configfakes"
	"code.cloudfoundry.org/lager/lagertest"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/testing"
)

const (
	opiConfigTemplate = `
opi:
  app_namespace: the-app-ns
  registry_secret_name: %s
`
	opiConfigTemplateAdditionalValue = `
opi:
  app_namespace: the-app-ns
  registry_secret_name: %s
  foo: bar
`
)

var _ = Describe("ConfigMap Informer", func() {
	var (
		client          *fake.Clientset
		informerStopper chan struct{}
		watcher         *watch.FakeWatcher
		logger          *lagertest.TestLogger

		secretReplicator *configfakes.FakeSecretReplicator
	)

	BeforeEach(func() {
		secretReplicator = new(configfakes.FakeSecretReplicator)
		informerStopper = make(chan struct{})

		logger = lagertest.NewTestLogger("configmap-informer-test-logger")
		client = fake.NewSimpleClientset()

		watcher = watch.NewFake()
		client.PrependWatchReactor("configmaps", testing.DefaultWatchReactor(watcher, nil))
		configInformer := config.NewInformer(
			client,
			0,
			"the-configmap-ns",
			secretReplicator,
			informerStopper,
			logger,
		)

		go configInformer.Start()

		watcher.Add(configMap("foo-1"))
	})

	AfterEach(func() {
		close(informerStopper)
	})

	It("replicates the registry secret", func() {
		Eventually(secretReplicator.ReplicateSecretCallCount).Should(Equal(1))
		srcNamespace, srcSecretName, dstNamespace, dstSecretName := secretReplicator.ReplicateSecretArgsForCall(0)
		Expect(srcNamespace).To(Equal("the-configmap-ns"))
		Expect(srcSecretName).To(Equal("foo-1"))
		Expect(dstNamespace).To(Equal("the-app-ns"))
		Expect(dstSecretName).To(Equal(eirini.RegistrySecretName))
	})

	When("the secretReplicator errors", func() {
		BeforeEach(func() {
			secretReplicator.ReplicateSecretReturns(errors.New("boom"))
		})

		It("logs the incident", func() {
			Eventually(logger.LogMessages).Should(HaveLen(1))
			Expect(logger.LogMessages()[0]).To(ContainSubstring("failed-to-replicate-registry-secret"))
		})
	})

	When("the registry secret name is modified in the eirini configmap", func() {
		BeforeEach(func() {
			Eventually(secretReplicator.ReplicateSecretCallCount).Should(Equal(1))
			watcher.Modify(configMap("foo-2"))
		})

		It("replicates the new secret in the current namespace", func() {
			Eventually(secretReplicator.ReplicateSecretCallCount).Should(Equal(2))
			srcNamespace, srcSecretName, dstNamespace, dstSecretName := secretReplicator.ReplicateSecretArgsForCall(1)
			Expect(srcNamespace).To(Equal("the-configmap-ns"))
			Expect(srcSecretName).To(Equal("foo-2"))
			Expect(dstNamespace).To(Equal("the-app-ns"))
			Expect(dstSecretName).To(Equal(eirini.RegistrySecretName))
		})

		When("the secretReplicator errors", func() {
			BeforeEach(func() {
				secretReplicator.ReplicateSecretReturns(errors.New("boom"))
			})

			It("logs the incident", func() {
				Eventually(logger.LogMessages).Should(HaveLen(1))
				Expect(logger.LogMessages()[0]).To(ContainSubstring("failed-to-replicate-registry-secret"))
			})
		})
	})

	When("the eirini configmap is updated, but registry secret is not changed", func() {
		BeforeEach(func() {
			Eventually(secretReplicator.ReplicateSecretCallCount).Should(Equal(1))
			watcher.Modify(&corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      config.EiriniConfigMapName,
					Namespace: "the-configmap-ns",
				},
				Data: map[string]string{
					config.OpiConfigName: fmt.Sprintf(opiConfigTemplateAdditionalValue, "foo-1"),
				},
			})
		})

		It("does not replicate the registry secret", func() {
			Consistently(secretReplicator.ReplicateSecretCallCount).Should(Equal(1))
		})
	})

	When("the eirini configmap is updated, with an invalid yaml", func() {
		BeforeEach(func() {
			Eventually(secretReplicator.ReplicateSecretCallCount).Should(Equal(1))
			watcher.Modify(configMapWithData(map[string]string{
				config.OpiConfigName: "\t",
			}))
		})

		It("does not replicate the registry secret", func() {
			Consistently(secretReplicator.ReplicateSecretCallCount).Should(Equal(1))
		})

		It("logs the incident", func() {
			Eventually(logger.LogMessages).Should(HaveLen(1))
			Expect(logger.LogMessages()[0]).To(ContainSubstring("error-unmarshaling-opi-config"))
		})
	})
})

func configMap(secretName string) *corev1.ConfigMap {
	return configMapWithData(map[string]string{
		config.OpiConfigName: fmt.Sprintf(opiConfigTemplate, secretName),
	})
}

func configMapWithData(data map[string]string) *corev1.ConfigMap {
	return &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      config.EiriniConfigMapName,
			Namespace: "the-configmap-ns",
		},
		Data: data,
	}
}
