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
)

var _ = Describe("RootfsPatcher", func() {
	var (
		desirer     opi.Desirer
		odinLRP     *opi.LRP
		thorLRP     *opi.LRP
		patcherPath string
	)

	BeforeEach(func() {
		desirer = k8s.NewStatefulSetDesirer(
			clientset,
			namespace,
			"old_rootfsversion",
		)
		odinLRP = createLRP("ödin")
		thorLRP = createLRP("thor")

		var err error
		patcherPath, err = gexec.Build("code.cloudfoundry.org/eirini/cmd/rootfs-patcher")
		Expect(err).ToNot(HaveOccurred())
	})

	It("should update rootfs version label and wait for pods to restart", func() {
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
			"--kubeconfig", kubeConfigPath,
			"--namespace", namespace,
			"--rootfs-version", newVersion,
			"--timeout=10m")
		session, err := gexec.Start(command, GinkgoWriter, GinkgoWriter)
		Expect(err).ToNot(HaveOccurred())
		Eventually(session, "10m").Should(gexec.Exit(0))

		odinPods := listPods(odinLRP.LRPIdentifier)
		thorPods := listPods(thorLRP.LRPIdentifier)
		Expect(odinPods[0].Labels).To(HaveKeyWithValue(rootfspatcher.RootfsVersionLabel, newVersion))
		Expect(odinPods[1].Labels).To(HaveKeyWithValue(rootfspatcher.RootfsVersionLabel, newVersion))
		Expect(thorPods[0].Labels).To(HaveKeyWithValue(rootfspatcher.RootfsVersionLabel, newVersion))
		Expect(thorPods[1].Labels).To(HaveKeyWithValue(rootfspatcher.RootfsVersionLabel, newVersion))
	})
})
