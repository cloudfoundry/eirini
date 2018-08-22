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

var _ = Describe("Interpolate", func() {
	Describe("(ch *CredHub) InterpolateString()", func() {
		Context("when VCAP_SERVICES does not contain credhub refs", func() {
			It("does not make any interpolation requests and returns the original VCAP_SERVICES body", func() {
				dummy := &DummyAuth{Response: &http.Response{
					StatusCode: http.StatusOK,
					Body:       ioutil.NopCloser(bytes.NewBufferString("{}")),
				}}
				ch, _ := New("https://example.com", Auth(dummy.Builder()))

				payloadWithoutRefs := `{"my-server":[{"credentials":{"dont-need-no-credhub-refs-here":""}}]}`
				returnedValue, err := ch.InterpolateString(payloadWithoutRefs)

				Expect(err).ToNot(HaveOccurred())
				Expect(dummy.Request).To(BeNil())
				Expect(returnedValue).To(Equal(payloadWithoutRefs))
			})
		})

		Context("when VCAP_SERVICES contains credhub refs", func() {
			validPayload := `{"my-server":[{"credentials":{"credhub-ref":"(//my-server/creds)"}}]}`
			It("requests to interpolate the VCAP_SERVICES object", func() {
				dummy := &DummyAuth{Response: &http.Response{
					Body: ioutil.NopCloser(bytes.NewBufferString("")),
				}}

				ch, _ := New("https://example.com", Auth(dummy.Builder()))

				ch.InterpolateString(validPayload)

				urlPath := dummy.Request.URL.Path
				Expect(urlPath).To(Equal("/api/v1/interpolate"))
				Expect(dummy.Request.Method).To(Equal(http.MethodPost))
				Expect(ioutil.ReadAll(dummy.Request.Body)).To(MatchJSON(validPayload))
			})

			Context("when successful", func() {
				It("returns the interpolated credential", func() {
					interpolatedResponse := `{ "Your VCAP_SERVICES": "totally-interpolated" }`
					dummy := &DummyAuth{Response: &http.Response{
						StatusCode: http.StatusOK,
						Body:       ioutil.NopCloser(bytes.NewBufferString(interpolatedResponse)),
					}}

					ch, _ := New("https://example.com", Auth(dummy.Builder()))

					interpolatedServices, err := ch.InterpolateString(validPayload)
					Expect(err).ToNot(HaveOccurred())

					Expect(interpolatedServices).To(MatchJSON(interpolatedResponse))
				})
			})

			Context("when request fails", func() {
				It("returns an error", func() {
					networkError := errors.New("Network error occurred")
					dummy := &DummyAuth{Error: networkError}
					ch, _ := New("https://example.com", Auth(dummy.Builder()))

					_, err := ch.InterpolateString(validPayload)

					Expect(err).To(HaveOccurred())
				})
			})

			Context("when vcapServicesBody is invalid", func() {
				It("returns an error", func() {
					dummy := &DummyAuth{Response: &http.Response{
						Body: ioutil.NopCloser(bytes.NewBufferString("")),
					}}

					ch, _ := New("https://example.com", Auth(dummy.Builder()))

					_, err := ch.InterpolateString(`{ "my-server": [{"credentials":{"credhub-ref":"oh no invalid json with"text outside quotes}}]`)
					Expect(err).To(HaveOccurred())
				})
			})
		})
	})
})
