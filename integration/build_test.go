package integration_test

import (
	"io/ioutil"
	"os/exec"
	"path/filepath"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"

	yaml "gopkg.in/yaml.v2"

	"code.cloudfoundry.org/eirini"
)

var _ = Describe("Build {SYSTEM}", func() {
	Context("When building OPI", func() {
		var (
			opiPath    string
			err        error
			opiConfig  eirini.Config
			tmpDir     string
			config     []byte
			configPath string
			session    *gexec.Session
		)

		BeforeEach(func() {
			opiPath, err = gexec.Build("code.cloudfoundry.org/eirini/cmd/opi")
			Expect(err).ToNot(HaveOccurred())
		})

		JustBeforeEach(func() {
			cmd := exec.Command(opiPath, "connect", "--config", configPath)
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

				configPath = filepath.Join(tmpDir, "opi.yml")
				err = ioutil.WriteFile(configPath, config, 0644)
				Expect(err).ToNot(HaveOccurred())
			})

			It("should exit with a non-zero code", func() {
				<-session.Exited
				Expect(session.ExitCode()).ToNot(Equal(0))
			})
		})
	})
})
