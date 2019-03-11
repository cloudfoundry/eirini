package recipe_test

import (
	"net/http"

	. "code.cloudfoundry.org/eirini/recipe"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/ghttp"
)

var _ = Describe("BuildpackManager", func() {

	var (
		buildpack       Buildpack
		server          *ghttp.Server
		responseContent string
		downloadURL     string
		actualBytes     []byte
		expectedBytes   []byte
		err             error
		client          *http.Client
	)

	Context("when a buildpack URL is given", func() {
		BeforeEach(func() {
			client = http.DefaultClient
			server = ghttp.NewServer()
		})

		JustBeforeEach(func() {
			expectedBytes = []byte(responseContent)
			actualBytes, err = OpenBuildpackURL(&buildpack, client)
		})

		Context("and it is a valid URL", func() {
			BeforeEach(func() {
				responseContent = "the content"

				server.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("GET", "/buildpack"),
						ghttp.RespondWith(http.StatusOK, responseContent),
					),
				)
				downloadURL = server.URL() + "/buildpack"

				buildpack = Buildpack{
					Name: "custom",
					Key:  "some_key",
					URL:  downloadURL,
				}
			})

			It("should not fail", func() {
				Expect(err).ToNot(HaveOccurred())
			})

			It("it downloads the buildpack contents", func() {
				Expect(actualBytes).To(Equal(expectedBytes))
			})
		})

		Context("and it is NOT a valid url", func() {
			BeforeEach(func() {
				buildpack = Buildpack{
					Name: "custom",
					Key:  "some_key",
					URL:  "___terrible::::__url",
				}
			})

			It("should fail", func() {
				Expect(err).To(HaveOccurred())
			})

			It("should return a meaningful error message", func() {
				Expect(err).To(MatchError(ContainSubstring("failed to request buildpack")))
			})
		})

		Context("and it is an unresponsive url", func() {
			BeforeEach(func() {
				server.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("GET", "/buildpack"),
						ghttp.RespondWith(http.StatusNotFound, responseContent),
					),
				)

				buildpack = Buildpack{
					Name: "custom",
					Key:  "some_key",
					URL:  server.URL() + "/buildpack",
				}
			})

			It("should fail", func() {
				Expect(err).To(HaveOccurred())
			})

			It("should return a meaningful error message", func() {
				Expect(err).To(MatchError(ContainSubstring("downloading buildpack failed with status code")))
			})
		})
	})
})
