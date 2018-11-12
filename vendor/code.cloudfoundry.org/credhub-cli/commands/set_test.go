package commands_test

import (
	"fmt"
	"net/http"
	"os"
	"runtime"
	"strings"

	"code.cloudfoundry.org/credhub-cli/commands"
	"code.cloudfoundry.org/credhub-cli/test"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gbytes"
	. "github.com/onsi/gomega/gexec"
	. "github.com/onsi/gomega/ghttp"
)

var _ = Describe("Set", func() {
	BeforeEach(func() {
		login()
	})

	ItRequiresAuthentication("set", "-n", "test-credential", "-t", "password", "-w", "value")
	ItRequiresAnAPIToBeSet("set", "-n", "test-credential", "-t", "password", "-w", "value")
	ItAutomaticallyLogsIn("PUT", "set_response.json", "/api/v1/data", "set", "-n", "test-credential", "-t", "password", "-w", "test-value")

	Describe("not specifying type", func() {
		It("returns an error", func() {
			session := runCommand("set", "-n", "my-password", "-w", "potatoes")

			Eventually(session).Should(Exit(1))
			Eventually(session.Err).Should(Say("A type must be specified when setting a credential. Valid types include 'value', 'json', 'password', 'user', 'certificate', 'ssh' and 'rsa'."))
		})
	})

	Describe("setting value secrets", func() {
		It("puts a secret using explicit value type", func() {
			SetupPutValueServer("my-value", "value", "potatoes")

			session := runCommand("set", "-n", "my-value", "-v", "potatoes", "-t", "value")

			Eventually(session).Should(Exit(0))
			Eventually(string(session.Out.Contents())).Should(ContainSubstring("name: my-value"))
			Eventually(string(session.Out.Contents())).Should(ContainSubstring("type: value"))
			Eventually(string(session.Out.Contents())).Should(ContainSubstring("value: <redacted>"))
		})

		It("escapes special characters in the value", func() {
			SetupPutValueServer("my-character-test", "value", `{\"password\":\"some-still-bad-password\"}`)

			session := runCommand("set", "-t", "value", "-n", "my-character-test", "-v", `{"password":"some-still-bad-password"}`)

			Eventually(session).Should(Exit(0))
			Eventually(string(session.Out.Contents())).Should(ContainSubstring("name: my-character-test"))
			Eventually(string(session.Out.Contents())).Should(ContainSubstring("type: value"))
			Eventually(string(session.Out.Contents())).Should(ContainSubstring(`value: <redacted>`))
		})

		It("puts a secret using explicit value type and returns in json format", func() {
			SetupPutValueServer("my-value", "value", "potatoes")

			session := runCommand("set", "-n", "my-value", "-v", "potatoes", "-t", "value", "--output-json")

			Eventually(session).Should(Exit(0))
			Eventually(string(session.Out.Contents())).Should(MatchJSON(responseSetMyValuePotatoesJson))
		})

		It("accepts case-insensitive type", func() {
			SetupPutValueServer("my-value", "value", "potatoes")

			session := runCommand("set", "-n", "my-value", "-v", "potatoes", "-t", "VALUE", "--output-json")

			Eventually(session).Should(Exit(0))
			Eventually(string(session.Out.Contents())).Should(MatchJSON(responseSetMyValuePotatoesJson))
		})
	})

	Describe("setting json secrets", func() {
		It("puts a secret using explicit json type", func() {
			jsonValue := `{"foo":"bar","nested":{"a":1},"an":["array"]}`
			setupPutJsonServer("json-secret", jsonValue)

			session := runCommand("set", "-n", "json-secret", "-v", jsonValue, "-t", "json")

			Eventually(session).Should(Exit(0))
			Eventually(string(session.Out.Contents())).Should(ContainSubstring("name: json-secret"))
			Eventually(string(session.Out.Contents())).Should(ContainSubstring("type: json"))
			Eventually(string(session.Out.Contents())).Should(ContainSubstring("value: <redacted>"))
		})

		It("escapes special characters in the json", func() {
			setupPutJsonServer("my-character-test", `{"foo":"b\"ar"}`)

			session := runCommand("set", "-t", "json", "-n", "my-character-test", "-v", `{"foo":"b\"ar"}`)

			Eventually(session).Should(Exit(0))
			Eventually(string(session.Out.Contents())).Should(ContainSubstring("name: my-character-test"))
			Eventually(string(session.Out.Contents())).Should(ContainSubstring("type: json"))
			Eventually(string(session.Out.Contents())).Should(ContainSubstring("value: <redacted>"))
		})

		It("puts a secret using explicit json type and returns in json format", func() {
			jsonValue := `{"foo":"bar","nested":{"a":1},"an":["array"]}`
			setupPutJsonServer("json-secret", jsonValue)

			session := runCommand("set", "-n", "json-secret", "-v", jsonValue, "-t", "json", "--output-json")

			Eventually(session).Should(Exit(0))
			responseJson := `{
			"id": "5a2edd4f-1686-4c8d-80eb-5daa866f9f86",
			"name": "json-secret",
			"type": "json",
			"value": "<redacted>",
			"version_created_at": "2016-01-01T12:00:00Z"
			}`
			Eventually(string(session.Out.Contents())).Should(MatchJSON(responseJson))
		})

		It("accepts case-insensitive type", func() {
			jsonValue := `{"foo":"bar","nested":{"a":1},"an":["array"]}`
			setupPutJsonServer("json-secret", jsonValue)

			session := runCommand("set", "-n", "json-secret", "-v", jsonValue, "-t", "JSON")

			Eventually(session).Should(Exit(0))
			Eventually(string(session.Out.Contents())).Should(ContainSubstring("name: json-secret"))
			Eventually(string(session.Out.Contents())).Should(ContainSubstring("type: json"))
			Eventually(string(session.Out.Contents())).Should(ContainSubstring("value: <redacted>"))
		})
	})

	Describe("setting SSH secrets", func() {
		It("puts a secret using explicit ssh type", func() {
			SetupPutSshServer("foo-ssh-key", "ssh", "some-public-key", "some-private-key")

			session := runCommand("set", "-n", "foo-ssh-key", "-u", "some-public-key", "-p", "some-private-key", "-t", "ssh")

			Eventually(session).Should(Exit(0))
			Eventually(string(session.Out.Contents())).Should(ContainSubstring("name: foo-ssh-key"))
			Eventually(string(session.Out.Contents())).Should(ContainSubstring("type: ssh"))
			Eventually(string(session.Out.Contents())).Should(ContainSubstring("value: <redacted>"))
		})

		It("puts a secret using values read from files", func() {
			SetupPutSshServer("foo-ssh-key", "ssh", "some-public-key", "some-private-key")

			tempDir := test.CreateTempDir("sshFilesForTesting")
			publicFileName := test.CreateCredentialFile(tempDir, "rsa.pub", "some-public-key")
			privateFilename := test.CreateCredentialFile(tempDir, "rsa.key", "some-private-key")

			session := runCommand("set", "-n", "foo-ssh-key",
				"-t", "ssh",
				"-u", publicFileName,
				"-p", privateFilename)

			os.RemoveAll(tempDir)
			Eventually(session).Should(Exit(0))
			Eventually(string(session.Out.Contents())).Should(ContainSubstring("name: foo-ssh-key"))
			Eventually(string(session.Out.Contents())).Should(ContainSubstring("type: ssh"))
			Eventually(string(session.Out.Contents())).Should(ContainSubstring("value: <redacted>"))
		})

		It("puts a secret using explicit ssh type and returns in json format", func() {
			SetupPutSshServer("foo-ssh-key", "ssh", "some-public-key", "some-private-key")

			session := runCommand("set", "-n", "foo-ssh-key", "-u", "some-public-key", "-p", "some-private-key", "-t", "ssh", "--output-json")

			Eventually(session).Should(Exit(0))
			responseJson := `{
        	"id": "5a2edd4f-1686-4c8d-80eb-5daa866f9f86",
        	"name": "foo-ssh-key",
        	"type": "ssh",
        	"value": "<redacted>",
        	"version_created_at": "2016-01-01T12:00:00Z"
        	}`
			Eventually(string(session.Out.Contents())).Should(MatchJSON(responseJson))
		})

		It("accepts case-insensitive type", func() {
			SetupPutSshServer("foo-ssh-key", "ssh", "some-public-key", "some-private-key")

			session := runCommand("set", "-n", "foo-ssh-key", "-u", "some-public-key", "-p", "some-private-key", "-t", "SSH")

			Eventually(session).Should(Exit(0))
			Eventually(string(session.Out.Contents())).Should(ContainSubstring("name: foo-ssh-key"))
			Eventually(string(session.Out.Contents())).Should(ContainSubstring("type: ssh"))
			Eventually(string(session.Out.Contents())).Should(ContainSubstring("value: <redacted>"))
		})

		It("handles newline characters", func() {
			SetupPutSshServer("foo-ssh-key", "ssh", `some\npublic\nkey`, `some\nprivate\nkey`)
			session := runCommand("set", "-n", "foo-ssh-key", "-u", `some\npublic\nkey`, "-p", `some\nprivate\nkey`, "-t", "ssh", "--output-json")

			responseJson := `{
        	"id": "5a2edd4f-1686-4c8d-80eb-5daa866f9f86",
        	"name": "foo-ssh-key",
        	"type": "ssh",
        	"value": "<redacted>",
        	"version_created_at": "2016-01-01T12:00:00Z"
        	}`

			Eventually(session).Should(Exit(0))
			Eventually(string(session.Out.Contents())).Should(MatchJSON(responseJson))
		})
	})

	Describe("setting RSA secrets", func() {
		It("puts a secret using explicit rsa type", func() {
			SetupPutRsaServer("foo-rsa-key", "rsa", "some-public-key", "some-private-key")

			session := runCommand("set", "-n", "foo-rsa-key", "-u", "some-public-key", "-p", "some-private-key", "-t", "rsa")

			Eventually(session).Should(Exit(0))
			Eventually(string(session.Out.Contents())).Should(ContainSubstring("name: foo-rsa-key"))
			Eventually(string(session.Out.Contents())).Should(ContainSubstring("type: rsa"))
			Eventually(string(session.Out.Contents())).Should(ContainSubstring("value: <redacted>"))
		})

		It("puts a secret using values read from files", func() {
			SetupPutRsaServer("foo-rsa-key", "rsa", "some-public-key", "some-private-key")

			tempDir := test.CreateTempDir("rsaFilesForTesting")
			publicFileName := test.CreateCredentialFile(tempDir, "rsa.pub", "some-public-key")
			privateFilename := test.CreateCredentialFile(tempDir, "rsa.key", "some-private-key")

			session := runCommand("set", "-n", "foo-rsa-key",
				"-t", "rsa",
				"-u", publicFileName,
				"-p", privateFilename)

			os.RemoveAll(tempDir)
			Eventually(session).Should(Exit(0))
			Eventually(string(session.Out.Contents())).Should(ContainSubstring("name: foo-rsa-key"))
			Eventually(string(session.Out.Contents())).Should(ContainSubstring("type: rsa"))
			Eventually(string(session.Out.Contents())).Should(ContainSubstring("value: <redacted>"))
		})

		It("puts a secret using explicit rsa type and returns in json format", func() {
			SetupPutRsaServer("foo-rsa-key", "rsa", "some-public-key", "some-private-key")

			session := runCommand("set", "-n", "foo-rsa-key", "-u", "some-public-key", "-p", "some-private-key", "-t", "rsa", "--output-json")

			Eventually(session).Should(Exit(0))
			Eventually(string(session.Out.Contents())).Should(ContainSubstring(`"name": "foo-rsa-key"`))
			Eventually(string(session.Out.Contents())).Should(ContainSubstring(`"type": "rsa"`))
			Eventually(string(session.Out.Contents())).Should(ContainSubstring(`"value": "<redacted>"`))

		})

		It("accepts case-insensitive type", func() {
			SetupPutRsaServer("foo-rsa-key", "rsa", "some-public-key", "some-private-key")

			session := runCommand("set", "-n", "foo-rsa-key", "-u", "some-public-key", "-p", "some-private-key", "-t", "RSA")

			Eventually(session).Should(Exit(0))
			Eventually(string(session.Out.Contents())).Should(ContainSubstring("name: foo-rsa-key"))
			Eventually(string(session.Out.Contents())).Should(ContainSubstring("type: rsa"))
			Eventually(string(session.Out.Contents())).Should(ContainSubstring("value: <redacted>"))
		})

		It("handles newline characters", func() {
			SetupPutRsaServer("foo-rsa-key", "rsa", `some\npublic\nkey`, `some\nprivate\nkey`)
			session := runCommand("set", "-n", "foo-rsa-key", "-u", `some\npublic\nkey`, "-p", `some\nprivate\nkey`, "-t", "rsa", "--output-json")

			Eventually(session).Should(Exit(0))
			Expect(string(session.Out.Contents())).Should(MatchJSON(responseSetMyRSAWithNewlinesJson))
		})
	})

	Describe("setting password secrets", func() {

		It("puts a secret using explicit password type  and returns in yaml format", func() {
			SetupPutValueServer("my-password", "password", "potatoes")

			session := runCommand("set", "-n", "my-password", "-w", "potatoes", "-t", "password")

			Eventually(session).Should(Exit(0))
			Eventually(string(session.Out.Contents())).Should(ContainSubstring("name: my-password"))
			Eventually(string(session.Out.Contents())).Should(ContainSubstring("type: password"))
			Eventually(string(session.Out.Contents())).Should(ContainSubstring("value: <redacted>"))
		})

		It("prompts for value if value is not provided", func() {
			SetupPutValueServer("my-password", "password", "potatoes")

			session := runCommandWithStdin(strings.NewReader("potatoes\n"), "set", "-n", "my-password", "-t", "password")

			Eventually(string(session.Out.Contents())).Should(ContainSubstring("password: ********"))
			Eventually(session).Should(Exit(0))
			Eventually(string(session.Out.Contents())).Should(ContainSubstring("name: my-password"))
			Eventually(string(session.Out.Contents())).Should(ContainSubstring("type: password"))
			Eventually(string(session.Out.Contents())).Should(ContainSubstring("value: <redacted>"))
		})

		It("can set password that contains spaces interactively", func() {
			SetupPutValueServer("my-password", "password", "potatoes potatoes")

			session := runCommandWithStdin(strings.NewReader("potatoes potatoes\n"), "set", "-n", "my-password", "-t", "password")

			Eventually(string(session.Out.Contents())).Should(ContainSubstring("password:"))
			Eventually(session).Should(Exit(0))
			Eventually(string(session.Out.Contents())).Should(ContainSubstring("name: my-password"))
			Eventually(string(session.Out.Contents())).Should(ContainSubstring("type: password"))
			Eventually(string(session.Out.Contents())).Should(ContainSubstring("value: <redacted>"))
		})

		It("escapes special characters in the password", func() {
			SetupPutValueServer("my-character-test", "password", `{\"password\":\"some-still-bad-password\"}`)

			session := runCommand("set", "-t", "password", "-n", "my-character-test", "-w", `{"password":"some-still-bad-password"}`)

			Eventually(session).Should(Exit(0))
			Eventually(string(session.Out.Contents())).Should(ContainSubstring(`value: <redacted>`))
		})

		It("puts a secret using explicit password type and returns in json format", func() {
			SetupPutValueServer("my-password", "password", "potatoes")

			session := runCommand("set", "-n", "my-password", "-w", "potatoes", "-t", "password", "--output-json")

			Eventually(session).Should(Exit(0))
			Eventually(string(session.Out.Contents())).Should(MatchJSON(responseSetMyPasswordPotatoesJson))
		})

		It("accepts case-insensitive type", func() {
			SetupPutValueServer("my-password", "password", "potatoes")

			session := runCommand("set", "-n", "my-password", "-w", "potatoes", "-t", "PASSWORD")

			Eventually(session).Should(Exit(0))
			Eventually(string(session.Out.Contents())).Should(ContainSubstring("name: my-password"))
			Eventually(string(session.Out.Contents())).Should(ContainSubstring("type: password"))
			Eventually(string(session.Out.Contents())).Should(ContainSubstring("value: <redacted>"))
		})
	})

	Describe("setting certificate secrets", func() {
		It("puts a secret using explicit certificate type and string values", func() {
			SetupPutCertificateServer("my-secret", "my-ca", "my-cert", "my-priv")

			session := runCommand("set", "-n", "my-secret",
				"-t", "certificate", "--root", "my-ca",
				"--certificate", "my-cert", "--private", "my-priv")

			Eventually(session).Should(Exit(0))
			Eventually(string(session.Out.Contents())).Should(ContainSubstring("name: my-secret"))
			Eventually(string(session.Out.Contents())).Should(ContainSubstring("type: certificate"))
			Eventually(string(session.Out.Contents())).Should(ContainSubstring("value: <redacted>"))
		})

		It("puts a secret using explicit certificate type, string values, and certificate authority name", func() {
			SetupPutCertificateWithCaNameServer("my-secret", "my-ca", "my-cert", "my-priv")

			session := runCommand("set", "-n", "my-secret",
				"-t", "certificate", "--ca-name", "my-ca",
				"--certificate", "my-cert", "--private", "my-priv")

			Eventually(session).Should(Exit(0))
			Eventually(string(session.Out.Contents())).Should(ContainSubstring("name: my-secret"))
			Eventually(string(session.Out.Contents())).Should(ContainSubstring("type: certificate"))
			Eventually(string(session.Out.Contents())).Should(ContainSubstring("value: <redacted>"))
		})

		It("puts a secret using explicit certificate type and values read from files", func() {
			SetupPutCertificateServer("my-secret", "my-ca", "my-cert", "my-priv")
			tempDir := test.CreateTempDir("certFilesForTesting")
			caFilename := test.CreateCredentialFile(tempDir, "ca.txt", "my-ca")
			certificateFilename := test.CreateCredentialFile(tempDir, "certificate.txt", "my-cert")
			privateFilename := test.CreateCredentialFile(tempDir, "private.txt", "my-priv")

			session := runCommand("set", "-n", "my-secret",
				"-t", "certificate", "--root", caFilename,
				"--certificate", certificateFilename, "--private", privateFilename)

			os.RemoveAll(tempDir)
			Eventually(session).Should(Exit(0))
			Eventually(string(session.Out.Contents())).Should(ContainSubstring("name: my-secret"))
			Eventually(string(session.Out.Contents())).Should(ContainSubstring("type: certificate"))
			Eventually(string(session.Out.Contents())).Should(ContainSubstring("value: <redacted>"))
		})

		if runtime.GOOS != "windows" {
			It("fails to put a secret when reading from unreadable file", func() {
				testSetFileFailure("unreadable.txt", "", "")
				testSetFileFailure("", "unreadable.txt", "")
				testSetFileFailure("", "", "unreadable.txt")
			})
		}

		It("puts a secret using explicit certificate type and string values in json format", func() {
			SetupPutCertificateServer("my-secret", "my-ca", "my-cert", "my-priv")

			session := runCommand("set", "-n", "my-secret",
				"-t", "certificate", "--root", "my-ca",
				"--certificate", "my-cert", "--private", "my-priv", "--output-json")

			Eventually(session).Should(Exit(0))
			Eventually(string(session.Out.Contents())).Should(ContainSubstring(`"name": "my-secret"`))
			Eventually(string(session.Out.Contents())).Should(ContainSubstring(`"type": "certificate"`))
			Eventually(string(session.Out.Contents())).Should(ContainSubstring(`"value": "<redacted>"`))
		})

		It("accepts case insensitive type", func() {
			SetupPutCertificateServer("my-secret", "my-ca", "my-cert", "my-priv")

			session := runCommand("set", "-n", "my-secret",
				"-t", "CERTIFICATE", "--root", "my-ca",
				"--certificate", "my-cert", "--private", "my-priv")

			Eventually(session).Should(Exit(0))
			Eventually(string(session.Out.Contents())).Should(ContainSubstring("name: my-secret"))
			Eventually(string(session.Out.Contents())).Should(ContainSubstring("type: certificate"))
			Eventually(string(session.Out.Contents())).Should(ContainSubstring("value: <redacted>"))
		})

		It("handles newline characters", func() {
			SetupPutCertificateServer("my-secret", `my\nca`, `my\ncert`, `my\npriv`)
			session := runCommand("set", "-n", "my-secret",
				"-t", "certificate", "--root", `my\nca`,
				"--certificate", `my\ncert`, "--private", `my\npriv`, "--output-json")
			Eventually(session).Should(Exit(0))
			Expect(string(session.Out.Contents())).Should(MatchJSON(responseSetMyCertificateWithNewlinesJson))
		})
	})

	Describe("setting User secrets", func() {
		It("puts a secret using explicit user type", func() {
			SetupPutUserServer("my-username-credential", `{"username": "my-username", "password": "test-password"}`, "my-username", "test-password", "passw0rd-H4$h")

			session := runCommand("set", "-n", "my-username-credential", "-z", "my-username", "-w", "test-password", "-t", "user")

			Eventually(session).Should(Exit(0))
			Eventually(string(session.Out.Contents())).Should(ContainSubstring("name: my-username-credential"))
			Eventually(string(session.Out.Contents())).Should(ContainSubstring("type: user"))
			Eventually(string(session.Out.Contents())).Should(ContainSubstring("value: <redacted>"))
			Eventually(string(session.Out.Contents())).Should(ContainSubstring(`version_created_at: "2016-01-01T12:00:00Z"`))
		})

		It("should set password interactively for user", func() {
			SetupPutUserServer("my-username-credential", `{"username": "my-username", "password": "test-password"}`, "my-username", "test-password", "passw0rd-H4$h")

			session := runCommandWithStdin(strings.NewReader("test-password\n"), "set", "-n", "my-username-credential", "-t", "user", "--username", "my-username")

			Eventually(string(session.Out.Contents())).Should(ContainSubstring("name: my-username-credential"))
			Eventually(string(session.Out.Contents())).Should(ContainSubstring("type: user"))
			Eventually(string(session.Out.Contents())).Should(ContainSubstring("value: <redacted>"))
			Eventually(session).Should(Exit(0))
		})

		It("should set null username when it isn't provided", func() {
			SetupPutUserWithoutUsernameServer("my-username-credential", `{"username":"","password": "test-password"}`, "test-password", "passw0rd-H4$h")

			session := runCommandWithStdin(strings.NewReader("test-password\n"), "set", "-n", "my-username-credential", "-t", "user")

			//response := fmt.Sprintf(USER_WITHOUT_USERNAME_CREDENTIAL_RESPONSE_YAML, "my-username-credential", "test-password", "passw0rd-H4$h")

			Eventually(session).Should(Exit(0))
			Eventually(string(session.Out.Contents())).Should(ContainSubstring("name: my-username-credential"))
			Eventually(string(session.Out.Contents())).Should(ContainSubstring("type: user"))
			Eventually(string(session.Out.Contents())).Should(ContainSubstring("value: <redacted>"))
		})

		It("puts a secret using explicit user type in json format", func() {
			SetupPutUserServer("my-username-credential", `{"username": "my-username", "password": "test-password"}`, "my-username", "test-password", "passw0rd-H4$h")

			session := runCommand("set", "-n", "my-username-credential", "-z", "my-username", "-w", "test-password", "-t", "user",
				"--output-json")

			Eventually(session).Should(Exit(0))
			Eventually(string(session.Out.Contents())).Should(ContainSubstring(`"name": "my-username-credential"`))
			Eventually(string(session.Out.Contents())).Should(ContainSubstring(`"type": "user"`))
			Eventually(string(session.Out.Contents())).Should(ContainSubstring(`"value": "<redacted>"`))
			Eventually(string(session.Out.Contents())).Should(ContainSubstring(`"version_created_at": "2016-01-01T12:00:00Z"`))
		})

		It("accepts case-insensitive type", func() {
			SetupPutUserServer("my-username-credential", `{"username": "my-username", "password": "test-password"}`, "my-username", "test-password", "passw0rd-H4$h")

			session := runCommand("set", "-n", "my-username-credential", "-z", "my-username", "-w", "test-password", "-t", "USER")

			Eventually(session).Should(Exit(0))
			Eventually(string(session.Out.Contents())).Should(ContainSubstring("name: my-username-credential"))
			Eventually(string(session.Out.Contents())).Should(ContainSubstring("type: user"))
			Eventually(string(session.Out.Contents())).Should(ContainSubstring("value: <redacted>"))
			Eventually(string(session.Out.Contents())).Should(ContainSubstring(`version_created_at: "2016-01-01T12:00:00Z"`))
		})
	})

	Describe("Help", func() {
		It("short flags", func() {
			Expect(commands.SetCommand{}).To(SatisfyAll(
				commands.HaveFlag("name", "n"),
				commands.HaveFlag("type", "t"),
				commands.HaveFlag("value", "v"),
				commands.HaveFlag("root", "r"),
				commands.HaveFlag("certificate", "c"),
				commands.HaveFlag("private", "p"),
			))
		})

		ItBehavesLikeHelp("set", "s", func(session *Session) {
			Expect(session.Err).To(Say("set"))
			Expect(session.Err).To(Say("name"))
			Expect(session.Err).To(Say("credential"))
		})

		It("displays missing 'n' option as required parameter", func() {
			session := runCommand("set", "-v", "potatoes")

			Eventually(session).Should(Exit(1))
			if runtime.GOOS == "windows" {
				Expect(session.Err).To(Say("the required flag `/n, /name' was not specified"))
			} else {
				Expect(session.Err).To(Say("the required flag `-n, --name' was not specified"))
			}
		})

		It("displays the server provided error when an error is received", func() {
			server.AppendHandlers(
				RespondWith(http.StatusBadRequest, `{"error": "test error"}`),
			)

			session := runCommand("set", "-n", "my-value", "-t", "value", "-v", "tomatoes")

			Eventually(session).Should(Exit(1))

			Expect(session.Err).To(Say("test error"))
		})
	})
})

