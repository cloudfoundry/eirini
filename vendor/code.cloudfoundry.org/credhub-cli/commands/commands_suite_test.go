package commands_test

import (
	"crypto/tls"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gbytes"
	. "github.com/onsi/gomega/gexec"
	. "github.com/onsi/gomega/ghttp"

	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"os/exec"
	"runtime"
	"testing"

	"path/filepath"

	"code.cloudfoundry.org/credhub-cli/config"
	test_util "code.cloudfoundry.org/credhub-cli/test"
)

const TIMESTAMP = `2016-01-01T12:00:00Z`
const UUID = `5a2edd4f-1686-4c8d-80eb-5daa866f9f86`

const VALID_ACCESS_TOKEN = "eyJhbGciOiJSUzI1NiIsImtpZCI6ImxlZ2FjeS10b2tlbi1rZXkiLCJ0eXAiOiJKV1QifQ.eyJqdGkiOiI3NTY5MTc5OTgzOTY0M2Y4OWI2NGZlNDQ2MWU0OWJlMCIsInN1YiI6IjY3ODdiYjdlLTc4YmItNGJlNi05NTgzLTQyYTc1ZGRiYTNkNSIsInNjb3BlIjpbImNyZWRodWIud3JpdGUiLCJjcmVkaHViLnJlYWQiXSwiY2xpZW50X2lkIjoiY3JlZGh1Yl9jbGkiLCJjaWQiOiJjcmVkaHViX2NsaSIsImF6cCI6ImNyZWRodWJfY2xpIiwicmV2b2NhYmxlIjp0cnVlLCJncmFudF90eXBlIjoicGFzc3dvcmQiLCJ1c2VyX2lkIjoiNjc4N2JiN2UtNzhiYi00YmU2LTk1ODMtNDJhNzVkZGJhM2Q1Iiwib3JpZ2luIjoidWFhIiwidXNlcl9uYW1lIjoiY3JlZGh1YiIsImVtYWlsIjoiY3JlZGh1YiIsImF1dGhfdGltZSI6MTUwNDgyMTU4NSwicmV2X3NpZyI6ImU0Yjg2ODVlIiwiaWF0IjoxNTA0ODIxNTg1LCJleHAiOjE1MDQ5MDc5ODUsImlzcyI6Imh0dHBzOi8vMzQuMjA2LjIzMy4xOTU6ODQ0My9vYXV0aC90b2tlbiIsInppZCI6InVhYSIsImF1ZCI6WyJjcmVkaHViX2NsaSIsImNyZWRodWIiXX0.Ubi5k3Sy4CkcTqKvKuSkLJFpA5zfwWPlhImuwMW3HyKd6iEPuteXqnSE9r6ndvcKf_B3PS0ZduPg7v81RiZyfTGu3ObWIEdYExlmI97yfg4OQMCfo4jdr2wSzpcwixTK2FeZ2RcDklMfaSp_CTAnNcY4Lj2Jlk2eagWOCXizxsB1SHfegtGWH3FSUT5I3nJVcWAsRCMLqjHzRWYdP3CfpnMhnrjic1Ok_f2HKygiG44uUx2u3yQOV1CiZJwhxPODTuhI8X9kkQ0rLW9jW9ADVFstfXOglr-_k6tJMKMNpbXuCd_XaxOIXsxrSdFwcZw56KjuAA4iMuSfMxCbu1UyFw"
const VALID_ACCESS_TOKEN_JTI = "75691799839643f89b64fe4461e49be0"

