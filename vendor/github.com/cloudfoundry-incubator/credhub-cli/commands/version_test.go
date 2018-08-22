package commands_test

import (
	"net/http"

	"code.cloudfoundry.org/credhub-cli/config"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gexec"
	. "github.com/onsi/gomega/ghttp"
)

var _ = Describe("Version", func() {
	Context("when the user is authenticated", func() {
		BeforeEach(func() {
			login()
			resetCachedServerVersion()

			server.RouteToHandler("GET", "/api/v1/data",
				RespondWith(http.StatusOK, `{"data": []}`),
			)
		})

		Context("when the info endpoint returns the server version", func() {
			BeforeEach(func() {
				responseJson := `{"app":{"name":"CredHub","version":"0.2.0"}}`

				server.RouteToHandler("GET", "/info",
					RespondWith(http.StatusOK, responseJson),
				)
			})

			It("displays the version with --version", func() {
				session := runCommand("--version")

				Eventually(session).Should(Exit(0))
				sout := string(session.Out.Contents())
				testVersion(sout)
				Expect(sout).To(ContainSubstring("Server Version: 0.2.0"))
			})
		})

		Context("when the user has not provided an API URL", func() {
			BeforeEach(func() {
				cfg := config.ReadConfig()
				cfg.ApiURL = ""
				config.WriteConfig(cfg)
			})

			It("displays the version with --version", func() {
				session := runCommand("--version")

				Eventually(session).Should(Exit(0))
				sout := string(session.Out.Contents())
				testVersion(sout)
				Expect(sout).To(ContainSubstring("Server Version: Not Found. Have you targeted and authenticated against a CredHub server?"))
			})
		})

		Context("when the info endpoint does not return the server version", func() {
			BeforeEach(func() {
				infoResponseJson := `{"app":{"name":"CredHub"}}`
				versionResponseJson := `{"version":"1.2.3"}`

				server.RouteToHandler("GET", "/info",
					RespondWith(http.StatusOK, infoResponseJson),
				)

				server.RouteToHandler("GET", "/version",
					RespondWith(http.StatusOK, versionResponseJson),
				)
			})

			It("displays the version with --version", func() {
				session := runCommand("--version")

				Eventually(session).Should(Exit(0))
				sout := string(session.Out.Contents())
				testVersion(sout)
				Expect(sout).To(ContainSubstring("Server Version: 1.2.3"))
			})
		})

		Context("when the request fails", func() {
			BeforeEach(func() {
				server.RouteToHandler("GET", "/info",
					RespondWith(http.StatusNotFound, ""),
				)
			})

			It("displays the version with --version", func() {
				session := runCommand("--version")

				Eventually(session).Should(Exit(0))
				sout := string(session.Out.Contents())
				testVersion(sout)
				Expect(sout).To(ContainSubstring("Server Version: Not Found"))
			})
		})
	})

	Context("when the user is logged out", func() {
		BeforeEach(func() {
			responseJson := `{"app":{"name":"CredHub","version":"0.2.0"}}`

			server.RouteToHandler("GET", "/info",
				RespondWith(http.StatusOK, responseJson),
			)
		})

		It("returns the CLI version but not the server version", func() {
			session := runCommand("--version")

			Eventually(session).Should(Exit(0))
			sout := string(session.Out.Contents())
			testVersion(sout)
			Expect(sout).To(ContainSubstring("Server Version: Not Found. Have you targeted and authenticated against a CredHub server?"))
		})
	})

	Context("when the config contains invalid tokens", func() {
		BeforeEach(func() {
			responseJson := "Server Version: Not Found. Have you targeted and authenticated against a CredHub server?"

			server.RouteToHandler("GET", "/info",
				RespondWith(http.StatusOK, responseJson),
			)

			server.RouteToHandler("GET", "/api/v1/data",
				RespondWith(http.StatusUnauthorized, ""),
			)

			cfg := config.ReadConfig()
			cfg.RefreshToken = "foo"
			cfg.AccessToken = "bar"
			config.WriteConfig(cfg)
		})

		It("returns the server error message", func() {
			session := runCommand("--version")

			Eventually(session).Should(Exit(0))
			sout := string(session.Out.Contents())
			testVersion(sout)
			Expect(sout).To(ContainSubstring("Server Version: Not Found. Have you targeted and authenticated against a CredHub server?"))
		})
	})
})

func testVersion(sout string) {
	Expect(sout).To(ContainSubstring("CLI Version: test-version"))
	Expect(sout).ToNot(ContainSubstring("build DEV"))
}
