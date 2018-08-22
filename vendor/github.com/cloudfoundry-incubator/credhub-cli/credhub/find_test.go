package credhub_test

import (
	"bytes"
	"errors"
	"io/ioutil"
	"net/http"

	. "code.cloudfoundry.org/credhub-cli/credhub"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Find", func() {

	Describe("FindByPath()", func() {
		It("requests credentials for a specified path", func() {
			dummy := &DummyAuth{Response: &http.Response{
				Body: ioutil.NopCloser(bytes.NewBufferString("")),
			}}

			ch, _ := New("https://example.com", Auth(dummy.Builder()))

			ch.FindByPath("/some/example/path")
			url := dummy.Request.URL
			Expect(url.String()).To(Equal("https://example.com/api/v1/data?path=%2Fsome%2Fexample%2Fpath"))
			Expect(dummy.Request.Method).To(Equal(http.MethodGet))
		})

		Context("when successful", func() {
			It("returns a list of stored credential names which are within the specified path", func() {
				expectedResponse := `{
  "credentials": [
    {
      "version_created_at": "2017-05-09T21:09:26Z",
      "name": "/some/example/path/example-cred-0"
    },
    {
      "version_created_at": "2017-05-09T21:09:07Z",
      "name": "/some/example/path/example-cred-1"
    }
  ]
}`
				dummy := &DummyAuth{Response: &http.Response{
					StatusCode: http.StatusOK,
					Body:       ioutil.NopCloser(bytes.NewBufferString(expectedResponse)),
				}}

				ch, _ := New("https://example.com", Auth(dummy.Builder()))

				creds, err := ch.FindByPath("/some/example/path")

				Expect(err).ToNot(HaveOccurred())
				Expect(creds.Credentials[0].Name).To(Equal("/some/example/path/example-cred-0"))
				Expect(creds.Credentials[0].VersionCreatedAt).To(Equal("2017-05-09T21:09:26Z"))
				Expect(creds.Credentials[1].Name).To(Equal("/some/example/path/example-cred-1"))
				Expect(creds.Credentials[1].VersionCreatedAt).To(Equal("2017-05-09T21:09:07Z"))

			})
		})

		Context("when request fails", func() {
			It("returns an error", func() {
				dummy := &DummyAuth{Error: errors.New("Network error occurred")}

				ch, _ := New("https://example.com", Auth(dummy.Builder()))

				_, err := ch.FindByPath("/some/example/path")

				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("Network error occurred"))
			})
		})

		Context("when response body cannot be unmarshalled", func() {
			It("returns an error", func() {
				dummy := &DummyAuth{Response: &http.Response{
					StatusCode: http.StatusOK,
					Body:       ioutil.NopCloser(bytes.NewBufferString("something-invalid")),
				}}

				ch, _ := New("https://example.com", Auth(dummy.Builder()))

				_, err := ch.FindByPath("/some/example/path")
				Expect(err).To(HaveOccurred())
			})
		})
	})

	Describe("FindByPartialName()", func() {
		It("requests credentials for a specified partial name", func() {
			dummy := &DummyAuth{Response: &http.Response{
				Body: ioutil.NopCloser(bytes.NewBufferString("")),
			}}

			ch, _ := New("https://example.com", Auth(dummy.Builder()))

			ch.FindByPartialName("/some/example/name")
			url := dummy.Request.URL
			Expect(url.String()).To(Equal("https://example.com/api/v1/data?name-like=%2Fsome%2Fexample%2Fname"))
			Expect(dummy.Request.Method).To(Equal(http.MethodGet))
		})

		Context("when successful", func() {
			It("returns a list of stored credential names which match specified name-like string", func() {
				expectedResponse := `{
  "credentials": [
    {
      "version_created_at": "2017-05-09T21:09:26Z",
      "name": "/some/example/path/example-cred-0"
    },
    {
      "version_created_at": "2017-05-09T21:09:07Z",
      "name": "/some/example/path/example-cred-1"
    }
  ]
}`
				dummy := &DummyAuth{Response: &http.Response{
					StatusCode: http.StatusOK,
					Body:       ioutil.NopCloser(bytes.NewBufferString(expectedResponse)),
				}}

				ch, _ := New("https://example.com", Auth(dummy.Builder()))

				creds, err := ch.FindByPartialName("example-cred")

				Expect(err).ToNot(HaveOccurred())
				Expect(creds.Credentials[0].Name).To(Equal("/some/example/path/example-cred-0"))
				Expect(creds.Credentials[0].VersionCreatedAt).To(Equal("2017-05-09T21:09:26Z"))
				Expect(creds.Credentials[1].Name).To(Equal("/some/example/path/example-cred-1"))
				Expect(creds.Credentials[1].VersionCreatedAt).To(Equal("2017-05-09T21:09:07Z"))

			})
		})

		Context("when request fails", func() {
			It("returns an error", func() {
				dummy := &DummyAuth{Error: errors.New("Network error occurred")}

				ch, _ := New("https://example.com", Auth(dummy.Builder()))

				_, err := ch.FindByPartialName("/some/example/path")

				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("Network error occurred"))
			})
		})

		Context("when response body cannot be unmarshalled", func() {
			It("returns an error", func() {
				dummy := &DummyAuth{Response: &http.Response{
					StatusCode: http.StatusOK,
					Body:       ioutil.NopCloser(bytes.NewBufferString("something-invalid")),
				}}

				ch, _ := New("https://example.com", Auth(dummy.Builder()))

				_, err := ch.FindByPartialName("/some/example/path")
				Expect(err).To(HaveOccurred())
			})
		})
	})

})
