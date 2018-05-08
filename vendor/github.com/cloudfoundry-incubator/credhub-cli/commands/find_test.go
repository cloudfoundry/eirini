package commands_test

import (
	"net/http"

	"runtime"

	"github.com/cloudfoundry-incubator/credhub-cli/commands"
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
				commands.HaveFlag("all-paths", "a"),
			))
		})
	})

	Describe("finding all paths in the system", func() {
		It("lists all existing credential paths in yaml format", func() {
			responseJson := `{
				"paths": [
						{
							"path": "consul/"
						},
						{
							"path": "consul/deploy123/"
						},
						{
							"path": "deploy12/"
						},
						{
							"path": "deploy123/"
						},
						{
							"path": "deploy123/dan/"
						},
						{
							"path": "deploy123/dan/consul/"
						}
				]
			}`

			// language=YAML
			responseTable :=
				"paths:\n- path: consul/\n- path: consul/deploy123/\n- path: deploy12/\n- path: deploy123/\n- path: deploy123/dan/\n- path: deploy123/dan/consul/"

			server.RouteToHandler("GET", "/api/v1/data",
				CombineHandlers(
					VerifyRequest("GET", "/api/v1/data", "paths=true"),
					RespondWith(http.StatusOK, responseJson),
				),
			)

			session := runCommand("find", "-a")

			Eventually(session.Out).Should(Say(responseTable))
			Eventually(session).Should(Exit(0))
		})

		It("lists all existing credential paths in json format", func() {
			//language=JSOn
			responseJson := `{
   "paths": [
              {
                  "path": "consul/"
              },
              {
                  "path": "consul/deploy123/"
              },
              {
                  "path": "deploy12/"
              },
              {
                  "path": "deploy123/"
              },
              {
                  "path": "deploy123/dan/"
              },
              {
                  "path": "deploy123/dan/consul/"
              }
   ]
}`

			server.RouteToHandler("GET", "/api/v1/data",
				CombineHandlers(
					VerifyRequest("GET", "/api/v1/data", "paths=true"),
					RespondWith(http.StatusOK, responseJson),
				),
			)

			session := runCommand("find", "-a", "--output-json")

			Eventually(string(session.Out.Contents())).Should(MatchJSON(responseJson))
			Eventually(session).Should(Exit(0))
		})

		It("displays error message when no credentials are found", func() {
			//language=JSOn
			responseJson := `{
				"paths": []
			}`

			server.RouteToHandler("GET", "/api/v1/data",
				CombineHandlers(
					VerifyRequest("GET", "/api/v1/data", "paths=true"),
					RespondWith(http.StatusOK, responseJson),
				),
			)

			session := runCommand("find", "-a", "--output-json")

			Eventually(session.Err).Should(Say("No credentials exist which match the provided parameters."))
			Eventually(session).Should(Exit(1))
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
})