func SetupPutRsaServer(name, keyType, publicKey, privateKey string) {
	var jsonRequest string
	jsonRequest = fmt.Sprintf(RSA_SSH_CREDENTIAL_REQUEST_JSON, keyType, name, publicKey, privateKey)
	server.AppendHandlers(
		CombineHandlers(
			VerifyRequest("PUT", "/api/v1/data"),
			VerifyJSON(jsonRequest),
			RespondWith(http.StatusOK, fmt.Sprintf(RSA_CREDENTIAL_RESPONSE_JSON, keyType, name, publicKey, privateKey)),
		),
	)
}

func SetupPutSshServer(name, keyType, publicKey, privateKey string) {
	var jsonRequest string
	jsonRequest = fmt.Sprintf(RSA_SSH_CREDENTIAL_REQUEST_JSON, keyType, name, publicKey, privateKey)
	server.AppendHandlers(
		CombineHandlers(
			VerifyRequest("PUT", "/api/v1/data"),
			VerifyJSON(jsonRequest),
			RespondWith(http.StatusOK, fmt.Sprintf(SSH_CREDENTIAL_RESPONSE_JSON, keyType, name, publicKey, privateKey)),
		),
	)
}

func SetupPutValueServer(name, credentialType, value string) {
	var jsonRequest string
	jsonRequest = fmt.Sprintf(STRING_CREDENTIAL_REQUEST_JSON, credentialType, name, value)
	server.AppendHandlers(
		CombineHandlers(
			VerifyRequest("PUT", "/api/v1/data"),
			VerifyJSON(jsonRequest),
			RespondWith(http.StatusOK, fmt.Sprintf(STRING_CREDENTIAL_RESPONSE_JSON, credentialType, name, value)),
		),
	)
}

