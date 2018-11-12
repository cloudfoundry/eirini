package credhub_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"math/rand"
	"net/http"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/ghttp"

	. "code.cloudfoundry.org/credhub-cli/credhub"
	"code.cloudfoundry.org/credhub-cli/credhub/credentials/values"
)

var _ = Describe("Set", func() {
	Describe("SetCertificate()", func() {
		It("requests to set the certificate", func() {
			dummy := &DummyAuth{Response: &http.Response{
				Body: ioutil.NopCloser(bytes.NewBufferString("")),
			}}

			ch, _ := New("https://example.com", Auth(dummy.Builder()), ServerVersion("2.0.0"))

			certificate := values.Certificate{
				Ca:     "some-ca",
				CaName: "/some-ca-name",
			}
			ch.SetCertificate("/example-certificate", certificate)

			urlPath := dummy.Request.URL.Path
			Expect(urlPath).To(Equal("/api/v1/data"))
			Expect(dummy.Request.Method).To(Equal(http.MethodPut))

			var requestBody map[string]interface{}
			body, _ := ioutil.ReadAll(dummy.Request.Body)
			json.Unmarshal(body, &requestBody)

			Expect(requestBody["name"]).To(Equal("/example-certificate"))
			Expect(requestBody["type"]).To(Equal("certificate"))

			Expect(requestBody["value"].(map[string]interface{})["ca"]).To(Equal("some-ca"))
			Expect(requestBody["value"].(map[string]interface{})["ca_name"]).To(Equal("/some-ca-name"))
		})

		Context("when server version is not provided", func() {
			var server *ghttp.Server

			BeforeEach(func() {
				server = ghttp.NewServer()
				server.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("GET", "/info"),
						ghttp.RespondWith(http.StatusOK, `{}`),
					),
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("GET", "/version"),
						ghttp.RespondWith(http.StatusOK, `{"version": "1.9.0"}`),
					),
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("PUT", "/api/v1/data"),
						ghttp.RespondWith(http.StatusOK, `{}`),
					),
				)
			})

			It("send request for server version", func() {
				ch, _ := New(server.URL())
				_, err := ch.SetCertificate("/example-certificate", values.Certificate{})
				Expect(err).NotTo(HaveOccurred())
			})
		})

		Context("when server version is older than 2.x", func() {
			It("set overwrite mode", func() {
				dummy := &DummyAuth{Response: &http.Response{
					Body: ioutil.NopCloser(bytes.NewBufferString("")),
				}}

				version := fmt.Sprintf("1.%d.0", rand.Intn(10))
				ch, _ := New("https://example.com", Auth(dummy.Builder()), ServerVersion(version))
				ch.SetCertificate("/example-certificate", values.Certificate{})

				var requestBody map[string]interface{}
				body, _ := ioutil.ReadAll(dummy.Request.Body)
				json.Unmarshal(body, &requestBody)

				Expect(requestBody["mode"]).To(Equal("overwrite"))
			})
		})

		Context("when server version is invalid", func() {
			It("returns an error", func() {
				ch, _ := New("https://example.com", ServerVersion("invalid-version"))
				_, err := ch.SetCertificate("/example-certificate", values.Certificate{})
				Expect(err).To(MatchError("Malformed version: invalid-version"))
			})
		})

		Context("when successful", func() {
			It("returns the credential that has been set", func() {
				dummy := &DummyAuth{Response: &http.Response{
					StatusCode: http.StatusOK,
					Body: ioutil.NopCloser(bytes.NewBufferString(`{
		  "id": "some-id",
		  "name": "/example-certificate",
		  "type": "certificate",
		  "value": {
		    "ca": "some-ca",
		    "certificate": "some-certificate",
		    "private_key": "some-private-key"
		  },
		  "version_created_at": "2017-01-01T04:07:18Z"
		}`)),
				}}

				ch, _ := New("https://example.com", Auth(dummy.Builder()), ServerVersion("2.0.0"))

				certificate := values.Certificate{
					Certificate: "some-cert",
				}
				cred, _ := ch.SetCertificate("/example-certificate", certificate)

				Expect(cred.Name).To(Equal("/example-certificate"))
				Expect(cred.Type).To(Equal("certificate"))
				Expect(cred.Value.Ca).To(Equal("some-ca"))
				Expect(cred.Value.Certificate).To(Equal("some-certificate"))
				Expect(cred.Value.PrivateKey).To(Equal("some-private-key"))
			})
		})
		Context("when request fails", func() {
			It("returns an error", func() {
				dummy := &DummyAuth{Error: errors.New("network error occurred")}
				ch, _ := New("https://example.com", Auth(dummy.Builder()), ServerVersion("2.0.0"))
				certificate := values.Certificate{
					Ca: "some-ca",
				}
				_, err := ch.SetCertificate("/example-certificate", certificate)

				Expect(err).To(MatchError("network error occurred"))
			})
		})

		Context("when response body cannot be unmarshalled", func() {
			It("returns an error", func() {
				dummy := &DummyAuth{Response: &http.Response{
					Body: ioutil.NopCloser(bytes.NewBufferString("something-invalid")),
				}}
				ch, _ := New("https://example.com", Auth(dummy.Builder()), ServerVersion("2.0.0"))
				certificate := values.Certificate{
					Ca: "some-ca",
				}
				_, err := ch.SetCertificate("/example-certificate", certificate)

				Expect(err).To(MatchError(ContainSubstring("invalid character 's'")))
			})
		})
	})

	Describe("SetPassword()", func() {
		It("requests to set the password", func() {
			dummy := &DummyAuth{Response: &http.Response{
				Body: ioutil.NopCloser(bytes.NewBufferString("")),
			}}

			ch, _ := New("https://example.com", Auth(dummy.Builder()), ServerVersion("2.0.0"))
			password := values.Password("some-password")

			ch.SetPassword("/example-password", password)

			urlPath := dummy.Request.URL.Path
			Expect(urlPath).To(Equal("/api/v1/data"))
			Expect(dummy.Request.Method).To(Equal(http.MethodPut))

			var requestBody map[string]interface{}
			body, _ := ioutil.ReadAll(dummy.Request.Body)
			json.Unmarshal(body, &requestBody)

			Expect(requestBody["name"]).To(Equal("/example-password"))
			Expect(requestBody["type"]).To(Equal("password"))
			Expect(requestBody["value"]).To(BeEquivalentTo("some-password"))
		})

		Context("when server version is not provided", func() {
			var server *ghttp.Server

			BeforeEach(func() {
				server = ghttp.NewServer()
				server.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("GET", "/info"),
						ghttp.RespondWith(http.StatusOK, `{}`),
					),
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("GET", "/version"),
						ghttp.RespondWith(http.StatusOK, `{"version": "1.9.0"}`),
					),
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("PUT", "/api/v1/data"),
						ghttp.RespondWith(http.StatusOK, `{}`),
					),
				)
			})

			It("send request for server version", func() {
				ch, _ := New(server.URL())
				_, err := ch.SetPassword("/example-password", values.Password(""))
				Expect(err).NotTo(HaveOccurred())
			})
		})

		Context("when server version is older than 2.x", func() {
			It("set overwrite mode", func() {
				dummy := &DummyAuth{Response: &http.Response{
					Body: ioutil.NopCloser(bytes.NewBufferString("")),
				}}

				version := fmt.Sprintf("1.%d.0", rand.Intn(10))
				ch, _ := New("https://example.com", Auth(dummy.Builder()), ServerVersion(version))
				ch.SetPassword("/example-password", values.Password(""))

				var requestBody map[string]interface{}
				body, _ := ioutil.ReadAll(dummy.Request.Body)
				json.Unmarshal(body, &requestBody)

				Expect(requestBody["mode"]).To(Equal("overwrite"))
			})
		})

		Context("when server version is invalid", func() {
			It("returns an error", func() {
				ch, _ := New("https://example.com", ServerVersion("invalid-version"))
				_, err := ch.SetPassword("/example-password", values.Password(""))
				Expect(err).To(MatchError("Malformed version: invalid-version"))
			})
		})

		Context("when successful", func() {
			It("returns the credential that has been set", func() {
				dummy := &DummyAuth{Response: &http.Response{
					StatusCode: http.StatusOK,
					Body: ioutil.NopCloser(bytes.NewBufferString(`{
		  "id": "some-id",
		  "name": "/example-password",
		  "type": "password",
		  "value": "some-password",
		  "version_created_at": "2017-01-01T04:07:18Z"
		}`)),
				}}

				ch, _ := New("https://example.com", Auth(dummy.Builder()), ServerVersion("2.0.0"))

				password := values.Password("some-password")

				cred, _ := ch.SetPassword("/example-password", password)

				Expect(cred.Name).To(Equal("/example-password"))
				Expect(cred.Type).To(Equal("password"))

				Expect(cred.Value).To(BeEquivalentTo("some-password"))

			})
		})
		Context("when request fails", func() {
			It("returns an error", func() {
				dummy := &DummyAuth{Error: errors.New("network error occurred")}
				ch, _ := New("https://example.com", Auth(dummy.Builder()), ServerVersion("2.0.0"))
				password := values.Password("some-password")

				_, err := ch.SetPassword("/example-password", password)

				Expect(err).To(MatchError("network error occurred"))
			})
		})

		Context("when response body cannot be unmarshalled", func() {
			It("returns an error", func() {
				dummy := &DummyAuth{Response: &http.Response{
					Body: ioutil.NopCloser(bytes.NewBufferString("something-invalid")),
				}}

				ch, _ := New("https://example.com", Auth(dummy.Builder()), ServerVersion("2.0.0"))
				password := values.Password("some-password")

				_, err := ch.SetPassword("/example-password", password)

				Expect(err).To(MatchError(ContainSubstring("invalid character 's'")))
			})
		})
	})

	Describe("SetUser()", func() {
		It("requests to set the user", func() {
			username := "some-user"
			dummy := &DummyAuth{Response: &http.Response{
				Body: ioutil.NopCloser(bytes.NewBufferString("")),
			}}

			ch, _ := New("https://example.com", Auth(dummy.Builder()), ServerVersion("2.0.0"))
			user := values.User{Username: username, Password: "some-password"}

			ch.SetUser("/example-user", user)

			urlPath := dummy.Request.URL.Path
			Expect(urlPath).To(Equal("/api/v1/data"))
			Expect(dummy.Request.Method).To(Equal(http.MethodPut))

			body, _ := ioutil.ReadAll(dummy.Request.Body)
			Expect(body).To(MatchJSON(`
			{
				"name" : "/example-user",
				"type" : "user",
				"value": {
					"username" : "some-user",
					"password" : "some-password"
				}
			}`))
		})

		Context("when server version is not provided", func() {
			var server *ghttp.Server

			BeforeEach(func() {
				server = ghttp.NewServer()
				server.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("GET", "/info"),
						ghttp.RespondWith(http.StatusOK, `{}`),
					),
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("GET", "/version"),
						ghttp.RespondWith(http.StatusOK, `{"version": "1.9.0"}`),
					),
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("PUT", "/api/v1/data"),
						ghttp.RespondWith(http.StatusOK, `{}`),
					),
				)
			})

			It("send request for server version", func() {
				ch, _ := New(server.URL())
				_, err := ch.SetUser("/example-user", values.User{})
				Expect(err).NotTo(HaveOccurred())
			})
		})

		Context("when server version is older than 2.x", func() {
			It("set overwrite mode", func() {
				dummy := &DummyAuth{Response: &http.Response{
					Body: ioutil.NopCloser(bytes.NewBufferString("")),
				}}

				version := fmt.Sprintf("1.%d.0", rand.Intn(10))
				ch, _ := New("https://example.com", Auth(dummy.Builder()), ServerVersion(version))
				ch.SetUser("/example-user", values.User{})

				var requestBody map[string]interface{}
				body, _ := ioutil.ReadAll(dummy.Request.Body)
				json.Unmarshal(body, &requestBody)

				Expect(requestBody["mode"]).To(Equal("overwrite"))
			})
		})

		Context("when server version is invalid", func() {
			It("returns an error", func() {
				ch, _ := New("https://example.com", ServerVersion("invalid-version"))
				_, err := ch.SetUser("/example-user", values.User{})
				Expect(err).To(MatchError("Malformed version: invalid-version"))
			})
		})

		user := "username"
		Context("when successful", func() {
			It("returns the credential that has been set", func() {
				dummy := &DummyAuth{Response: &http.Response{
					StatusCode: http.StatusOK,
					Body: ioutil.NopCloser(bytes.NewBufferString(`
					{
						"id": "67fc3def-bbfb-4953-83f8-4ab0682ad675",
						"name": "/example-user",
						"type": "user",
						"value": {
							"username": "FQnwWoxgSrDuqDLmeLpU",
							"password": "6mRPZB3bAfb8lRpacnXsHfDhlPqFcjH2h9YDvLpL",
							"password_hash": "some-hash"
						},
						"version_created_at": "2017-01-05T01:01:01Z"
					}`)),
				}}

				ch, _ := New("https://example.com", Auth(dummy.Builder()), ServerVersion("2.0.0"))

				user := values.User{Username: user, Password: "some-password"}
				cred, _ := ch.SetUser("/example-user", user)

				Expect(cred.Name).To(Equal("/example-user"))
				Expect(cred.Type).To(Equal("user"))
				alternateUsername := "FQnwWoxgSrDuqDLmeLpU"
				Expect(cred.Value.User).To(Equal(values.User{
					Username: alternateUsername,
					Password: "6mRPZB3bAfb8lRpacnXsHfDhlPqFcjH2h9YDvLpL",
				}))

				Expect(cred.Value.PasswordHash).To(Equal("some-hash"))

			})
		})

		Context("when request fails", func() {
			It("returns an error", func() {
				dummy := &DummyAuth{Error: errors.New("network error occurred")}
				ch, _ := New("https://example.com", Auth(dummy.Builder()), ServerVersion("2.0.0"))
				user := values.User{Username: user, Password: "some-password"}
				_, err := ch.SetUser("/example-user", user)
				Expect(err).To(MatchError("network error occurred"))
			})
		})

		Context("when response body cannot be unmarshalled", func() {
			It("returns an error", func() {
				dummy := &DummyAuth{Response: &http.Response{
					Body: ioutil.NopCloser(bytes.NewBufferString("something-invalid")),
				}}
				ch, _ := New("https://example.com", Auth(dummy.Builder()), ServerVersion("2.0.0"))

				user := values.User{Username: user, Password: "some-password"}
				_, err := ch.SetUser("/example-user", user)
				Expect(err).To(MatchError(ContainSubstring("invalid character 's'")))
			})
		})
	})

	Describe("SetRSA()", func() {
		It("requests to set the RSA", func() {
			dummy := &DummyAuth{Response: &http.Response{
				Body: ioutil.NopCloser(bytes.NewBufferString("")),
			}}

			ch, _ := New("https://example.com", Auth(dummy.Builder()), ServerVersion("2.0.0"))
			RSA := values.RSA{PrivateKey: "private-key", PublicKey: "public-key"}

			ch.SetRSA("/example-rsa", RSA)

			urlPath := dummy.Request.URL.Path
			Expect(urlPath).To(Equal("/api/v1/data"))
			Expect(dummy.Request.Method).To(Equal(http.MethodPut))

			body, _ := ioutil.ReadAll(dummy.Request.Body)
			Expect(body).To(MatchJSON(`
			{
				"name": "/example-rsa",
				"type": "rsa",
				"value": {
					"public_key": "public-key",
					"private_key": "private-key"
				}
			}`))
		})

		Context("when server version is not provided", func() {
			var server *ghttp.Server

			BeforeEach(func() {
				server = ghttp.NewServer()
				server.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("GET", "/info"),
						ghttp.RespondWith(http.StatusOK, `{}`),
					),
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("GET", "/version"),
						ghttp.RespondWith(http.StatusOK, `{"version": "1.9.0"}`),
					),
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("PUT", "/api/v1/data"),
						ghttp.RespondWith(http.StatusOK, `{}`),
					),
				)
			})

			It("send request for server version", func() {
				ch, _ := New(server.URL())
				_, err := ch.SetRSA("/example-rsa", values.RSA{})
				Expect(err).NotTo(HaveOccurred())
			})
		})

		Context("when server version is older than 2.x", func() {
			It("set overwrite mode", func() {
				dummy := &DummyAuth{Response: &http.Response{
					Body: ioutil.NopCloser(bytes.NewBufferString("")),
				}}

				version := fmt.Sprintf("1.%d.0", rand.Intn(10))
				ch, _ := New("https://example.com", Auth(dummy.Builder()), ServerVersion(version))
				ch.SetRSA("/example-rsa", values.RSA{})

				var requestBody map[string]interface{}
				body, _ := ioutil.ReadAll(dummy.Request.Body)
				json.Unmarshal(body, &requestBody)

				Expect(requestBody["mode"]).To(Equal("overwrite"))
			})
		})

		Context("when server version is invalid", func() {
			It("returns an error", func() {
				ch, _ := New("https://example.com", ServerVersion("invalid-version"))
				_, err := ch.SetRSA("/example-rsa", values.RSA{})
				Expect(err).To(MatchError("Malformed version: invalid-version"))
			})
		})

		Context("when successful", func() {
			It("returns the credential that has been set", func() {
				dummy := &DummyAuth{Response: &http.Response{
					StatusCode: http.StatusOK,
					Body: ioutil.NopCloser(bytes.NewBufferString(`
					{
						"id": "67fc3def-bbfb-4953-83f8-4ab0682ad676",
						"name": "/example-rsa",
						"type": "rsa",
						"value": {
							"public_key": "public-key",
							"private_key": "private-key"
						},
						"version_created_at": "2017-01-01T04:07:18Z"
					}`)),
				}}

				ch, _ := New("https://example.com", Auth(dummy.Builder()), ServerVersion("2.0.0"))

				cred, _ := ch.SetRSA("/example-rsa", values.RSA{})

				Expect(cred.Name).To(Equal("/example-rsa"))
				Expect(cred.Type).To(Equal("rsa"))
				Expect(cred.Value).To(Equal(values.RSA{
					PrivateKey: "private-key",
					PublicKey:  "public-key",
				}))
			})
		})

		Context("when request fails", func() {
			It("returns an error", func() {
				dummy := &DummyAuth{Error: errors.New("network error occurred")}
				ch, _ := New("https://example.com", Auth(dummy.Builder()), ServerVersion("2.0.0"))
				_, err := ch.SetRSA("/example-rsa", values.RSA{})
				Expect(err).To(MatchError("network error occurred"))
			})
		})

		Context("when response body cannot be unmarshalled", func() {
			It("returns an error", func() {
				dummy := &DummyAuth{Response: &http.Response{
					Body: ioutil.NopCloser(bytes.NewBufferString("something-invalid")),
				}}

				ch, _ := New("https://example.com", Auth(dummy.Builder()), ServerVersion("2.0.0"))
				_, err := ch.SetRSA("/example-rsa", values.RSA{})
				Expect(err).To(MatchError(ContainSubstring("invalid character 's'")))
			})
		})
	})

	Describe("SetSSH()", func() {
		It("requests to set the SSH", func() {
			dummy := &DummyAuth{Response: &http.Response{
				Body: ioutil.NopCloser(bytes.NewBufferString("")),
			}}

			ch, _ := New("https://example.com", Auth(dummy.Builder()), ServerVersion("2.0.0"))
			SSH := values.SSH{PrivateKey: "private-key", PublicKey: "public-key"}

			ch.SetSSH("/example-ssh", SSH)

			urlPath := dummy.Request.URL.Path
			Expect(urlPath).To(Equal("/api/v1/data"))
			Expect(dummy.Request.Method).To(Equal(http.MethodPut))

			body, _ := ioutil.ReadAll(dummy.Request.Body)
			Expect(body).To(MatchJSON(`
			{
				"name": "/example-ssh",
				"type": "ssh",
				"value": {
					"public_key": "public-key",
					"private_key": "private-key"
				}
			}`))
		})

		Context("when server version is not provided", func() {
			var server *ghttp.Server

			BeforeEach(func() {
				server = ghttp.NewServer()
				server.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("GET", "/info"),
						ghttp.RespondWith(http.StatusOK, `{}`),
					),
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("GET", "/version"),
						ghttp.RespondWith(http.StatusOK, `{"version": "1.9.0"}`),
					),
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("PUT", "/api/v1/data"),
						ghttp.RespondWith(http.StatusOK, `{}`),
					),
				)
			})

			It("send request for server version", func() {
				ch, _ := New(server.URL())
				_, err := ch.SetSSH("/example-ssh", values.SSH{})
				Expect(err).NotTo(HaveOccurred())
			})
		})

		Context("when server version is older than 2.x", func() {
			It("set overwrite mode", func() {
				dummy := &DummyAuth{Response: &http.Response{
					Body: ioutil.NopCloser(bytes.NewBufferString("")),
				}}

				version := fmt.Sprintf("1.%d.0", rand.Intn(10))
				ch, _ := New("https://example.com", Auth(dummy.Builder()), ServerVersion(version))
				ch.SetSSH("/example-ssh", values.SSH{})

				var requestBody map[string]interface{}
				body, _ := ioutil.ReadAll(dummy.Request.Body)
				json.Unmarshal(body, &requestBody)

				Expect(requestBody["mode"]).To(Equal("overwrite"))
			})
		})

		Context("when server version is invalid", func() {
			It("returns an error", func() {
				ch, _ := New("https://example.com", ServerVersion("invalid-version"))
				_, err := ch.SetSSH("/example-ssh", values.SSH{})
				Expect(err).To(MatchError("Malformed version: invalid-version"))
			})
		})

		Context("when successful", func() {
			It("returns the credential that has been set", func() {
				dummy := &DummyAuth{Response: &http.Response{
					StatusCode: http.StatusOK,
					Body: ioutil.NopCloser(bytes.NewBufferString(`
					{
						"id": "67fc3def-bbfb-4953-83f8-4ab0682ad676",
						"name": "/example-ssh",
						"type": "ssh",
						"value": {
							"public_key": "public-key",
							"private_key": "private-key"
						},
						"version_created_at": "2017-01-01T04:07:18Z"
					}`)),
				}}

				ch, _ := New("https://example.com", Auth(dummy.Builder()), ServerVersion("2.0.0"))

				cred, _ := ch.SetSSH("/example-ssh", values.SSH{})

				Expect(cred.Name).To(Equal("/example-ssh"))
				Expect(cred.Type).To(Equal("ssh"))
				Expect(cred.Value.SSH).To(Equal(values.SSH{
					PrivateKey: "private-key",
					PublicKey:  "public-key",
				}))
			})
		})

		Context("when request fails", func() {
			It("returns an error", func() {
				dummy := &DummyAuth{Error: errors.New("network error occurred")}
				ch, _ := New("https://example.com", Auth(dummy.Builder()), ServerVersion("2.0.0"))
				_, err := ch.SetSSH("/example-ssh", values.SSH{})
				Expect(err).To(MatchError("network error occurred"))
			})
		})

		Context("when response body cannot be unmarshalled", func() {
			It("returns an error", func() {
				dummy := &DummyAuth{Response: &http.Response{
					Body: ioutil.NopCloser(bytes.NewBufferString("something-invalid")),
				}}

				ch, _ := New("https://example.com", Auth(dummy.Builder()), ServerVersion("2.0.0"))
				_, err := ch.SetSSH("/example-ssh", values.SSH{})
				Expect(err).To(MatchError(ContainSubstring("invalid character 's'")))
			})
		})
	})

	Describe("SetJSON()", func() {
		JSONValue := `{
					"key": 123,
					"key_list": [
					  "val1",
					  "val2"
					],
					"is_true": true
				}`
		It("requests to set the JSON", func() {
			dummy := &DummyAuth{Response: &http.Response{
				Body: ioutil.NopCloser(bytes.NewBufferString("")),
			}}

			ch, _ := New("https://example.com", Auth(dummy.Builder()), ServerVersion("2.0.0"))
			JSON := make(map[string]interface{})
			json.Unmarshal([]byte(JSONValue), &JSON)

			ch.SetJSON("/example-json", JSON)

			urlPath := dummy.Request.URL.Path
			Expect(urlPath).To(Equal("/api/v1/data"))
			Expect(dummy.Request.Method).To(Equal(http.MethodPut))

			body, _ := ioutil.ReadAll(dummy.Request.Body)
			Expect(body).To(MatchJSON(`
			{
			  "name": "/example-json",
			  "type": "json",
			  "value": {
				"key": 123,
				"key_list": [
				  "val1",
				  "val2"
				],
				"is_true": true
			  }
			}`))
		})

		Context("when server version is not provided", func() {
			var server *ghttp.Server

			BeforeEach(func() {
				server = ghttp.NewServer()
				server.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("GET", "/info"),
						ghttp.RespondWith(http.StatusOK, `{}`),
					),
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("GET", "/version"),
						ghttp.RespondWith(http.StatusOK, `{"version": "1.9.0"}`),
					),
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("PUT", "/api/v1/data"),
						ghttp.RespondWith(http.StatusOK, `{}`),
					),
				)
			})

			It("send request for server version", func() {
				ch, _ := New(server.URL())
				_, err := ch.SetJSON("/example-json", nil)
				Expect(err).NotTo(HaveOccurred())
			})
		})

		Context("when server version is older than 2.x", func() {
			It("set overwrite mode", func() {
				dummy := &DummyAuth{Response: &http.Response{
					Body: ioutil.NopCloser(bytes.NewBufferString("")),
				}}

				version := fmt.Sprintf("1.%d.0", rand.Intn(10))
				ch, _ := New("https://example.com", Auth(dummy.Builder()), ServerVersion(version))
				ch.SetJSON("/example-json", nil)

				var requestBody map[string]interface{}
				body, _ := ioutil.ReadAll(dummy.Request.Body)
				json.Unmarshal(body, &requestBody)

				Expect(requestBody["mode"]).To(Equal("overwrite"))
			})
		})

		Context("when server version is invalid", func() {
			It("returns an error", func() {
				ch, _ := New("https://example.com", ServerVersion("invalid-version"))
				_, err := ch.SetJSON("/example-json", nil)
				Expect(err).To(MatchError("Malformed version: invalid-version"))
			})
		})

		Context("when successful", func() {
			It("returns the credential that has been set", func() {
				dummy := &DummyAuth{Response: &http.Response{
					StatusCode: http.StatusOK,
					Body: ioutil.NopCloser(bytes.NewBufferString(`
					{
						"id": "some-id",
						"name": "/example-json",
						"type": "json",
						"value": {
							"key": 123,
							"key_list": [
							  "val1",
							  "val2"
							],
							"is_true": true
						  },
						"version_created_at": "2017-01-01T04:07:18Z"
					}`)),
				}}

				ch, _ := New("https://example.com", Auth(dummy.Builder()), ServerVersion("2.0.0"))

				cred, _ := ch.SetJSON("/example-json", nil)

				var unmarshalled values.JSON
				json.Unmarshal([]byte(JSONValue), &unmarshalled)

				Expect(cred.Name).To(Equal("/example-json"))
				Expect(cred.Type).To(Equal("json"))
				Expect(cred.Value).To(Equal(unmarshalled))
			})
		})

		Context("when request fails", func() {
			It("returns an error", func() {
				dummy := &DummyAuth{Error: errors.New("network error occurred")}
				ch, _ := New("https://example.com", Auth(dummy.Builder()), ServerVersion("2.0.0"))
				_, err := ch.SetJSON("/example-json", nil)
				Expect(err).To(MatchError("network error occurred"))
			})
		})

		Context("when response body cannot be unmarshalled", func() {
			It("returns an error", func() {
				dummy := &DummyAuth{Response: &http.Response{
					Body: ioutil.NopCloser(bytes.NewBufferString("something-invalid")),
				}}

				ch, _ := New("https://example.com", Auth(dummy.Builder()), ServerVersion("2.0.0"))
				_, err := ch.SetJSON("/example-json", nil)
				Expect(err).To(MatchError(ContainSubstring("invalid character 's'")))
			})
		})
	})

	Describe("SetValue()", func() {
		It("requests to set the Value", func() {
			dummy := &DummyAuth{Response: &http.Response{
				Body: ioutil.NopCloser(bytes.NewBufferString("")),
			}}

			ch, _ := New("https://example.com", Auth(dummy.Builder()), ServerVersion("2.0.0"))
			value := values.Value("some string value")

			ch.SetValue("/example-value", value)

			urlPath := dummy.Request.URL.Path
			Expect(urlPath).To(Equal("/api/v1/data"))
			Expect(dummy.Request.Method).To(Equal(http.MethodPut))

			body, _ := ioutil.ReadAll(dummy.Request.Body)
			Expect(body).To(MatchJSON(`
			{
			  "name": "/example-value",
			  "type": "value",
			  "value": "some string value"
			}`))
		})

		Context("when server version is not provided", func() {
			var server *ghttp.Server

			BeforeEach(func() {
				server = ghttp.NewServer()
				server.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("GET", "/info"),
						ghttp.RespondWith(http.StatusOK, `{}`),
					),
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("GET", "/version"),
						ghttp.RespondWith(http.StatusOK, `{"version": "1.9.0"}`),
					),
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("PUT", "/api/v1/data"),
						ghttp.RespondWith(http.StatusOK, `{}`),
					),
				)
			})

			It("send request for server version", func() {
				ch, _ := New(server.URL())
				_, err := ch.SetValue("/example-value", values.Value(""))
				Expect(err).NotTo(HaveOccurred())
			})
		})

		Context("when server version is older than 2.x", func() {
			It("set overwrite mode", func() {
				dummy := &DummyAuth{Response: &http.Response{
					Body: ioutil.NopCloser(bytes.NewBufferString("")),
				}}

				version := fmt.Sprintf("1.%d.0", rand.Intn(10))
				ch, _ := New("https://example.com", Auth(dummy.Builder()), ServerVersion(version))
				ch.SetValue("/example-value", values.Value(""))

				var requestBody map[string]interface{}
				body, _ := ioutil.ReadAll(dummy.Request.Body)
				json.Unmarshal(body, &requestBody)

				Expect(requestBody["mode"]).To(Equal("overwrite"))
			})
		})

		Context("when server version is invalid", func() {
			It("returns an error", func() {
				ch, _ := New("https://example.com", ServerVersion("invalid-version"))
				_, err := ch.SetValue("/example-value", values.Value(""))
				Expect(err).To(MatchError("Malformed version: invalid-version"))
			})
		})

		Context("when successful", func() {
			It("returns the credential that has been set", func() {
				dummy := &DummyAuth{Response: &http.Response{
					StatusCode: http.StatusOK,
					Body: ioutil.NopCloser(bytes.NewBufferString(`
					{
						"id": "some-id",
						"name": "/example-value",
						"type": "value",
						"value": "some string value",
						"version_created_at": "2017-01-01T04:07:18Z"
					}`)),
				}}

				ch, _ := New("https://example.com", Auth(dummy.Builder()), ServerVersion("2.0.0"))

				cred, _ := ch.SetValue("/example-value", values.Value(""))

				Expect(cred.Name).To(Equal("/example-value"))
				Expect(cred.Type).To(Equal("value"))
				Expect(cred.Value).To(Equal(values.Value("some string value")))
			})
		})

		Context("when request fails", func() {
			It("returns an error", func() {
				dummy := &DummyAuth{Error: errors.New("network error occurred")}
				ch, _ := New("https://example.com", Auth(dummy.Builder()), ServerVersion("2.0.0"))
				_, err := ch.SetValue("/example-value", values.Value(""))
				Expect(err).To(MatchError("network error occurred"))
			})
		})

		Context("when response body cannot be unmarshalled", func() {
			It("returns an error", func() {
				dummy := &DummyAuth{Response: &http.Response{
					Body: ioutil.NopCloser(bytes.NewBufferString("something-invalid")),
				}}

				ch, _ := New("https://example.com", Auth(dummy.Builder()), ServerVersion("2.0.0"))
				_, err := ch.SetValue("/example-value", values.Value(""))
				Expect(err).To(MatchError(ContainSubstring("invalid character 's'")))
			})
		})
	})

})