const STRING_CREDENTIAL_REQUEST_JSON = `{"type":"%s","name":"%s","value":"%s"}`
const JSON_CREDENTIAL_REQUEST_JSON = `{"name":"%s","type":"json","value":%s}`
const CERTIFICATE_CREDENTIAL_REQUEST_JSON = `{"type":"certificate","name":"%s","value":{"ca":"%s","certificate":"%s","private_key":"%s"}}`
const CERTIFICATE_CREDENTIAL_WITH_NAMED_CA_REQUEST_JSON = `{"type":"certificate","name":"%s","value":{"ca_name":"%s","ca":"","certificate":"%s","private_key":"%s"}}`
const GENERATE_CREDENTIAL_REQUEST_JSON = `{"name":"%s","type":"%s","overwrite":%t,"parameters":%s}`
const RSA_SSH_CREDENTIAL_REQUEST_JSON = `{"type":"%s","name":"%s","value":{"public_key":"%s","private_key":"%s"}}`
const GENERATE_DEFAULT_TYPE_REQUEST_JSON = `{"name":"%s","type":"password","overwrite":%t,"parameters":%s}`
const USER_GENERATE_CREDENTIAL_REQUEST_JSON = `{"name":"%s","type":"user","overwrite":%t,"parameters":%s,"value":%s}`
const USER_SET_CREDENTIAL_REQUEST_JSON = `{"type":"user","name":"%s","value":%s}`

const JSON_CREDENTIAL_RESPONSE_JSON = `{"type":"json","id":"` + UUID + `","name":"%s","version_created_at":"` + TIMESTAMP + `","value":%s}`
const STRING_CREDENTIAL_RESPONSE_JSON = `{"type":"%s","id":"` + UUID + `","name":"%s","version_created_at":"` + TIMESTAMP + `","value":"%s"}`
const CERTIFICATE_CREDENTIAL_RESPONSE_JSON = `{"type":"certificate","id":"` + UUID + `","name":"%s","version_created_at":"` + TIMESTAMP + `","value":{"ca":"%s","certificate":"%s","private_key":"%s"}}`
const RSA_CREDENTIAL_RESPONSE_JSON = `{"type":"%s","id":"` + UUID + `","name":"%s","version_created_at":"` + TIMESTAMP + `","value":{"public_key":"%s","private_key":"%s"},"version_created_at":"` + TIMESTAMP + `"}`
const SSH_CREDENTIAL_RESPONSE_JSON = `{"type":"%s","id":"` + UUID + `","name":"%s","version_created_at":"` + TIMESTAMP + `","value":{"public_key":"%s","private_key":"%s", "public_key_fingerprint":"fingerprint"},"version_created_at":"` + TIMESTAMP + `"}`
const USER_CREDENTIAL_RESPONSE_JSON = `{"type":"user","id":"` + UUID + `","name":"%s","version_created_at":"` + TIMESTAMP + `","value":{"username":"%s", "password":"%s", "password_hash":"%s"}}`
const USER_WITHOUT_USERNAME_CREDENTIAL_RESPONSE_JSON = `{"type":"user","id":"` + UUID + `","name":"%s","version_created_at":"` + TIMESTAMP + `","value":{"username":null, "password":"%s", "password_hash":"%s"}}`

const SET_CERTIFICATE_CREDENTIAL_RESPONSE_JSON = `{"type":"certificate","id":"` + UUID + `","name":"%s","version_created_at":"` + TIMESTAMP + `","value": "<redacted>"}`
const SET_RSA_CREDENTIAL_RESPONSE_JSON = `{"type":"%s","id":"` + UUID + `","name":"%s","version_created_at":"` + TIMESTAMP + `","value": "<redacted>","version_created_at":"` + TIMESTAMP + `"}`
const SET_STRING_CREDENTIAL_RESPONSE_JSON = `{"type":"%s","id":"` + UUID + `","name":"%s","version_created_at":"` + TIMESTAMP + `","value": "<redacted>"}`

const STRING_CREDENTIAL_ARRAY_RESPONSE_JSON = `{"data":[` + STRING_CREDENTIAL_RESPONSE_JSON + `]}`
const JSON_CREDENTIAL_ARRAY_RESPONSE_JSON = `{"data":[` + JSON_CREDENTIAL_RESPONSE_JSON + `]}`
const CERTIFICATE_CREDENTIAL_ARRAY_RESPONSE_JSON = `{"data":[` + CERTIFICATE_CREDENTIAL_RESPONSE_JSON + `]}`
const RSA_SSH_CREDENTIAL_ARRAY_RESPONSE_JSON = `{"data":[` + RSA_CREDENTIAL_RESPONSE_JSON + `]}`
const USER_CREDENTIAL_ARRAY_RESPONSE_JSON = `{"data":[` + USER_CREDENTIAL_RESPONSE_JSON + `]}`

