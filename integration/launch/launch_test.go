package integration_test

import (
	"os/exec"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"
)

var _ = Describe("Launch", func() {
	var (
		launchPath   string
		session      *gexec.Session
		err          error
		envs         []string
		mockLauncher string
	)

	Context("When launcher is provided", func() {

		BeforeEach(func() {
			envs = []string{"START_COMMAND=dummy"}
			launchPath, err = gexec.Build("code.cloudfoundry.org/eirini/launcher/launchcmd")
			Expect(err).ToNot(HaveOccurred())
		})

		JustBeforeEach(func() {
			cmd := exec.Command(launchPath, mockLauncher)
			cmd.Env = envs
			session, err = gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
			Expect(err).NotTo(HaveOccurred())
		})

		Context("Launcher args", func() {
			BeforeEach(func() {
				mockLauncher = "mock-launchers/verify-args.sh"
				envs = append(envs, "POD_NAME=my-super-pod-42")
			})

			Context("when START_COMMAND env var is provided", func() {
				It("should exit with a zero exit code", func() {
					<-session.Exited
					Expect(session.ExitCode()).To(Equal(0))
				})
			})

			Context("when START_COMMAND env var is NOT provided", func() {
				BeforeEach(func() {
					envs = []string{}
				})

				It("should exit with a non-zero exit code", func() {
					<-session.Exited
					Expect(session.ExitCode()).ToNot(Equal(0))
				})
			})
		})

		Context("Environment variables", func() {
			BeforeEach(func() {
				mockLauncher = "mock-launchers/verify-env-vars.sh"
			})

			Context("when CF_INSTANCE_INDEX is provided", func() {
				BeforeEach(func() {
					envs = append(envs, "POD_NAME=my-super-pod-42")
				})

				It("should exit with a zero exit code", func() {
					<-session.Exited
					Expect(session.ExitCode()).To(Equal(0))
				})

				It("should expose the INSTANCE_INDEX as environment name", func() {
					Eventually(session.Out, 5*time.Second).Should(gbytes.Say("INSTANCE_INDEX:42"))
				})
			})

			Context("when INSTANCE_INDEX is NOT provided", func() {
				It("should exit with a non-zero exit code", func() {
					<-session.Exited
					Expect(session.ExitCode()).ToNot(Equal(0))
				})
			})

			Context("when CF_INSTANCE_INDEX is provided", func() {
				BeforeEach(func() {
					envs = append(envs, "POD_NAME=my-super-pod-11")
				})

				It("should exit with a zero exit code", func() {
					<-session.Exited
					Expect(session.ExitCode()).To(Equal(0))
				})

				It("should expose the CF_INSTANCE_INDEX as environment name", func() {
					Eventually(session.Out, 5*time.Second).Should(gbytes.Say("CF_INSTANCE_INDEX:11"))
				})
			})

			Context("when CF_INSTANCE_INDEX is NOT provided", func() {
				It("should exit with a non-zero exit code", func() {
					<-session.Exited
					Expect(session.ExitCode()).ToNot(Equal(0))
				})
			})
		})
	})

	Context("When no launcher is provided", func() {
		BeforeEach(func() {
			envs = []string{"START_COMMAND=run-it", "POD_NAME=my-super-pod-11"}
			launchPath, err = gexec.Build("code.cloudfoundry.org/eirini/launcher/launchcmd")
			Expect(err).ToNot(HaveOccurred())
		})

		JustBeforeEach(func() {
			cmd := exec.Command(launchPath)
			cmd.Env = envs
			session, err = gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
			Expect(err).NotTo(HaveOccurred())
		})

		It("should fallback to the default command, which is not present on your machine", func() {
			<-session.Exited
			Expect(session.ExitCode()).ToNot(Equal(0))
		})
	})

})
