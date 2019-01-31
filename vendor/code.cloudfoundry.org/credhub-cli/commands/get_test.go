package commands_test

import (
	"bytes"
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

	Describe("value type", func() {
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


		Context("with --quiet flag", func() {
			It("returns only the value", func() {
				responseJson := fmt.Sprintf(STRING_CREDENTIAL_ARRAY_RESPONSE_JSON, "value", "my-value", "potatoes")

				server.RouteToHandler("GET", "/api/v1/data",
					CombineHandlers(
						VerifyRequest("GET", "/api/v1/data", "current=true&name=my-value"),
						RespondWith(http.StatusOK, responseJson),
					),
				)

				session := runCommand("get", "-n", "my-value", "-q")

				Eventually(session).Should(Exit(0))
				contents := string(bytes.TrimSpace(session.Out.Contents()))
				Eventually(contents).Should(Equal("potatoes"))
			})
		})


		Context("multiple versions with --quiet flag", func() {
			It("returns array of values", func() {
				responseJson := fmt.Sprintf(MULTIPLE_STRING_CREDENTIAL_ARRAY_RESPONSE_JSON, "value", "my-cred", "potatoes", "value", "my-cred", "tomatoes")

				server.RouteToHandler("GET", "/api/v1/data",
					CombineHandlers(
						VerifyRequest("GET", "/api/v1/data", "name=my-cred&versions=2"),
						RespondWith(http.StatusOK, responseJson),
					),
				)

				session := runCommand("get", "-n", "my-cred", "-q", "--versions", "2")

				Eventually(session).Should(Exit(0))
				contents := string(bytes.TrimSpace(session.Out.Contents()))
				Eventually(contents).Should(Equal(`versions:
- potatoes
- tomatoes`))
			})
		})

	})

	Describe("password type", func() {
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

		Context("with key and version", func() {
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

		Context("with --quiet flag", func() {
			It("can quiet output for password", func() {
				responseJson := fmt.Sprintf(STRING_CREDENTIAL_ARRAY_RESPONSE_JSON, "password", "my-password", "potatoes")

				server.RouteToHandler("GET", "/api/v1/data",
					CombineHandlers(
						VerifyRequest("GET", "/api/v1/data", "current=true&name=my-password"),
						RespondWith(http.StatusOK, responseJson),
					),
				)

				session := runCommand("get", "-n", "my-password", "-q")

				Eventually(session).Should(Exit(0))
				contents := string(bytes.TrimSpace(session.Out.Contents()))
				Eventually(contents).Should(Equal("potatoes"))
			})
		})

		Context("with --versions flag", func() {
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
		})

		Context("multiple versions with --quiet flag", func() {
			It("returns an error", func() {
				responseJson := `{"data":[{"type":"password","id":"` + UUID + `","name":"my-password","version_created_at":"` + TIMESTAMP + `","value":"new-password"},{"type":"password","id":"` + UUID + `","name":"my-password","version_created_at":"` + TIMESTAMP + `","value":"old-password"}]}`

				server.RouteToHandler("GET", "/api/v1/data",
					CombineHandlers(
						VerifyRequest("GET", "/api/v1/data", "name=my-password&versions=2"),
						RespondWith(http.StatusOK, responseJson),
					),
				)

				session := runCommand("get", "-n", "my-password", "--versions", "2", "-q")

				Eventually(session).Should(Exit(0))
				contents := string(bytes.TrimSpace(session.Out.Contents()))
				Eventually(contents).Should(Equal(`versions:
- new-password
- old-password`))
			})
		})

	})

	Describe("json type", func() {
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

		Context("with --output-json flag", func() {
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
		})

		Context("with --output-json and --quiet flags", func() {
			It("should return an error", func() {
				responseJson := fmt.Sprintf(STRING_CREDENTIAL_ARRAY_RESPONSE_JSON, "password", "my-password", "potatoes")

				server.RouteToHandler("GET", "/api/v1/data",
					CombineHandlers(
						VerifyRequest("GET", "/api/v1/data", "current=true&name=my-password"),
						RespondWith(http.StatusOK, responseJson),
					),
				)

				session := runCommand("get", "-n", "my-password", "--output-json", "-q")

				Eventually(session).Should(Exit(1))
				Eventually(session.Err).Should(Say("The --output-json flag and --quiet flag are incompatible"))
			})
		})

		Context("with --key flag", func() {
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

		Context("with --quiet flag", func() {
			It("only return the value", func() {
				responseJson := fmt.Sprintf(JSON_CREDENTIAL_ARRAY_RESPONSE_JSON, "json-secret", `{"foo":"bar","nested":{"a":1},"an":["array"]}`)

				server.RouteToHandler("GET", "/api/v1/data",
					CombineHandlers(
						VerifyRequest("GET", "/api/v1/data", "current=true&name=json-secret"),
						RespondWith(http.StatusOK, responseJson),
					),
				)

				session := runCommand("get", "-n", "json-secret", "-q")

				Eventually(session).Should(Exit(0))
				contents := string(bytes.TrimSpace(session.Out.Contents()))
				Eventually(contents).Should(Equal(`an:
- array
foo: bar
nested:
  a: 1`))
			})
		})

		Context("multiple versions with --quiet flag", func() {
			It("returns an array of values", func() {
				responseJson := `{"data":[{"type":"json","id":"` + UUID + `","name":"my-json","version_created_at":"` + TIMESTAMP + `","value":{"secret":"newSecret"}},{"type":"json","id":"` + UUID + `","name":"my-json","version_created_at":"` + TIMESTAMP + `","value":{"secret":"oldSecret"}}]}`
				server.RouteToHandler("GET", "/api/v1/data",
					CombineHandlers(
						VerifyRequest("GET", "/api/v1/data", "name=my-json&versions=2"),
						RespondWith(http.StatusOK, responseJson),
					),
				)

				session := runCommand("get", "-n", "my-json", "-q", "--versions", "2")

				Eventually(session).Should(Exit(0))
				contents := string(bytes.TrimSpace(session.Out.Contents()))
				Eventually(contents).Should(Equal(`versions:
- secret: newSecret
- secret: oldSecret`))
			})
		})

	})

	Describe("certificate type", func() {
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

		Context("with --key flag", func() {
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
		})

		Context("with invalid key", func() {
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

		Context("with --quiet flag", func() {
			It("only returns the value", func() {
				responseJson := fmt.Sprintf(CERTIFICATE_CREDENTIAL_ARRAY_RESPONSE_JSON, "my-secret", "----begin----\\nmy-ca\\n-----end------", "----begin----\\nmy-cert\\n-----end------", "----begin----\\nmy-priv\\n-----end------")

				server.RouteToHandler("GET", "/api/v1/data",
					CombineHandlers(
						VerifyRequest("GET", "/api/v1/data", "current=true&name=my-secret"),
						RespondWith(http.StatusOK, responseJson),
					),
				)

				session := runCommand("get", "-n", "my-secret", "-q")

				Eventually(session).Should(Exit(0))
				Eventually(string(session.Out.Contents())).Should(Equal(`ca: |-
  ----begin----
  my-ca
  -----end------
certificate: |-
  ----begin----
  my-cert
  -----end------
private_key: |-
  ----begin----
  my-priv
  -----end------

`))
			})
		})

		Context("multiple versions with --quiet flag", func() {
			It("only returns the value", func() {
				responseJson := fmt.Sprintf(MULTIPLE_CERTIFICATE_CREDENTIAL_ARRAY_RESPONSE_JSON,
					"my-secret",
					"----begin----\\nmy-new-ca\\n-----end------",
					"----begin----\\nmy-new-cert\\n-----end------",
					"----begin----\\nmy-new-priv\\n-----end------",
					"my-secret",
					"----begin----\\nmy-old-ca\\n-----end------",
					"----begin----\\nmy-old-cert\\n-----end------",
					"----begin----\\nmy-old-priv\\n-----end------")
				server.RouteToHandler("GET", "/api/v1/data",
					CombineHandlers(
						VerifyRequest("GET", "/api/v1/data", "name=my-secret&versions=2"),
						RespondWith(http.StatusOK, responseJson),
					),
				)

				session := runCommand("get", "-n", "my-secret", "-q", "--versions", "2")

				Eventually(session).Should(Exit(0))
				Eventually(string(bytes.TrimSpace(session.Out.Contents()))).Should(Equal(`versions:
- ca: |-
    ----begin----
    my-new-ca
    -----end------
  certificate: |-
    ----begin----
    my-new-cert
    -----end------
  private_key: |-
    ----begin----
    my-new-priv
    -----end------
- ca: |-
    ----begin----
    my-old-ca
    -----end------
  certificate: |-
    ----begin----
    my-old-cert
    -----end------
  private_key: |-
    ----begin----
    my-old-priv
    -----end------`))
			})
		})

		Context("--quiet flag with key", func() {
			It("should not only return the value", func() {
				responseJson := fmt.Sprintf(CERTIFICATE_CREDENTIAL_ARRAY_RESPONSE_JSON, "my-secret", "----begin----\\nmy-ca\\n-----end------", "----begin----\\nmy-cert\\n-----end------", "----begin----\\nmy-priv\\n-----end------")

				server.RouteToHandler("GET", "/api/v1/data",
					CombineHandlers(
						VerifyRequest("GET", "/api/v1/data", "current=true&name=my-secret"),
						RespondWith(http.StatusOK, responseJson),
					),
				)

				session := runCommand("get", "-n", "my-secret", "-q", "-k", "ca")

				Eventually(session).Should(Exit(0))
				Eventually(string(session.Out.Contents())).ShouldNot(Equal(`ca: |-
  ----begin----
  my-ca
  -----end------
certificate: |-
  ----begin----
  my-cert
  -----end------
private_key: |-
  ----begin----
  my-priv
  -----end------

`))
			})
		})

	})

	Describe("rsa type", func() {
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

		Context("with --quiet flag", func() {
			It("gets only the value", func() {
				responseJson := fmt.Sprintf(RSA_SSH_CREDENTIAL_ARRAY_RESPONSE_JSON, "rsa", "foo-rsa-key", "some-public-key", "some-private-key")

				server.RouteToHandler("GET", "/api/v1/data",
					CombineHandlers(
						VerifyRequest("GET", "/api/v1/data", "current=true&name=foo-rsa-key"),
						RespondWith(http.StatusOK, responseJson),
					),
				)

				session := runCommand("get", "-n", "foo-rsa-key", "-q")

				Eventually(session).Should(Exit(0))
				Eventually(session.Out).ShouldNot(Say("name: foo-rsa-key"))
				Eventually(session.Out).ShouldNot(Say("type: rsa"))
				Eventually(session.Out).Should(Say("private_key: some-private-key"))
				Eventually(session.Out).Should(Say("public_key: some-public-key"))
			})
		})

		Context("multiple versions with --quiet flag", func() {
			It("only returns the value", func() {
				responseJson := fmt.Sprintf(MULTIPLE_RSA_SSH_CREDENTIAL_ARRAY_RESPONSE_JSON,
					"rsa",
					"foo-rsa-key",
					"new-public-key",
					"new-private-key",
					"rsa",
					"foo-rsa-key",
					"old-public-key",
					"old-private-key")

				server.RouteToHandler("GET", "/api/v1/data",
					CombineHandlers(
						VerifyRequest("GET", "/api/v1/data", "versions=2&name=foo-rsa-key"),
						RespondWith(http.StatusOK, responseJson),
					),
				)

				session := runCommand("get", "-n", "foo-rsa-key", "-q", "--versions", "2")

				Eventually(session).Should(Exit(0))
				contents := string(bytes.TrimSpace(session.Out.Contents()))
				Eventually(contents).Should(Equal(`versions:
- private_key: new-private-key
  public_key: new-public-key
- private_key: old-private-key
  public_key: old-public-key`))
			})
		})

		Context("--quiet flag with key", func() {
			It("should not only return the value", func() {
				responseJson := fmt.Sprintf(RSA_SSH_CREDENTIAL_ARRAY_RESPONSE_JSON, "rsa", "foo-rsa-key", "some-public-key", "some-private-key")

				server.RouteToHandler("GET", "/api/v1/data",
					CombineHandlers(
						VerifyRequest("GET", "/api/v1/data", "current=true&name=foo-rsa-key"),
						RespondWith(http.StatusOK, responseJson),
					),
				)

				session := runCommand("get", "-n", "foo-rsa-key", "-q", "-k", "public_key")

				Eventually(session).Should(Exit(0))
				Eventually(session.Out).ShouldNot(Say("name: foo-rsa-key"))
				Eventually(session.Out).ShouldNot(Say("type: rsa"))
				Eventually(session.Out).Should(Say("some-public-key"))
			})
		})
	})

	Describe("user type", func() {
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

		Context("with --quiet flag", func() {
			It("gets only the value", func() {
				responseJson := fmt.Sprintf(USER_CREDENTIAL_ARRAY_RESPONSE_JSON, "my-username-credential", "my-username", "test-password", "passw0rd-H4$h")

				server.RouteToHandler("GET", "/api/v1/data",
					CombineHandlers(
						VerifyRequest("GET", "/api/v1/data", "current=true&name=my-username"),
						RespondWith(http.StatusOK, responseJson),
					),
				)

				session := runCommand("get", "-n", "my-username", "-q")

				Eventually(session).Should(Exit(0))
				Eventually(session.Out).ShouldNot(Say("name: my-username-credential"))
				Eventually(session.Out).ShouldNot(Say("type: user"))
				Eventually(session.Out).Should(Say("password: test-password"))
				Eventually(session.Out).Should(Say(`password_hash: passw0rd-H4\$h`))
				Eventually(session.Out).Should(Say("username: my-username"))
			})
		})

		Context("multiple versions with the --quiet flag", func() {
			It("returns an array of values", func() {
				responseJson := fmt.Sprintf(MULTIPLE_USER_CREDENTIAL_ARRAY_RESPONSE_JSON,
					"my-username-credential",
					"new-username",
					"new-password",
					"new-passw0rd-H4$h",
					"my-username-credential",
					"old-username",
					"old-password",
					"old-passw0rd-H4$h")

				server.RouteToHandler("GET", "/api/v1/data",
					CombineHandlers(
						VerifyRequest("GET", "/api/v1/data", "name=my-username-credential&versions=2"),
						RespondWith(http.StatusOK, responseJson),
					),
				)

				session := runCommand("get", "-n", "my-username-credential", "-q", "--versions", "2")

				Eventually(session).Should(Exit(0))
				Eventually(session.Out).ShouldNot(Say("name: my-username-credential"))
				Eventually(session.Out).ShouldNot(Say("type: user"))
				Eventually(session.Out).Should(Say("versions:"))
				Eventually(session.Out).Should(Say("- password: new-password"))
				Eventually(session.Out).Should(Say(`  password_hash: new-passw0rd-H4\$h`))
				Eventually(session.Out).Should(Say("  username: new-username"))
				Eventually(session.Out).Should(Say("- password: old-password"))
				Eventually(session.Out).Should(Say(`  password_hash: old-passw0rd-H4\$h`))
				Eventually(session.Out).Should(Say("  username: old-username"))
			})
		})

		Context("--quiet flag with key", func() {
			It("ignores the quiet flag", func() {
				responseJson := fmt.Sprintf(USER_CREDENTIAL_ARRAY_RESPONSE_JSON, "my-username-credential", "my-username", "test-password", "passw0rd-H4$h")

				server.RouteToHandler("GET", "/api/v1/data",
					CombineHandlers(
						VerifyRequest("GET", "/api/v1/data", "current=true&name=my-username"),
						RespondWith(http.StatusOK, responseJson),
					),
				)

				session := runCommand("get", "-n", "my-username", "-q", "-k", "password_hash")

				Eventually(session).Should(Exit(0))
				contents := string(bytes.TrimSpace(session.Out.Contents()))
				Eventually(contents).Should(Equal(`passw0rd-H4$h`))
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
