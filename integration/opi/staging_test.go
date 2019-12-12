package opi_test

import (
	"bytes"
	"fmt"
	"net/http"
	"os"
	"os/exec"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"
)

var _ = Describe("Staging", func() {
	var (
		httpClient *http.Client

		configFile *os.File
		session    *gexec.Session
		url        string
	)

	BeforeEach(func() {
		httpClient = makeTestHTTPClient()
		config := defaultEiriniConfig()
		configFile = createOpiConfigFromFixtures(config)

		command := exec.Command(pathToOpi, "connect", "-c", configFile.Name()) // #nosec G204
		var err error
		session, err = gexec.Start(command, GinkgoWriter, GinkgoWriter)
		Expect(err).ToNot(HaveOccurred())
		url = fmt.Sprintf("https://localhost:%d/", config.Properties.TLSPort)
		Eventually(func() error {
			_, err := httpClient.Get(url)
			return err
		}, "5s").Should(Succeed())
	})

	AfterEach(func() {
		if configFile != nil {
			os.Remove(configFile.Name())
		}
		if session != nil {
			session.Kill()
		}
	})

	It("creates staging job", func() {
		body := `{
				"app_guid": "our-app-id",
				"environment": [{"name": "HOWARD", "value": "the alien"}],
				"lifecycle_data": {
					"app_bits_download_uri": "example.com/download",
					"droplet_upload_uri": "example.com/upload",
					"buildpacks": []
				},
				"completion_callback": "example.com/call/me/maybe"
			}`
		resp, err := httpClient.Post(fmt.Sprintf("%s/%s", url, "stage/guid"), "json", bytes.NewReader([]byte(body)))
		Expect(err).NotTo(HaveOccurred())
		Expect(resp.StatusCode).To(Equal(http.StatusAccepted))
	})

})
