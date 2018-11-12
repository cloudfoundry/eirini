package credhub_test

import (
	"bytes"
	. "code.cloudfoundry.org/credhub-cli/credhub"
	"encoding/json"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"io/ioutil"
	"net/http"
)

var _ = Describe("Bulk Regenerate", func() {
	It("regenerates all certificates signed by the given ca ", func() {
		dummyAuth := &DummyAuth{Response: &http.Response{
			Body: ioutil.NopCloser(bytes.NewBufferString("")),
		}}

		ch, _ := New("https://example.com", Auth(dummyAuth.Builder()))

		ch.BulkRegenerate("/example-ca")
		url := dummyAuth.Request.URL.String()
		Expect(url).To(Equal("https://example.com/api/v1/bulk-regenerate"))
		Expect(dummyAuth.Request.Method).To(Equal(http.MethodPost))

		var requestBody map[string]interface{}
		body, _ := ioutil.ReadAll(dummyAuth.Request.Body)
		json.Unmarshal(body, &requestBody)

		Expect(requestBody["signed_by"]).To(Equal("/example-ca"))
	})

	Context("when successful", func() {
		It("returns the certificates signed by the ca", func() {
			responseString := `{
			"regenerated_credentials": [
			    "/example-cert1",
				"/example-cert2",
				"/example-cert3"
			]
		}`

			dummyAuth := &DummyAuth{Response: &http.Response{
				StatusCode: http.StatusOK,
				Body:       ioutil.NopCloser(bytes.NewBufferString(responseString)),
			}}

			ch, _ := New("https://example.com", Auth(dummyAuth.Builder()))

			creds, err := ch.BulkRegenerate("/example-ca")
			Expect(err).To(BeNil())
			Expect(creds.Certificates[0]).To(Equal("/example-cert1"))
			Expect(creds.Certificates[1]).To(Equal("/example-cert2"))
			Expect(creds.Certificates[2]).To(Equal("/example-cert3"))
		})
	})

	Context("when response body cannot be unmarshalled", func() {
		It("returns an error", func() {
			dummyAuth := &DummyAuth{Response: &http.Response{
				Body: ioutil.NopCloser(bytes.NewBufferString("something-invalid")),
			}}

			ch, _ := New("https://example.com", Auth(dummyAuth.Builder()))
			_, err := ch.BulkRegenerate("/example-ca")

			Expect(err).To(HaveOccurred())
		})
	})
})
