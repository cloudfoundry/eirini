package uaa_test

import (
	"net/http"
	"net/http/httptest"

	. "github.com/cloudfoundry-incubator/credhub-cli/credhub/auth/uaa"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
)

var _ = Describe("Client", func() {
	Context("ClientCredentialGrant()", func() {
		It("should make a token grant request", func() {
			uaaServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				r.ParseForm()

				Expect(r.Method).To(Equal(http.MethodPost))

				Expect(r.URL.Path).To(Equal("/oauth/token"))

				Expect(r.Header.Get("Accept")).To(Equal("application/json"))
				Expect(r.Header.Get("Content-Type")).To(Equal("application/x-www-form-urlencoded"))

				Expect(r.PostForm.Get("grant_type")).To(Equal("client_credentials"))
				Expect(r.PostForm.Get("response_type")).To(Equal("token"))

				Expect(r.PostForm.Get("client_id")).To(Equal("client-id"))
				Expect(r.PostForm.Get("client_secret")).To(Equal("client-secret"))

				w.Write([]byte(`{"access_token": "access-token", "token_type": "bearer"}`))
			}))
			defer uaaServer.Close()

			client := Client{
				AuthURL: uaaServer.URL,
				Client:  http.DefaultClient,
			}

			accessToken, err := client.ClientCredentialGrant("client-id", "client-secret")

			Expect(err).ToNot(HaveOccurred())
			Expect(accessToken).To(Equal("access-token"))
		})
	})

	Context("PasswordGrant()", func() {
		It("should make a password grant token request", func() {
			uaaServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				r.ParseForm()

				Expect(r.Method).To(Equal(http.MethodPost))

				Expect(r.URL.Path).To(Equal("/oauth/token"))

				Expect(r.Header.Get("Accept")).To(Equal("application/json"))
				Expect(r.Header.Get("Content-Type")).To(Equal("application/x-www-form-urlencoded"))

				Expect(r.PostForm.Get("grant_type")).To(Equal("password"))
				Expect(r.PostForm.Get("response_type")).To(Equal("token"))

				Expect(r.PostForm.Get("username")).To(Equal("username"))
				Expect(r.PostForm.Get("password")).To(Equal("password"))

				Expect(r.PostForm.Get("client_id")).To(Equal("some-client-id"))
				Expect(r.PostForm.Get("client_secret")).To(Equal("some-client-secret"))

				w.Write([]byte(`{"access_token": "some-access-token", "refresh_token": "some-refresh-token", "token_type": "bearer"}`))
			}))
			defer uaaServer.Close()

			client := Client{
				AuthURL: uaaServer.URL,
				Client:  http.DefaultClient,
			}

			accessToken, refreshToken, err := client.PasswordGrant("some-client-id", "some-client-secret", "username", "password")

			Expect(err).To(BeNil())
			Expect(accessToken).To(Equal("some-access-token"))
			Expect(refreshToken).To(Equal("some-refresh-token"))
		})
	})

	Context("PasscodeGrant()", func() {
		It("should make a passcode grant token request", func() {
			uaaServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				r.ParseForm()

				Expect(r.Method).To(Equal(http.MethodPost))

				Expect(r.URL.Path).To(Equal("/oauth/token"))

				Expect(r.Header.Get("Accept")).To(Equal("application/json"))
				Expect(r.Header.Get("Content-Type")).To(Equal("application/x-www-form-urlencoded"))

				Expect(r.PostForm.Get("grant_type")).To(Equal("password"))
				Expect(r.PostForm.Get("response_type")).To(Equal("token"))

				Expect(r.PostForm.Get("passcode")).To(Equal("passcode"))

				Expect(r.PostForm.Get("client_id")).To(Equal("some-client-id"))
				Expect(r.PostForm.Get("client_secret")).To(Equal("some-client-secret"))

				w.Write([]byte(`{"access_token": "some-access-token", "refresh_token": "some-refresh-token", "token_type": "bearer"}`))
			}))
			defer uaaServer.Close()

			client := Client{
				AuthURL: uaaServer.URL,
				Client:  http.DefaultClient,
			}

			accessToken, refreshToken, err := client.PasscodeGrant("some-client-id", "some-client-secret", "passcode")

			Expect(err).To(BeNil())
			Expect(accessToken).To(Equal("some-access-token"))
			Expect(refreshToken).To(Equal("some-refresh-token"))
		})
	})

	Context("Metadata()", func() {
		It("should make a metadata request", func() {
			uaaServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				Expect(r.Method).To(Equal(http.MethodGet))

				Expect(r.URL.Path).To(Equal("/info"))

				Expect(r.Header.Get("Accept")).To(Equal("application/json"))

				w.Write([]byte(`{
					"app" : {
					  "version" : "4.7.0-SNAPSHOT"
					},
					"showLoginLinks" : true,
					"links" : {
					  "uaa" : "http://localhost:8080/uaa",
					  "passwd" : "/forgot_password",
					  "login" : "http://localhost:8080/uaa",
					  "register" : "/create_account"
					},
					"zone_name" : "uaa",
					"entityID" : "cloudfoundry-saml-login",
					"commit_id" : "4bba13c",
					"idpDefinitions" : {
					  "SAMLMetadataUrl" : "http://localhost:8080/uaa/saml/discovery?returnIDParam=idp&entityID=cloudfoundry-saml-login&idp=SAMLMetadataUrl&isPassive=true",
					  "SAML" : "http://localhost:8080/uaa/saml/discovery?returnIDParam=idp&entityID=cloudfoundry-saml-login&idp=SAML&isPassive=true"
					},
					"prompts" : {
					  "username" : [ "text", "Email" ],
					  "password" : [ "password", "Password" ],
					  "passcode" : [ "password", "One Time Code ( Get one at http://fromprompt:8080/uaa/passcode )" ]
					},
					"timestamp" : "2017-09-08T23:11:58+0000"
				  }`))
			}))
			defer uaaServer.Close()

			client := Client{
				AuthURL: uaaServer.URL,
				Client:  http.DefaultClient,
			}

			md, err := client.Metadata()

			Expect(err).To(BeNil())
			Expect(md).NotTo(BeNil())
			Expect(md.PasscodePrompt()).To(Equal("One Time Code ( Get one at http://fromprompt:8080/uaa/passcode )"))
		})
		It("should be OK with no prompt field", func() {
			uaaServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				Expect(r.Method).To(Equal(http.MethodGet))

				Expect(r.URL.Path).To(Equal("/info"))

				Expect(r.Header.Get("Accept")).To(Equal("application/json"))

				w.Write([]byte(`{
					"links" : {
					  "login" : "http://foobar:8080/uaa"
					}
				  }`))
			}))
			defer uaaServer.Close()

			client := Client{
				AuthURL: uaaServer.URL,
				Client:  http.DefaultClient,
			}

			md, err := client.Metadata()

			Expect(err).To(BeNil())
			Expect(md).NotTo(BeNil())
			Expect(md.PasscodePrompt()).To(Equal("One Time Code ( Get one at http://foobar:8080/uaa/passcode )"))
		})
		It("should be OK with no prompt field or links", func() {
			uaaServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				Expect(r.Method).To(Equal(http.MethodGet))

				Expect(r.URL.Path).To(Equal("/info"))

				Expect(r.Header.Get("Accept")).To(Equal("application/json"))

				w.Write([]byte(`{
				  }`))
			}))
			defer uaaServer.Close()

			client := Client{
				AuthURL: uaaServer.URL,
				Client:  http.DefaultClient,
			}

			md, err := client.Metadata()

			Expect(err).To(BeNil())
			Expect(md).NotTo(BeNil())
			Expect(md.PasscodePrompt()).To(Equal("One Time Code ( Get one at https://login.system.example.com/passcode )"))
		})
	})

	Context("RefreshTokenGrant()", func() {
		It("should make a refresh grant token request", func() {
			uaaServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				r.ParseForm()

				Expect(r.Method).To(Equal(http.MethodPost))

				Expect(r.URL.Path).To(Equal("/oauth/token"))

				Expect(r.Header.Get("Accept")).To(Equal("application/json"))
				Expect(r.Header.Get("Content-Type")).To(Equal("application/x-www-form-urlencoded"))

				Expect(r.PostForm.Get("grant_type")).To(Equal("refresh_token"))
				Expect(r.PostForm.Get("response_type")).To(Equal("token"))

				Expect(r.PostForm.Get("client_id")).To(Equal("client-id"))
				Expect(r.PostForm.Get("client_secret")).To(Equal("client-secret"))

				Expect(r.PostForm.Get("refresh_token")).To(Equal("some-refresh-token"))

				w.Write([]byte(`{"access_token": "new-access-token", "refresh_token": "new-refresh-token", "token_type": "bearer"}`))
			}))
			defer uaaServer.Close()

			client := Client{
				AuthURL: uaaServer.URL,
				Client:  http.DefaultClient,
			}

			accessToken, refreshToken, err := client.RefreshTokenGrant("client-id", "client-secret", "some-refresh-token")

			Expect(err).To(BeNil())
			Expect(accessToken).To(Equal("new-access-token"))
			Expect(refreshToken).To(Equal("new-refresh-token"))
		})
	})

	Context("RevokeToken()", func() {
		It("requests to revoke the token", func() {
			token := "e30K.eyJqdGkiOiIxIn0K.e30K" // {}.{"jti":"1"}.{}

			var request *http.Request

			uaaServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				request = r
				w.WriteHeader(http.StatusOK)
			}))

			defer uaaServer.Close()

			client := Client{
				AuthURL: uaaServer.URL,
				Client:  http.DefaultClient,
			}
			err := client.RevokeToken(token)

			Expect(err).To(BeNil())
			Expect(request.Method).To(Equal(http.MethodDelete))
			Expect(request.Header.Get("Authorization")).To(Equal("Bearer " + token))
			Expect(request.URL.Path).To(Equal("/oauth/token/revoke/1"))
		})

		DescribeTable("token is invallid",
			func(token string) {
				uaaServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.WriteHeader(http.StatusOK)
				}))

				defer uaaServer.Close()

				client := Client{
					AuthURL: uaaServer.URL,
					Client:  http.DefaultClient,
				}

				err := client.RevokeToken(token)
				Expect(err).To(HaveOccurred())
			},
			Entry("missing segments", "first"),
			Entry("not base64", "^first.^second.^third"),
			Entry("not valid json", "bm90IGpzb24K.bm90IGpzb24K.bm90IGpzb24K"), // bm90IGpzb24K = not json
			Entry("missing jti claim", "e30K.e30K.e30K"),                      // e30K = {}
		)
	})

	DescribeTable("unable to complete the request",
		func(performAction func(*Client) error) {
			uaaServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				// Kill the connection without returning a status code
				hj, _ := w.(http.Hijacker)
				conn, _, _ := hj.Hijack()
				conn.Close()
			}))

			defer uaaServer.Close()

			client := &Client{
				AuthURL: uaaServer.URL,
				Client:  http.DefaultClient,
			}

			err := performAction(client)

			Expect(err).To(HaveOccurred())
		},
		Entry("client credentials", func(c *Client) error {
			_, err := c.ClientCredentialGrant("client-id", "client-secret")
			return err
		}),
		Entry("password grant", func(c *Client) error {
			_, _, err := c.PasswordGrant("some-client-id", "some-client-secret", "username", "password")
			return err
		}),
		Entry("refresh token grant", func(c *Client) error {
			_, _, err := c.RefreshTokenGrant("client-id", "client-secret", "some-refresh-token")
			return err
		}),
		Entry("revoke token", func(c *Client) error {
			return c.RevokeToken("e30K.eyJqdGkiOiIxIn0K.e30K") // {}.{"jti":"1"}.{}
		}),
	)

	DescribeTable("response body is invalid",
		func(performAction func(*Client) error) {
			uaaServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Write([]byte(`{`))
			}))

			defer uaaServer.Close()

			client := &Client{
				AuthURL: uaaServer.URL,
				Client:  http.DefaultClient,
			}

			err := performAction(client)
			Expect(err).To(HaveOccurred())
		},

		Entry("client credentials", func(c *Client) error {
			_, err := c.ClientCredentialGrant("client-id", "client-secret")
			return err
		}),
		Entry("password grant", func(c *Client) error {
			_, _, err := c.PasswordGrant("some-client-id", "some-client-secret", "username", "password")
			return err
		}),
		Entry("refresh token grant", func(c *Client) error {
			_, _, err := c.RefreshTokenGrant("client-id", "client-secret", "some-refresh-token")
			return err
		}),
	)

	DescribeTable("credentials are invalid",
		func(performAction func(*Client) error) {
			uaaServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusUnauthorized)
				w.Write([]byte(`{"error":"unauthorized","error_description":"Bad credentials"}`))
			}))

			defer uaaServer.Close()

			client := &Client{
				AuthURL: uaaServer.URL,
				Client:  http.DefaultClient,
			}

			err := performAction(client)
			Expect(err).To(HaveOccurred())
		},
		Entry("client credentials", func(c *Client) error {
			_, err := c.ClientCredentialGrant("client-id", "client-secret")
			return err
		}),
		Entry("password grant", func(c *Client) error {
			_, _, err := c.PasswordGrant("some-client-id", "some-client-secret", "username", "password")
			return err
		}),
		Entry("refresh token grant", func(c *Client) error {
			_, _, err := c.RefreshTokenGrant("client-id", "client-secret", "some-refresh-token")
			return err
		}),
	)

	DescribeTable("error response is invalid",
		func(performAction func(*Client) error) {
			uaaServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusUnauthorized)
				w.Write([]byte(`{`))
			}))

			defer uaaServer.Close()

			client := &Client{
				AuthURL: uaaServer.URL,
				Client:  http.DefaultClient,
			}

			err := performAction(client)
			Expect(err.Error()).ToNot(BeEmpty())
		},
		Entry("client credentials", func(c *Client) error {
			_, err := c.ClientCredentialGrant("client-id", "client-secret")
			return err
		}),
		Entry("password grant", func(c *Client) error {
			_, _, err := c.PasswordGrant("some-client-id", "some-client-secret", "username", "password")
			return err
		}),
		Entry("refresh token grant", func(c *Client) error {
			_, _, err := c.RefreshTokenGrant("client-id", "client-secret", "some-refresh-token")
			return err
		}),
	)
})
