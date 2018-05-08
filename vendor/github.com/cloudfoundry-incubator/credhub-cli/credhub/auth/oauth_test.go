package auth_test

import (
	"errors"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"strings"

	"github.com/cloudfoundry-incubator/credhub-cli/credhub/auth"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("OAuthStrategy", func() {
	var (
		mockUaaClient *dummyUaaClient
	)

	BeforeEach(func() {
		mockUaaClient = &dummyUaaClient{}
	})

	Context("Do()", func() {
		It("should add the bearer token to the request header", func() {
			var actualAuthHeader string
			var actualRequestPath string
			var actualRequestMethod string

			apiServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				actualAuthHeader = r.Header.Get("Authorization")
				actualRequestMethod = r.Method
				actualRequestPath = r.URL.Path
				w.WriteHeader(http.StatusOK)
				w.Write([]byte("success"))
			}))

			defer apiServer.Close()

			uaa := auth.OAuthStrategy{
				ApiClient:   http.DefaultClient,
				OAuthClient: mockUaaClient,
			}

			uaa.SetTokens("some-access-token", "")

			request, _ := http.NewRequest("GET", apiServer.URL+"/path/", nil)

			resp, err := uaa.Do(request)

			Expect(err).ToNot(HaveOccurred())
			Expect(resp.StatusCode).To(Equal(http.StatusOK))

			Expect(actualAuthHeader).To(Equal("Bearer some-access-token"))
			Expect(actualRequestMethod).To(Equal("GET"))
			Expect(actualRequestPath).To(Equal("/path/"))
		})

		It("forwards responses", func() {
			apiServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
				w.Write([]byte("success"))
			}))

			defer apiServer.Close()

			uaa := auth.OAuthStrategy{
				ApiClient:   http.DefaultClient,
				OAuthClient: mockUaaClient,
			}

			request, _ := http.NewRequest("GET", apiServer.URL, nil)
			resp, err := uaa.Do(request)

			Expect(err).ToNot(HaveOccurred())
			Expect(resp.StatusCode).To(Equal(http.StatusOK))

			defer resp.Body.Close()

			bytes, err := ioutil.ReadAll(resp.Body)

			Expect(err).ToNot(HaveOccurred())
			Expect(bytes).To(Equal([]byte("success")))
		})

		It("forwards connection errors", func() {
			apiServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				// Kill the connection without returning a status code
				hj, _ := w.(http.Hijacker)
				conn, _, _ := hj.Hijack()
				conn.Close()
			}))

			defer apiServer.Close()

			uaa := auth.OAuthStrategy{
				ApiClient:   http.DefaultClient,
				OAuthClient: mockUaaClient,
			}

			request, _ := http.NewRequest("GET", apiServer.URL, nil)
			resp, err := uaa.Do(request)

			Expect(err).To(HaveOccurred())
			Expect(resp).To(BeNil())
		})

		Context("when there is no access token", func() {
			It("should request an access token", func() {
				mockUaaClient.NewAccessToken = "new-access-token"
				mockUaaClient.NewRefreshToken = "new-refresh-token"

				var lastAuthHeader string

				apiServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					lastAuthHeader = r.Header.Get("Authorization")
					w.WriteHeader(http.StatusOK)
				}))

				defer apiServer.Close()

				oauth := auth.OAuthStrategy{
					OAuthClient:  mockUaaClient,
					ApiClient:    http.DefaultClient,
					ClientId:     "client-id",
					ClientSecret: "client-secret",
					Username:     "user-name",
					Password:     "user-password",
				}

				request, _ := http.NewRequest("GET", apiServer.URL, nil)

				oauth.Do(request)

				Expect(lastAuthHeader).To(Equal("Bearer new-access-token"))
				Expect(oauth.AccessToken()).To(Equal("new-access-token"))
				Expect(oauth.RefreshToken()).To(Equal("new-refresh-token"))
			})

			Context("when fetching token fails", func() {
				It("returns an error", func() {
					mockUaaClient.Error = errors.New("failed to login")
					oauth := auth.OAuthStrategy{
						OAuthClient:  mockUaaClient,
						ClientId:     "client-id",
						ClientSecret: "client-secret",
						Username:     "user-name",
						Password:     "user-password",
					}
					request, _ := http.NewRequest("GET", "https://some-endpoint.com/path/", nil)

					_, err := oauth.Do(request)
					Expect(err).To(MatchError("failed to login"))
				})
			})

		})

		Context("when the access token has expired", func() {
			It("should refresh the token and submit the request again", func() {
				apiServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					if r.Header.Get("Authorization") != "Bearer new-access-token" {
						w.WriteHeader(573)
						w.Write([]byte(`{"error": "access_token_expired"}`))
					} else {
						w.Write([]byte(`Success!`))
					}
				}))

				defer apiServer.Close()

				mockUaaClient.NewAccessToken = "new-access-token"
				mockUaaClient.NewRefreshToken = "new-refresh-token"

				uaa := auth.OAuthStrategy{
					ClientId:     "client-id",
					ClientSecret: "client-secret",
					ApiClient:    http.DefaultClient,
					OAuthClient:  mockUaaClient,
				}

				uaa.SetTokens("old-access-token", "old-refresh-token")

				request, _ := http.NewRequest("POST", apiServer.URL, strings.NewReader("some body"))

				response, err := uaa.Do(request)

				Expect(err).ToNot(HaveOccurred())

				Expect(mockUaaClient.ClientId).To(Equal("client-id"))
				Expect(mockUaaClient.ClientSecret).To(Equal("client-secret"))
				Expect(mockUaaClient.RefreshToken).To(Equal("old-refresh-token"))

				body, err := ioutil.ReadAll(response.Body)

				Expect(err).ToNot(HaveOccurred())
				Expect(string(body)).To(Equal("Success!"))
			})

			Context("when refreshing token fails", func() {
				It("returns an error", func() {
					mockUaaClient.Error = errors.New("failed to refresh")
					apiServer := fixedResponseServer(573, []byte(`{"error": "access_token_expired"}`))
					defer apiServer.Close()

					oauth := auth.OAuthStrategy{
						OAuthClient:  mockUaaClient,
						ApiClient:    http.DefaultClient,
						ClientId:     "client-id",
						ClientSecret: "client-secret",
						Username:     "user-name",
						Password:     "user-password",
					}
					oauth.SetTokens("some-access-token", "some-refresh-token")
					request, _ := http.NewRequest("GET", apiServer.URL, nil)

					_, err := oauth.Do(request)

					Expect(err).To(MatchError("failed to refresh"))
				})
			})

			Context("when refreshing token fails because token is expired", func() {
				It("returns an error prompting user to log in", func() {
					mockUaaClient.Error = errors.New("invalid_token")
					apiServer := fixedResponseServer(573, []byte(`{"error": "access_token_expired"}`))
					defer apiServer.Close()

					oauth := auth.OAuthStrategy{
						OAuthClient:  mockUaaClient,
						ApiClient:    http.DefaultClient,
						ClientId:     "client-id",
						ClientSecret: "client-secret",
						Username:     "user-name",
						Password:     "user-password",
					}
					oauth.SetTokens("some-access-token", "some-refresh-token")
					request, _ := http.NewRequest("GET", apiServer.URL, nil)

					_, err := oauth.Do(request)

					Expect(err).To(MatchError("You are not currently authenticated. Please log in to continue."))
				})
			})
		})

		Context("when cloning the request fails", func() {
			It("returns an error", func() {
				uaa := auth.OAuthStrategy{}
				uaa.SetTokens("old-access-token", "old-refresh-token")
				request, _ := http.NewRequest("POST", "http://some-domain.com", &errorReader{})

				_, err := uaa.Do(request)
				Expect(err).To(MatchError("failed to clone request body: error reading"))
			})
		})

		Context("when a non-auth error has occurred", func() {
			It("should forward the response untouched", func() {
				apiServer := fixedResponseServer(573, []byte(`{"error": "some other error"}`))
				defer apiServer.Close()

				uaa := auth.OAuthStrategy{
					ClientId:     "client-id",
					ClientSecret: "client-secret",
					ApiClient:    http.DefaultClient,
					OAuthClient:  mockUaaClient,
				}

				uaa.SetTokens("old-access-token", "old-refresh-token")

				request, _ := http.NewRequest("GET", apiServer.URL, nil)

				response, err := uaa.Do(request)

				Expect(err).ToNot(HaveOccurred())

				body, err := ioutil.ReadAll(response.Body)

				Expect(err).ToNot(HaveOccurred())
				Expect(body).To(MatchJSON(`{"error": "some other error"}`))
			})
		})

	})

	Context("Refresh()", func() {
		BeforeEach(func() {
			mockUaaClient.NewAccessToken = "new-access-token"
			mockUaaClient.NewRefreshToken = "new-refresh-token"
		})

		Context("with a refresh token", func() {
			It("should make a refresh grant token request and save the new tokens", func() {
				uaa := auth.OAuthStrategy{
					ClientId:     "client-id",
					ClientSecret: "client-secret",
					OAuthClient:  mockUaaClient,
				}

				uaa.SetTokens("", "some-refresh-token")
				uaa.Refresh()

				Expect(mockUaaClient.ClientId).To(Equal("client-id"))
				Expect(mockUaaClient.ClientSecret).To(Equal("client-secret"))
				Expect(mockUaaClient.RefreshToken).To(Equal("some-refresh-token"))

				Expect(uaa.AccessToken()).To(Equal("new-access-token"))
				Expect(uaa.RefreshToken()).To(Equal("new-refresh-token"))
			})

			Context("when the refresh token grant fails", func() {
				It("returns an error", func() {
					mockUaaClient.Error = errors.New("refresh token grant failed")

					uaa := auth.OAuthStrategy{
						ClientId:     "client-id",
						ClientSecret: "client-secret",
						OAuthClient:  mockUaaClient,
					}

					uaa.SetTokens("", "some-refresh-token")
					err := uaa.Refresh()

					Expect(err).To(MatchError("refresh token grant failed"))

				})
			})
		})

		Context("without a refresh token", func() {
			Context("with a username and password", func() {
				It("should make a password grant request", func() {
					uaa := auth.OAuthStrategy{
						ClientId:     "client-id",
						ClientSecret: "client-secret",
						Username:     "user-name",
						Password:     "user-password",
						OAuthClient:  mockUaaClient,
					}

					uaa.Refresh()

					Expect(mockUaaClient.ClientId).To(Equal("client-id"))
					Expect(mockUaaClient.ClientSecret).To(Equal("client-secret"))
					Expect(mockUaaClient.Username).To(Equal("user-name"))
					Expect(mockUaaClient.Password).To(Equal("user-password"))

					Expect(uaa.AccessToken()).To(Equal("new-access-token"))
					Expect(uaa.RefreshToken()).To(Equal("new-refresh-token"))
				})

				It("when performing the password grant returns an error", func() {
					mockUaaClient.Error = errors.New("password grant error")

					uaa := auth.OAuthStrategy{
						ClientId:     "client-id",
						ClientSecret: "client-secret",
						Username:     "user-name",
						Password:     "user-password",
						OAuthClient:  mockUaaClient,
					}

					err := uaa.Refresh()
					Expect(err).To(MatchError("password grant error"))
				})
			})

			Context("with client credentials", func() {
				It("should make a client credentials grant request", func() {
					uaa := auth.OAuthStrategy{
						ClientId:                "client-id",
						ClientSecret:            "client-secret",
						OAuthClient:             mockUaaClient,
						ClientCredentialRefresh: true,
					}

					uaa.Refresh()

					Expect(mockUaaClient.ClientId).To(Equal("client-id"))
					Expect(mockUaaClient.ClientSecret).To(Equal("client-secret"))

					Expect(uaa.AccessToken()).To(Equal("new-access-token"))
					Expect(uaa.RefreshToken()).To(BeEmpty())
				})

				Context("when the client credentials grant fails", func() {
					It("returns an error", func() {
						mockUaaClient.Error = errors.New("client credentials grant failed")

						uaa := auth.OAuthStrategy{
							ClientId:                "client-id",
							ClientSecret:            "client-secret",
							OAuthClient:             mockUaaClient,
							ClientCredentialRefresh: true,
						}

						err := uaa.Refresh()

						Expect(err).To(MatchError("client credentials grant failed"))

					})
				})

			})
		})
	})

	Context("Login()", func() {
		BeforeEach(func() {
			mockUaaClient.NewAccessToken = "new-access-token"
			mockUaaClient.NewRefreshToken = "new-refresh-token"
		})

		Context("when there is already an access token", func() {
			It("should do nothing", func() {
				uaa := auth.OAuthStrategy{}
				uaa.SetTokens("some-access-token", "")

				err := uaa.Login()

				Expect(err).To(BeNil())
			})
		})

		Context("with a username and password", func() {
			It("should make a password grant request", func() {
				uaa := auth.OAuthStrategy{
					ClientId:     "client-id",
					ClientSecret: "client-secret",
					Username:     "user-name",
					Password:     "user-password",
					OAuthClient:  mockUaaClient,
				}

				uaa.Refresh()

				Expect(mockUaaClient.ClientId).To(Equal("client-id"))
				Expect(mockUaaClient.ClientSecret).To(Equal("client-secret"))
				Expect(mockUaaClient.Username).To(Equal("user-name"))
				Expect(mockUaaClient.Password).To(Equal("user-password"))

				Expect(uaa.AccessToken()).To(Equal("new-access-token"))
				Expect(uaa.RefreshToken()).To(Equal("new-refresh-token"))
			})

			Context("when the refresh token grant fails", func() {
				It("returns an error", func() {
					mockUaaClient.Error = errors.New("refresh token grant failed")

					uaa := auth.OAuthStrategy{
						ClientId:     "client-id",
						ClientSecret: "client-secret",
						OAuthClient:  mockUaaClient,
					}
					uaa.SetTokens("", "some-refresh-token")

					err := uaa.Refresh()

					Expect(err).To(MatchError("refresh token grant failed"))

				})
			})
		})

		Context("with client credentials", func() {
			It("should make a client credentials grant request", func() {
				uaa := auth.OAuthStrategy{
					ClientId:                "client-id",
					ClientSecret:            "client-secret",
					OAuthClient:             mockUaaClient,
					ClientCredentialRefresh: true,
				}

				uaa.Refresh()

				Expect(mockUaaClient.ClientId).To(Equal("client-id"))
				Expect(mockUaaClient.ClientSecret).To(Equal("client-secret"))

				Expect(uaa.AccessToken()).To(Equal("new-access-token"))
				Expect(uaa.RefreshToken()).To(BeEmpty())
			})

			Context("when the client credentials grant fails", func() {
				It("returns an error", func() {
					mockUaaClient.Error = errors.New("client credentials grant failed")

					uaa := auth.OAuthStrategy{
						ClientId:                "client-id",
						ClientSecret:            "client-secret",
						OAuthClient:             mockUaaClient,
						ClientCredentialRefresh: true,
					}

					err := uaa.Refresh()

					Expect(err).To(MatchError("client credentials grant failed"))

				})
			})
		})
	})

	Context("Logout()", func() {
		var uaa auth.OAuthStrategy

		BeforeEach(func() {
			uaa = auth.OAuthStrategy{
				OAuthClient: mockUaaClient,
			}
			uaa.SetTokens("some-access-token", "some-refresh-token")
		})

		It("revokes the token", func() {
			uaa.Logout()
			Expect(mockUaaClient.RevokedToken).To(Equal("some-access-token"))
		})

		Context("when revoking the token succeeds", func() {
			It("clears the access and refresh token", func() {
				uaa.Logout()
				Expect(uaa.AccessToken()).To(BeEmpty())
				Expect(uaa.RefreshToken()).To(BeEmpty())
			})

			It("returns no error", func() {
				err := uaa.Logout()
				Expect(err).ToNot(HaveOccurred())
			})
		})

		Context("when revoking the token fails", func() {
			BeforeEach(func() {
				mockUaaClient.Error = errors.New("failed to revoke token")
			})

			It("returns the error", func() {
				err := uaa.Logout()
				Expect(err).To(MatchError("failed to revoke token"))
			})

			It("does not clear the access and refresh token", func() {
				uaa.Logout()
				Expect(uaa.AccessToken()).ToNot(BeEmpty())
				Expect(uaa.RefreshToken()).ToNot(BeEmpty())
			})
		})

		Context("when we are not logged in", func() {
			BeforeEach(func() {
				uaa.SetTokens("", "")
			})

			It("does not attempt to revoke the token", func() {
				uaa.Logout()
				Expect(mockUaaClient.RevokedToken).To(BeEmpty())
			})
		})
	})
})

func fixedResponseServer(statusCode int, body []byte) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(statusCode)
		w.Write(body)
	}))
}

type errorReader struct {
}

func (r *errorReader) Read(b []byte) (n int, err error) {
	return 0, errors.New("error reading")
}
