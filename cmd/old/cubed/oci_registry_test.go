package main_test

import (
	"net/http"
	"os"
	"os/exec"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"
)

var _ = Describe("Cubed", func() {
	// This test is a bit of a pain atm, you need to manually add
	// the current IP address to insecure_registry_list in docker so
	// it will pull
	PIt("serves up OCI images that can be downloaded by a docker client", func() {
		cubed, err := gexec.Build("github.com/julz/cube/cmd/cubed")
		Expect(err).NotTo(HaveOccurred())

		server, err := gexec.Start(exec.Command(cubed, "-rootfs", "/Users/julz/workspace/minirootfs.tar"), GinkgoWriter, GinkgoWriter)
		Expect(err).NotTo(HaveOccurred())
		defer server.Kill()

		Eventually(server, 5).Should(gbytes.Say("started"))

		dropletTar, err := os.Open("/Users/julz/workspace/mydroplet.tar")
		Expect(err).NotTo(HaveOccurred())

		resp, err := http.Post("http://10.200.120.208:8080/v2/myspace/myapp/manifests/the-droplet-guid", "cloudfoundry/droplet", dropletTar)
		Expect(err).NotTo(HaveOccurred())
		Expect(resp.StatusCode).To(Equal(200))

		docker, err := gexec.Start(exec.Command("docker", "pull", "10.200.120.208:the-droplet-guid"), GinkgoWriter, GinkgoWriter)
		Expect(err).NotTo(HaveOccurred())

		Eventually(docker, "5s").Should(gexec.Exit(0))
	})
})
