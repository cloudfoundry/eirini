package integration_test

// +build integration

import (
	"errors"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"

	yaml "gopkg.in/yaml.v2"

	"code.cloudfoundry.org/eirini"
)

var _ = Describe("Build", func() {
	Context("When building OPI", func() {
		var (
			opiPath       string
			err           error
			opiConfig     eirini.Config
			tmpDir        string
			config        []byte
			session       *gexec.Session
			opiConfigPath string
		)

		BeforeEach(func() {
			opiPath, err = gexec.Build("code.cloudfoundry.org/eirini/cmd/opi")
			Expect(err).ToNot(HaveOccurred())
		})

		AfterEach(func() {
			session.Terminate()
		})

		JustBeforeEach(func() {
			cmd := exec.Command(opiPath, "connect", "--config", opiConfigPath)
			session, err = gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
			Expect(err).NotTo(HaveOccurred())
		})

		Context("Using a invalid opi config file", func() {
			BeforeEach(func() {
				opiConfig = eirini.Config{}
				config, err = yaml.Marshal(&opiConfig)
				Expect(err).ToNot(HaveOccurred())

				tmpDir, err = ioutil.TempDir("", "opi-tmp-dir")
				Expect(err).NotTo(HaveOccurred())

				opiConfigPath = filepath.Join(tmpDir, "opi.yml")
				err = ioutil.WriteFile(opiConfigPath, config, 0644)
				Expect(err).ToNot(HaveOccurred())
			})

			It("should exit with a non-zero code", func() {
				<-session.Exited
				Expect(session.ExitCode()).ToNot(Equal(0))
			})
		})

		Context("Using a valid opi config file", func() {

			BeforeEach(func() {
				if validOpiConfigPath == "" {
					panic(errors.New("Valid OPI config file not provided"))
				}
				opiConfigPath = validOpiConfigPath
			})

			It("should print connected string", func() {
				Eventually(session.Err, 5*time.Second).Should(gbytes.Say(".*opi connected"))
			})

			It("should run the opi process", func() {
				pid := session.Command.Process.Pid
				process, err := os.FindProcess(int(pid))
				Expect(err).ToNot(HaveOccurred())
				err = process.Signal(syscall.Signal(0))
				Expect(err).ToNot(HaveOccurred())
			})

		})
	})
})
