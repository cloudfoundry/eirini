package opi_test

import (
	"os"
	"os/exec"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"
)

var _ = Describe("Staging", func() {
	var (
		// httpClient *http.Client

		configFile *os.File
		session    *gexec.Session
	)

	BeforeEach(func() {
		// httpClient = makeTestHTTPClient()
		config := defaultEiriniConfig()
		configFile = createOpiConfigFromFixtures(config)

		command := exec.Command(pathToOpi, "connect", "-c", configFile.Name()) // #nosec G204
		var err error
		session, err = gexec.Start(command, GinkgoWriter, GinkgoWriter)
		Expect(err).ToNot(HaveOccurred())
	})

	AfterEach(func() {
		if configFile != nil {
			os.Remove(configFile.Name())
		}
		if session != nil {
			session.Kill()
		}
	})

	It("can start", func() {
		Consistently(session).ShouldNot(gexec.Exit())
	})

})