var responseMyValuePotatoesJson = fmt.Sprintf(STRING_CREDENTIAL_RESPONSE_JSON, "value", "my-value", "potatoes")
var responseMyPasswordPotatoesJson = fmt.Sprintf(STRING_CREDENTIAL_RESPONSE_JSON, "password", "my-password", "potatoes")
var responseMyCertificateWithNewlinesJson = fmt.Sprintf(CERTIFICATE_CREDENTIAL_RESPONSE_JSON, "my-secret", `my\nca`, `my\ncert`, `my\npriv`)
var responseMyRSAWithNewlinesJson = fmt.Sprintf(RSA_CREDENTIAL_RESPONSE_JSON, "rsa", "foo-rsa-key", `some\npublic\nkey`, `some\nprivate\nkey`)

var responseSetMyCertificateWithNewlinesJson = fmt.Sprintf(SET_CERTIFICATE_CREDENTIAL_RESPONSE_JSON, "my-secret")
var responseSetMyRSAWithNewlinesJson = fmt.Sprintf(SET_RSA_CREDENTIAL_RESPONSE_JSON, "rsa", "foo-rsa-key")
var responseSetMyPasswordPotatoesJson = fmt.Sprintf(SET_STRING_CREDENTIAL_RESPONSE_JSON, "password", "my-password")
var responseSetMyValuePotatoesJson = fmt.Sprintf(SET_STRING_CREDENTIAL_RESPONSE_JSON, "value", "my-value")

func TestCommands(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Commands Suite")
}

var (
	commandPath string
	homeDir     string
	server      *Server
	authServer  *Server
	credhubEnv  map[string]string
)

var _ = BeforeEach(func() {
	var err error
	homeDir, err = ioutil.TempDir("", "cm-test")
	Expect(err).NotTo(HaveOccurred())

	if runtime.GOOS == "windows" {
		os.Setenv("USERPROFILE", homeDir)
	} else {
		os.Setenv("HOME", homeDir)
	}

	server = NewTlsServer("../test/server-tls-cert.pem", "../test/server-tls-key.pem")
	authServer = NewTlsServer("../test/auth-tls-cert.pem", "../test/auth-tls-key.pem")

	SetupServers(server, authServer)

	session := runCommand("api", server.URL(), "--ca-cert", "../test/server-tls-ca.pem", "--ca-cert", "../test/auth-tls-ca.pem")

	server.Reset()
	authServer.Reset()

	Eventually(session).Should(Exit(0))
})

var _ = AfterEach(func() {
	server.Close()
	authServer.Close()
	os.RemoveAll(homeDir)
})

var _ = SynchronizedBeforeSuite(func() []byte {
	executable_path, err := Build("code.cloudfoundry.org/credhub-cli", "-ldflags", "-X code.cloudfoundry.org/credhub-cli/version.Version=test-version")
	Expect(err).NotTo(HaveOccurred())
	return []byte(executable_path)
}, func(data []byte) {
	commandPath = string(data)
	credhubEnv = test_util.UnsetAndCacheCredHubEnvVars()
})

var _ = SynchronizedAfterSuite(func() {}, func() {
	CleanupBuildArtifacts()
	test_util.RestoreEnv(credhubEnv)
})

func login() {
	authServer.AppendHandlers(
		CombineHandlers(
			VerifyRequest("POST", "/oauth/token"),
			RespondWith(http.StatusOK, `{
			"access_token":"test-access-token",
			"refresh_token":"test-refresh-token",
			"token_type":"password",
			"expires_in":123456789
			}`),
		),
	)

	server.RouteToHandler("GET", "/info",
		RespondWith(http.StatusOK, `{
				"app":{"name":"CredHub"}
				}`),
	)

	server.RouteToHandler("GET", "/version",
		RespondWith(http.StatusOK, `{"version":"9.9.9"}`),
	)

	runCommand("login", "-u", "test-username", "-p", "test-password")

	authServer.Reset()
}

