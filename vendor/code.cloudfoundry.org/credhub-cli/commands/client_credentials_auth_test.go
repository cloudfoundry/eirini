package commands_test

import (
	"net/http"

	"strings"

	"code.cloudfoundry.org/credhub-cli/config"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gexec"
	. "github.com/onsi/gomega/ghttp"
)

var _ = Describe("Find", func() {
	Describe("authenticating and targeting without calling login/api commands", func() {
		It("successfully authenticates", func() {
			config.WriteConfig(config.Config{})

			responseJson := `{
			"credentials": []
			}`

			server.RouteToHandler("GET", "/info",
				RespondWith(http.StatusOK, `{
				"app":{"name":"CredHub"},
				"auth-server":{"url":"`+authServer.URL()+`"}
				}`),
			)

			server.RouteToHandler("GET", "/api/v1/data",
				CombineHandlers(
					VerifyRequest("GET", "/api/v1/data"),
					RespondWith(http.StatusOK, responseJson),
				),
			)

			authServer.RouteToHandler("POST", "/oauth/token",
				CombineHandlers(
					VerifyBody([]byte(`client_id=test_client&client_secret=test_secret&grant_type=client_credentials&response_type=token`)),
					RespondWith(http.StatusOK, `{
						"access_token":"2YotnFZFEjr1zCsicMWpAA",
						"refresh_token":"erousflkajqwer",
						"token_type":"bearer",
						"expires_in":3600}`),
				),
			)

			session := runCommandWithEnv([]string{"CREDHUB_CA_CERT=../test/server-and-auth-stacked-cert.pem", "CREDHUB_CLIENT=test_client", "CREDHUB_SECRET=test_secret", "CREDHUB_SERVER=" + server.URL()}, "find")

			Eventually(session).Should(Exit(0))
			Eventually(string(session.Out.Contents())).Should(Equal("credentials: []\n\n"))
		})

	})

	Describe("authenticating with username and password", func() {
		It("refreshes the token when it expires", func() {
			expiredAccessToken := "2YotnFZFEjr1zCsicMWpAA"
			newAccessToken := "3YotnFZFEjr1zCsicMWpAA"

			config.WriteConfig(
				config.Config{
					ConfigWithoutSecrets: config.ConfigWithoutSecrets{
						ApiURL:       server.URL(),
						AuthURL:      authServer.URL(),
						AccessToken:  expiredAccessToken,
						RefreshToken: "erousflkajqwer",
					},
				},
			)

			responseJson := `{
			"credentials": []
			}`

			server.RouteToHandler("GET", "/info",
				RespondWith(http.StatusOK, `{
				"app":{"name":"CredHub"},
				"auth-server":{"url":"`+authServer.URL()+`"}
				}`),
			)

			server.RouteToHandler("GET", "/api/v1/data", func(w http.ResponseWriter, r *http.Request) {
				if strings.HasSuffix(r.Header.Get("Authorization"), expiredAccessToken) {
					CombineHandlers(
						VerifyRequest("GET", "/api/v1/data"),
						RespondWith(http.StatusUnauthorized, `{"error": "access_token_expired"}`),
					)(w, r)
				} else if strings.HasSuffix(r.Header.Get("Authorization"), newAccessToken) {
					CombineHandlers(
						VerifyRequest("GET", "/api/v1/data"),
						RespondWith(http.StatusOK, responseJson),
					)(w, r)
				} else {
					RespondWith(http.StatusBadRequest, `{"error": "Invalid access token"}`)
				}
			})

			authServer.RouteToHandler("POST", "/oauth/token",
				CombineHandlers(
					VerifyBody([]byte(`client_id=credhub_cli&client_secret=&grant_type=refresh_token&refresh_token=erousflkajqwer&response_type=token`)),
					RespondWith(http.StatusOK, `{
						"access_token":"`+newAccessToken+`",
						"refresh_token":"erousflkajqwer",
						"token_type":"bearer"}`),
				),
			)

			session := runCommandWithEnv([]string{"CREDHUB_CA_CERT=../test/server-and-auth-stacked-cert.pem"}, "find")

			Eventually(session).Should(Exit(0))
			Eventually(string(session.Out.Contents())).Should(Equal("credentials: []\n\n"))
		})
	})
})
