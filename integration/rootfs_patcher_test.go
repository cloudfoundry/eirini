package statefulsets_test

import (
	"os/exec"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"

	"code.cloudfoundry.org/eirini/k8s"
	"code.cloudfoundry.org/eirini/k8s/utils"
	"code.cloudfoundry.org/eirini/opi"
	"code.cloudfoundry.org/eirini/rootfspatcher"
	"code.cloudfoundry.org/eirini/util"
	"code.cloudfoundry.org/lager/lagertest"
)

var _ = Describe("RootfsPatcher", func() {
	var (
		desirer     *k8s.StatefulSetDesirer
		odinLRP     *opi.LRP
		thorLRP     *opi.LRP
		patcherPath string
	)

	BeforeEach(func() {
		logger := lagertest.NewTestLogger("test")

		desirer = &k8s.StatefulSetDesirer{
			Pods:                      k8s.NewPodsClient(fixture.Clientset),
			Secrets:                   k8s.NewSecretsClient(fixture.Clientset),
			StatefulSets:              k8s.NewStatefulSetClient(fixture.Clientset),
			PodDisruptionBudets:       k8s.NewPodDisruptionBudgetClient(fixture.Clientset),
			Events:                    k8s.NewEventsClient(fixture.Clientset),
			StatefulSetToLRPMapper:    k8s.StatefulSetToLRP,
			RegistrySecretName:        "registry-secret",
			RootfsVersion:             "old_rootfsversion",
			LivenessProbeCreator:      k8s.CreateLivenessProbe,
			ReadinessProbeCreator:     k8s.CreateReadinessProbe,
			Hasher:                    util.TruncatedSHA256Hasher{},
			Logger:                    logger,
			ApplicationServiceAccount: "default",
		}
		odinLRP = createLRP("ödin")
		thorLRP = createLRP("thor")

		var err error
		patcherPath, err = gexec.Build("code.cloudfoundry.org/eirini/cmd/rootfs-patcher")
		Expect(err).ToNot(HaveOccurred())
	})

	It("should update rootfs version label", func() {
		err := desirer.Desire(odinLRP)
		Expect(err).ToNot(HaveOccurred())
		err = desirer.Desire(thorLRP)
		Expect(err).ToNot(HaveOccurred())
		Eventually(func() []string {
			pods := append(listPods(odinLRP.LRPIdentifier), listPods(thorLRP.LRPIdentifier)...)
			result := []string{}
			for _, p := range pods {
				result = append(result, utils.GetPodState(p))
			}
			return result
		}, timeout).Should(ConsistOf(opi.RunningState, opi.RunningState, opi.RunningState, opi.RunningState))

		newVersion := "new-rootfsversion"

		command := exec.Command(patcherPath,
			"--kubeconfig", fixture.KubeConfigPath,
			"--namespace", fixture.Namespace,
			"--rootfs-version", newVersion)
		session, err := gexec.Start(command, GinkgoWriter, GinkgoWriter)
		Expect(err).ToNot(HaveOccurred())
		Eventually(session, "1m").Should(gexec.Exit(0))

		statefulsets := listAllStatefulSets(odinLRP, thorLRP)
		Expect(statefulsets[0].Labels).To(HaveKeyWithValue(rootfspatcher.RootfsVersionLabel, newVersion))
		Expect(statefulsets[1].Labels).To(HaveKeyWithValue(rootfspatcher.RootfsVersionLabel, newVersion))
	})
})
