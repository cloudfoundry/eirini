package main_test

import (
	"bytes"
	"errors"
	"io/ioutil"
	"net/http"
	"os"

	"github.com/julz/cube/cubefakes"
	. "github.com/julz/cube/recipe"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Downloader", func() {
	var (
		downloader *Downloader
		//fakeServer *ghttp.Server
		cfclient *cubefakes.FakeCfClient
	)

	BeforeEach(func() {
		cfclient = new(cubefakes.FakeCfClient)
		downloader = &Downloader{cfclient}
		//fakeServer = ghttp.NewServer()
	})

	Context("Download", func() {

		It("should return an error if an empty appId is provided", func() {
			err := downloader.Download("", "")
			Expect(err).To(HaveOccurred())

			Expect(err).To(MatchError(ContainSubstring("empty appId provided")))
		})

		It("should return an error if an empty path name is provided", func() {
			err := downloader.Download("guid", "")
			Expect(err).To(HaveOccurred())

			Expect(err).To(MatchError(ContainSubstring("empty filepath provided")))
		})

		Context("When the downlad request is successful", func() {

			BeforeEach(func() {
				cfclient.GetAppBitsByAppGuidReturns(&http.Response{
					Body:       ioutil.NopCloser(bytes.NewBufferString("appbits")),
					StatusCode: http.StatusOK,
				}, nil)
			})

			AfterEach(func() {
				err := os.Remove("test/file")
				Expect(err).ToNot(HaveOccurred())
			})

			It("writes the downloaded content to the given file", func() {
				err := downloader.Download("guid", "test/file")
				Expect(err).ToNot(HaveOccurred())
				Expect("test/file").Should(BeAnExistingFile())

				file, err := ioutil.ReadFile("test/file")
				Expect(err).ToNot(HaveOccurred())
				Expect(string(file)).To(Equal("appbits"))
			})
		})

		Context("When the download fails", func() {
			Context("because of the cfclient", func() {
				BeforeEach(func() {
					cfclient.GetAppBitsByAppGuidReturns(&http.Response{
						StatusCode: http.StatusInternalServerError,
					}, errors.New("aargh"))
				})

				It("should error with an corresponding error message", func() {
					err := downloader.Download("guid", "test/file")
					Expect(err).To(HaveOccurred())
					Expect(err).To(MatchError(ContainSubstring("failed to perform request")))
				})
			})

			Context("but cfclient does not return an error", func() {
				BeforeEach(func() {
					cfclient.GetAppBitsByAppGuidReturns(&http.Response{
						StatusCode: http.StatusInternalServerError,
					}, nil)
				})

				It("should return an meaningful err message", func() {
					err := downloader.Download("guid", "test/file")
					Expect(err).To(HaveOccurred())
					Expect(err).To(MatchError(ContainSubstring("Download failed. Status Code")))
				})
			})

		})
	})
})
