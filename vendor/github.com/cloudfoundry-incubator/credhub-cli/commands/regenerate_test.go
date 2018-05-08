package commands_test

import (
	"net/http"

	"fmt"

	"github.com/cloudfoundry-incubator/credhub-cli/commands"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gbytes"
	. "github.com/onsi/gomega/gexec"
	. "github.com/onsi/gomega/ghttp"
)

const REGENERATE_CREDENTIAL_REQUEST_JSON = `{"name":"my-password-stuffs", "regenerate":true}`

var _ = Describe("Regenerate", func() {
	BeforeEach(func() {
		login()
	})

	ItRequiresAuthentication("regenerate", "-n", "test-credential")
	ItRequiresAnAPIToBeSet("regenerate", "-n", "test-credential")
	ItAutomaticallyLogsIn("POST", "regenerate_response.json", "/api/v1/data", "regenerate", "-n", "test-credential")

	Describe("Regenerating password", func() {
		It("prints the regenerated password secret in yaml format", func() {
			server.RouteToHandler("POST", "/api/v1/data",
				CombineHandlers(
					VerifyJSON(REGENERATE_CREDENTIAL_REQUEST_JSON),
					RespondWith(http.StatusOK, fmt.Sprintf(STRING_CREDENTIAL_RESPONSE_JSON, "password", "my-password-stuffs", "nu-potatoes")),
				),
			)

			session := runCommand("regenerate", "--name", "my-password-stuffs")

			Eventually(session).Should(Exit(0))
			Eventually(session.Out).Should(Say("name: my-password-stuff"))
			Eventually(session.Out).Should(Say("type: password"))
			Eventually(session.Out).Should(Say("value: nu-potatoes"))

		})

		It("prints the regenerated password secret in json format", func() {
			server.RouteToHandler("POST", "/api/v1/data",
				CombineHandlers(
					VerifyJSON(REGENERATE_CREDENTIAL_REQUEST_JSON),
					RespondWith(http.StatusOK, fmt.Sprintf(STRING_CREDENTIAL_RESPONSE_JSON, "password", "my-password-stuffs", "nu-potatoes")),
				),
			)

			session := runCommand("regenerate", "--name", "my-password-stuffs", "--output-json")

			Eventually(session).Should(Exit(0))
			Expect(string(session.Out.Contents())).To(MatchJSON(fmt.Sprintf(STRING_CREDENTIAL_RESPONSE_JSON, "password", "my-password-stuffs", "nu-potatoes")))
		})

		It("prints error when server returns an error", func() {
			server.RouteToHandler("POST", "/api/v1/data",
				CombineHandlers(
					VerifyJSON(REGENERATE_CREDENTIAL_REQUEST_JSON),
					RespondWith(http.StatusBadRequest, `{"error":"The password could not be regenerated because the value was statically set. Only generated passwords may be regenerated."}`),
				),
			)

			session := runCommand("regenerate", "--name", "my-password-stuffs")

			Eventually(session).Should(Exit(1))
			Expect(string(session.Err.Contents())).To(ContainSubstring("The password could not be regenerated because the value was statically set. Only generated passwords may be regenerated."))
		})
	})

	Describe("help", func() {
		ItBehavesLikeHelp("regenerate", "r", func(session *Session) {
			Expect(session.Err).To(Say("regenerate"))
			Expect(session.Err).To(Say("name"))
		})

		It("has short flags", func() {
			Expect(commands.RegenerateCommand{}).To(SatisfyAll(
				commands.HaveFlag("name", "n"),
			))
		})
	})
})
