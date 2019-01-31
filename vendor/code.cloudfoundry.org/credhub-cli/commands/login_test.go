package commands_test

import (
	"fmt"
	"net/http"

	"strings"

	"io/ioutil"

	"code.cloudfoundry.org/credhub-cli/config"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gbytes"
	. "github.com/onsi/gomega/gexec"
	. "github.com/onsi/gomega/ghttp"
)

const versionTwo = "2.0.0"

var _ = Describe("Login", func() {
	var (
		uaaServer *Server
	)

	BeforeEach(func() {
		uaaServer = NewServer()

		server.RouteToHandler("GET", "/info",
			RespondWith(http.StatusOK, `{
				"app":{"name":"CredHub"},
				"auth-server":{"url":"`+authServer.URL()+`"}
				}`),
		)
		server.RouteToHandler("GET", "/version",
			RespondWith(http.StatusOK, fmt.Sprintf(`{"version":"%s"}`, versionTwo)))

		authServer.RouteToHandler("GET", "/info", RespondWith(http.StatusOK, ""))
	})

	AfterEach(func() {
		config.RemoveConfig()
	})

	Describe("with mixed password and client parameters", func() {
		Context("with a client name and username", func() {
			It("fails with an error message", func() {
				session := runCommand("login", "--client-name", "test_client", "--username", "test-username")

				Expect(uaaServer.ReceivedRequests()).Should(HaveLen(0))
				Eventually(session).Should(Exit(1))
				Eventually(session.Err).Should(Say("Client, password, SSO and/or SSO passcode credentials may not be combined. Please update and retry your request with a single login method."))
			})
		})

		Context("with a client secret and username", func() {
			It("fails with an error message", func() {
				session := runCommand("login", "--client-secret", "test_secret", "--username", "test-username")

				Expect(uaaServer.ReceivedRequests()).Should(HaveLen(0))
				Eventually(session).Should(Exit(1))
				Eventually(session.Err).Should(Say("Client, password, SSO and/or SSO passcode credentials may not be combined. Please update and retry your request with a single login method."))
			})
		})

		Context("with a client name and password", func() {
			It("fails with an error message", func() {
				session := runCommand("login", "--client-name", "test_client", "--password", "test-password")

				Expect(uaaServer.ReceivedRequests()).Should(HaveLen(0))
				Eventually(session).Should(Exit(1))
				Eventually(session.Err).Should(Say("Client, password, SSO and/or SSO passcode credentials may not be combined. Please update and retry your request with a single login method."))
			})
		})

		Context("with a client secret and password", func() {
			It("fails with an error message", func() {
				session := runCommand("login", "--client-secret", "test_secret", "--password", "test-password")

				Expect(uaaServer.ReceivedRequests()).Should(HaveLen(0))
				Eventually(session).Should(Exit(1))
				Eventually(session.Err).Should(Say("Client, password, SSO and/or SSO passcode credentials may not be combined. Please update and retry your request with a single login method."))
			})
		})

		Context("with SSO and password", func() {
			It("fails with an error message", func() {
				session := runCommand("login", "--sso", "--password", "test-password")

				Expect(uaaServer.ReceivedRequests()).Should(HaveLen(0))
				Eventually(session).Should(Exit(1))
				Eventually(session.Err).Should(Say("Client, password, SSO and/or SSO passcode credentials may not be combined. Please update and retry your request with a single login method."))
			})
		})

		Context("with all parameters from both", func() {
			It("fails with an error message", func() {
				session := runCommand("login", "--client-name", "test_client", "--client-secret", "test_secret", "--username", "test-username", "--password", "test-password")

				Expect(uaaServer.ReceivedRequests()).Should(HaveLen(0))
				Eventually(session).Should(Exit(1))
				Eventually(session.Err).Should(Say("Client, password, SSO and/or SSO passcode credentials may not be combined. Please update and retry your request with a single login method."))
			})
		})
	})

	Describe("password flow", func() {
		BeforeEach(func() {
			uaaServer.RouteToHandler("POST", "/oauth/token",
				CombineHandlers(
					VerifyBody([]byte(`client_id=`+config.AuthClient+`&client_secret=`+config.AuthPassword+`&grant_type=password&password=pass&response_type=token&username=user`)),
					RespondWith(http.StatusOK, `{
						"access_token":"2YotnFZFEjr1zCsicMWpAA",
						"refresh_token":"erousflkajqwer",
						"token_type":"bearer",
						"expires_in":3600}`),
				),
			)

			setConfigAuthUrl(uaaServer.URL())
		})

		Context("with a username and a password", func() {
			It("authenticates with the UAA server and saves a token", func() {
				session := runCommand("login", "-u", "user", "-p", "pass")

				Expect(uaaServer.ReceivedRequests()).Should(HaveLen(1))
				Eventually(session).Should(Exit(0))
				Eventually(session.Out).Should(Say("Login Successful"))
				Eventually(session.Out.Contents()).ShouldNot(ContainSubstring("Setting the target url:"))
				cfg := config.ReadConfig()
				Expect(cfg.AccessToken).To(Equal("2YotnFZFEjr1zCsicMWpAA"))
			})
		})

		Context("with a username and no password", func() {
			It("prompts for a password", func() {
				session := runCommandWithStdin(strings.NewReader("pass\n"), "login", "-u", "user")
				Eventually(session.Out).Should(Say("password:"))
				Eventually(session.Wait("10s").Out).Should(Say("Login Successful"))
				Eventually(session).Should(Exit(0))
				cfg := config.ReadConfig()
				Expect(cfg.AccessToken).To(Equal("2YotnFZFEjr1zCsicMWpAA"))
			})
		})

		Context("with a password and no username", func() {
			It("fails authentication with an error message", func() {
				session := runCommand("login", "-p", "pass")

				Eventually(session).Should(Exit(1))
				Eventually(session.Err).Should(Say("The combination of parameters in the request is not allowed. Please validate your input and retry your request."))
			})
		})

		Context("with neither a username nor a password", func() {
			It("prompts for a username and password", func() {
				// TODO:  devise an input which echoes the input characters for the user name, much as gopass.GetPasswdMasked()
				// echoes '*', for that we may regression-test the echoing of the username
				setConfigAuthUrl(uaaServer.URL())
				session := runCommandWithStdin(strings.NewReader("user\npass\n"), "login")
				Eventually(session.Out).Should(Say("username:"))
				Eventually(session.Out).Should(Say(`password: \*\*\*\*`))
				Eventually(session.Wait("10s").Out).Should(Say("Login Successful"))
				Eventually(session).Should(Exit(0))
			})
		})
	})

	Describe("client flow", func() {
		BeforeEach(func() {
			uaaServer.RouteToHandler("POST", "/oauth/token",
				CombineHandlers(
					VerifyBody([]byte(`client_id=test_client&client_secret=test_secret&grant_type=client_credentials&response_type=token`)),
					RespondWith(http.StatusOK, `{
						"access_token":"2YotnFZFEjr1zCsicMWpAA",
						"refresh_token":"erousflkajqwer",
						"token_type":"bearer",
						"expires_in":3600}`),
				),
			)

			setConfigAuthUrl(uaaServer.URL())
		})

		Context("with the client name and secret from the CLI", func() {
			It("authenticates with the UAA server and saves a token", func() {
				session := runCommand("login", "--client-name", "test_client", "--client-secret", "test_secret")

				Expect(uaaServer.ReceivedRequests()).Should(HaveLen(1))
				Eventually(session).Should(Exit(0))
				Eventually(session.Out).Should(Say("Login Successful"))
				Eventually(session.Out.Contents()).ShouldNot(ContainSubstring("Setting the target url:"))
				cfg := config.ReadConfig()
				Expect(cfg.AccessToken).To(Equal("2YotnFZFEjr1zCsicMWpAA"))
			})
		})

		Context("with the client name and secret from the env", func() {
			It("authenticates with the UAA server and does not save the access token", func() {
				session := runCommandWithEnv([]string{"CREDHUB_CLIENT=test_client", "CREDHUB_SECRET=test_secret"}, "login")

				Expect(uaaServer.ReceivedRequests()).Should(HaveLen(2))
				Eventually(session).Should(Exit(0))
				Eventually(session.Out).Should(Say("Login Successful"))
				Eventually(session.Out.Contents()).ShouldNot(ContainSubstring("Setting the target url:"))
				cfg := config.ReadConfig()
				Expect(cfg.AccessToken).To(Equal(""))
			})
		})

		Context("with the client name from the env and client secret from the CLI", func() {
			It("authenticates with the UAA server and saves a token", func() {
				session := runCommandWithEnv([]string{"CREDHUB_CLIENT=test_client"}, "login", "--client-secret", "test_secret")

				Expect(uaaServer.ReceivedRequests()).Should(HaveLen(1))
				Eventually(session).Should(Exit(0))
				Eventually(session.Out).Should(Say("Login Successful"))
				Eventually(session.Out.Contents()).ShouldNot(ContainSubstring("Setting the target url:"))
				cfg := config.ReadConfig()
				Expect(cfg.AccessToken).To(Equal("2YotnFZFEjr1zCsicMWpAA"))
			})
		})

		Context("with the client name from the CLI and secret from the env", func() {
			It("authenticates with the UAA server and saves a token", func() {
				session := runCommandWithEnv([]string{"CREDHUB_SECRET=test_secret"}, "login", "--client-name", "test_client")

				Expect(uaaServer.ReceivedRequests()).Should(HaveLen(1))
				Eventually(session).Should(Exit(0))
				Eventually(session.Out).Should(Say("Login Successful"))
				Eventually(session.Out.Contents()).ShouldNot(ContainSubstring("Setting the target url:"))
				cfg := config.ReadConfig()
				Expect(cfg.AccessToken).To(Equal("2YotnFZFEjr1zCsicMWpAA"))
			})
		})

		Context("with the client name from the CLI and no client secret", func() {
			It("fails with an error message", func() {
				session := runCommand("login", "--client-name", "test_client")

				Expect(uaaServer.ReceivedRequests()).Should(HaveLen(0))
				Eventually(session).Should(Exit(1))
				Eventually(session.Err).Should(Say("Both client name and client secret must be provided to authenticate. Please update and retry your request."))
			})
		})

		Context("with the client name from the environment and no client secret", func() {
			It("fails with an error message", func() {
				session := runCommandWithEnv([]string{"CREDHUB_CLIENT=test_client"}, "login")

				Expect(uaaServer.ReceivedRequests()).Should(HaveLen(0))
				Eventually(session).Should(Exit(1))
				Eventually(session.Err).Should(Say("Both client name and client secret must be provided to authenticate. Please update and retry your request."))
			})
		})

		Context("with the client secret from the CLI and no client name", func() {
			It("fails with an error message", func() {
				session := runCommand("login", "--client-secret", "test_secret")

				Expect(uaaServer.ReceivedRequests()).Should(HaveLen(0))
				Eventually(session).Should(Exit(1))
				Eventually(session.Err).Should(Say("Both client name and client secret must be provided to authenticate. Please update and retry your request."))
			})
		})

		Context("with the client secret from the environment and no client name", func() {
			It("fails with an error message", func() {
				session := runCommandWithEnv([]string{"CREDHUB_SECRET=test_secret"}, "login")

				Expect(uaaServer.ReceivedRequests()).Should(HaveLen(0))
				Eventually(session).Should(Exit(1))
				Eventually(session.Err).Should(Say("Both client name and client secret must be provided to authenticate. Please update and retry your request."))
			})
		})
	})

	Describe("sso flow", func() {
		BeforeEach(func() {
			uaaServer.RouteToHandler("POST", "/oauth/token",
				CombineHandlers(
					VerifyBody([]byte(`client_id=`+config.AuthClient+`&client_secret=`+config.AuthPassword+`&grant_type=password&passcode=passcode&response_type=token`)),
					RespondWith(http.StatusOK, `{
						"access_token":"2YotnFZFEjr1zCsicMWpAA",
						"refresh_token":"erousflkajqwer",
						"token_type":"bearer",
						"expires_in":3600}`),
				),
			)
			uaaServer.RouteToHandler("GET", "/info",
				CombineHandlers(
					RespondWith(http.StatusOK, `{
						"prompts": {
							"passcode": ["password", "foobar"]
						}
					}`),
				),
			)

			setConfigAuthUrl(uaaServer.URL())
		})

		Context("with a passcode", func() {
			It("authenticates with the UAA server and saves a token", func() {
				session := runCommand("login", "--sso-passcode", "passcode")

				Expect(uaaServer.ReceivedRequests()).Should(HaveLen(1))
				Eventually(session).Should(Exit(0))
				Eventually(session.Out).Should(Say("Login Successful"))
				Eventually(session.Out.Contents()).ShouldNot(ContainSubstring("Setting the target url:"))
				cfg := config.ReadConfig()
				Expect(cfg.AccessToken).To(Equal("2YotnFZFEjr1zCsicMWpAA"))
			})
		})

		Context("prompting for passcode", func() {
			It("prompts for a passcode", func() {
				session := runCommandWithStdin(strings.NewReader("passcode\n"), "login", "--sso")
				Eventually(session.Out).Should(Say("foobar :"))
				Eventually(session.Wait("10s").Out).Should(Say("Login Successful"))
				Eventually(session).Should(Exit(0))
				cfg := config.ReadConfig()
				Expect(cfg.AccessToken).To(Equal("2YotnFZFEjr1zCsicMWpAA"))
			})
		})

		Context("with both specified", func() {
			It("fails with an error message", func() {
				session := runCommand("login", "--sso", "--sso-passcode", "passcode")

				Eventually(session).Should(Exit(1))
				Eventually(session.Err).Should(Say("Client, password, SSO and/or SSO passcode credentials may not be combined. Please update and retry your request with a single login method."))
			})
		})
	})

	Describe("sso flow with server that doesn't give prompt", func() {
		BeforeEach(func() {
			uaaServer.RouteToHandler("POST", "/oauth/token",
				CombineHandlers(
					VerifyBody([]byte(`client_id=`+config.AuthClient+`&client_secret=`+config.AuthPassword+`&grant_type=password&passcode=passcode&response_type=token`)),
					RespondWith(http.StatusOK, `{
						"access_token":"2YotnFZFEjr1zCsicMWpAA",
						"refresh_token":"erousflkajqwer",
						"token_type":"bearer",
						"expires_in":3600}`),
				),
			)
			uaaServer.RouteToHandler("GET", "/info",
				CombineHandlers(
					RespondWith(http.StatusOK, `{}`),
				),
			)

			setConfigAuthUrl(uaaServer.URL())
		})

		Context("prompting for passcode", func() {
			It("prompts for a passcode", func() {
				session := runCommandWithStdin(strings.NewReader("passcode\n"), "login", "--sso")
				Eventually(session.Out).Should(Say("One Time Code \\( Get one at https://login.system.example.com/passcode \\) :"))
				Eventually(session.Wait("10s").Out).Should(Say("Login Successful"))
				Eventually(session).Should(Exit(0))
				cfg := config.ReadConfig()
				Expect(cfg.AccessToken).To(Equal("2YotnFZFEjr1zCsicMWpAA"))
			})
		})
	})

	Context("when logging in with server api target", func() {
		var (
			uaaServer *Server
			apiServer *Server
		)

		BeforeEach(func() {
			uaaServer = NewServer()
			uaaServer.RouteToHandler("POST", "/oauth/token",
				CombineHandlers(
					VerifyBody([]byte(`client_id=`+config.AuthClient+`&client_secret=`+config.AuthPassword+`&grant_type=password&password=pass&response_type=token&username=user`)),
					RespondWith(http.StatusOK, `{
						"access_token":"2YotnFZFEjr1zCsicMWpAA",
						"refresh_token":"erousflkajqwer",
						"token_type":"bearer",
						"expires_in":3600}`),
				),
			)

			uaaServer.RouteToHandler("GET", "/info", RespondWith(http.StatusOK, ""))

			apiServer = NewServer()
			setupServer(apiServer, uaaServer.URL())
		})

		AfterEach(func() {
			apiServer.Close()
			uaaServer.Close()
		})

		It("sets the target to the server's url and auth server url", func() {
			session := runCommand("login", "-u", "user", "-p", "pass", "-s", apiServer.URL())

			Expect(apiServer.ReceivedRequests()).Should(HaveLen(3))
			Expect(uaaServer.ReceivedRequests()).Should(HaveLen(2))
			Eventually(session).Should(Exit(0))
			Eventually(session.Out).Should(Say("Login Successful"))
			cfg := config.ReadConfig()
			Expect(cfg.ApiURL).To(Equal(apiServer.URL()))
			Expect(cfg.AuthURL).To(Equal(uaaServer.URL()))
			Expect(cfg.ServerVersion).To(Equal(versionTwo))
		})

		Context("when the provided server url does not have a scheme specified", func() {
			It("sets a default scheme", func() {
				server := NewTLSServer()
				server.RouteToHandler("GET", "/info", RespondWith(http.StatusOK, `{
						"app":{"name":"CredHub"},
						"auth-server":{"url":"`+uaaServer.URL()+`"}
						}`),
				)
				server.RouteToHandler("GET", "/version",
					RespondWith(http.StatusOK, fmt.Sprintf(`{"version":"%s"}`, versionTwo)))

				session := runCommand("login", "-u", "user", "-p", "pass", "-s", server.Addr(), "--skip-tls-validation")

				Eventually(session).Should(Exit(0))
			})
		})

		It("saves caCert to config when it is provided", func() {
			testCa, _ := ioutil.ReadFile("../test/server-tls-ca.pem")
			session := runCommand("login", "-u", "user", "-p", "pass", "-s", apiServer.URL(), "--ca-cert", "../test/server-tls-ca.pem")

			Expect(session).Should(Exit(0))
			cfg := config.ReadConfig()

			Expect(cfg.CaCerts).Should(Equal([]string{string(testCa)}))
		})

		It("accepts the ca cert through the environment", func() {
			authServer.Close()

			authServer = NewTlsServer("../test/server-tls-cert.pem", "../test/server-tls-key.pem")
			SetupServers(server, authServer)

			authServer.RouteToHandler("POST", "/oauth/token",
				CombineHandlers(
					VerifyBody([]byte(`client_id=`+config.AuthClient+`&client_secret=`+config.AuthPassword+`&grant_type=password&password=pass&response_type=token&username=user`)),
					RespondWith(http.StatusOK, `{
						"access_token":"2YotnFZFEjr1zCsicMWpAA",
						"refresh_token":"erousflkajqwer",
						"token_type":"bearer",
						"expires_in":3600}`),
				),
			)

			serverCa, err := ioutil.ReadFile("../test/server-tls-ca.pem")
			Expect(err).To(BeNil())

			session := runCommandWithEnv([]string{"CREDHUB_CA_CERT=../test/server-tls-ca.pem"}, "login", "-s", server.URL(), "-u", "user", "-p", "pass")
			Eventually(session).Should(Exit(0))

			cfg := config.ReadConfig()
			Expect(cfg.CaCerts).To(ConsistOf([]string{string(serverCa)}))
		})

		Context("when the user skips TLS validation", func() {

			It("prints warning and deprecation notice when --skip-tls-validation flag is present", func() {
				apiServer.Close()
				apiServer = NewTLSServer()
				setupServer(apiServer, uaaServer.URL())
				session := runCommand("login", "-s", apiServer.URL(), "-u", "user", "-p", "pass", "--skip-tls-validation")

				Eventually(session).Should(Exit(0))
				Eventually(session.Out).Should(Say("Warning: The targeted TLS certificate has not been verified for this connection."))
				Eventually(session.Out).Should(Say("Warning: The --skip-tls-validation flag is deprecated. Please use --ca-cert instead."))
			})

			It("sets skip-tls flag in the config file", func() {
				apiServer.Close()
				apiServer = NewTLSServer()
				setupServer(apiServer, uaaServer.URL())
				session := runCommand("login", "-s", apiServer.URL(), "-u", "user", "-p", "pass", "--skip-tls-validation")

				Eventually(session).Should(Exit(0))
				cfg := config.ReadConfig()
				Expect(cfg.InsecureSkipVerify).To(Equal(true))
			})

			It("resets skip-tls flag in the config file", func() {
				cfg := config.ReadConfig()
				cfg.InsecureSkipVerify = true
				err := config.WriteConfig(cfg)
				Expect(err).NotTo(HaveOccurred())

				session := runCommand("login", "-s", apiServer.URL(), "-u", "user", "-p", "pass")

				Eventually(session).Should(Exit(0))
				cfg = config.ReadConfig()
				Expect(cfg.InsecureSkipVerify).To(Equal(false))
			})

			It("using a TLS server without the skip-tls flag set will fail on certificate verification", func() {
				apiServer.Close()
				apiServer = NewTLSServer()
				setupServer(apiServer, uaaServer.URL())
				session := runCommand("login", "-s", apiServer.URL(), "-u", "user", "-p", "pass")

				Eventually(session).Should(Exit(1))
				Eventually(session.Err).Should(Say("Error connecting to the targeted API"))
			})

			It("using a TLS server with the skip-tls flag set will succeed", func() {
				apiServer.Close()
				apiServer = NewTLSServer()
				setupServer(apiServer, uaaServer.URL())
				session := runCommand("login", "-s", apiServer.URL(), "-u", "user", "-p", "pass", "--skip-tls-validation")

				Eventually(session).Should(Exit(0))
			})

			It("records skip-tls into config file even with http URLs (will do nothing with that value)", func() {
				session := runCommand("login", "-s", apiServer.URL(), "-u", "user", "-p", "pass", "--skip-tls-validation")
				cfg := config.ReadConfig()

				Eventually(session).Should(Exit(0))
				Expect(cfg.InsecureSkipVerify).To(Equal(true))
			})
		})

		It("saves the oauth tokens", func() {
			runCommand("login", "-u", "user", "-p", "pass", "-s", apiServer.URL())

			cfg := config.ReadConfig()
			Expect(cfg.AccessToken).To(Equal("2YotnFZFEjr1zCsicMWpAA"))
			Expect(cfg.RefreshToken).To(Equal("erousflkajqwer"))
		})

		It("returns an error if no cert is valid for CredHub", func() {
			previousCfg := config.ReadConfig()
			session := runCommand("login", "-s", server.URL(), "-u", "user", "-p", "pass", "--ca-cert", "../test/auth-tls-ca.pem")

			Eventually(session).Should(Exit(1))
			Eventually(session.Err).Should(Say("certificate signed by unknown authority"))

			cfg := config.ReadConfig()
			Expect(cfg.CaCerts).To(Equal(previousCfg.CaCerts))
		})

		It("returns an error if no cert is valid for the auth server", func() {
			previousCfg := config.ReadConfig()
			session := runCommand("login", "-s", server.URL(), "-u", "user", "-p", "pass", "--ca-cert", "../test/server-tls-ca.pem")

			Eventually(session).Should(Exit(1))
			Eventually(session.Err).Should(Say("certificate signed by unknown authority"))

			cfg := config.ReadConfig()
			Expect(cfg.CaCerts).To(Equal(previousCfg.CaCerts))
		})

		It("accepts the API URL from the environment", func() {
			authServer.RouteToHandler("POST", "/oauth/token",
				CombineHandlers(
					VerifyBody([]byte(`client_id=`+config.AuthClient+`&client_secret=`+config.AuthPassword+`&grant_type=password&password=pass&response_type=token&username=user`)),
					RespondWith(http.StatusOK, `{
						"access_token":"2YotnFZFEjr1zCsicMWpAA",
						"refresh_token":"erousflkajqwer",
						"token_type":"bearer",
						"expires_in":3600}`),
				),
			)

			config.RemoveConfig()

			session := runCommandWithEnv([]string{"CREDHUB_SERVER=" + server.URL()}, "login", "-u", "user", "-p", "pass", "--ca-cert", "../test/server-tls-ca.pem", "--ca-cert", "../test/auth-tls-ca.pem")

			Eventually(session).Should(Exit(0))

			cfg := config.ReadConfig()
			Expect(cfg.ApiURL).To(Equal(server.URL()))
		})

		Context("when api server is unavailable", func() {
			var (
				badServer *Server
			)

			BeforeEach(func() {
				badServer = NewServer()
				badServer.RouteToHandler("GET", "/info", RespondWith(http.StatusBadGateway, nil))
			})

			It("should not login", func() {
				session := runCommand("login", "-u", "user", "-p", "pass", "-s", badServer.URL())

				Eventually(session).Should(Exit(1))
				Eventually(session.Err).Should(Say("Error connecting to the targeted API"))
				Expect(uaaServer.ReceivedRequests()).Should(HaveLen(0))
			})

			It("should not override config's existing API URL value", func() {
				cfg := config.ReadConfig()
				cfg.ApiURL = "foo"
				config.WriteConfig(cfg)

				session := runCommand("login", "-u", "user", "-p", "pass", "-s", badServer.URL())

				Eventually(session).Should(Exit(1))
				Eventually(session.Err).Should(Say("Error connecting to the targeted API"))
				Expect(uaaServer.ReceivedRequests()).Should(HaveLen(0))
				cfg2 := config.ReadConfig()
				Expect(cfg2.ApiURL).To(Equal("foo"))
			})
		})

		Context("when UAA client returns an error", func() {
			Context("when client credentials are invalid", func() {
				var (
					apiServer    *Server
					badUaaServer *Server
					session      *Session
				)

				BeforeEach(func() {
					badUaaServer = NewServer()
					badUaaServer.RouteToHandler("POST", "/oauth/token",
						CombineHandlers(
							VerifyBody([]byte(`client_id=`+config.AuthClient+`&client_secret=`+config.AuthPassword+`&grant_type=password&password=pass&response_type=token&username=user`)),
							RespondWith(http.StatusUnauthorized, `{
						"error":"unauthorized",
						"error_description":"Bad credentials"
						}`),
						))
					badUaaServer.RouteToHandler("DELETE", "/oauth/token/revoke/"+VALID_ACCESS_TOKEN_JTI,
						RespondWith(http.StatusOK, ""),
					)
					badUaaServer.RouteToHandler("GET", "/info", RespondWith(http.StatusOK, ""))

					apiServer = NewServer()
					setupServer(apiServer, badUaaServer.URL())

					cfg := config.ReadConfig()
					cfg.AuthURL = badUaaServer.URL()
					cfg.AccessToken = VALID_ACCESS_TOKEN
					config.WriteConfig(cfg)
				})

				It("fails to login", func() {
					session = runCommand("login", "-u", "user", "-p", "pass")
					Eventually(session).Should(Exit(1))
					Eventually(session.Err).Should(Say("UAA error: unauthorized Bad credentials"))
					Expect(badUaaServer.ReceivedRequests()).Should(HaveLen(2))
				})

				It("revokes any existing tokens", func() {
					session = runCommand("login", "-u", "user", "-p", "pass")
					Eventually(session).Should(Exit(1))
					cfg := config.ReadConfig()
					Expect(cfg.AccessToken).To(Equal("revoked"))
					Expect(cfg.RefreshToken).To(Equal("revoked"))
					Expect(badUaaServer.ReceivedRequests()).Should(HaveLen(2))
				})

				It("doesn't print 'Setting the target url' message with -s flag", func() {
					session = runCommand("login", "-u", "user", "-p", "pass", "-s", apiServer.URL())
					Eventually(session).Should(Exit(1))
					Expect(session.Out).NotTo(Say("Setting the target url: " + apiServer.URL()))
				})
			})

			Context("when client credentials do not have proper grant type", func() {
				var (
					apiServer    *Server
					badUaaServer *Server
					session      *Session
				)

				BeforeEach(func() {
					badUaaServer = NewServer()
					badUaaServer.RouteToHandler("POST", "/oauth/token",
						CombineHandlers(
							VerifyBody([]byte(`client_id=`+config.AuthClient+`&client_secret=`+config.AuthPassword+`&grant_type=password&password=pass&response_type=token&username=user`)),
							RespondWith(http.StatusUnauthorized, `{
						"error":"invalid_client",
						"error_description":"Unauthorized grant type: password"
						}`),
						))
					badUaaServer.RouteToHandler("DELETE", "/oauth/token/revoke/"+VALID_ACCESS_TOKEN_JTI,
						RespondWith(http.StatusOK, ""),
					)
					badUaaServer.RouteToHandler("GET", "/info", RespondWith(http.StatusOK, ""))

					apiServer = NewServer()
					setupServer(apiServer, badUaaServer.URL())

					cfg := config.ReadConfig()
					cfg.AuthURL = badUaaServer.URL()
					cfg.AccessToken = VALID_ACCESS_TOKEN
					config.WriteConfig(cfg)
				})

				It("fails to login", func() {
					session = runCommand("login", "-u", "user", "-p", "pass")
					Eventually(session).Should(Exit(1))
					Eventually(session.Err).Should(Say("UAA error: invalid_client Unauthorized grant type: password"))
					Expect(badUaaServer.ReceivedRequests()).Should(HaveLen(2))
				})

				It("revokes any existing tokens", func() {
					session = runCommand("login", "-u", "user", "-p", "pass")
					Eventually(session).Should(Exit(1))
					cfg := config.ReadConfig()
					Expect(cfg.AccessToken).To(Equal("revoked"))
					Expect(cfg.RefreshToken).To(Equal("revoked"))
					Expect(badUaaServer.ReceivedRequests()).Should(HaveLen(2))
				})

				It("doesn't print 'Setting the target url' message with -s flag", func() {
					session = runCommand("login", "-u", "user", "-p", "pass", "-s", apiServer.URL())
					Eventually(session).Should(Exit(1))
					Expect(session.Out).NotTo(Say("Setting the target url: " + apiServer.URL()))
				})
			})

			Context("when uaa token endpoint 500s", func() {
				var (
					apiServer    *Server
					badUaaServer *Server
					session      *Session
				)

				BeforeEach(func() {
					badUaaServer = NewServer()
					badUaaServer.RouteToHandler("POST", "/oauth/token",
						CombineHandlers(
							VerifyBody([]byte(`client_id=`+config.AuthClient+`&client_secret=`+config.AuthPassword+`&grant_type=password&password=pass&response_type=token&username=user`)),
							RespondWith(http.StatusInternalServerError, ``),
						))
					badUaaServer.RouteToHandler("DELETE", "/oauth/token/revoke/"+VALID_ACCESS_TOKEN_JTI,
						RespondWith(http.StatusOK, ""),
					)
					badUaaServer.RouteToHandler("GET", "/info", RespondWith(http.StatusOK, ""))

					apiServer = NewServer()
					setupServer(apiServer, badUaaServer.URL())

					cfg := config.ReadConfig()
					cfg.AuthURL = badUaaServer.URL()
					cfg.AccessToken = VALID_ACCESS_TOKEN
					config.WriteConfig(cfg)
				})

				It("fails to login", func() {
					session = runCommand("login", "-u", "user", "-p", "pass")
					Eventually(session).Should(Exit(1))
					Eventually(session.Err).Should(Say("UAA error: EOF"))
					Expect(badUaaServer.ReceivedRequests()).Should(HaveLen(2))
				})

				It("revokes any existing tokens", func() {
					session = runCommand("login", "-u", "user", "-p", "pass")
					Eventually(session).Should(Exit(1))
					cfg := config.ReadConfig()
					Expect(cfg.AccessToken).To(Equal("revoked"))
					Expect(cfg.RefreshToken).To(Equal("revoked"))
					Expect(badUaaServer.ReceivedRequests()).Should(HaveLen(2))
				})

				It("doesn't print 'Setting the target url' message with -s flag", func() {
					session = runCommand("login", "-u", "user", "-p", "pass", "-s", apiServer.URL())
					Eventually(session).Should(Exit(1))
					Expect(session.Out).NotTo(Say("Setting the target url: " + apiServer.URL()))
				})
			})

			Context("when uaa info endpoint 500s", func() {
				var (
					apiServer    *Server
					badUaaServer *Server
					session      *Session
				)

				BeforeEach(func() {
					badUaaServer = NewServer()
					badUaaServer.RouteToHandler("POST", "/oauth/token",
						CombineHandlers(
							VerifyBody([]byte(`client_id=`+config.AuthClient+`&client_secret=`+config.AuthPassword+`&grant_type=password&password=pass&response_type=token&username=user`)),
							RespondWith(http.StatusOK, `{
						"access_token":"2YotnFZFEjr1zCsicMWpAA",
						"refresh_token":"erousflkajqwer",
						"token_type":"bearer",
						"expires_in":3600}`),
						))
					badUaaServer.RouteToHandler("DELETE", "/oauth/token/revoke/"+VALID_ACCESS_TOKEN_JTI,
						RespondWith(http.StatusOK, ""),
					)
					badUaaServer.RouteToHandler("GET", "/info", RespondWith(http.StatusInternalServerError, ""))
					apiServer = NewServer()
					setupServer(apiServer, badUaaServer.URL())

					cfg := config.ReadConfig()
					cfg.AuthURL = badUaaServer.URL()
					cfg.AccessToken = VALID_ACCESS_TOKEN
					config.WriteConfig(cfg)
				})

				It("fails to login", func() {
					session = runCommand("login", "--sso")
					Eventually(session).Should(Exit(1))
					Eventually(session.Err).Should(Say("UAA error: unable to fetch metadata successfully"))
					Expect(badUaaServer.ReceivedRequests()).Should(HaveLen(2))
				})

				It("doesn't print 'Setting the target url' message with -s flag", func() {
					session = runCommand("login", "--sso", "-s", apiServer.URL())
					Eventually(session).Should(Exit(1))
					Expect(session.Out).NotTo(Say("Setting the target url: " + apiServer.URL()))
				})
			})
		})
	})

	Describe("when logging in without server api target", func() {
		var (
			apiUrl string
		)

		BeforeEach(func() {
			cfg := config.ReadConfig()
			apiUrl = cfg.ApiURL
			cfg.ApiURL = ""
			config.WriteConfig(cfg)
		})

		AfterEach(func() {
			cfg := config.ReadConfig()
			cfg.ApiURL = apiUrl
			config.WriteConfig(cfg)
		})

		Context("with no user or password flags", func() {
			It("returns an error message", func() {
				session := runCommand("login")

				Eventually(session).Should(Exit(1))
				Eventually(session.Err).Should(Say("An API target is not set. Please target the location of your server with `credhub api --server api.example.com` to continue."))
			})
		})

		Context("with user and password flags", func() {
			It("returns an error message", func() {
				session := runCommand("login", "-u", "user", "-p", "pass")

				Eventually(session).Should(Exit(1))
				Eventually(session.Err).Should(Say("An API target is not set. Please target the location of your server with `credhub api --server api.example.com` to continue."))
			})
		})
	})

	Describe("Help", func() {
		ItBehavesLikeHelp("login", "l", func(session *Session) {
			Expect(session.Err).To(Say("login"))
			Expect(session.Err).To(Say("username"))
			Expect(session.Err).To(Say("password"))
			Expect(session.Err).To(Say("client-name"))
			Expect(session.Err).To(Say("client-secret"))
		})
	})
})

func setConfigAuthUrl(authUrl string) {
	cfg := config.ReadConfig()
	cfg.AuthURL = authUrl
	config.WriteConfig(cfg)
}

func setupServer(theServer *Server, uaaUrl string) {
	theServer.RouteToHandler("GET", "/info",
		RespondWith(http.StatusOK, fmt.Sprintf(`{
					"app":{"name":"CredHub"},
					"auth-server":{"url":"%s"}
					}`, uaaUrl)),
	)
	theServer.RouteToHandler("GET", "/version",
		RespondWith(http.StatusOK, fmt.Sprintf(`{"version":"%s"}`, versionTwo)),
	)
}
