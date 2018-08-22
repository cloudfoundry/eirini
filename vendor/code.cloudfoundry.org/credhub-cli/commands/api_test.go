package commands_test

import (
	"io/ioutil"
	"net/http"
	"os"

	"code.cloudfoundry.org/credhub-cli/config"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gbytes"
	. "github.com/onsi/gomega/gexec"
	. "github.com/onsi/gomega/ghttp"
)

var _ = Describe("API", func() {
	BeforeEach(func() {
		server.RouteToHandler("GET", "/info",
			RespondWith(http.StatusOK, `{
					"app":{"name":"CredHub"},
					"auth-server":{"url":"`+authServer.URL()+`"}
					}`),
		)

		authServer.RouteToHandler("GET", "/info", RespondWith(http.StatusOK, ""))
	})

	ItBehavesLikeHelp("api", "a", func(session *Session) {
		Expect(session.Err).To(Say("api"))
		Expect(session.Err).To(Say("SERVER"))
	})

	Describe("when no new API is provided", func() {
		It("shows the currently set API", func() {
			session := runCommand("api")

			Eventually(session).Should(Exit(0))
			Eventually(session.Out).Should(Say(server.URL()))
		})

		Context("with no set API", func() {
			BeforeEach(func() {
				config.WriteConfig(config.Config{})
			})

			It("errors with a helpful message", func() {
				session := runCommand("api")

				Eventually(session).Should(Exit(1))
				Expect(session.Err).To(Say("An API target is not set. Please target the location of your server with `credhub api --server api.example.com` to continue."))
			})
		})
	})

	Describe("when a new API is provided", func() {
		It("revokes existing auth tokens when setting a new api successfully with a different auth server", func() {
			newAuthServer := NewServer()

			apiServer := NewServer()
			apiServer.RouteToHandler("GET", "/info", RespondWith(http.StatusOK, `{
						"app":{"name":"CredHub"},
						"auth-server":{"url":"`+newAuthServer.URL()+`"}
						}`),
			)

			authServer.RouteToHandler("DELETE", "/oauth/token/revoke/"+VALID_ACCESS_TOKEN_JTI, RespondWith(http.StatusOK, ""))

			newAuthServer.RouteToHandler("GET", "/info", RespondWith(http.StatusOK, ""))

			cfg := config.ReadConfig()
			cfg.AuthURL = authServer.URL()
			cfg.AccessToken = VALID_ACCESS_TOKEN
			config.WriteConfig(cfg)

			session := runCommand("api", apiServer.URL())

			Eventually(session).Should(Exit(0))
			Expect(authServer.ReceivedRequests()).Should(HaveLen(1))
		})

		It("leaves existing auth tokens intact when setting a new api with the same auth server", func() {
			apiServer := NewServer()
			apiServer.RouteToHandler("GET", "/info",
				RespondWith(http.StatusOK, `{
						"app":{"name":"CredHub"},
						"auth-server":{"url":"`+authServer.URL()+`"}
						}`),
			)

			cfg := config.ReadConfig()
			cfg.AccessToken = "fake_token"
			cfg.RefreshToken = "fake_refresh"
			config.WriteConfig(cfg)

			session := runCommand("api", apiServer.URL(), "--skip-tls-validation")

			Eventually(session).Should(Exit(0))
			newCfg := config.ReadConfig()
			Expect(newCfg.AccessToken).To(Equal("fake_token"))
			Expect(newCfg.RefreshToken).To(Equal("fake_refresh"))
			Expect(authServer.ReceivedRequests()).Should(HaveLen(0))
		})

		It("retains existing tokens when setting the api fails", func() {
			apiServer := NewServer()
			apiServer.RouteToHandler("GET", "/info", RespondWith(http.StatusNotFound, ""))

			cfg := config.ReadConfig()
			cfg.AuthURL = authServer.URL()
			cfg.AccessToken = "fake_token"
			cfg.RefreshToken = "fake_refresh"
			config.WriteConfig(cfg)

			session := runCommand("api", apiServer.URL())

			Eventually(session).Should(Exit(1))
			newCfg := config.ReadConfig()
			Expect(newCfg.AccessToken).To(Equal("fake_token"))
			Expect(newCfg.RefreshToken).To(Equal("fake_refresh"))
			Expect(authServer.ReceivedRequests()).Should(HaveLen(0))
		})

		It("accepts the API URL from the environment", func() {
			config.RemoveConfig()

			session := runCommandWithEnv([]string{"CREDHUB_SERVER=" + server.URL()}, "api", "--ca-cert", "../test/server-tls-ca.pem", "--ca-cert", "../test/auth-tls-ca.pem")

			Eventually(session).Should(Exit(0))

			cfg := config.ReadConfig()
			Expect(cfg.ApiURL).To(Equal(server.URL()))
		})

		Context("when the provided server url does not have a scheme specified", func() {
			It("sets a default scheme", func() {
				apiServer := NewTLSServer()

				apiServer.RouteToHandler("GET", "/info", RespondWith(http.StatusOK, `{
						"app":{"name":"CredHub"},
						"auth-server":{"url":"`+authServer.URL()+`"}
						}`),
				)

				session := runCommand("api", apiServer.Addr(), "--skip-tls-validation")

				Eventually(session).Should(Exit(0))
			})
		})

		Context("when the provided server url's scheme is https", func() {
			var (
				theServer    *Server
				theServerUrl string
			)

			BeforeEach(func() {
				theServer = NewServer()
				theServerUrl = setUpServer(theServer)
			})

			AfterEach(func() {
				theServer.Close()
			})

			It("sets the target URL and resets ca-cert value", func() {
				session := runCommand("api", theServerUrl)

				Eventually(session).Should(Exit(0))

				session = runCommand("api")

				Eventually(session).Should(Exit(0))
				Eventually(session.Out).Should(Say(theServerUrl))

				cfg := config.ReadConfig()

				Expect(cfg.AuthURL).To(Equal("https://example.com"))
				Expect(len(cfg.CaCerts)).To(Equal(0))
			})

			It("sets the target URL using a flag", func() {
				session := runCommand("api", "-s", theServerUrl)

				Eventually(session).Should(Exit(0))

				session = runCommand("api")

				Eventually(session).Should(Exit(0))
				Eventually(session.Out).Should(Say(theServerUrl))
			})

			It("will prefer the command's argument URL over the flag's argument", func() {
				session := runCommand("api", theServerUrl, "-s", "woooo.com")

				Eventually(session).Should(Exit(0))
				Eventually(session.Out).Should(Say(theServerUrl))

				session = runCommand("api")

				Eventually(session).Should(Exit(0))
				Eventually(session.Out).Should(Say(theServerUrl))
			})

			Context("when the provided server url is not valid", func() {
				var (
					badServer *Server
				)

				BeforeEach(func() {
					// confirm we have original good server
					session := runCommand("api", theServerUrl)

					Eventually(session).Should(Exit(0))

					badServer = NewServer()
					badServer.RouteToHandler("GET", "/info", RespondWith(http.StatusNotFound, ""))
				})

				AfterEach(func() {
					badServer.Close()
				})

				It("retains previous target when the url is not valid", func() {
					// fail to validate on bad server
					session := runCommand("api", badServer.URL())

					Eventually(session).Should(Exit(1))
					Eventually(session.Err).Should(Say("Error connecting to the targeted API"))

					// previous value remains
					session = runCommand("api")

					Eventually(session).Should(Exit(0))
					Eventually(session.Out).Should(Say(theServer.URL()))
				})
			})

			Context("saving configuration from server", func() {
				It("saves config", func() {
					session := runCommand("api", theServer.URL())
					Eventually(session).Should(Exit(0))

					cfg := config.ReadConfig()
					Expect(cfg.ApiURL).To(Equal(theServer.URL()))
					Expect(cfg.AuthURL).To(Equal("https://example.com"))
					Expect(cfg.InsecureSkipVerify).To(Equal(false))
				})

				It("sets file permissions so that the configuration is readable and writeable only by the owner", func() {
					configPath := config.ConfigPath()
					os.Remove(configPath)
					session := runCommand("api", theServer.URL())
					Eventually(session).Should(Exit(0))

					statResult, _ := os.Stat(configPath)

					Expect(statResult.Mode().String(), "-rw-------")
				})

				Context("when the user skips TLS validation", func() {
					BeforeEach(func() {
						cfg := config.ReadConfig()
						cfg.CaCerts = []string{}
						config.WriteConfig(cfg)
					})

					It("prints warning and deprecation notice when --skip-tls-validation flag is present", func() {
						theServer.Close()
						theServer = NewTLSServer()
						theServerUrl = setUpServer(NewTLSServer())
						session := runCommand("api", "-s", theServerUrl, "--skip-tls-validation")

						Eventually(session).Should(Exit(0))
						Eventually(session.Out).Should(Say("Warning: The targeted TLS certificate has not been verified for this connection."))
						Eventually(session.Out).Should(Say("Warning: The --skip-tls-validation flag is deprecated. Please use --ca-cert instead."))
					})

					It("sets skip-tls flag in the config file", func() {
						theServer.Close()
						theServer = NewTLSServer()
						theServerUrl = setUpServer(theServer)
						session := runCommand("api", "-s", theServerUrl, "--skip-tls-validation")

						Eventually(session).Should(Exit(0))
						cfg := config.ReadConfig()
						Expect(cfg.InsecureSkipVerify).To(Equal(true))
					})

					It("resets skip-tls flag in the config file", func() {
						cfg := config.ReadConfig()
						cfg.InsecureSkipVerify = true
						err := config.WriteConfig(cfg)
						Expect(err).NotTo(HaveOccurred())

						session := runCommand("api", "-s", theServerUrl)

						Eventually(session).Should(Exit(0))
						cfg = config.ReadConfig()
						Expect(cfg.InsecureSkipVerify).To(Equal(false))
					})

					It("using a TLS server without the skip-tls flag set will fail on certificate verification", func() {
						theServer.Close()
						theServer = NewTLSServer()
						theServerUrl = setUpServer(theServer)
						session := runCommand("api", "-s", theServerUrl)

						Eventually(session).Should(Exit(1))
						Eventually(session.Err).Should(Say("Error connecting to the targeted API"))
					})

					It("using a TLS server with the skip-tls flag set will succeed", func() {
						theServer.Close()
						theServer = NewTLSServer()
						theServerUrl = setUpServer(theServer)
						session := runCommand("api", "-s", theServerUrl, "--skip-tls-validation")

						Eventually(session).Should(Exit(0))
					})

					It("records skip-tls into config file even with http URLs (will do nothing with that value)", func() {
						session := runCommand("api", theServer.URL(), "--skip-tls-validation")
						cfg := config.ReadConfig()

						Eventually(session).Should(Exit(0))
						Expect(cfg.InsecureSkipVerify).To(Equal(true))
					})
				})
			})

			Describe("ca certs", func() {
				Context("with a single ca cert", func() {
					It("saves the caCert in the config", func() {
						testCa, err := ioutil.ReadFile("../test/server-tls-ca.pem")
						Expect(err).To(BeNil())

						session := runCommand("api", "-s", theServer.URL(), "--ca-cert", "../test/server-tls-ca.pem")
						Eventually(session).Should(Exit(0))

						cfg := config.ReadConfig()
						Expect(cfg.CaCerts).To(Equal([]string{string(testCa)}))
					})
				})

				Context("with multiple ca certs", func() {
					It("saves the certs in the config", func() {
						ca1, err := ioutil.ReadFile("../test/server-tls-ca.pem")
						Expect(err).To(BeNil())
						ca2, err := ioutil.ReadFile("../test/extra-ca.pem")
						Expect(err).To(BeNil())

						session := runCommand("api", "-s", theServer.URL(), "--ca-cert", "../test/server-tls-ca.pem", "--ca-cert", "../test/extra-ca.pem")
						Eventually(session).Should(Exit(0))

						cfg := config.ReadConfig()
						Expect(cfg.CaCerts).To(Equal([]string{string(ca1), string(ca2)}))
					})
				})

				It("overwrites previously set certificates", func() {
					serverCa, err := ioutil.ReadFile("../test/server-tls-ca.pem")
					Expect(err).To(BeNil())
					authCa, err := ioutil.ReadFile("../test/auth-tls-ca.pem")
					Expect(err).To(BeNil())
					extraCa, err := ioutil.ReadFile("../test/extra-ca.pem")
					Expect(err).To(BeNil())

					session := runCommand("api", "-s", server.URL(), "--ca-cert", "../test/server-tls-ca.pem", "--ca-cert", "../test/auth-tls-ca.pem", "--ca-cert", "../test/extra-ca.pem")
					Eventually(session).Should(Exit(0))

					cfg := config.ReadConfig()
					Expect(cfg.CaCerts).To(Equal([]string{string(serverCa), string(authCa), string(extraCa)}))

					session = runCommand("api", "-s", server.URL(), "--ca-cert", "../test/server-tls-ca.pem", "--ca-cert", "../test/auth-tls-ca.pem")
					Eventually(session).Should(Exit(0))

					cfg = config.ReadConfig()
					Expect(cfg.CaCerts).To(Equal([]string{string(serverCa), string(authCa)}))
				})

				It("returns an error if no cert is valid for CredHub", func() {
					previousCfg := config.ReadConfig()
					session := runCommand("api", "-s", server.URL(), "--ca-cert", "../test/auth-tls-ca.pem")

					Eventually(session).Should(Exit(1))
					Eventually(session.Err).Should(Say("certificate signed by unknown authority"))

					cfg := config.ReadConfig()
					Expect(cfg.CaCerts).To(Equal(previousCfg.CaCerts))
				})

				It("returns an error if no cert is valid for the auth server", func() {
					previousCfg := config.ReadConfig()
					session := runCommand("api", "-s", server.URL(), "--ca-cert", "../test/server-tls-ca.pem")

					Eventually(session).Should(Exit(1))
					Eventually(session.Err).Should(Say("certificate signed by unknown authority"))

					cfg := config.ReadConfig()
					Expect(cfg.CaCerts).To(Equal(previousCfg.CaCerts))
				})

				It("accepts the ca cert through the environment", func() {
					authServer.Close()

					authServer = NewTlsServer("../test/server-tls-cert.pem", "../test/server-tls-key.pem")
					SetupServers(server, authServer)

					serverCa, err := ioutil.ReadFile("../test/server-tls-ca.pem")
					Expect(err).To(BeNil())

					session := runCommandWithEnv([]string{"CREDHUB_CA_CERT=../test/server-tls-ca.pem"}, "api", server.URL())
					Eventually(session).Should(Exit(0))

					cfg := config.ReadConfig()
					Expect(cfg.CaCerts).To(ConsistOf([]string{string(serverCa)}))
				})
			})
		})

		Context("when the provided server url's scheme is http", func() {
			var (
				httpServer *Server
			)

			BeforeEach(func() {
				httpServer = NewServer()

				httpServer.RouteToHandler("GET", "/info",
					RespondWith(http.StatusOK, `{
						"app":{"name":"CredHub"},
						"auth-server":{"url":"https://example.com"}
						}`),
				)
			})

			AfterEach(func() {
				httpServer.Close()
			})

			It("does not use TLS", func() {
				session := runCommand("api", httpServer.URL())
				Eventually(session).Should(Exit(0))
				Eventually(session.Out).Should(Say(httpServer.URL()))

				session = runCommand("api")

				Eventually(session).Should(Exit(0))
				Eventually(session.Out).Should(Say(httpServer.URL()))
			})

			It("prints warning text", func() {
				session := runCommand("api", httpServer.URL())
				Eventually(session).Should(Exit(0))
				Eventually(session).Should(Say("Warning: Insecure HTTP API detected. Data sent to this API could be intercepted" +
					" in transit by third parties. Secure HTTPS API endpoints are recommended."))
			})
		})
	})
})

func setUpServer(aServer *Server) string {
	aUrl := aServer.URL()

	aServer.RouteToHandler("GET", "/info",
		RespondWith(http.StatusOK, `{
					"app":{"name":"CredHub"},
					"auth-server":{"url":"https://example.com"}
					}`),
	)

	return aUrl
}
