package credhub_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"io/ioutil"
	"net/http"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "code.cloudfoundry.org/credhub-cli/credhub"
	"code.cloudfoundry.org/credhub-cli/credhub/credentials/generate"
)

var _ = Describe("Generate", func() {

	Describe("GenerateCertificate()", func() {
		It("requests to generate the certificate", func() {
			dummy := &DummyAuth{Response: &http.Response{
				Body: ioutil.NopCloser(bytes.NewBufferString("")),
			}}

			ch, _ := New("https://example.com", Auth(dummy.Builder()))

			cert := generate.Certificate{
				Ca: "some-ca",
			}
			ch.GenerateCertificate("/example-certificate", cert, NoOverwrite)
			urlPath := dummy.Request.URL.Path
			Expect(urlPath).To(Equal("/api/v1/data"))
			Expect(dummy.Request.Method).To(Equal(http.MethodPost))

			var requestBody map[string]interface{}
			body, _ := ioutil.ReadAll(dummy.Request.Body)
			json.Unmarshal(body, &requestBody)

			Expect(requestBody["name"]).To(Equal("/example-certificate"))
			Expect(requestBody["type"]).To(Equal("certificate"))
			Expect(requestBody["overwrite"]).To(BeFalse())
			Expect(requestBody["parameters"].(map[string]interface{})["ca"]).To(Equal("some-ca"))
		})

		It("requests to set the certificate with the correct overwrite mode if server version isn't less than 1.6.0", func() {
			dummy := &DummyAuth{Response: &http.Response{
				Body: ioutil.NopCloser(bytes.NewBufferString("")),
			}}

			ch, _ := New("https://example.com", Auth(dummy.Builder()))

			certificate := generate.Certificate{
				Ca:         "some-ca",
				CommonName: "/some-ca-name",
			}
			ch.GenerateCertificate("/example-certificate", certificate, Converge)

			urlPath := dummy.Request.URL.Path
			Expect(urlPath).To(Equal("/api/v1/data"))
			Expect(dummy.Request.Method).To(Equal(http.MethodPost))

			var requestBody map[string]interface{}
			body, _ := ioutil.ReadAll(dummy.Request.Body)
			json.Unmarshal(body, &requestBody)

			Expect(requestBody["name"]).To(Equal("/example-certificate"))
			Expect(requestBody["type"]).To(Equal("certificate"))
			Expect(requestBody["mode"]).To(Equal("converge"))
			Expect(requestBody["parameters"].(map[string]interface{})["ca"]).To(Equal("some-ca"))
		})

		Context("when successful", func() {
			It("returns the generated certificate", func() {
				dummy := &DummyAuth{Response: &http.Response{
					StatusCode: http.StatusOK,
					Body: ioutil.NopCloser(bytes.NewBufferString(`{
      "id": "some-id",
      "name": "/example-certificate",
      "type": "certificate",
      "value": {
        "ca": "some-ca",
        "certificate": "-----BEGIN CERTIFICATE-----\n...\n-----END CERTIFICATE-----",
        "private_key": "-----BEGIN RSA PRIVATE KEY-----\n...\n-----END RSA PRIVATE KEY-----"
      },
      "version_created_at": "2017-01-01T04:07:18Z"
}`)),
				}}

				ch, _ := New("https://example.com", Auth(dummy.Builder()))

				cert := generate.Certificate{
					Ca: "some-ca",
				}

				generatedCert, err := ch.GenerateCertificate("/example-certificate", cert, NoOverwrite)
				Expect(err).ToNot(HaveOccurred())
				Expect(generatedCert.Id).To(Equal("some-id"))
				Expect(generatedCert.Name).To(Equal("/example-certificate"))
				Expect(generatedCert.Value.Ca).To(Equal("some-ca"))
				Expect(generatedCert.Value.Certificate).To(Equal("-----BEGIN CERTIFICATE-----\n...\n-----END CERTIFICATE-----"))
				Expect(generatedCert.Value.PrivateKey).To(Equal("-----BEGIN RSA PRIVATE KEY-----\n...\n-----END RSA PRIVATE KEY-----"))

			})
		})

		Context("when request fails", func() {
			var err error
			It("returns an error", func() {
				networkError := errors.New("Network error occurred")
				dummy := &DummyAuth{Error: networkError}
				ch, _ := New("https://example.com", Auth(dummy.Builder()))

				cert := generate.Certificate{
					Ca: "some-ca",
				}

				_, err = ch.GenerateCertificate("/example-certificate", cert, NoOverwrite)

				Expect(err).To(HaveOccurred())
			})
		})

		Context("when response body cannot be unmarshalled", func() {
			It("returns an error", func() {

				dummy := &DummyAuth{Response: &http.Response{
					Body: ioutil.NopCloser(bytes.NewBufferString("invalid-response")),
				}}
				ch, _ := New("https://example.com", Auth(dummy.Builder()))

				cert := generate.Certificate{
					Ca: "some-ca",
				}

				_, err := ch.GenerateCertificate("/example-certificate", cert, NoOverwrite)

				Expect(err).To(HaveOccurred())
			})

		})
	})

	Describe("GeneratePassword()", func() {
		It("requests to generate the password", func() {
			dummy := &DummyAuth{Response: &http.Response{
				Body: ioutil.NopCloser(bytes.NewBufferString("")),
			}}

			ch, _ := New("https://example.com", Auth(dummy.Builder()))

			passwordOptions := generate.Password{
				Length: 12,
			}
			ch.GeneratePassword("/example-password", passwordOptions, Overwrite)
			urlPath := dummy.Request.URL.Path
			Expect(urlPath).To(Equal("/api/v1/data"))

			Expect(dummy.Request.Method).To(Equal(http.MethodPost))

			var requestBody map[string]interface{}
			body, _ := ioutil.ReadAll(dummy.Request.Body)
			json.Unmarshal(body, &requestBody)

			Expect(requestBody["name"]).To(Equal("/example-password"))
			Expect(requestBody["type"]).To(Equal("password"))
			Expect(requestBody["overwrite"]).To(BeTrue())
			Expect(requestBody["parameters"].(map[string]interface{})["length"]).To(BeEquivalentTo(12))
		})

		Context("when successful", func() {
			It("returns the generated password", func() {
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

				ch, _ := New("https://example.com", Auth(dummy.Builder()))

				p := generate.Password{
					Length: 12,
				}

				generatedPassword, err := ch.GeneratePassword("/example-password", p, NoOverwrite)
				Expect(err).ToNot(HaveOccurred())
				Expect(generatedPassword.Id).To(Equal("some-id"))
				Expect(generatedPassword.Type).To(Equal("password"))
				Expect(generatedPassword.Name).To(Equal("/example-password"))
				Expect(generatedPassword.Value).To(BeEquivalentTo("some-password"))
			})
		})

		Context("when request fails to complete", func() {
			var err error
			It("returns an error", func() {
				networkError := errors.New("Network error occurred")
				dummy := &DummyAuth{Error: networkError}
				ch, _ := New("https://example.com", Auth(dummy.Builder()))

				_, err = ch.GeneratePassword("/example-password", generate.Password{}, NoOverwrite)

				Expect(err).To(HaveOccurred())
			})
		})

		Context("when response body cannot be unmarshalled", func() {
			It("returns an error", func() {

				dummy := &DummyAuth{Response: &http.Response{
					Body: ioutil.NopCloser(bytes.NewBufferString("invalid-response")),
				}}
				ch, _ := New("https://example.com", Auth(dummy.Builder()))

				_, err := ch.GeneratePassword("/example-password", generate.Password{}, NoOverwrite)

				Expect(err).To(HaveOccurred())
			})

		})
	})

	Describe("GenerateUser()", func() {
		It("requests to generate the user", func() {
			dummy := &DummyAuth{Response: &http.Response{
				Body: ioutil.NopCloser(bytes.NewBufferString("")),
			}}

			ch, _ := New("https://example.com", Auth(dummy.Builder()))

			userOptions := generate.User{
				Username: "name",
				Length:   12,
			}

			ch.GenerateUser("/example-user", userOptions, Overwrite)
			urlPath := dummy.Request.URL.Path
			Expect(urlPath).To(Equal("/api/v1/data"))

			Expect(dummy.Request.Method).To(Equal(http.MethodPost))

			var requestBody map[string]interface{}
			body, _ := ioutil.ReadAll(dummy.Request.Body)
			json.Unmarshal(body, &requestBody)

			Expect(requestBody["name"]).To(Equal("/example-user"))
			Expect(requestBody["type"]).To(Equal("user"))
			Expect(requestBody["overwrite"]).To(BeTrue())
			Expect(requestBody["value"]).To(Equal(map[string]interface{}{"username": "name"}))
			Expect(requestBody["parameters"]).To(Equal(map[string]interface{}{"length": float64(12)}))
		})

		Context("when successful", func() {
			It("returns the generated user", func() {
				dummy := &DummyAuth{Response: &http.Response{
					StatusCode: http.StatusOK,
					Body: ioutil.NopCloser(bytes.NewBufferString(`
					 {
  "id": "some-id",
  "name": "/example-user",
  "type": "user",
  "value": {
    "username": "generated-username",
    "password": "generated-password"
  },
  "version_created_at": "2017-01-05T01:01:01Z"
}`)),
				}}

				ch, _ := New("https://example.com", Auth(dummy.Builder()))

				p := generate.User{
					Length: 12,
				}

				generatedUser, err := ch.GenerateUser("/example-user", p, NoOverwrite)
				Expect(err).ToNot(HaveOccurred())
				Expect(generatedUser.Id).To(Equal("some-id"))
				Expect(generatedUser.Type).To(Equal("user"))
				Expect(generatedUser.Name).To(Equal("/example-user"))
				Expect(generatedUser.Value.Username).To(BeEquivalentTo("generated-username"))
				Expect(generatedUser.Value.Password).To(BeEquivalentTo("generated-password"))
			})
		})

		Context("when request fails to complete", func() {
			var err error
			It("returns an error", func() {
				networkError := errors.New("Network error occurred")
				dummy := &DummyAuth{Error: networkError}
				ch, _ := New("https://example.com", Auth(dummy.Builder()))

				_, err = ch.GenerateUser("/example-user", generate.User{}, NoOverwrite)

				Expect(err).To(HaveOccurred())
			})
		})

		Context("when response body cannot be unmarshalled", func() {
			It("returns an error", func() {

				dummy := &DummyAuth{Response: &http.Response{
					Body: ioutil.NopCloser(bytes.NewBufferString("invalid-response")),
				}}
				ch, _ := New("https://example.com", Auth(dummy.Builder()))

				_, err := ch.GenerateUser("/example-user", generate.User{}, NoOverwrite)

				Expect(err).To(HaveOccurred())
			})

		})
	})

	Describe("GenerateRSA()", func() {
		It("requests to generate the RSA", func() {
			dummy := &DummyAuth{Response: &http.Response{
				Body: ioutil.NopCloser(bytes.NewBufferString("")),
			}}

			ch, _ := New("https://example.com", Auth(dummy.Builder()))

			rsaOptions := generate.RSA{
				KeyLength: 2048,
			}

			ch.GenerateRSA("/example-rsa", rsaOptions, Overwrite)
			urlPath := dummy.Request.URL.Path
			Expect(urlPath).To(Equal("/api/v1/data"))

			Expect(dummy.Request.Method).To(Equal(http.MethodPost))

			body, _ := ioutil.ReadAll(dummy.Request.Body)
			Expect(body).To(MatchJSON(`
			{
				"name": "/example-rsa",
				"type": "rsa",
				"parameters": {
					"key_length": 2048
				},
				"overwrite": true
			}`))
		})

		Context("when successful", func() {
			It("returns the generated RSA", func() {
				dummy := &DummyAuth{Response: &http.Response{
					StatusCode: http.StatusOK,
					Body: ioutil.NopCloser(bytes.NewBufferString(`
					{
					  "id": "some-id",
					  "name": "/example-rsa",
					  "type": "rsa",
					  "value": {
						"public_key": "generated-public-key",
						"private_key": "generated-private-key"
					  },
					  "version_created_at": "2017-01-01T04:07:18Z"
					}`)),
				}}

				ch, _ := New("https://example.com", Auth(dummy.Builder()))

				p := generate.RSA{
					KeyLength: 2048,
				}

				generatedRSA, err := ch.GenerateRSA("/example-rsa", p, NoOverwrite)

				Expect(err).ToNot(HaveOccurred())
				Expect(generatedRSA.Id).To(Equal("some-id"))
				Expect(generatedRSA.Type).To(Equal("rsa"))
				Expect(generatedRSA.Name).To(Equal("/example-rsa"))
				Expect(generatedRSA.Value.PublicKey).To(Equal("generated-public-key"))
				Expect(generatedRSA.Value.PrivateKey).To(Equal("generated-private-key"))
			})
		})

		Context("when request fails to complete", func() {
			var err error
			It("returns an error", func() {
				networkError := errors.New("Network error occurred")
				dummy := &DummyAuth{Error: networkError}
				ch, _ := New("https://example.com", Auth(dummy.Builder()))

				_, err = ch.GenerateRSA("/example-rsa", generate.RSA{}, NoOverwrite)

				Expect(err).To(HaveOccurred())
			})
		})

		Context("when response body cannot be unmarshalled", func() {
			It("returns an error", func() {

				dummy := &DummyAuth{Response: &http.Response{
					Body: ioutil.NopCloser(bytes.NewBufferString("invalid-response")),
				}}
				ch, _ := New("https://example.com", Auth(dummy.Builder()))

				_, err := ch.GenerateRSA("/example-rsa", generate.RSA{}, NoOverwrite)

				Expect(err).To(HaveOccurred())
			})

		})
	})

	Describe("GenerateSSH()", func() {
		It("requests to generate the SSH", func() {
			dummy := &DummyAuth{Response: &http.Response{
				Body: ioutil.NopCloser(bytes.NewBufferString("")),
			}}

			ch, _ := New("https://example.com", Auth(dummy.Builder()))

			sshOptions := generate.SSH{
				KeyLength: 2048,
			}

			ch.GenerateSSH("/example-ssh", sshOptions, Overwrite)
			urlPath := dummy.Request.URL.Path
			Expect(urlPath).To(Equal("/api/v1/data"))

			Expect(dummy.Request.Method).To(Equal(http.MethodPost))

			body, _ := ioutil.ReadAll(dummy.Request.Body)
			Expect(body).To(MatchJSON(`
			{
				"name": "/example-ssh",
				"type": "ssh",
				"parameters": {
					"key_length": 2048
				},
				"overwrite": true
			}`))
		})

		Context("when successful", func() {
			It("returns the generated SSH", func() {
				dummy := &DummyAuth{Response: &http.Response{
					StatusCode: http.StatusOK,
					Body: ioutil.NopCloser(bytes.NewBufferString(`
					{
					  "id": "some-id",
					  "name": "/example-ssh",
					  "type": "ssh",
					  "value": {
						"public_key": "generated-public-key",
						"private_key": "generated-private-key"
					  },
					  "version_created_at": "2017-01-01T04:07:18Z"
					}`)),
				}}

				ch, _ := New("https://example.com", Auth(dummy.Builder()))

				p := generate.SSH{
					KeyLength: 2048,
				}

				generatedSSH, err := ch.GenerateSSH("/example-ssh", p, NoOverwrite)

				Expect(err).ToNot(HaveOccurred())
				Expect(generatedSSH.Id).To(Equal("some-id"))
				Expect(generatedSSH.Type).To(Equal("ssh"))
				Expect(generatedSSH.Name).To(Equal("/example-ssh"))
				Expect(generatedSSH.Value.PublicKey).To(Equal("generated-public-key"))
				Expect(generatedSSH.Value.PrivateKey).To(Equal("generated-private-key"))
			})
		})

		Context("when request fails to complete", func() {
			var err error
			It("returns an error", func() {
				networkError := errors.New("Network error occurred")
				dummy := &DummyAuth{Error: networkError}
				ch, _ := New("https://example.com", Auth(dummy.Builder()))

				_, err = ch.GenerateSSH("/example-ssh", generate.SSH{}, NoOverwrite)

				Expect(err).To(HaveOccurred())
			})
		})

		Context("when response body cannot be unmarshalled", func() {
			It("returns an error", func() {

				dummy := &DummyAuth{Response: &http.Response{
					Body: ioutil.NopCloser(bytes.NewBufferString("invalid-response")),
				}}
				ch, _ := New("https://example.com", Auth(dummy.Builder()))

				_, err := ch.GenerateSSH("/example-ssh", generate.SSH{}, NoOverwrite)

				Expect(err).To(HaveOccurred())
			})

		})
	})
})
