package commands_test

import (
	"net/http"
	"runtime"

	"code.cloudfoundry.org/credhub-cli/config"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gbytes"
	. "github.com/onsi/gomega/gexec"
)

var _ = Describe("Logout", func() {
	AfterEach(func() {
		config.RemoveConfig()
	})

	It("marks the access token and refresh token as revoked if no config exists", func() {
		config.RemoveConfig()
		runLogoutCommand()
	})

	It("leaves the access token and refresh token as revoked if config exists and they were already revoked", func() {
		cfg := config.Config{
			ConfigWithoutSecrets: config.ConfigWithoutSecrets{
				RefreshToken: "revoked",
				AccessToken:  "revoked",
			},
		}
		config.WriteConfig(cfg)
		runLogoutCommand()
	})

	It("asks UAA to revoke the token (and UAA succeeds)", func() {
		setupUAAConfig(http.StatusOK)
		runLogoutCommand()
	})

	It("asks UAA to revoke the token (and reports error when UAA fails)", func() {
		setupUAAConfig(http.StatusUnauthorized)

		session := runCommand("logout")
		Eventually(session.Err).Should(Say("Received HTTP 401 error while revoking token"))
		Eventually(session).Should(Exit(1))
	})

	ItBehavesLikeHelp("logout", "o", func(session *Session) {
		Expect(session.Err).To(Say("Usage:"))
		if runtime.GOOS == "windows" {
			Expect(session.Err).To(Say("credhub-cli.exe \\[OPTIONS\\] logout"))
		} else {
			Expect(session.Err).To(Say("credhub-cli \\[OPTIONS\\] logout"))
		}
	})
})

func runLogoutCommand() {
	session := runCommand("logout")
	Eventually(session).Should(Exit(0))
	Eventually(session).Should(Say("Logout Successful"))
	cfg := config.ReadConfig()
	Expect(cfg.AccessToken).To(Equal("revoked"))
	Expect(cfg.RefreshToken).To(Equal("revoked"))
}
