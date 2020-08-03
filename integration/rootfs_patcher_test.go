package statefulsets_test

import (
	"os/exec"

	intutil "code.cloudfoundry.org/eirini/integration/util"
	"code.cloudfoundry.org/eirini/k8s"
	"code.cloudfoundry.org/eirini/k8s/client"
	"code.cloudfoundry.org/eirini/k8s/utils"
	"code.cloudfoundry.org/eirini/opi"
	"code.cloudfoundry.org/eirini/rootfspatcher"
	"code.cloudfoundry.org/lager/lagertest"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"
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
			Pods:                      client.NewPod(fixture.Clientset),
			Secrets:                   client.NewSecret(fixture.Clientset),
			StatefulSets:              client.NewStatefulSet(fixture.Clientset),
			PodDisruptionBudets:       client.NewPodDisruptionBudget(fixture.Clientset),
			Events:                    client.NewEvent(fixture.Clientset),
			StatefulSetToLRPMapper:    k8s.StatefulSetToLRP,
			RegistrySecretName:        "registry-secret",
			RootfsVersion:             "old_rootfsversion",
			LivenessProbeCreator:      k8s.CreateLivenessProbe,
			ReadinessProbeCreator:     k8s.CreateReadinessProbe,
			Logger:                    logger,
			ApplicationServiceAccount: intutil.GetApplicationServiceAccount(),
		}
		odinLRP = createLRP("ödin")
		thorLRP = createLRP("thor")

		var err error
		patcherPath, err = gexec.Build("code.cloudfoundry.org/eirini/cmd/rootfs-patcher")
		Expect(err).ToNot(HaveOccurred())
	})

	It("should update rootfs version label", func() {
		err := desirer.Desire(fixture.Namespace, odinLRP)
		Expect(err).ToNot(HaveOccurred())
		err = desirer.Desire(fixture.Namespace, thorLRP)
		Expect(err).ToNot(HaveOccurred())
		Eventually(func() []string {
			pods := append(listPods(odinLRP.LRPIdentifier), listPods(thorLRP.LRPIdentifier)...)
			result := []string{}
			for _, p := range pods {
				result = append(result, utils.GetPodState(p))
			}

			return result
		}).Should(ConsistOf(opi.RunningState, opi.RunningState, opi.RunningState, opi.RunningState))

		newVersion := "new-rootfsversion"

		command := exec.Command(patcherPath,
			"--kubeconfig", fixture.KubeConfigPath,
			"--namespace", fixture.Namespace,
			"--rootfs-version", newVersion)
		session, err := gexec.Start(command, GinkgoWriter, GinkgoWriter)
		Expect(err).ToNot(HaveOccurred())
		Eventually(session).Should(gexec.Exit(0))

		statefulsets := listAllStatefulSets(odinLRP, thorLRP)
		Expect(statefulsets[0].Labels).To(HaveKeyWithValue(rootfspatcher.RootfsVersionLabel, newVersion))
		Expect(statefulsets[1].Labels).To(HaveKeyWithValue(rootfspatcher.RootfsVersionLabel, newVersion))
	})
})
