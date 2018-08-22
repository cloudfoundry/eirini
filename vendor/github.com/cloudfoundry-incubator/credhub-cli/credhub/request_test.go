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

var _ = Describe("Request()", func() {

	var (
		mockAuth *DummyAuth
		ch       *CredHub
	)

	BeforeEach(func() {
		mockAuth = &DummyAuth{}
		ch, _ = New("http://example.com/", Auth(mockAuth.Builder()))
	})

	It("should send the requested using the provided auth to the ApiURL", func() {
		payload := map[string]interface{}{
			"some-field":  1,
			"other-field": "blah",
		}

		mockAuth.Response = &http.Response{}
		mockAuth.Error = errors.New("Some error")

		response, err := ch.Request("PATCH", "/api/v1/some-endpoint", nil, payload, true)

		Expect(response).To(Equal(mockAuth.Response))
		Expect(err).To(Equal(mockAuth.Error))

		Expect(mockAuth.Request.Method).To(Equal("PATCH"))
		Expect(mockAuth.Request.URL.String()).To(Equal("http://example.com/api/v1/some-endpoint"))

		body, err := ioutil.ReadAll(mockAuth.Request.Body)

		Expect(err).To(BeNil())
		Expect(body).To(MatchJSON(`{"some-field": 1, "other-field": "blah"}`))
	})

	It("fails to send the request when the body cannot be marshalled to JSON", func() {
		_, err := ch.Request("PATCH", "/api/v1/some-endpoint", nil, &NotMarshallable{}, true)
		Expect(err).To(HaveOccurred())
	})

	It("fails to send when the request method is invalid", func() {
		_, err := ch.Request(" ", "/api/v1/some-endpoint", nil, nil, true)
		Expect(err).To(HaveOccurred())
	})

	Context("when response body is an error ", func() {
		Context("when checkServerError is true", func() {
			var err error
			It("returns an error", func() {
				dummy := &DummyAuth{Response: &http.Response{
					StatusCode: 400,
					Body:       ioutil.NopCloser(bytes.NewBufferString(`{"error" : "error occurred" }`)),
				}}

				ch, _ := New("https://example.com", Auth(dummy.Builder()))

				_, err = ch.Request("GET", "/example-password", nil, nil, true)

				Expect(err).To(MatchError("error occurred"))
			})
		})

		Context("when checkServerError is false", func() {
			It("returns the raw response json", func() {
				dummy := &DummyAuth{Response: &http.Response{
					StatusCode: 400,
					Body:       ioutil.NopCloser(bytes.NewBufferString(`{"error" : "error occurred" }`)),
				}}

				ch, _ := New("https://example.com", Auth(dummy.Builder()))

				r, err := ch.Request("GET", "/example-password", nil, nil, false)
				resp, err := ioutil.ReadAll(r.Body)
				Expect(err).ToNot(HaveOccurred())

				Expect(string(resp)).To(Equal(`{"error" : "error occurred" }`))
			})
		})
	})
})