func setupPutJsonServer(name, value string) {
	var jsonRequest string
	jsonRequest = fmt.Sprintf(JSON_CREDENTIAL_REQUEST_JSON, name, value)
	server.AppendHandlers(
		CombineHandlers(
			VerifyRequest("PUT", "/api/v1/data"),
			VerifyJSON(jsonRequest),
			RespondWith(http.StatusOK, fmt.Sprintf(JSON_CREDENTIAL_RESPONSE_JSON, name, value)),
		),
	)
}

func SetupPutCertificateServer(name, ca, cert, priv string) {
	var jsonRequest string
	jsonRequest = fmt.Sprintf(CERTIFICATE_CREDENTIAL_REQUEST_JSON, name, ca, cert, priv)
	server.AppendHandlers(
		CombineHandlers(
			VerifyRequest("PUT", "/api/v1/data"),
			VerifyJSON(jsonRequest),
			RespondWith(http.StatusOK, fmt.Sprintf(CERTIFICATE_CREDENTIAL_RESPONSE_JSON, name, ca, cert, priv)),
		),
	)
}

func SetupPutCertificateWithCaNameServer(name, caName, cert, priv string) {
	var jsonRequest string
	jsonRequest = fmt.Sprintf(CERTIFICATE_CREDENTIAL_WITH_NAMED_CA_REQUEST_JSON, name, caName, cert, priv)
	server.AppendHandlers(
		CombineHandlers(
			VerifyRequest("PUT", "/api/v1/data"),
			VerifyJSON(jsonRequest),
			RespondWith(http.StatusOK, fmt.Sprintf(CERTIFICATE_CREDENTIAL_RESPONSE_JSON, name, "known-ca-value", cert, priv)),
		),
	)
}

