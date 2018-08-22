package credhub_test

import (
	"net/http"

	. "code.cloudfoundry.org/credhub-cli/credhub"
	version "github.com/hashicorp/go-version"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/ghttp"
)

var _ = Describe("ServerVersion()", func() {
	var server *ghttp.Server

	BeforeEach(func() {
		server = ghttp.NewServer()
	})

	Context("when the /info endpoint reports server version", func() {
		BeforeEach(func() {
			server.AppendHandlers(
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("GET", "/info"),
					ghttp.RespondWith(http.StatusOK, `{
							"auth-server": {
								"url": "https://uaa.example.com:8443"
							},
							"app": {
								"name": "CredHub",
								"version": "1.2.3"
							}
						}`)),
			)
		})

		It("should obtain the server version from the /info endpoint", func() {
			expectedVersion, err := version.NewVersion("1.2.3")
			Expect(err).To(BeNil())

			ch, err := New(server.URL())
			Expect(err).To(BeNil())

			serverVersion, err := ch.ServerVersion()
			Expect(err).To(BeNil())
			Expect(serverVersion).To(Equal(expectedVersion))
		})
	})

	Context("when the /info endpoint does not report server version", func() {
		BeforeEach(func() {
			server.AppendHandlers(
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("GET", "/info"),
					ghttp.RespondWith(http.StatusOK, `{
							"auth-server": {
								"url": "https://uaa.example.com:8443"
							},
							"app": {
								"name": "CredHub"
							}
						}`)),
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("GET", "/version"),
					ghttp.RespondWith(http.StatusOK, `{
							"version": "1.2.3"
						}`)),
			)
		})

		It("should obtain the server version from the /version endpoint", func() {
			expectedVersion, err := version.NewVersion("1.2.3")
			Expect(err).To(BeNil())

			ch, err := New(server.URL())
			Expect(err).To(BeNil())

			serverVersion, err := ch.ServerVersion()
			Expect(err).To(BeNil())
			Expect(serverVersion).To(Equal(expectedVersion))
		})
	})

	AfterEach(func() {
		server.Close()
	})
})
