package recipe_test

import (
	"fmt"
	"net/http"

	"code.cloudfoundry.org/eirini/recipe"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/ghttp"
)

var _ = Describe("Uploader", func() {

	var (
		server       *ghttp.Server
		uploader     recipe.Uploader
		testFilePath string
		url          string
		err          error
	)

	BeforeEach(func() {
		server = ghttp.NewServer()
		url = fmt.Sprintf("%s/dog/pictures/upload", server.URL())

		server.RouteToHandler("POST", "/dog/pictures/upload",
			ghttp.CombineHandlers(
				ghttp.VerifyContentType("application/octet-stream"),
				ghttp.VerifyHeaderKV("Content-Length", "30"),
				ghttp.VerifyBody([]byte("This is definitely not a zip.\n")),
			),
		)
	})

	JustBeforeEach(func() {
		uploader = &recipe.DropletUploader{
			Client: &http.Client{},
		}

		err = uploader.Upload(
			url,
			testFilePath,
		)
	})

	Context("Upload a file", func() {

		BeforeEach(func() {
			testFilePath = "testdata/file.notzip"
		})

		It("should not return an error", func() {
			Expect(err).ToNot(HaveOccurred())
		})

		It("should post the file contents", func() {
			Expect(server.ReceivedRequests()).To(HaveLen(1))
		})

		Context("When the file is missing", func() {
			BeforeEach(func() {
				testFilePath = "wat"
			})

			It("should return an error", func() {
				Expect(err).To(HaveOccurred())
			})

			It("should not post anything", func() {
				Expect(server.ReceivedRequests()).To(HaveLen(0))
			})

		})

		Context("When the url is invalid", func() {
			BeforeEach(func() {
				url = "very.invalid/url%&"
			})

			It("should return an error", func() {
				Expect(err).To(HaveOccurred())
			})

			It("should not post anything", func() {
				Expect(server.ReceivedRequests()).To(HaveLen(0))
			})
		})

		Context("When the response is 400", func() {

			BeforeEach(func() {
				server.RouteToHandler("POST", "/dog/pictures/upload",
					ghttp.RespondWith(http.StatusUnavailableForLegalReasons, nil),
				)
			})

			It("should return an error", func() {
				Expect(err).To(HaveOccurred())
			})

			It("should post the request body", func() {
				Expect(server.ReceivedRequests()).To(HaveLen(1))
			})

		})

	})
})
