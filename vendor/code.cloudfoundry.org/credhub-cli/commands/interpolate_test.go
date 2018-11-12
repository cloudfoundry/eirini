package commands_test

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"runtime"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"
	. "github.com/onsi/gomega/ghttp"
)

var (
	session      *gexec.Session
	templateFile *os.File
	templateText string
	err          error
)

var _ = Describe("interpolate", func() {
	BeforeEach(func() {
		login()
		templateFile, err = ioutil.TempFile("", "credhub_test_interpolate_template_")
	})

	Describe("behavior shared with other commands", func() {
		templateFile, err = ioutil.TempFile("", "credhub_test_interpolate_template_")
		templateFile.WriteString("---")
		ItAutomaticallyLogsIn("GET", "get_response.json", "/api/v1/data", "interpolate", "-f", templateFile.Name())

		ItBehavesLikeHelp("interpolate", "interpolate", func(session *gexec.Session) {
			Expect(session.Err).To(Say("Usage"))
			if runtime.GOOS == "windows" {
				Expect(session.Err).To(Say("credhub-cli.exe \\[OPTIONS\\] interpolate \\[interpolate-OPTIONS\\]"))
			} else {
				Expect(session.Err).To(Say("credhub-cli \\[OPTIONS\\] interpolate \\[interpolate-OPTIONS\\]"))
			}
		})
		ItRequiresAuthentication("interpolate", "-f", "testinterpolationtemplate.yml")
		ItRequiresAnAPIToBeSet("interpolate", "-f", "testinterpolationtemplate.yml")
	})

	Describe("interpolating various types of credentials", func() {
		It("queries for string creds and prints them in the template as strings", func() {
			templateText = `---
value-cred: ((relative/value/cred/path))
static-value: a normal string`
			templateFile.WriteString(templateText)
			responseValueJson := fmt.Sprintf(STRING_CREDENTIAL_ARRAY_RESPONSE_JSON, "value", "relative/value/cred/path", `{\"value\": \"should not be interpolated\"}`)

			server.RouteToHandler("GET", "/api/v1/data",
				CombineHandlers(
					VerifyRequest("GET", "/api/v1/data", "current=true&name=relative/value/cred/path"),
					RespondWith(http.StatusOK, responseValueJson),
				),
			)

			session = runCommand("interpolate", "-f", templateFile.Name())
			Eventually(session).Should(gexec.Exit(0))
			Expect(string(session.Out.Contents())).To(MatchYAML(`
value-cred: "{\"value\": \"should not be interpolated\"}"
static-value: a normal string
`))
		})

		It("queries for multi-line, multi-part credential types and prints them in the template", func() {
			templateText = `---
full-certificate-cred: ((relative/certificate/cred/path))
cert-only-certificate-cred: ((relative/certificate/cred/path.certificate))
static-value: a normal string`
			templateFile.WriteString(templateText)

			responseCertJson := fmt.Sprintf(CERTIFICATE_CREDENTIAL_ARRAY_RESPONSE_JSON, "test-cert", "", "-----BEGIN FAKE CERTIFICATE-----\\n-----END FAKE CERTIFICATE-----", "-----BEGIN FAKE RSA PRIVATE KEY-----\\n-----END FAKE RSA PRIVATE KEY-----")

			server.RouteToHandler("GET", "/api/v1/data",
				CombineHandlers(
					VerifyRequest("GET", "/api/v1/data", "current=true&name=relative/certificate/cred/path"),
					RespondWith(http.StatusOK, responseCertJson),
				),
			)

			session = runCommand("interpolate", "-f", templateFile.Name())
			Eventually(session).Should(gexec.Exit(0))
			Expect(string(session.Out.Contents())).To(MatchYAML(`
full-certificate-cred:
  ca: ""
  certificate: |-
    -----BEGIN FAKE CERTIFICATE-----
    -----END FAKE CERTIFICATE-----
  private_key: |-
    -----BEGIN FAKE RSA PRIVATE KEY-----
    -----END FAKE RSA PRIVATE KEY-----
cert-only-certificate-cred: |-
  -----BEGIN FAKE CERTIFICATE-----
  -----END FAKE CERTIFICATE-----
static-value: a normal string
`))
		})

		It("queries for json creds and prints them in the template rendered as yaml", func() {
			templateText = `json-cred: ((relative/json/cred/path))`
			templateFile.WriteString(templateText)

			responseJson := fmt.Sprintf(JSON_CREDENTIAL_ARRAY_RESPONSE_JSON, "test-json", `{"whatthing":"something"}`)

			server.RouteToHandler("GET", "/api/v1/data",
				CombineHandlers(
					VerifyRequest("GET", "/api/v1/data", "current=true&name=relative/json/cred/path"),
					RespondWith(http.StatusOK, responseJson),
				),
			)

			session = runCommand("interpolate", "-f", templateFile.Name())
			Eventually(session).Should(gexec.Exit(0))
			Expect(string(session.Out.Contents())).To(MatchYAML(`json-cred: {"whatthing":"something"}`))
		})
	})

	Describe("the optional --prefix flag", func() {
		BeforeEach(func() {
			templateText = `---
full-certificate-cred: ((certificate/cred/path))
cert-only-certificate-cred: ((/relative/certificate/cred/path.certificate))
static-value: a normal string`
			templateFile.WriteString(templateText)

			responseCertJson := fmt.Sprintf(CERTIFICATE_CREDENTIAL_ARRAY_RESPONSE_JSON, "test-cert", "", "-----BEGIN FAKE CERTIFICATE-----\\n-----END FAKE CERTIFICATE-----", "-----BEGIN FAKE RSA PRIVATE KEY-----\\n-----END FAKE RSA PRIVATE KEY-----")

			server.RouteToHandler("GET", "/api/v1/data",
				CombineHandlers(
					VerifyRequest("GET", "/api/v1/data", "current=true&name=/relative/certificate/cred/path"),
					RespondWith(http.StatusOK, responseCertJson),
				),
			)
		})
		It("prints the values of credential names derived from the prefix, unless the cred paths start with /", func() {
			session = runCommand("interpolate", "-f", templateFile.Name(), "-p=/relative")
			Eventually(session).Should(gexec.Exit(0))
			Expect(string(session.Out.Contents())).To(MatchYAML(`
full-certificate-cred:
  ca: ""
  certificate: |-
    -----BEGIN FAKE CERTIFICATE-----
    -----END FAKE CERTIFICATE-----
  private_key: |-
    -----BEGIN FAKE RSA PRIVATE KEY-----
    -----END FAKE RSA PRIVATE KEY-----
cert-only-certificate-cred: |-
  -----BEGIN FAKE CERTIFICATE-----
  -----END FAKE CERTIFICATE-----
static-value: a normal string
`))
		})
	})

	Describe("Errors", func() {
		Context("when no template file is provided", func() {
			BeforeEach(func() {
				session = runCommand("interpolate")
			})
			It("prints missing required parameter", func() {
				Eventually(session).Should(gexec.Exit(1))
				Expect(session.Err).To(Say("A file to interpolate must be provided. Please add a file flag and try again."))
			})
		})

		Context("when the --file doesn't exist", func() {
			var invalidFile = "does-not-exist.yml"

			It("prints an error that includes the filepath and the filesystem error", func() {
				session := runCommand("interpolate", "-f", invalidFile)
				Eventually(session).Should(gexec.Exit(1), "interpolate should have failed")
				Expect(session.Err).To(Say(invalidFile))
			})
		})

		Context("when the template file is empty", func() {
			It("prints and errors to require data in the template", func() {
				session := runCommand("interpolate", "-f", templateFile.Name())
				Eventually(session).Should(gexec.Exit(1), "interpolate should have failed")
				Expect(session.Err.Contents()).To(ContainSubstring(fmt.Sprintf("Error: %s was an empty file", templateFile.Name())))
			})
		})

		Context("when the template file contains no credentials to resolve", func() {
			BeforeEach(func() {
				templateText = `---
yaml-key-with-static-value: a normal string`
				templateFile.WriteString(templateText)
			})
			It("succeeds and prints the template to stdout", func() {
				session := runCommand("interpolate", "-f", templateFile.Name())
				Eventually(session).Should(gexec.Exit(0), "command should succeed")
				Expect(session.Out).To(Say("yaml-key-with-static-value: a normal string"))
			})
		})

		Context("when a path in the --file can't be found", func() {
			BeforeEach(func() {
				templateText = `---
yaml-key-with-template-value: ((relative/cred/path))
yaml-key-with-static-value: a normal string`
				templateFile.WriteString(templateText)

				server.RouteToHandler("GET", "/api/v1/data",
					CombineHandlers(
						VerifyRequest("GET", "/api/v1/data", "current=true&name=relative/cred/path"),
						RespondWith(http.StatusOK, `{"data":[]}`),
					),
				)
			})

			It("prints an error that includes the credential path and the underlying error", func() {
				session = runCommand("interpolate", "-f", templateFile.Name())
				Eventually(session).Should(gexec.Exit(1))
				Expect(session.Err).To(Say("Finding variable 'relative/cred/path': response did not contain any credentials"))
			})
		})
	})
})