func SetupPutUserServer(name, value, username, password, passwordHash string) {
	var jsonRequest string
	jsonRequest = fmt.Sprintf(USER_SET_CREDENTIAL_REQUEST_JSON, name, value)
	server.AppendHandlers(
		CombineHandlers(
			VerifyRequest("PUT", "/api/v1/data"),
			VerifyJSON(jsonRequest),
			RespondWith(http.StatusOK, fmt.Sprintf(USER_CREDENTIAL_RESPONSE_JSON, name, username, password, passwordHash)),
		),
	)
}

func SetupPutUserWithoutUsernameServer(name, value, password, passwordHash string) {
	var jsonRequest string
	jsonRequest = fmt.Sprintf(USER_SET_CREDENTIAL_REQUEST_JSON, name, value)
	server.AppendHandlers(
		CombineHandlers(
			VerifyRequest("PUT", "/api/v1/data"),
			VerifyJSON(jsonRequest),
			RespondWith(http.StatusOK, fmt.Sprintf(USER_WITHOUT_USERNAME_CREDENTIAL_RESPONSE_JSON, name, password, passwordHash)),
		),
	)
}

func SetupPutBadRequestServer(body string) {
	server.AppendHandlers(
		CombineHandlers(
			VerifyRequest("PUT", "/api/v1/data"),
			VerifyJSON(body),
			RespondWith(http.StatusBadRequest, `{"error":"test error"}`),
		),
	)
}

func testSetFileFailure(caFilename, certificateFilename, privateFilename string) {
	tempDir := test.CreateTempDir("certFilesForTesting")
	if caFilename == "unreadable.txt" {
		caFilename = test.CreateCredentialFile(tempDir, caFilename, "my-ca")
		err := os.Chmod(caFilename, 0222)
		Expect(err).To(BeNil())
	}
	if certificateFilename == "unreadable.txt" {
		certificateFilename = test.CreateCredentialFile(tempDir, certificateFilename, "my-cert")
		err := os.Chmod(certificateFilename, 0222)
		Expect(err).To(BeNil())
	}
	if privateFilename == "unreadable.txt" {
		privateFilename = test.CreateCredentialFile(tempDir, privateFilename, "my-priv")
		err := os.Chmod(privateFilename, 0222)
		Expect(err).To(BeNil())
	}

	session := runCommand("set", "-n", "my-secret",
		"-t", "certificate", "--root", caFilename,
		"--certificate", certificateFilename, "--private", privateFilename)

	os.RemoveAll(tempDir)
	Eventually(session).Should(Exit(1))
	Eventually(session.Err).Should(Say("A referenced file could not be opened. Please validate the provided filenames and permissions, then retry your request."))
}