func resetCachedServerVersion() {
	cfg := config.ReadConfig()
	cfg.ServerVersion = ""
	config.WriteConfig(cfg)
}

func runCommand(args ...string) *Session {
	cmd := exec.Command(commandPath, args...)
	session, err := Start(cmd, GinkgoWriter, GinkgoWriter)
	Expect(err).NotTo(HaveOccurred())
	<-session.Exited

	return session
}

func runCommandWithEnv(env []string, args ...string) *Session {
	cmd := exec.Command(commandPath, args...)
	existing := os.Environ()
	for _, env_var := range env {
		existing = append(existing, env_var)
	}
	cmd.Env = existing
	session, err := Start(cmd, GinkgoWriter, GinkgoWriter)
	Expect(err).NotTo(HaveOccurred())
	<-session.Exited

	return session
}

func runCommandWithStdin(stdin io.Reader, args ...string) *Session {
	cmd := exec.Command(commandPath, args...)
	cmd.Stdin = stdin
	session, err := Start(cmd, GinkgoWriter, GinkgoWriter)
	Expect(err).NotTo(HaveOccurred())
	<-session.Exited

	return session
}

func setupUAAConfig(uaaResponseStatus int) {
	cfg := config.Config{
		RefreshToken: "5b9c9fd51ba14838ac2e6b222d487106-r",
		AccessToken:  "e30K.eyJqdGkiOiIxIn0K.e30K",
		AuthURL:      authServer.URL(),
		ApiURL:       server.URL(),
	}

	Expect(cfg.UpdateTrustedCAs([]string{"../test/auth-tls-ca.pem", "../test/server-tls-ca.pem"})).To(Succeed())
	Expect(config.WriteConfig(cfg)).To(Succeed())

	authServer.RouteToHandler(
		"DELETE", "/oauth/token/revoke/1",
		RespondWith(uaaResponseStatus, ""),
	)
}

func NewTlsServer(certPath, keyPath string) *Server {
	tlsServer := NewUnstartedServer()

	cert, err := ioutil.ReadFile(certPath)
	Expect(err).To(BeNil())
	key, err := ioutil.ReadFile(keyPath)
	Expect(err).To(BeNil())

	tlsCert, err := tls.X509KeyPair(cert, key)
	Expect(err).To(BeNil())

	tlsServer.HTTPTestServer.TLS = &tls.Config{
		Certificates: []tls.Certificate{tlsCert},
	}

	tlsServer.HTTPTestServer.StartTLS()

	return tlsServer
}

func SetupServers(chServer, uaaServer *Server) {
	chServer.RouteToHandler("GET", "/info",
		RespondWith(http.StatusOK, `{
				"app":{"name":"CredHub"},
				"auth-server":{"url":"`+uaaServer.URL()+`"}
				}`),
	)

	chServer.RouteToHandler("GET", "/version",
		RespondWith(http.StatusOK, `{"version":"9.9.9"}`),
	)

	uaaServer.RouteToHandler("GET", "/info", RespondWith(http.StatusOK, ""))
}

func ItBehavesLikeHelp(command string, alias string, validate func(*Session)) {
	It("displays help", func() {
		session := runCommand(command, "-h")
		Eventually(session).Should(Exit(1))
		validate(session)
	})

	It("displays help using the alias", func() {
		session := runCommand(alias, "-h")
		Eventually(session).Should(Exit(1))
		validate(session)
	})
}

func ItRequiresAuthentication(args ...string) {
	It("requires authentication", func() {
		setupUAAConfig(http.StatusOK)

		runCommand("logout")

		session := runCommand(args...)

		Eventually(session).Should(Exit(1))
		Expect(session.Err).To(Say("You are not currently authenticated. Please log in to continue."))
	})
}

