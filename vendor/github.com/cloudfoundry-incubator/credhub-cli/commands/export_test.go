package commands_test

import (
	"io/ioutil"
	"net/http"
	"os"

	"runtime"

	"github.com/cloudfoundry-incubator/credhub-cli/config"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gbytes"
	. "github.com/onsi/gomega/gexec"
	. "github.com/onsi/gomega/ghttp"
)

func withTemporaryFile(wantingFile func(string)) error {
	f, err := ioutil.TempFile("", "credhub_tests_")

	if err != nil {
		return err
	}

	name := f.Name()

	f.Close()
	wantingFile(name)

	return os.Remove(name)
}

var _ = Describe("Export", func() {
	BeforeEach(func() {
		login()
	})

	ItRequiresAuthentication("export")
	ItRequiresAnAPIToBeSet("export")
	ItAutomaticallyLogsIn("GET", "get_response.json", "/api/v1/data", "export")

	ItBehavesLikeHelp("export", "e", func(session *Session) {
		Expect(session.Err).To(Say("Usage"))
		if runtime.GOOS == "windows" {
			Expect(session.Err).To(Say("credhub-cli.exe \\[OPTIONS\\] export \\[export-OPTIONS\\]"))
		} else {
			Expect(session.Err).To(Say("credhub-cli \\[OPTIONS\\] export \\[export-OPTIONS\\]"))
		}
	})

	Describe("Exporting", func() {
		It("queries for the most recent version of all credentials", func() {
			findJson := `{
				"credentials": [
					{
						"version_created_at": "idc",
						"name": "/path/to/cred"
					},
					{
						"version_created_at": "idc",
						"name": "/path/to/another/cred"
					}
				]
			}`

			getJson := `{
				"data": [{
					"type":"value",
					"id":"some_uuid",
					"name":"/path/to/cred",
					"version_created_at":"idc",
					"value": "foo"
				}]
			}`

			responseTable := `credentials:
- name: /path/to/cred
  type: value
  value: foo
- name: /path/to/cred
  type: value
  value: foo`

			server.AppendHandlers(
				CombineHandlers(
					VerifyRequest("GET", "/api/v1/data", "path="),
					RespondWith(http.StatusOK, findJson),
				),
				CombineHandlers(
					VerifyRequest("GET", "/api/v1/data", "name=/path/to/cred&current=true"),
					RespondWith(http.StatusOK, getJson),
				),
				CombineHandlers(
					VerifyRequest("GET", "/api/v1/data", "name=/path/to/another/cred&current=true"),
					RespondWith(http.StatusOK, getJson),
				),
			)

			session := runCommand("export")

			Eventually(session).Should(Exit(0))
			Eventually(session.Out).Should(Say(responseTable))
		})

		Context("when given a path", func() {
			It("queries for credentials matching that path", func() {
				noCredsJson := `{ "credentials" : [] }`

				server.AppendHandlers(
					CombineHandlers(
						VerifyRequest("GET", "/api/v1/data", "path=some/path"),
						RespondWith(http.StatusOK, noCredsJson),
					),
				)

				session := runCommand("export", "-p", "some/path")

				Eventually(session).Should(Exit(0))
			})
		})

		Context("when given a file", func() {
			It("writes the YAML to that file", func() {
				withTemporaryFile(func(filename string) {
					noCredsJson := `{ "credentials" : [] }`
					noCredsYaml := `credentials: []
`

					server.AppendHandlers(
						CombineHandlers(
							VerifyRequest("GET", "/api/v1/data", "path="),
							RespondWith(http.StatusOK, noCredsJson),
						),
					)

					session := runCommand("export", "-f", filename)

					Eventually(session).Should(Exit(0))

					Expect(filename).To(BeAnExistingFile())

					fileContents, _ := ioutil.ReadFile(filename)

					Expect(string(fileContents)).To(Equal(noCredsYaml))
				})
			})
		})
	})

	Describe("Errors", func() {
		It("prints an error when the network request fails", func() {
			cfg := config.ReadConfig()
			cfg.ApiURL = "mashed://potatoes"
			config.WriteConfig(cfg)

			session := runCommand("export")

			Eventually(session).Should(Exit(1))
			Eventually(string(session.Err.Contents())).Should(ContainSubstring("Get mashed://potatoes/api/v1/data?path=: unsupported protocol scheme \"mashed\""))
		})

		It("prints an error if the specified output file cannot be opened", func() {
			noCredsJson := `{ "credentials" : [] }`

			server.AppendHandlers(
				CombineHandlers(
					VerifyRequest("GET", "/api/v1/data", "path="),
					RespondWith(http.StatusOK, noCredsJson),
				),
			)

			session := runCommand("export", "-f", "this/should/not/exist/anywhere")

			Eventually(session).Should(Exit(1))
			if runtime.GOOS == "windows" {
				Eventually(string(session.Err.Contents())).Should(ContainSubstring("open this/should/not/exist/anywhere: The system cannot find the path specified"))
			} else {
				Eventually(string(session.Err.Contents())).Should(ContainSubstring("open this/should/not/exist/anywhere: no such file or directory"))
			}
		})
	})
})
