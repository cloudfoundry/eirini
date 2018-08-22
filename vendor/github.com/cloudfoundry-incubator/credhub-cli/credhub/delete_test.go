package credhub_test

import (
	. "code.cloudfoundry.org/credhub-cli/credhub"

	"bytes"
	"io/ioutil"
	"net/http"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Delete", func() {

	It("requests a delete by name", func() {
		dummy := &DummyAuth{Response: &http.Response{
			StatusCode: http.StatusOK,
			Body:       ioutil.NopCloser(bytes.NewBufferString("")),
		}}

		ch, _ := New("https://example.com", Auth(dummy.Builder()))
		ch.Delete("/example-password")

		url := dummy.Request.URL.String()
		Expect(url).To(Equal("https://example.com/api/v1/data?name=%2Fexample-password"))
		Expect(dummy.Request.Method).To(Equal(http.MethodDelete))

	})

	Context("when the credential exists", func() {
		It("deletes the credential", func() {
			dummy := &DummyAuth{Response: &http.Response{
				StatusCode: http.StatusNoContent,
				Body:       ioutil.NopCloser(bytes.NewBufferString("")),
			}}

			ch, _ := New("https://example.com", Auth(dummy.Builder()))
			err := ch.Delete("/example-password")
			Expect(err).ToNot(HaveOccurred())
		})
	})

	Context("when the credential does not exist", func() {
		It("returns an error", func() {
			dummy := &DummyAuth{Response: &http.Response{
				StatusCode: http.StatusNotFound,
				Body:       ioutil.NopCloser(bytes.NewBufferString(`{"error":"The request could not be completed because the credential does not exist or you do not have sufficient authorization."}`)),
			}}

			ch, _ := New("https://example.com", Auth(dummy.Builder()))
			err := ch.Delete("/example-password")
			Expect(err).To(MatchError("The request could not be completed because the credential does not exist or you do not have sufficient authorization."))
		})
	})
})
