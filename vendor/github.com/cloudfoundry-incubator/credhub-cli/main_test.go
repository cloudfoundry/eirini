package main_test

import (
	"os/exec"

	"runtime"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gbytes"
	. "github.com/onsi/gomega/gexec"
)

var _ = Describe("main", func() {
	Context("when no command is provided", func() {
		It("prints help and exits", func() {
			cmd := exec.Command(commandPath)
			session, err := Start(cmd, GinkgoWriter, GinkgoWriter)
			Expect(err).NotTo(HaveOccurred())
			<-session.Exited

			Eventually(session).Should(Exit(1))

			if runtime.GOOS == "windows" {
				Expect(session.Err).To(Say("credhub-cli.exe \\[OPTIONS\\] \\[command\\]"))
			} else {
				Expect(session.Err).To(Say("credhub-cli \\[OPTIONS\\] \\[command\\]"))
			}
		})
	})
})
