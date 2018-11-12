package commands_test

import (
	"net/http"

	"runtime"

	"fmt"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gbytes"
	. "github.com/onsi/gomega/gexec"
	. "github.com/onsi/gomega/ghttp"
)

var _ = Describe("Get", func() {
	BeforeEach(func() {
		login()
	})

	ItRequiresAuthentication("get", "-n", "test-credential")
	ItRequiresAnAPIToBeSet("get", "-n", "test-credential")
	ItAutomaticallyLogsIn("GET", "get_response.json", "/api/v1/data", "get", "-n", "test-credential")

	ItBehavesLikeHelp("get", "g", func(session *Session) {
		Expect(session.Err).To(Say("Usage"))
		if runtime.GOOS == "windows" {
			Expect(session.Err).To(Say("credhub-cli.exe \\[OPTIONS\\] get \\[get-OPTIONS\\]"))
		} else {
			Expect(session.Err).To(Say("credhub-cli \\[OPTIONS\\] get \\[get-OPTIONS\\]"))
		}
	})

	It("displays missing required parameter", func() {
		session := runCommand("get")

		Eventually(session).Should(Exit(1))

		if runtime.GOOS == "windows" {
			Expect(session.Err).To(Say("A name or ID must be provided. Please update and retry your request."))
		} else {
			Expect(session.Err).To(Say("A name or ID must be provided. Please update and retry your request."))
		}
	})

	It("gets a value secret", func() {
		responseJson := fmt.Sprintf(STRING_CREDENTIAL_ARRAY_RESPONSE_JSON, "value", "my-value", "potatoes")

		server.RouteToHandler("GET", "/api/v1/data",
			CombineHandlers(
				VerifyRequest("GET", "/api/v1/data", "current=true&name=my-value"),
				RespondWith(http.StatusOK, responseJson),
			),
		)

		session := runCommand("get", "-n", "my-value")

		Eventually(session).Should(Exit(0))
		Eventually(session.Out).Should(Say("name: my-value"))
		Eventually(session.Out).Should(Say("type: value"))
		Eventually(session.Out).Should(Say("value: potatoes"))
	})

	It("gets a password secret", func() {
		responseJson := fmt.Sprintf(STRING_CREDENTIAL_ARRAY_RESPONSE_JSON, "password", "my-password", "potatoes")

		server.RouteToHandler("GET", "/api/v1/data",
			CombineHandlers(
				VerifyRequest("GET", "/api/v1/data", "current=true&name=my-password"),
				RespondWith(http.StatusOK, responseJson),
			),
		)

		session := runCommand("get", "-n", "my-password")

		Eventually(session).Should(Exit(0))
		Eventually(session.Out).Should(Say("name: my-password"))
		Eventually(session.Out).Should(Say("type: password"))
		Eventually(session.Out).Should(Say("value: potatoes"))
	})

	It("gets a json secret", func() {
		serverResponse := fmt.Sprintf(JSON_CREDENTIAL_ARRAY_RESPONSE_JSON, "json-secret", `{"foo":"bar","nested":{"a":1},"an":["array"]}`)

		server.RouteToHandler("GET", "/api/v1/data",
			CombineHandlers(
				VerifyRequest("GET", "/api/v1/data", "current=true&name=json-secret"),
				RespondWith(http.StatusOK, serverResponse),
			),
		)

		session := runCommand("get", "-n", "json-secret")

		Eventually(session).Should(Exit(0))
		Eventually(session.Out).Should(Say("name: json-secret"))
		Eventually(session.Out).Should(Say("type: json"))
		Eventually(session.Out).Should(Say(`value:
  an:
  - array
  foo: bar
  nested:
    a: 1`))

	})

	It("gets a certificate secret", func() {
		responseJson := fmt.Sprintf(CERTIFICATE_CREDENTIAL_ARRAY_RESPONSE_JSON, "my-secret", "my-ca", "my-cert", "my-priv")

		server.RouteToHandler("GET", "/api/v1/data",
			CombineHandlers(
				VerifyRequest("GET", "/api/v1/data", "current=true&name=my-secret"),
				RespondWith(http.StatusOK, responseJson),
			),
		)

		session := runCommand("get", "-n", "my-secret")

		Eventually(session).Should(Exit(0))
		Eventually(session.Out).Should(Say("name: my-secret"))
		Eventually(session.Out).Should(Say("type: certificate"))
		Eventually(session.Out).Should(Say("ca: my-ca"))
		Eventually(session.Out).Should(Say("certificate: my-cert"))
		Eventually(session.Out).Should(Say("private_key: my-priv"))
	})

	It("gets an rsa secret", func() {
		responseJson := fmt.Sprintf(RSA_SSH_CREDENTIAL_ARRAY_RESPONSE_JSON, "rsa", "foo-rsa-key", "some-public-key", "some-private-key")

		server.RouteToHandler("GET", "/api/v1/data",
			CombineHandlers(
				VerifyRequest("GET", "/api/v1/data", "current=true&name=foo-rsa-key"),
				RespondWith(http.StatusOK, responseJson),
			),
		)

		session := runCommand("get", "-n", "foo-rsa-key")

		Eventually(session).Should(Exit(0))
		Eventually(session.Out).Should(Say("name: foo-rsa-key"))
		Eventually(session.Out).Should(Say("type: rsa"))
		Eventually(session.Out).Should(Say("private_key: some-private-key"))
		Eventually(session.Out).Should(Say("public_key: some-public-key"))
	})

	It("can output json", func() {
		responseJson := fmt.Sprintf(STRING_CREDENTIAL_ARRAY_RESPONSE_JSON, "password", "my-password", "potatoes")

		server.RouteToHandler("GET", "/api/v1/data",
			CombineHandlers(
				VerifyRequest("GET", "/api/v1/data", "current=true&name=my-password"),
				RespondWith(http.StatusOK, responseJson),
			),
		)

		session := runCommand("get", "-n", "my-password", "--output-json")

		Eventually(session).Should(Exit(0))
		Eventually(string(session.Out.Contents())).Should(MatchJSON(`{
			"id": "` + UUID + `",
			"type": "password",
			"name": "my-password",
			"version_created_at": "` + TIMESTAMP + `",
			"value": "potatoes"
		}`))
	})

	It("gets a user secret", func() {
		responseJson := fmt.Sprintf(USER_CREDENTIAL_ARRAY_RESPONSE_JSON, "my-username-credential", "my-username", "test-password", "passw0rd-H4$h")

		server.RouteToHandler("GET", "/api/v1/data",
			CombineHandlers(
				VerifyRequest("GET", "/api/v1/data", "current=true&name=my-username-credential"),
				RespondWith(http.StatusOK, responseJson),
			),
		)

		session := runCommand("get", "-n", "my-username-credential")

		Eventually(session).Should(Exit(0))
		Eventually(session.Out).Should(Say("name: my-username-credential"))
		Eventually(session.Out).Should(Say("type: user"))
		Eventually(session.Out).Should(Say("password: test-password"))
		Eventually(session.Out).Should(Say(`password_hash: passw0rd-H4\$h`))
		Eventually(session.Out).Should(Say("username: my-username"))
	})

	It("gets a secret by ID", func() {
		responseJson := fmt.Sprintf(STRING_CREDENTIAL_RESPONSE_JSON, "password", "my-password", "potatoes")

		server.RouteToHandler("GET", "/api/v1/data/"+UUID,
			CombineHandlers(
				VerifyRequest("GET", "/api/v1/data/"+UUID),
				RespondWith(http.StatusOK, responseJson),
			),
		)

		session := runCommand("get", "--id", UUID)

		Eventually(session).Should(Exit(0))
		Eventually(session).Should(Exit(0))
		Eventually(session.Out).Should(Say("name: my-password"))
		Eventually(session.Out).Should(Say("type: password"))
		Eventually(session.Out).Should(Say("value: potatoes"))

	})

	It("gets the specified number of versions of a secret", func() {
		responseJson := `{"data":[{"type":"password","id":"` + UUID + `","name":"my-password","version_created_at":"` + TIMESTAMP + `","value":"old-password"},{"type":"password","id":"` + UUID + `","name":"my-password","version_created_at":"` + TIMESTAMP + `","value":"new-password"}]}`

		server.RouteToHandler("GET", "/api/v1/data",
			CombineHandlers(
				VerifyRequest("GET", "/api/v1/data", "name=my-password&versions=2"),
				RespondWith(http.StatusOK, responseJson),
			),
		)

		session := runCommand("get", "-n", "my-password", "--versions", "2")

		Eventually(session).Should(Exit(0))
		Eventually(session.Out).Should(Say(`versions:
- id: ` + UUID + `
  name: my-password
  type: password
  value: old-password
  version_created_at: "` + TIMESTAMP + `"
- id: ` + UUID + `
  name: my-password
  type: password
  value: new-password
  version_created_at: "` + TIMESTAMP + `"
`))

	})

	Context("when a key is specified", func() {
		Context("when the key is valid", func() {
			It("only returns the request field from the value object", func() {
				responseJson := fmt.Sprintf(CERTIFICATE_CREDENTIAL_ARRAY_RESPONSE_JSON, "my-secret", "----begin----\\nmy-ca\\n-----end------", "my-cert", "my-priv")

				server.RouteToHandler("GET", "/api/v1/data",
					CombineHandlers(
						VerifyRequest("GET", "/api/v1/data", "current=true&name=my-secret"),
						RespondWith(http.StatusOK, responseJson),
					),
				)

				session := runCommand("get", "-n", "my-secret", "-k", "ca")

				Eventually(session).Should(Exit(0))
				Eventually(string(session.Out.Contents())).Should(Equal(`----begin----
my-ca
-----end------
`))
			})

			Context("when there is nested JSON", func() {
				It("returns only the requested JSON field from the value object", func() {
					responseJson := fmt.Sprintf(JSON_CREDENTIAL_ARRAY_RESPONSE_JSON, "json-secret", `{"foo":"bar","nested":{"a":1},"an":["array"]}`)

					server.RouteToHandler("GET", "/api/v1/data",
						CombineHandlers(
							VerifyRequest("GET", "/api/v1/data", "current=true&name=json-secret"),
							RespondWith(http.StatusOK, responseJson),
						),
					)

					session := runCommand("get", "-n", "json-secret", "-k", "nested")

					Eventually(session).Should(Exit(0))
					Eventually(string(session.Out.Contents())).Should(Equal(`a: 1

`))
				})
			})
		})

		Context("when the key is invalid", func() {
			It("returns nothing", func() {
				responseJson := fmt.Sprintf(CERTIFICATE_CREDENTIAL_ARRAY_RESPONSE_JSON, "my-secret", "my-ca", "my-cert", "my-priv")

				server.RouteToHandler("GET", "/api/v1/data",
					CombineHandlers(
						VerifyRequest("GET", "/api/v1/data", "current=true&name=my-secret"),
						RespondWith(http.StatusOK, responseJson),
					),
				)

				session := runCommand("get", "-n", "my-secret", "-k", "invalidkey")

				Eventually(session).Should(Exit(0))
				Eventually(string(session.Out.Contents())).Should(Equal(``))

			})
		})

		Context("when there are a specified number of versions", func() {
			It("returns an error", func() {
				responseJson := `{"data":[{"type":"password","id":"` + UUID + `","name":"my-password","version_created_at":"` + TIMESTAMP + `","value":"old-password"},{"type":"password","id":"` + UUID + `","name":"my-password","version_created_at":"` + TIMESTAMP + `","value":"new-password"}]}`

				server.RouteToHandler("GET", "/api/v1/data",
					CombineHandlers(
						VerifyRequest("GET", "/api/v1/data", "name=my-password&versions=2"),
						RespondWith(http.StatusOK, responseJson),
					),
				)

				session := runCommand("get", "-n", "my-password", "--versions", "2", "-k", "someflag")
				Eventually(session).Should(Exit(1))
				Eventually(session.Err).Should(Say("The --version flag and --key flag are incompatible"))
			})
		})
	})

	It("does not use Printf on user-supplied data", func() {
		responseJson := fmt.Sprintf(STRING_CREDENTIAL_ARRAY_RESPONSE_JSON, "password", "injected", "et''%/7(V&`|?m|Ckih$")

		server.RouteToHandler("GET", "/api/v1/data",
			CombineHandlers(
				VerifyRequest("GET", "/api/v1/data", "current=true&name=injected"),
				RespondWith(http.StatusOK, responseJson),
			),
		)

		session := runCommand("get", "-n", "injected")

		Eventually(session).Should(Exit(0))
		outStr := "et''%/7\\(V&`|\\?m\\|Ckih\\$"
		Eventually(session.Out).Should(Say(outStr + TIMESTAMP))
	})
})
