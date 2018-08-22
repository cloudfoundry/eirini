package credhub_test

import (
	. "code.cloudfoundry.org/credhub-cli/credhub"

	"bytes"
	"io/ioutil"
	"net/http"

	"code.cloudfoundry.org/credhub-cli/credhub/permissions"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Permissions", func() {
	Context("GetPermissions", func() {
		It("returns the permissions", func() {
			responseString := `{
	"credential_name":"/test-password",
	"permissions":[{
			"actor":"uaa-user:e3366b5c-1c5a-4df8-8b0f-9001ee5a0cf0",
			"operations":["read"]
			}]
	}`

			dummyAuth := &DummyAuth{Response: &http.Response{
				StatusCode: http.StatusOK,
				Body:       ioutil.NopCloser(bytes.NewBufferString(responseString)),
			}}

			ch, _ := New("https://example.com", Auth(dummyAuth.Builder()))
			actualPermissions, err := ch.GetPermissions("/example-password")
			Expect(err).NotTo(HaveOccurred())

			expectedPermissions := []permissions.Permission{
				{
					Actor:      "uaa-user:e3366b5c-1c5a-4df8-8b0f-9001ee5a0cf0",
					Operations: []string{"read"},
				},
			}
			Expect(actualPermissions).To(Equal(expectedPermissions))
		})

		It("returns an empty set of permissions", func() {
			responseString := `{
	"credential_name":"/test-password",
	"permissions":[]
	}`

			dummyAuth := &DummyAuth{Response: &http.Response{
				StatusCode: http.StatusOK,
				Body:       ioutil.NopCloser(bytes.NewBufferString(responseString)),
			}}

			ch, _ := New("https://example.com", Auth(dummyAuth.Builder()))
			actualPermissions, err := ch.GetPermissions("/example-password")
			Expect(err).NotTo(HaveOccurred())

			Expect(actualPermissions).To(Equal([]permissions.Permission{}))
		})
	})

	Context("AddPermissions", func() {
		Context("when a credential exists", func() {
			It("can add permissions to it", func() {
				dummy := &DummyAuth{Response: &http.Response{
					StatusCode: http.StatusCreated,
					Body:       ioutil.NopCloser(bytes.NewBufferString("")),
				}}
				ch, _ := New("https://example.com", Auth(dummy.Builder()))

				_, err := ch.AddPermissions("/example-password", []permissions.Permission{
					{
						Actor:      "some-actor",
						Operations: []string{"operation-1", "operation-2"},
						Path: "/example-password",
					},
				})

				Expect(err).NotTo(HaveOccurred())

				By("calling the right endpoints")
				url := dummy.Request.URL.String()
				Expect(url).To(Equal("https://example.com/api/v1/permissions"))
				Expect(dummy.Request.Method).To(Equal(http.MethodPost))
				params, err := ioutil.ReadAll(dummy.Request.Body)
				Expect(err).NotTo(HaveOccurred())

				expectedParams := `{
			  "credential_name": "/example-password",
			  "permissions": [
			  {
				"actor": "some-actor",
				"operations": ["operation-1", "operation-2"],
				"path": "/example-password"
			  }]
			}`
				Expect(params).To(MatchJSON(expectedParams))
			})
		})

		Context("when a credential doesn't exist", func() {
			It("cannot add permissions to it", func() {
				dummy := &DummyAuth{Response: &http.Response{
					StatusCode: http.StatusNotFound,
					Body:       ioutil.NopCloser(bytes.NewBufferString(`{"error":"The request could not be completed because the credential does not exist or you do not have sufficient authorization."}`)),
				}}
				ch, _ := New("https://example.com", Auth(dummy.Builder()))

				_, err := ch.AddPermissions("/example-password", []permissions.Permission{
					{
						Actor:      "some-actor",
						Operations: []string{"operation-1", "operation-2"},
						Path: "/example-password",
					},
				})

				Expect(err).To(MatchError(ContainSubstring("The request could not be completed because the credential does not exist or you do not have sufficient authorization.")))
			})
		})
	})
})
