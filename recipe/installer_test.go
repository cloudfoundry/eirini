package recipe_test

import (
	"archive/zip"
	"bytes"
	"errors"
	"io/ioutil"
	"net/http"
	"path/filepath"

	"code.cloudfoundry.org/eirini/eirinifakes"
	. "code.cloudfoundry.org/eirini/recipe"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/ghttp"
)

var _ = Describe("PackageInstaller", func() {
	var (
		err         error
		downloadURL string
		targetDir   string
		installer   Installer
		server      *ghttp.Server
		extractor   *eirinifakes.FakeExtractor
		zipPath     string
	)

	BeforeEach(func() {
		zippedPackage, err := makeZippedPackage()
		Expect(err).ToNot(HaveOccurred())

		extractor = new(eirinifakes.FakeExtractor)
		installer = &PackageInstaller{Client: &http.Client{}, Extractor: extractor}

		server = ghttp.NewServer()
		server.AppendHandlers(
			ghttp.CombineHandlers(
				ghttp.VerifyRequest("GET", "/some-app-guid"),
				ghttp.RespondWith(http.StatusOK, zippedPackage),
			),
		)
		downloadURL = server.URL() + "/some-app-guid"

		targetDir, err = ioutil.TempDir("", "targetDir")
		Expect(err).ToNot(HaveOccurred())

		packageDir, err := ioutil.TempDir("", "package")
		Expect(err).ToNot(HaveOccurred())
		zipPath = filepath.Join(packageDir, "app.zip")

		Expect(err).ToNot(HaveOccurred())
	})

	JustBeforeEach(func() {
		err = installer.Install(downloadURL, zipPath, targetDir)
	})

	AfterEach(func() {
		server.Close()
	})

	assertNoInteractionsWithExtractor := func() {
		It("shoud not interact with the extractor", func() {
			Expect(extractor.Invocations()).To(BeEmpty())
		})
	}

	assertExtractorInteractions := func() {
		It("should use the extractor to extract the zip file", func() {
			_, actualTargetDir := extractor.ExtractArgsForCall(0)
			Expect(extractor.ExtractCallCount()).To(Equal(1))
			// Expect(src).To(Equal(zipFilePath))
			Expect(actualTargetDir).To(Equal(targetDir))
		})
	}

	Context("package is installed successfully", func() {
		It("succeeds", func() {
			Expect(err).ToNot(HaveOccurred())
		})

		It("writes the ZIP file to the given temp directory", func() {
			Expect(zipPath).To(BeAnExistingFile())
		})
	})

	Context("When an empty downloadURL is provided", func() {
		BeforeEach(func() {
			downloadURL = ""
		})

		It("should return an error", func() {
			Expect(err).To(HaveOccurred())
			Expect(err).To(MatchError(ContainSubstring("empty downloadURL provided")))
		})
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
		assertNoInteractionsWithExtractor()
	})

	Context("When the download fails", func() {
		Context("When the http server returns an error code", func() {
			BeforeEach(func() {
				server.Close()
				server = ghttp.NewUnstartedServer()
			})

			It("should error with an corresponding error message", func() {
				Expect(err).To(HaveOccurred())
				Expect(err).To(MatchError(ContainSubstring("failed to perform request")))
			})

			assertNoInteractionsWithExtractor()
		})

		Context("When the server does not return OK HTTP status", func() {
			BeforeEach(func() {
				server.RouteToHandler("GET", "/some-app-guid",
					ghttp.RespondWith(http.StatusTeapot, nil),
				)
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
				extractor.ExtractReturns(errors.New(expectedErrorMessage))
			})

			assertExtractorInteractions()

			It("should return an error", func() {
				Expect(err).To(MatchError(ContainSubstring(expectedErrorMessage)))
			})
		})

		Context("When the app id creates an invalid URL", func() {
			BeforeEach(func() {
				downloadURL = "%&"
			})

			It("should return an error", func() {
				Expect(err).To(HaveOccurred())
			})

			It("should return the right error message", func() {
				Expect(err).To(MatchError(ContainSubstring("failed to perform request")))
				Expect(err).To(MatchError(ContainSubstring(downloadURL)))
			})
		})
	})
})

// straight from https://golang.org/pkg/archive/zip/#example_Writer
func makeZippedPackage() ([]byte, error) {
	buf := bytes.Buffer{}
	w := zip.NewWriter(&buf)

	// the ZIP file is intentionally left empty

	err := w.Close()
	if err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}
