package commands_test

import (
	"net/http"

	"runtime"

	"code.cloudfoundry.org/credhub-cli/commands"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gbytes"
	. "github.com/onsi/gomega/gexec"
	. "github.com/onsi/gomega/ghttp"
)

var _ = Describe("Find", func() {
	BeforeEach(func() {
		login()
	})

	ItRequiresAuthentication("find", "-n", "test-credential")
	ItRequiresAnAPIToBeSet("find", "-n", "test-credential")
	ItAutomaticallyLogsIn("GET", "find_response.json", "/api/v1/data", "find")

	Describe("Help", func() {
		ItBehavesLikeHelp("find", "f", func(session *Session) {
			Expect(session.Err).To(Say("Usage"))
			if runtime.GOOS == "windows" {
				Expect(session.Err).To(Say("credhub-cli.exe \\[OPTIONS\\] find \\[find-OPTIONS\\]"))
			} else {
				Expect(session.Err).To(Say("credhub-cli \\[OPTIONS\\] find \\[find-OPTIONS\\]"))
			}
		})

		It("short flags", func() {
			Expect(commands.FindCommand{}).To(SatisfyAll(
				commands.HaveFlag("name-like", "n"),
				commands.HaveFlag("path", "p"),
			))
		})
	})

	Describe("finds a set of credentials matching a supplied string", func() {
		Describe("when searching for 'name-like'", func() {
			It("gets a list of string secret names and last-modified dates", func() {
				responseJson := `{
					"credentials": [
							{
								"name": "dan.password",
								"version_created_at": "2016-09-06T23:26:58Z"
							},
							{
								"name": "deploy1/dan/id.key",
								"version_created_at": "2016-09-06T23:26:58Z"
							}
					]
				}`
				// language=YAML
				responseTable :=
					`credentials:\n- name: dan.password\n  version_created_at: "2016-09-06T23:26:58Z"\n- name: deploy1/dan/id.key\n  version_created_at: "2016-09-06T23:26:58Z"`

				server.RouteToHandler("GET", "/api/v1/data",
					CombineHandlers(
						VerifyRequest("GET", "/api/v1/data", "name-like=dan"),
						RespondWith(http.StatusOK, responseJson),
					),
				)

				session := runCommand("find", "-n", "dan")

				Eventually(session).Should(Exit(0))
				Eventually(session.Out).Should(Say(responseTable))
			})

			Describe("when there are no matches for the supplied string", func() {
				var session *Session

				BeforeEach(func() {
					responseJson := `{
						"credentials": []
					}`

					server.RouteToHandler("GET", "/api/v1/data",
						CombineHandlers(
							VerifyRequest("GET", "/api/v1/data", "name-like=dan"),
							RespondWith(http.StatusOK, responseJson),
						),
					)

					session = runCommand("find", "-n", "dan")
				})

				It("lets the user know that there are no results", func() {
					expectedMessage := "No credentials exist which match the provided parameters."

					Eventually(session.Err).Should(Say(expectedMessage))
				})

				It("exits with code 1", func() {
					Eventually(session).Should(Exit(1))
				})
			})
		})

		Describe("when searching by path", func() {
			It("gets a list of string secret names and last-modified dates", func() {
				responseJson := `{
					"credentials": [
							{
								"name": "deploy123/dan.password",
								"version_created_at": "2016-09-06T23:26:58Z"
							},
							{
								"name": "deploy123/dan.key",
								"version_created_at": "2016-09-06T23:26:58Z"
							},
							{
								"name": "deploy123/dan/id.key",
								"version_created_at": "2016-09-06T23:26:58Z"
							}
					]
				}`
				// language=YAML
				responseTable :=
					`credentials:\n- name: deploy123/dan.password\n  version_created_at: "2016-09-06T23:26:58Z"\n- name: deploy123/dan.key\n  version_created_at: "2016-09-06T23:26:58Z"\n- name: deploy123/dan/id.key\n  version_created_at: "2016-09-06T23:26:58Z"`

				server.RouteToHandler("GET", "/api/v1/data",
					CombineHandlers(
						VerifyRequest("GET", "/api/v1/data", "path=deploy123"),
						RespondWith(http.StatusOK, responseJson),
					),
				)

				session := runCommand("find", "-p", "deploy123")

				Eventually(session.Out).Should(Say(responseTable))
				Eventually(session).Should(Exit(0))
			})
		})
	})

	Describe("when an error is received from the server", func() {
		It("shows the error name and description", func() {
			server.AppendHandlers(
				RespondWith(http.StatusBadRequest, `{"error": "test error", "error_description": "test description"}`),
			)

			session := runCommand("find")

			Eventually(session).Should(Exit(1))

			Expect(session.Err).To(Say("test error: test description"))
		})
	})
})
