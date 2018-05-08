package commands_test

import (
	"net/http"

	"github.com/cloudfoundry-incubator/credhub-cli/commands"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gbytes"
	. "github.com/onsi/gomega/gexec"
	. "github.com/onsi/gomega/ghttp"
)

const BULK_REGENERATE_CREDENTIAL_REQUEST_JSON = `{"signed_by": "example-ca"}`
const RESPONSE_JSON = `{"regenerated_credentials":["cert1","cert2","cert3"]}`

var _ = Describe("Bulk-regenerate", func() {
	BeforeEach(func() {
		login()
	})

	ItRequiresAuthentication("bulk-regenerate", "--signed-by", "example-ca")
	ItRequiresAnAPIToBeSet("bulk-regenerate", "--signed-by", "example-ca")
	ItAutomaticallyLogsIn("POST", "bulk_regenerate_response.json", "/api/v1/bulk-regenerate", "bulk-regenerate", "--signed-by", "example-ca")

	Describe("Regenerating all certificates signed by the given CA", func() {
		It("prints the regenerated certificates in yaml format", func() {
			server.RouteToHandler("POST", "/api/v1/bulk-regenerate",
				CombineHandlers(
					VerifyJSON(BULK_REGENERATE_CREDENTIAL_REQUEST_JSON),
					RespondWith(http.StatusOK, RESPONSE_JSON),
				),
			)

			session := runCommand("bulk-regenerate", "--signed-by", "example-ca")

			Eventually(session).Should(Exit(0))
			Eventually(string(session.Out.Contents())).Should(Equal(`regenerated_credentials:
- cert1
- cert2
- cert3

`))
		})

		It("prints the regenerated certs in json format", func() {
			server.RouteToHandler("POST", "/api/v1/bulk-regenerate",
				CombineHandlers(
					VerifyJSON(BULK_REGENERATE_CREDENTIAL_REQUEST_JSON),
					RespondWith(http.StatusOK, RESPONSE_JSON)))

			session := runCommand("bulk-regenerate", "--signed-by", "example-ca", "--output-json")

			Eventually(session).Should(Exit(0))
			Expect(string(session.Out.Contents())).To(MatchJSON(RESPONSE_JSON))
		})

		It("prints error when server returns an error", func() {
			server.RouteToHandler("POST", "/api/v1/bulk-regenerate",
				CombineHandlers(
					VerifyJSON(BULK_REGENERATE_CREDENTIAL_REQUEST_JSON),
					RespondWith(http.StatusBadRequest, `{"error":"The certs could not be regenerated"}`),
				),
			)

			session := runCommand("bulk-regenerate", "--signed-by", "example-ca")

			Eventually(session).Should(Exit(1))
			Expect(string(session.Err.Contents())).To(ContainSubstring("The certs could not be regenerated"))
		})
	})

	Describe("help", func() {
		It("Behaves like help", func() {
			session := runCommand("bulk-regenerate", "-h")
			Eventually(session).Should(Exit(1))
			Expect(session.Err).To(Say("bulk-regenerate"))
			Expect(session.Err).To(Say("signed-by"))
		})

		It("has short flags", func() {
			Expect(commands.BulkRegenerateCommand{}).To(SatisfyAll(
				commands.HaveFlag("signed-by", ""),
			))
		})
	})
})
