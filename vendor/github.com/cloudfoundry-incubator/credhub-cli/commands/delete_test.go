package commands_test

import (
	"net/http"

	"runtime"

	"github.com/cloudfoundry-incubator/credhub-cli/config"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gbytes"
	. "github.com/onsi/gomega/gexec"
	. "github.com/onsi/gomega/ghttp"
)

var _ = Describe("Delete", func() {
	BeforeEach(func() {
		login()
	})

	ItRequiresAuthentication("delete", "-n", "test-credential")
	ItRequiresAnAPIToBeSet("delete", "-n", "test-credential")
	ItAutomaticallyLogsIn("DELETE", "delete_test.json", "/api/v1/data", "delete", "-n", "test-credential")

	Describe("Help", func() {
		ItBehavesLikeHelp("delete", "d", func(session *Session) {
			Expect(session.Err).To(Say("delete"))
			Expect(session.Err).To(Say("name"))
		})
	})

	It("deletes a secret", func() {
		server.RouteToHandler("DELETE", "/api/v1/data",
			CombineHandlers(
				VerifyRequest("DELETE", "/api/v1/data", "name=my-secret"),
				RespondWith(http.StatusOK, ""),
			),
		)

		session := runCommand("delete", "-n", "my-secret")

		Eventually(session).Should(Exit(0))
		Eventually(session.Out).Should(Say("Credential successfully deleted"))
	})

	Describe("Errors", func() {
		It("prints an error when the network request fails", func() {
			cfg := config.ReadConfig()
			cfg.ApiURL = "mashed://potatoes"
			config.WriteConfig(cfg)

			session := runCommand("delete", "-n", "my-secret")

			Eventually(session).Should(Exit(1))
			Eventually(string(session.Err.Contents())).Should(ContainSubstring("Delete mashed://potatoes/api/v1/data?name=my-secret: unsupported protocol scheme \"mashed\""))
		})

		It("displays missing required parameter", func() {
			session := runCommand("delete")

			Eventually(session).Should(Exit(1))

			if runtime.GOOS == "windows" {
				Expect(session.Err).To(Say("the required flag `/n, /name' was not specified"))
			} else {
				Expect(session.Err).To(Say("the required flag `-n, --name' was not specified"))
			}
		})
	})
})
