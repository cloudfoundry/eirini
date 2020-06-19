package config_updater_test

import (
	"fmt"
	"os"
	"reflect"

	"code.cloudfoundry.org/eirini"
	"code.cloudfoundry.org/eirini/k8s/informers/config"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	_ "k8s.io/client-go/plugin/pkg/client/auth"
)

const configTemplate = `
opi:
  registry_secret_name: %s
  app_namespace: %s
`

var _ = Describe("ConfigUpdater", func() {
	var (
		session         *gexec.Session
		configFile      string
		systemNamespace string
		appNamespace    string
	)

	secretUpdatedWith := func(data map[string][]byte) func() bool {
		return func() bool {
			secret := getSecret(appNamespace, "default-image-pull-secret")
			if secret == nil {
				_, err := GinkgoWriter.Write([]byte("secret not found\n"))
				Expect(err).NotTo(HaveOccurred())
				return false
			}

			Expect(secret.Type).To(Equal(corev1.SecretTypeOpaque))
			return reflect.DeepEqual(secret.Data, data)
		}
	}

	BeforeEach(func() {
		systemNamespace = fixture.Namespace
		appNamespace = fixture.DefaultNamespace

		_, err := fixture.Clientset.CoreV1().Secrets(systemNamespace).Create(&corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{Name: "reg-secret"},
			Type:       corev1.SecretTypeOpaque,
			Data:       map[string][]byte{"FOO": []byte("BAR")},
		})
		Expect(err).NotTo(HaveOccurred())

		_, err = fixture.Clientset.CoreV1().ConfigMaps(systemNamespace).Create(&corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name: config.EiriniConfigMapName,
			},
			Data: map[string]string{
				config.OpiConfigName: fmt.Sprintf(configTemplate, "reg-secret", appNamespace),
			},
		})
		Expect(err).NotTo(HaveOccurred())

		config := eirini.ConfigUpdaterConfig{
			KubeConfig: eirini.KubeConfig{
				ConfigPath: fixture.KubeConfigPath,
			},
		}
		session, configFile = eiriniBins.ConfigUpdater.Run(config, fmt.Sprintf("%s=%s", eirini.EnvEiriniNamespace, systemNamespace))
	})

	AfterEach(func() {
		if session != nil {
			session.Kill()
		}
		Expect(os.Remove(configFile)).To(Succeed())
	})

	It("replicates the registry secret in the app namespace", func() {
		Eventually(secretUpdatedWith(
			map[string][]byte{
				"FOO": []byte("BAR"),
			}),
		).Should(BeTrue())
	})

	When("registry secret is updated in the configmap", func() {
		BeforeEach(func() {
			_, err := fixture.Clientset.CoreV1().Secrets(systemNamespace).Create(&corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name: "reg-secret-new",
				},
				Type: corev1.SecretTypeOpaque,
				Data: map[string][]byte{"BAR": []byte("BAZ")},
			})
			Expect(err).NotTo(HaveOccurred())

			_, err = fixture.Clientset.CoreV1().ConfigMaps(systemNamespace).Update(&corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name: config.EiriniConfigMapName,
				},
				Data: map[string]string{
					config.OpiConfigName: fmt.Sprintf(configTemplate, "reg-secret-new", appNamespace),
				},
			})
			Expect(err).NotTo(HaveOccurred())
		})

		It("updates the registry secret in the app namespace", func() {
			Eventually(secretUpdatedWith(
				map[string][]byte{
					"BAR": []byte("BAZ"),
				}),
			).Should(BeTrue())
		})
	})
})

func getSecret(namespace, secretName string) *corev1.Secret {
	sec, err := fixture.Clientset.CoreV1().Secrets(namespace).Get(secretName, metav1.GetOptions{})
	if err != nil {
		return nil
	}
	return sec
}
