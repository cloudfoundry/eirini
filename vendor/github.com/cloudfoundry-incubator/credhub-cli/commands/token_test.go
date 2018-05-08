package commands_test

import (
	"net/http"

	"github.com/cloudfoundry-incubator/credhub-cli/config"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gexec"
	. "github.com/onsi/gomega/ghttp"
)

var _ = Describe("Token", func() {
	var (
		uaaServer *Server
	)

	BeforeEach(func() {
		uaaServer = NewServer()
	})

	AfterEach(func() {
		config.RemoveConfig()
	})

	Context("when the config file has a token", func() {

		BeforeEach(func() {
			cfg := config.ReadConfig()
			cfg.AccessToken = "2YotnFZFEjr1zCsicMWpAA"
			config.WriteConfig(cfg)

			uaaServer.RouteToHandler("POST", "/oauth/token",
				CombineHandlers(
					VerifyBody([]byte(`client_id=credhub_cli&client_secret=&grant_type=refresh_token&refresh_token=revoked&response_type=token`)),
					RespondWith(http.StatusOK, `{
						"access_token":"2YotnFZFEjr1zCsicMWpAA",
						"refresh_token":"erousflkajqwer",
						"token_type":"bearer",
						"expires_in":3600}`),
				),
			)

			setConfigAuthUrl(uaaServer.URL())
		})

		It("refreshes the token with --token", func() {
			session := runCommand("--token")

			Expect(uaaServer.ReceivedRequests()).Should(HaveLen(1))

			Eventually(session).Should(Exit(0))
			sout := string(session.Out.Contents())
			Expect(sout).To(ContainSubstring("Bearer 2YotnFZFEjr1zCsicMWpAA"))
			cfg := config.ReadConfig()
			Expect(cfg.AccessToken).To(ContainSubstring("2YotnFZFEjr1zCsicMWpAA"))
		})
	})

	Context("when the config file does not have a token", func() {
		BeforeEach(func() {
			cfg := config.ReadConfig()
			cfg.AccessToken = ""
			config.WriteConfig(cfg)
		})

		It("displays nothing", func() {
			session := runCommand("--token")

			Eventually(session).Should(Exit(0))
			sout := string(session.Out.Contents())
			Expect(sout).To(Equal(""))
		})

		It("gets a new token if using client creds from environment", func() {
			uaaServer.RouteToHandler("POST", "/oauth/token",
				CombineHandlers(
					VerifyBody([]byte(`client_id=credhub_cli&client_secret=secret&grant_type=client_credentials&response_type=token`)),
					RespondWith(http.StatusOK, `{
						"access_token":"2YotnFZFEjr1zCsicMWpAA",
						"refresh_token":"erousflkajqwer",
						"token_type":"bearer",
						"expires_in":3600}`),
				),
			)

			setConfigAuthUrl(uaaServer.URL())
			session := runCommandWithEnv([]string{"CREDHUB_CLIENT=credhub_cli", "CREDHUB_SECRET=secret"}, "--token")

			Expect(uaaServer.ReceivedRequests()).Should(HaveLen(1))

			Eventually(session).Should(Exit(0))
			sout := string(session.Out.Contents())
			Expect(sout).To(ContainSubstring("Bearer 2YotnFZFEjr1zCsicMWpAA"))
			cfg := config.ReadConfig()
			Expect(cfg.AccessToken).To(Equal(""))
		})
	})
})