func ItRequiresAnAPIToBeSet(args ...string) {
	Describe("requires an API endpoint", func() {
		BeforeEach(func() {
			cfg := config.ReadConfig()
			cfg.ApiURL = ""
			config.WriteConfig(cfg)
		})

		Context("when using password grant", func() {
			It("requires an API endpoint", func() {
				session := runCommand(args...)

				Eventually(session).Should(Exit(1))
				Expect(session.Err).To(Say("An API target is not set. Please target the location of your server with `credhub api --server api.example.com` to continue."))
			})
		})

		Context("when using client_credentials", func() {
			It("requires an API endpoint", func() {
				session := runCommandWithEnv([]string{"CREDHUB_CLIENT=test_client", "CREDHUB_SECRET=test_secret"}, args...)

				Eventually(session).Should(Exit(1))
				Expect(session.Err).To(Say("An API target is not set. Please target the location of your server with `credhub api --server api.example.com` to continue."))
			})
		})
	})
}

func ItAutomaticallyLogsIn(method string, responseFixtureFile string, endpoint string, args ...string) {
	var serverResponse string
	Describe("automatic authentication", func() {
		BeforeEach(func() {
			buf, _ := ioutil.ReadFile(filepath.Join("testdata", responseFixtureFile))
			serverResponse = string(buf)
		})
		AfterEach(func() {
			server.Reset()
		})

		Context("with correct environment and unauthenticated", func() {
			BeforeEach(func() {
				server.AppendHandlers(
					CombineHandlers(
						VerifyRequest(method, endpoint),
						VerifyHeader(http.Header{
							"Authorization": []string{"Bearer 2YotnFZFEjr1zCsicMWpAA"},
						}),
						RespondWith(http.StatusOK, serverResponse),
					),
				)
			})

			It("automatically authenticates", func() {
				setupUAAConfig(http.StatusOK)

				authServer.AppendHandlers(
					CombineHandlers(
						VerifyRequest("POST", "/oauth/token"),
						VerifyBody([]byte(`client_id=test_client&client_secret=test_secret&grant_type=client_credentials&response_type=token`)),
						RespondWith(http.StatusOK, `{
								"access_token":"2YotnFZFEjr1zCsicMWpAA",
								"token_type":"bearer",
								"expires_in":3600}`),
					),
				)

				runCommand("logout")

				session := runCommandWithEnv([]string{"CREDHUB_CLIENT=test_client", "CREDHUB_SECRET=test_secret"}, args...)

				Eventually(session).Should(Exit(0))
			})
		})

		Context("with correct environment and expired token", func() {
			BeforeEach(func() {
				server.AppendHandlers(
					CombineHandlers(
						VerifyRequest(method, endpoint),
						VerifyHeader(http.Header{
							"Authorization": []string{"Bearer test-access-token"},
						}),
						RespondWith(http.StatusUnauthorized, `{
						"error":"access_token_expired",
						"error_description":"error description"}`),
					),
				)

				authServer.AppendHandlers(
					CombineHandlers(
						VerifyRequest("POST", "/oauth/token"),
						VerifyBody([]byte(`client_id=test_client&client_secret=test_secret&grant_type=client_credentials&response_type=token`)),
						RespondWith(http.StatusOK, `{
								"access_token":"new-token",
								"token_type":"bearer",
								"expires_in":3600}`),
					),
				)

				server.AppendHandlers(
					CombineHandlers(
						VerifyRequest(method, endpoint),
						VerifyHeader(http.Header{
							"Authorization": []string{"Bearer new-token"},
						}),
						RespondWith(http.StatusOK, serverResponse),
					),
				)
			})

			It("automatically authenticates", func() {
				session := runCommandWithEnv([]string{"CREDHUB_CLIENT=test_client", "CREDHUB_SECRET=test_secret"}, args...)
				Eventually(session).Should(Exit(0))
			})
		})
	})
}
