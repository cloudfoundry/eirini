package credhub_test

import (
	"bytes"
	"errors"
	"io/ioutil"
	"net/http"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"encoding/json"

	. "code.cloudfoundry.org/credhub-cli/credhub"
	"code.cloudfoundry.org/credhub-cli/credhub/credentials/values"
	. "github.com/onsi/ginkgo/extensions/table"
)

var _ = Describe("Get", func() {

	Describe("GetLatestVersion()", func() {
		It("requests the credential by name using the 'current' query parameter", func() {
			dummyAuth := &DummyAuth{Response: &http.Response{
				Body: ioutil.NopCloser(bytes.NewBufferString("")),
			}}

			ch, _ := New("https://example.com", Auth(dummyAuth.Builder()))

			ch.GetLatestVersion("/example-password")
			url := dummyAuth.Request.URL.String()
			Expect(url).To(Equal("https://example.com/api/v1/data?current=true&name=%2Fexample-password"))
			Expect(dummyAuth.Request.Method).To(Equal(http.MethodGet))
		})

		Context("when successful", func() {
			It("returns a credential by name", func() {
				responseString := `{
	"data": [
	{
      "id": "some-id",
      "name": "/example-password",
      "type": "password",
      "value": "some-password",
      "version_created_at": "2017-01-05T01:01:01Z"
    }
    ]}`
				dummyAuth := &DummyAuth{Response: &http.Response{
					StatusCode: http.StatusOK,
					Body:       ioutil.NopCloser(bytes.NewBufferString(responseString)),
				}}

				ch, _ := New("https://example.com", Auth(dummyAuth.Builder()))

				cred, err := ch.GetLatestVersion("/example-password")
				Expect(err).To(BeNil())
				Expect(cred.Id).To(Equal("some-id"))
				Expect(cred.Name).To(Equal("/example-password"))
				Expect(cred.Type).To(Equal("password"))
				Expect(cred.Value.(string)).To(Equal("some-password"))
				Expect(cred.VersionCreatedAt).To(Equal("2017-01-05T01:01:01Z"))
			})
		})

		Context("when response body cannot be unmarshalled", func() {
			It("returns an error", func() {
				dummyAuth := &DummyAuth{Response: &http.Response{
					Body: ioutil.NopCloser(bytes.NewBufferString("something-invalid")),
				}}

				ch, _ := New("https://example.com", Auth(dummyAuth.Builder()))
				_, err := ch.GetLatestVersion("/example-password")

				Expect(err).To(HaveOccurred())
			})
		})

		Context("when the response body contains an empty list", func() {
			It("returns an error", func() {
				dummyAuth := &DummyAuth{Response: &http.Response{
					StatusCode: http.StatusOK,
					Body:       ioutil.NopCloser(bytes.NewBufferString(`{"data":[]}`)),
				}}
				ch, _ := New("https://example.com", Auth(dummyAuth.Builder()))
				_, err := ch.GetLatestVersion("/example-password")

				Expect(err).To(MatchError("response did not contain any credentials"))
			})
		})
	})

	Describe("GetById()", func() {
		It("requests the credential by id", func() {
			dummyAuth := &DummyAuth{Response: &http.Response{
				Body: ioutil.NopCloser(bytes.NewBufferString("")),
			}}

			ch, _ := New("https://example.com", Auth(dummyAuth.Builder()))

			ch.GetById("0239482304958")
			url := dummyAuth.Request.URL.String()
			Expect(url).To(Equal("https://example.com/api/v1/data/0239482304958"))
			Expect(dummyAuth.Request.Method).To(Equal(http.MethodGet))
		})

		Context("when successful", func() {
			It("returns a credential by name", func() {
				responseString := `{
      "id": "0239482304958",
      "name": "/reasonable-password",
      "type": "password",
      "value": "some-password",
      "version_created_at": "2017-01-05T01:01:01Z"
    }`
				dummyAuth := &DummyAuth{Response: &http.Response{
					StatusCode: http.StatusOK,
					Body:       ioutil.NopCloser(bytes.NewBufferString(responseString)),
				}}

				ch, _ := New("https://example.com", Auth(dummyAuth.Builder()))

				cred, err := ch.GetById("0239482304958")
				Expect(err).To(BeNil())
				Expect(cred.Id).To(Equal("0239482304958"))
				Expect(cred.Name).To(Equal("/reasonable-password"))
				Expect(cred.Type).To(Equal("password"))
				Expect(cred.Value.(string)).To(Equal("some-password"))
				Expect(cred.VersionCreatedAt).To(Equal("2017-01-05T01:01:01Z"))
			})
		})

		Context("when response body cannot be unmarshalled", func() {
			It("returns an error", func() {
				dummyAuth := &DummyAuth{Response: &http.Response{
					Body: ioutil.NopCloser(bytes.NewBufferString("something-invalid")),
				}}

				ch, _ := New("https://example.com", Auth(dummyAuth.Builder()))
				_, err := ch.GetById("0239482304958")

				Expect(err).To(HaveOccurred())
			})
		})

		Context("when the credential does not exist", func() {
			It("returns the error from the server", func() {
				dummyAuth := &DummyAuth{Response: &http.Response{
					StatusCode: http.StatusNotFound,
					Body:       ioutil.NopCloser(bytes.NewBufferString(`{"error":"The request could not be completed because the credential does not exist or you do not have sufficient authorization."}`)),
				}}
				ch, _ := New("https://example.com", Auth(dummyAuth.Builder()))
				_, err := ch.GetById("0239482304958")

				Expect(err).To(MatchError("The request could not be completed because the credential does not exist or you do not have sufficient authorization."))
			})
		})
	})

	Describe("GetAllVersions()", func() {
		It("makes a request for all versions of a credential", func() {
			dummyAuth := &DummyAuth{Response: &http.Response{
				Body: ioutil.NopCloser(bytes.NewBufferString("")),
			}}

			ch, _ := New("https://example.com", Auth(dummyAuth.Builder()))

			ch.GetAllVersions("/example-password")
			url := dummyAuth.Request.URL.String()
			Expect(url).To(Equal("https://example.com/api/v1/data?name=%2Fexample-password"))
			Expect(dummyAuth.Request.Method).To(Equal(http.MethodGet))
		})

		Context("when successful", func() {
			It("returns a list of all passwords", func() {
				responseString := `{
	"data": [
	{
      "id": "some-id",
      "name": "/example-password",
      "type": "password",
      "value": "some-password",
      "version_created_at": "2017-01-05T01:01:01Z"
    },
	{
      "id": "some-id",
      "name": "/example-password",
      "type": "password",
      "value": "some-other-password",
      "version_created_at": "2017-01-05T01:01:01Z"
    }
    ]}`
				dummyAuth := &DummyAuth{Response: &http.Response{
					StatusCode: http.StatusOK,
					Body:       ioutil.NopCloser(bytes.NewBufferString(responseString)),
				}}

				ch, _ := New("https://example.com", Auth(dummyAuth.Builder()))

				creds, err := ch.GetAllVersions("/example-password")
				Expect(err).To(BeNil())
				Expect(creds[0].Id).To(Equal("some-id"))
				Expect(creds[0].Name).To(Equal("/example-password"))
				Expect(creds[0].Type).To(Equal("password"))
				Expect(creds[0].Value.(string)).To(Equal("some-password"))
				Expect(creds[0].VersionCreatedAt).To(Equal("2017-01-05T01:01:01Z"))

				Expect(creds[1].Value.(string)).To(Equal("some-other-password"))
			})

			It("returns a list of all users", func() {
				responseString := `{
	"data": [
	{
      "id": "some-id",
      "name": "/example-user",
      "type": "user",
      "value": {
      	"username": "first-username",
      	"password": "dummy_password",
      	"password_hash": "$6$kjhlkjh$lkjhasdflkjhasdflkjh"
      },
      "version_created_at": "2017-01-05T01:01:01Z"
    },
	{
      "id": "some-id",
      "name": "/example-user",
      "type": "user",
      "value": {
      	"username": "second-username",
      	"password": "another_random_dummy_password",
      	"password_hash": "$6$kjhlkjh$lkjhasdflkjhasdflkjh"
      },
      "version_created_at": "2017-01-05T01:01:01Z"
    }
    ]}`
				dummyAuth := &DummyAuth{Response: &http.Response{
					StatusCode: http.StatusOK,
					Body:       ioutil.NopCloser(bytes.NewBufferString(responseString)),
				}}

				ch, _ := New("https://example.com", Auth(dummyAuth.Builder()))

				creds, err := ch.GetAllVersions("/example-user")
				Expect(err).To(BeNil())
				Expect(creds[0].Id).To(Equal("some-id"))
				Expect(creds[0].Name).To(Equal("/example-user"))
				Expect(creds[0].Type).To(Equal("user"))
				firstCredValue := creds[0].Value.(map[string]interface{})
				Expect(firstCredValue["username"]).To(Equal("first-username"))
				Expect(firstCredValue["password"]).To(Equal("dummy_password"))
				Expect(firstCredValue["password_hash"]).To(Equal("$6$kjhlkjh$lkjhasdflkjhasdflkjh"))
				Expect(creds[0].VersionCreatedAt).To(Equal("2017-01-05T01:01:01Z"))

				Expect(creds[1].Value.(map[string]interface{})["username"]).To(Equal("second-username"))
			})
		})

		Context("when response body cannot be unmarshalled", func() {
			It("returns an error", func() {
				dummyAuth := &DummyAuth{Response: &http.Response{
					Body: ioutil.NopCloser(bytes.NewBufferString("something-invalid")),
				}}

				ch, _ := New("https://example.com", Auth(dummyAuth.Builder()))
				_, err := ch.GetAllVersions("/example-password")

				Expect(err).To(HaveOccurred())
			})
		})

		Context("when the response body contains an empty list", func() {
			It("returns an error", func() {
				dummyAuth := &DummyAuth{Response: &http.Response{
					StatusCode: http.StatusOK,
					Body:       ioutil.NopCloser(bytes.NewBufferString(`{"data":[]}`)),
				}}
				ch, _ := New("https://example.com", Auth(dummyAuth.Builder()))
				_, err := ch.GetAllVersions("/example-password")

				Expect(err).To(MatchError("response did not contain any credentials"))
			})
		})
	})

	Describe("GetNVersions()", func() {
		It("makes a request for N versions of a credential", func() {
			dummyAuth := &DummyAuth{Response: &http.Response{
				Body: ioutil.NopCloser(bytes.NewBufferString("")),
			}}

			ch, _ := New("https://example.com", Auth(dummyAuth.Builder()))

			ch.GetNVersions("/example-password", 3)
			url := dummyAuth.Request.URL.String()
			Expect(url).To(Equal("https://example.com/api/v1/data?name=%2Fexample-password&versions=3"))
			Expect(dummyAuth.Request.Method).To(Equal(http.MethodGet))
		})

		Context("when successful", func() {
			It("returns a list of N passwords", func() {
				responseString := `{
	"data": [
	{
      "id": "some-id",
      "name": "/example-password",
      "type": "password",
      "value": "some-password",
      "version_created_at": "2017-01-05T01:01:01Z"
    },
	{
      "id": "some-id",
      "name": "/example-password",
      "type": "password",
      "value": "some-other-password",
      "version_created_at": "2017-01-05T01:01:01Z"
    }
    ]}`
				dummyAuth := &DummyAuth{Response: &http.Response{
					StatusCode: http.StatusOK,
					Body:       ioutil.NopCloser(bytes.NewBufferString(responseString)),
				}}

				ch, _ := New("https://example.com", Auth(dummyAuth.Builder()))

				creds, err := ch.GetNVersions("/example-password", 2)
				Expect(err).To(BeNil())
				Expect(creds[0].Id).To(Equal("some-id"))
				Expect(creds[0].Name).To(Equal("/example-password"))
				Expect(creds[0].Type).To(Equal("password"))
				Expect(creds[0].Value.(string)).To(Equal("some-password"))
				Expect(creds[0].VersionCreatedAt).To(Equal("2017-01-05T01:01:01Z"))

				Expect(creds[1].Value.(string)).To(Equal("some-other-password"))
			})

			It("returns a list of N users", func() {
				responseString := `{
	"data": [
	{
      "id": "some-id",
      "name": "/example-user",
      "type": "user",
      "value": {
      	"username": "first-username",
      	"password": "dummy_password",
      	"password_hash": "$6$kjhlkjh$lkjhasdflkjhasdflkjh"
      },
      "version_created_at": "2017-01-05T01:01:01Z"
    },
	{
      "id": "some-id",
      "name": "/example-user",
      "type": "user",
      "value": {
      	"username": "second-username",
      	"password": "another_random_dummy_password",
      	"password_hash": "$6$kjhlkjh$lkjhasdflkjhasdflkjh"
      },
      "version_created_at": "2017-01-05T01:01:01Z"
    }
    ]}`
				dummyAuth := &DummyAuth{Response: &http.Response{
					StatusCode: http.StatusOK,
					Body:       ioutil.NopCloser(bytes.NewBufferString(responseString)),
				}}

				ch, _ := New("https://example.com", Auth(dummyAuth.Builder()))

				creds, err := ch.GetNVersions("/example-user", 2)
				Expect(err).To(BeNil())
				Expect(creds[0].Id).To(Equal("some-id"))
				Expect(creds[0].Name).To(Equal("/example-user"))
				Expect(creds[0].Type).To(Equal("user"))
				firstCredValue := creds[0].Value.(map[string]interface{})
				Expect(firstCredValue["username"]).To(Equal("first-username"))
				Expect(firstCredValue["password"]).To(Equal("dummy_password"))
				Expect(firstCredValue["password_hash"]).To(Equal("$6$kjhlkjh$lkjhasdflkjhasdflkjh"))
				Expect(creds[0].VersionCreatedAt).To(Equal("2017-01-05T01:01:01Z"))

				Expect(creds[1].Value.(map[string]interface{})["username"]).To(Equal("second-username"))
			})
		})

		Context("when response body cannot be unmarshalled", func() {
			It("returns an error", func() {
				dummyAuth := &DummyAuth{Response: &http.Response{
					Body: ioutil.NopCloser(bytes.NewBufferString("something-invalid")),
				}}

				ch, _ := New("https://example.com", Auth(dummyAuth.Builder()))
				_, err := ch.GetNVersions("/example-password", 2)

				Expect(err).To(HaveOccurred())
			})
		})

		Context("when the response body contains an empty list", func() {
			It("returns an error", func() {
				dummyAuth := &DummyAuth{Response: &http.Response{
					StatusCode: http.StatusOK,
					Body:       ioutil.NopCloser(bytes.NewBufferString(`{"data":[]}`)),
				}}
				ch, _ := New("https://example.com", Auth(dummyAuth.Builder()))
				_, err := ch.GetNVersions("/example-password", 2)

				Expect(err).To(MatchError("response did not contain any credentials"))
			})
		})
	})

	Describe("GetLatestPassword()", func() {
		It("requests the credential by name", func() {
			dummyAuth := &DummyAuth{Response: &http.Response{
				Body: ioutil.NopCloser(bytes.NewBufferString("")),
			}}

			ch, _ := New("https://example.com", Auth(dummyAuth.Builder()))
			ch.GetLatestPassword("/example-password")
			url := dummyAuth.Request.URL
			Expect(url.String()).To(ContainSubstring("https://example.com/api/v1/data"))
			Expect(url.String()).To(ContainSubstring("name=%2Fexample-password"))
			Expect(url.String()).To(ContainSubstring("current=true"))
			Expect(dummyAuth.Request.Method).To(Equal(http.MethodGet))
		})

		Context("when successful", func() {
			It("returns a password credential", func() {
				responseString := `{
  "data": [
    {
      "id": "some-id",
      "name": "/example-password",
      "type": "password",
      "value": "some-password",
      "version_created_at": "2017-01-05T01:01:01Z"
    }]}`
				dummyAuth := &DummyAuth{Response: &http.Response{
					StatusCode: http.StatusOK,
					Body:       ioutil.NopCloser(bytes.NewBufferString(responseString)),
				}}

				ch, _ := New("https://example.com", Auth(dummyAuth.Builder()))
				cred, err := ch.GetLatestPassword("/example-password")
				Expect(err).ToNot(HaveOccurred())
				Expect(cred.Value).To(BeEquivalentTo("some-password"))
			})
		})

		Context("when response body cannot be unmarshalled", func() {
			It("returns an error", func() {
				dummyAuth := &DummyAuth{Response: &http.Response{
					Body: ioutil.NopCloser(bytes.NewBufferString("something-invalid")),
				}}
				ch, _ := New("https://example.com", Auth(dummyAuth.Builder()))
				_, err := ch.GetLatestPassword("/example-cred")

				Expect(err).To(HaveOccurred())
			})
		})
	})

	Describe("GetLatestCertificate()", func() {
		It("requests the credential by name", func() {
			dummyAuth := &DummyAuth{Response: &http.Response{
				Body: ioutil.NopCloser(bytes.NewBufferString("")),
			}}

			ch, _ := New("https://example.com", Auth(dummyAuth.Builder()))
			ch.GetLatestCertificate("/example-certificate")
			url := dummyAuth.Request.URL
			Expect(url.String()).To(ContainSubstring("https://example.com/api/v1/data"))
			Expect(url.String()).To(ContainSubstring("name=%2Fexample-certificate"))
			Expect(url.String()).To(ContainSubstring("current=true"))

			Expect(dummyAuth.Request.Method).To(Equal(http.MethodGet))
		})

		Context("when successful", func() {
			It("returns a certificate credential", func() {
				responseString := `{
				  "data": [{
	"id": "some-id",
	"name": "/example-certificate",
	"type": "certificate",
	"value": {
		"ca": "-----BEGIN CERTIFICATE-----\n...\n-----END CERTIFICATE-----",
		"certificate": "-----BEGIN CERTIFICATE-----\n...\n-----END CERTIFICATE-----",
		"private_key": "-----BEGIN RSA PRIVATE KEY-----\n...\n-----END RSA PRIVATE KEY-----"
	},
	"version_created_at": "2017-01-01T04:07:18Z"
}]}`
				dummyAuth := &DummyAuth{Response: &http.Response{
					StatusCode: http.StatusOK,
					Body:       ioutil.NopCloser(bytes.NewBufferString(responseString)),
				}}

				ch, _ := New("https://example.com", Auth(dummyAuth.Builder()))

				cred, err := ch.GetLatestCertificate("/example-certificate")
				Expect(err).ToNot(HaveOccurred())
				Expect(cred.Value.Ca).To(Equal("-----BEGIN CERTIFICATE-----\n...\n-----END CERTIFICATE-----"))
				Expect(cred.Value.Certificate).To(Equal("-----BEGIN CERTIFICATE-----\n...\n-----END CERTIFICATE-----"))
				Expect(cred.Value.PrivateKey).To(Equal("-----BEGIN RSA PRIVATE KEY-----\n...\n-----END RSA PRIVATE KEY-----"))
				Expect(cred.VersionCreatedAt).To(Equal("2017-01-01T04:07:18Z"))
			})
		})

		Context("when response body cannot be unmarshalled", func() {
			It("returns an error", func() {
				dummyAuth := &DummyAuth{Response: &http.Response{
					Body: ioutil.NopCloser(bytes.NewBufferString("something-invalid")),
				}}
				ch, _ := New("https://example.com", Auth(dummyAuth.Builder()))
				_, err := ch.GetLatestCertificate("/example-cred")

				Expect(err).To(HaveOccurred())
			})
		})
	})

	Describe("GetLatestUser()", func() {
		It("requests the credential by name", func() {
			dummyAuth := &DummyAuth{Response: &http.Response{
				Body: ioutil.NopCloser(bytes.NewBufferString("")),
			}}

			ch, _ := New("https://example.com", Auth(dummyAuth.Builder()))
			ch.GetLatestUser("/example-user")
			url := dummyAuth.Request.URL
			Expect(url.String()).To(ContainSubstring("https://example.com/api/v1/data"))
			Expect(url.String()).To(ContainSubstring("name=%2Fexample-user"))
			Expect(url.String()).To(ContainSubstring("current=true"))

			Expect(dummyAuth.Request.Method).To(Equal(http.MethodGet))
		})

		Context("when successful", func() {
			It("returns a user credential", func() {
				responseString := `{
				  "data": [
					{
					  "id": "some-id",
					  "name": "/example-user",
					  "type": "user",
					  "value": {
						"username": "some-username",
						"password": "some-password",
						"password_hash": "some-hash"
					  },
					  "version_created_at": "2017-01-05T01:01:01Z"
					}
				  ]
				}`
				dummyAuth := &DummyAuth{Response: &http.Response{
					StatusCode: http.StatusOK,
					Body:       ioutil.NopCloser(bytes.NewBufferString(responseString)),
				}}

				ch, _ := New("https://example.com", Auth(dummyAuth.Builder()))
				cred, err := ch.GetLatestUser("/example-user")
				Expect(err).ToNot(HaveOccurred())
				Expect(cred.Value.PasswordHash).To(Equal("some-hash"))
				username := "some-username"
				Expect(cred.Value.User).To(Equal(values.User{
					Username: username,
					Password: "some-password",
				}))
			})
		})

		Context("when response body cannot be unmarshalled", func() {
			It("returns an error", func() {
				dummyAuth := &DummyAuth{Response: &http.Response{
					Body: ioutil.NopCloser(bytes.NewBufferString("something-invalid")),
				}}
				ch, _ := New("https://example.com", Auth(dummyAuth.Builder()))
				_, err := ch.GetLatestUser("/example-cred")

				Expect(err).To(HaveOccurred())
			})
		})
	})

	Describe("GetLatestRSA()", func() {
		It("requests the credential by name", func() {
			dummyAuth := &DummyAuth{Response: &http.Response{
				Body: ioutil.NopCloser(bytes.NewBufferString("")),
			}}

			ch, _ := New("https://example.com", Auth(dummyAuth.Builder()))
			ch.GetLatestRSA("/example-rsa")
			url := dummyAuth.Request.URL
			Expect(url.String()).To(ContainSubstring("https://example.com/api/v1/data"))
			Expect(url.String()).To(ContainSubstring("name=%2Fexample-rsa"))
			Expect(url.String()).To(ContainSubstring("current=true"))

			Expect(dummyAuth.Request.Method).To(Equal(http.MethodGet))
		})

		Context("when successful", func() {
			It("returns a rsa credential", func() {
				responseString := `{
				  "data": [
					{
					  "id": "67fc3def-bbfb-4953-83f8-4ab0682ad677",
					  "name": "/example-rsa",
					  "type": "rsa",
					  "value": {
						"public_key": "public-key",
						"private_key": "private-key"
					  },
					  "version_created_at": "2017-01-01T04:07:18Z"
					}
				  ]
				}`

				dummyAuth := &DummyAuth{Response: &http.Response{
					StatusCode: http.StatusOK,
					Body:       ioutil.NopCloser(bytes.NewBufferString(responseString)),
				}}

				ch, _ := New("https://example.com", Auth(dummyAuth.Builder()))
				cred, err := ch.GetLatestRSA("/example-rsa")
				Expect(err).ToNot(HaveOccurred())
				Expect(cred.Value).To(Equal(values.RSA{
					PublicKey:  "public-key",
					PrivateKey: "private-key",
				}))
			})
		})

		Context("when response body cannot be unmarshalled", func() {
			It("returns an error", func() {
				dummyAuth := &DummyAuth{Response: &http.Response{
					Body: ioutil.NopCloser(bytes.NewBufferString("something-invalid")),
				}}
				ch, _ := New("https://example.com", Auth(dummyAuth.Builder()))
				_, err := ch.GetLatestRSA("/example-cred")

				Expect(err).To(HaveOccurred())
			})
		})
	})

	Describe("GetLatestSSH()", func() {
		It("requests the credential by name", func() {
			dummyAuth := &DummyAuth{Response: &http.Response{
				Body: ioutil.NopCloser(bytes.NewBufferString("")),
			}}

			ch, _ := New("https://example.com", Auth(dummyAuth.Builder()))
			ch.GetLatestSSH("/example-ssh")
			url := dummyAuth.Request.URL
			Expect(url.String()).To(ContainSubstring("https://example.com/api/v1/data"))
			Expect(url.String()).To(ContainSubstring("name=%2Fexample-ssh"))
			Expect(url.String()).To(ContainSubstring("current=true"))

			Expect(dummyAuth.Request.Method).To(Equal(http.MethodGet))
		})

		Context("when successful", func() {
			It("returns a ssh credential", func() {
				responseString := `{
				  "data": [
					{
					  "id": "some-id",
					  "name": "/example-ssh",
					  "type": "ssh",
					  "value": {
						"public_key": "public-key",
						"private_key": "private-key",
						"public_key_fingerprint": "public-key-fingerprint"
					  },
					  "version_created_at": "2017-01-01T04:07:18Z"
					}
				  ]
				}`

				dummyAuth := &DummyAuth{Response: &http.Response{
					StatusCode: http.StatusOK,
					Body:       ioutil.NopCloser(bytes.NewBufferString(responseString)),
				}}

				ch, _ := New("https://example.com", Auth(dummyAuth.Builder()))
				cred, err := ch.GetLatestSSH("/example-ssh")
				Expect(err).ToNot(HaveOccurred())
				Expect(cred.Value.PublicKeyFingerprint).To(Equal("public-key-fingerprint"))
				Expect(cred.Value.SSH).To(Equal(values.SSH{
					PublicKey:  "public-key",
					PrivateKey: "private-key",
				}))
			})
		})

		Context("when response body cannot be unmarshalled", func() {
			It("returns an error", func() {
				dummyAuth := &DummyAuth{Response: &http.Response{
					Body: ioutil.NopCloser(bytes.NewBufferString("something-invalid")),
				}}
				ch, _ := New("https://example.com", Auth(dummyAuth.Builder()))
				_, err := ch.GetLatestSSH("/example-cred")

				Expect(err).To(HaveOccurred())
			})
		})
	})

	Describe("GetLatestJSON()", func() {
		It("requests the credential by name", func() {
			dummyAuth := &DummyAuth{Response: &http.Response{
				Body: ioutil.NopCloser(bytes.NewBufferString("")),
			}}

			ch, _ := New("https://example.com", Auth(dummyAuth.Builder()))
			ch.GetLatestJSON("/example-json")
			url := dummyAuth.Request.URL
			Expect(url.String()).To(ContainSubstring("https://example.com/api/v1/data"))
			Expect(url.String()).To(ContainSubstring("name=%2Fexample-json"))
			Expect(url.String()).To(ContainSubstring("current=true"))

			Expect(dummyAuth.Request.Method).To(Equal(http.MethodGet))
		})

		Context("when successful", func() {
			It("returns a json credential", func() {
				responseString := `{
				  "data": [
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
					}
				  ]
				}`

				dummyAuth := &DummyAuth{Response: &http.Response{
					StatusCode: http.StatusOK,
					Body:       ioutil.NopCloser(bytes.NewBufferString(responseString)),
				}}

				ch, _ := New("https://example.com", Auth(dummyAuth.Builder()))
				cred, err := ch.GetLatestJSON("/example-json")

				JSONResult := `{
						"key": 123,
						"key_list": [
						  "val1",
						  "val2"
						],
						"is_true": true
					}`

				var unmarshalled values.JSON
				json.Unmarshal([]byte(JSONResult), &unmarshalled)

				Expect(err).ToNot(HaveOccurred())
				Expect(cred.Value).To(Equal(unmarshalled))
			})
		})

		Context("when response body cannot be unmarshalled", func() {
			It("returns an error", func() {
				dummyAuth := &DummyAuth{Response: &http.Response{
					Body: ioutil.NopCloser(bytes.NewBufferString("something-invalid")),
				}}
				ch, _ := New("https://example.com", Auth(dummyAuth.Builder()))
				_, err := ch.GetLatestJSON("/example-cred")

				Expect(err).To(HaveOccurred())
			})
		})
	})

	Describe("GetLatestValue()", func() {
		It("requests the credential by name", func() {
			dummyAuth := &DummyAuth{Response: &http.Response{
				Body: ioutil.NopCloser(bytes.NewBufferString("")),
			}}

			ch, _ := New("https://example.com", Auth(dummyAuth.Builder()))
			ch.GetLatestValue("/example-value")
			url := dummyAuth.Request.URL
			Expect(url.String()).To(ContainSubstring("https://example.com/api/v1/data"))
			Expect(url.String()).To(ContainSubstring("name=%2Fexample-value"))
			Expect(url.String()).To(ContainSubstring("current=true"))

			Expect(dummyAuth.Request.Method).To(Equal(http.MethodGet))
		})

		Context("when successful", func() {
			It("returns a value credential", func() {
				responseString := `{
				  "data": [
					{
					  "id": "some-id",
					  "name": "/example-value",
					  "type": "value",
					  "value": "some-value",
					  "version_created_at": "2017-01-05T01:01:01Z"
				}]}`

				dummyAuth := &DummyAuth{Response: &http.Response{
					StatusCode: http.StatusOK,
					Body:       ioutil.NopCloser(bytes.NewBufferString(responseString)),
				}}

				ch, _ := New("https://example.com", Auth(dummyAuth.Builder()))
				cred, err := ch.GetLatestValue("/example-value")
				Expect(err).ToNot(HaveOccurred())
				Expect(cred.Value).To(Equal(values.Value("some-value")))
			})
		})

		Context("when response body cannot be unmarshalled", func() {
			It("returns an error", func() {
				dummyAuth := &DummyAuth{Response: &http.Response{
					Body: ioutil.NopCloser(bytes.NewBufferString("something-invalid")),
				}}
				ch, _ := New("https://example.com", Auth(dummyAuth.Builder()))
				_, err := ch.GetLatestValue("/example-cred")

				Expect(err).To(HaveOccurred())
			})
		})
	})

	DescribeTable("request fails due to network error",
		func(performAction func(*CredHub) error) {
			networkError := errors.New("Network error occurred making an http request")
			dummyAuth := &DummyAuth{Error: networkError}
			ch, err := New("https://example.com", Auth(dummyAuth.Builder()))
			Expect(err).NotTo(HaveOccurred())

			err = performAction(ch)

			Expect(err).To(Equal(networkError))
		},

		Entry("GetNVersions", func(ch *CredHub) error {
			_, err := ch.GetNVersions("/example-password", 47)
			return err
		}),
		Entry("GetLatestVersion", func(ch *CredHub) error {
			_, err := ch.GetLatestVersion("/example-password")
			return err
		}),
		Entry("GetPassword", func(ch *CredHub) error {
			_, err := ch.GetLatestPassword("/example-password")
			return err
		}),
		Entry("GetCertificate", func(ch *CredHub) error {
			_, err := ch.GetLatestCertificate("/example-certificate")
			return err
		}),
		Entry("GetUser", func(ch *CredHub) error {
			_, err := ch.GetLatestUser("/example-password")
			return err
		}),
		Entry("GetRSA", func(ch *CredHub) error {
			_, err := ch.GetLatestRSA("/example-password")
			return err
		}),
		Entry("GetSSH", func(ch *CredHub) error {
			_, err := ch.GetLatestSSH("/example-password")
			return err
		}),
		Entry("GetJSON", func(ch *CredHub) error {
			_, err := ch.GetLatestJSON("/example-password")
			return err
		}),
		Entry("GetValue", func(ch *CredHub) error {
			_, err := ch.GetLatestValue("/example-password")
			return err
		}),
	)
})
