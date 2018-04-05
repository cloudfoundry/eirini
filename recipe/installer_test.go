package main_test

import (
	"bytes"
	"errors"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"

	"github.com/julz/cube/cubefakes"
	. "github.com/julz/cube/recipe"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("PackageInstaller", func() {
	var (
		err         error
		appId       string
		targetDir   string
		zipFilePath string
		installer   *PackageInstaller
		cfclient    *cubefakes.FakeCfClient
		extractor   *cubefakes.FakeExtractor
	)

	BeforeEach(func() {
		appId = "guid"
		targetDir = "testdata"
		zipFilePath = filepath.Join(targetDir, appId) + ".zip"
		cfclient = new(cubefakes.FakeCfClient)
		extractor = new(cubefakes.FakeExtractor)
		installer = &PackageInstaller{cfclient, extractor}
	})

	JustBeforeEach(func() {
		err = installer.Install(appId, targetDir)
	})

	Context("Install", func() {

		assertNoInteractionsWithCfclient := func() {
			It("should not interact with the cfclient", func() {
				Expect(cfclient.Invocations()).To(BeEmpty())
			})
		}

		assertNoInteractionsWithExtractor := func() {
			It("shoud not interact with the extractor", func() {
				Expect(extractor.Invocations()).To(BeEmpty())
			})
		}

		assertCfclientInteractions := func() {
			It("should use the cfclient to download the file", func() {
				actualAppId := cfclient.GetAppBitsByAppGuidArgsForCall(0)
				Expect(cfclient.GetAppBitsByAppGuidCallCount()).To(Equal(1))
				Expect(actualAppId).To(Equal(appId))
			})
		}

		assertExtractorInteractions := func() {
			It("should use the extractor to extract the zip file", func() {
				src, actualTargetDir := extractor.ExtractArgsForCall(0)
				Expect(extractor.ExtractCallCount()).To(Equal(1))
				Expect(src).To(Equal(zipFilePath))
				Expect(actualTargetDir).To(Equal(targetDir))
			})
		}

		mockCfclient := func(httpStatus int, err error) {
			cfclient.GetAppBitsByAppGuidReturns(&http.Response{
				Body:       ioutil.NopCloser(bytes.NewBufferString("appbits")),
				StatusCode: httpStatus,
			}, err)
		}

		Context("When an empty appId is provided", func() {
			BeforeEach(func() {
				appId = ""
			})

			It("should return an error", func() {
				Expect(err).To(HaveOccurred())
				Expect(err).To(MatchError(ContainSubstring("empty appId provided")))
			})
			assertNoInteractionsWithCfclient()
			assertNoInteractionsWithExtractor()
		})

		Context("When an empty targetDir is provided", func() {
			BeforeEach(func() {
				targetDir = ""
			})

			It("should return an error", func() {
				Expect(err).To(HaveOccurred())
				Expect(err).To(MatchError(ContainSubstring("empty targetDir provided")))
			})
			assertNoInteractionsWithCfclient()
			assertNoInteractionsWithExtractor()
		})

		Context("When package is installed successfully", func() {
			var expectedZipContents string

			BeforeEach(func() {
				expectedZipContents = "appbits"
				mockCfclient(http.StatusOK, nil)
			})

			AfterEach(func() {
				osError := os.Remove(zipFilePath)
				Expect(osError).ToNot(HaveOccurred())
			})

			It("writes the downloaded content to the given file", func() {
				Expect(err).ToNot(HaveOccurred())
				Expect(zipFilePath).Should(BeAnExistingFile())

				file, err := ioutil.ReadFile(zipFilePath)
				Expect(err).ToNot(HaveOccurred())
				Expect(string(file)).To(Equal(expectedZipContents))
			})
			assertCfclientInteractions()
			assertExtractorInteractions()

		})

		Context("When the download fails", func() {
			Context("When the cfclient returns an error", func() {
				BeforeEach(func() {
					mockCfclient(http.StatusOK, errors.New("failed to download appbits"))
				})

				It("should error with an corresponding error message", func() {
					Expect(err).To(HaveOccurred())
					Expect(err).To(MatchError(ContainSubstring("failed to perform request")))
				})

				assertCfclientInteractions()
				assertNoInteractionsWithExtractor()
			})

			Context("When the cfclient does not return OK HTTP status", func() {
				BeforeEach(func() {
					mockCfclient(http.StatusInternalServerError, nil)
				})

				It("should return an meaningful err message", func() {
					Expect(err).To(HaveOccurred())
					Expect(err).To(MatchError(ContainSubstring("Download failed. Status Code")))
				})
			})

			Context("When the extractor returns an error", func() {
				var expectedErrorMessage string

				BeforeEach(func() {
					expectedErrorMessage = "failed to extract zip"
					mockCfclient(http.StatusOK, nil)
					extractor.ExtractReturns(errors.New(expectedErrorMessage))
				})

				AfterEach(func() {
					osError := os.Remove(zipFilePath)
					Expect(osError).ToNot(HaveOccurred())
				})

				assertExtractorInteractions()

				It("should return an error", func() {
					Expect(err).To(HaveOccurred())
					Expect(err).To(MatchError(ContainSubstring(expectedErrorMessage)))
				})
			})

		})
	})
})
