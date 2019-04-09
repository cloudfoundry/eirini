package recipe_test

import (
	"archive/zip"
	"bytes"
	"io/ioutil"
	"net/http"

	"code.cloudfoundry.org/eirini/recipe"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/ghttp"
)

var _ = Describe("PackageInstaller", func() {
	var (
		err           error
		downloadURL   string
		downloadDir   string
		installer     recipe.Installer
		server        *ghttp.Server
		zippedPackage []byte
	)

	BeforeEach(func() {
		zippedPackage, err = makeZippedPackage()
		Expect(err).ToNot(HaveOccurred())

		server = ghttp.NewServer()
		server.AppendHandlers(
			ghttp.CombineHandlers(
				ghttp.VerifyRequest("GET", "/some-app-guid"),
				ghttp.RespondWith(http.StatusOK, zippedPackage),
			),
		)
		downloadURL = server.URL() + "/some-app-guid"

		downloadDir, err = ioutil.TempDir("", "downloadDir")
		Expect(err).ToNot(HaveOccurred())
	})

	JustBeforeEach(func() {
		installer = recipe.NewPackageManager(&http.Client{}, downloadURL, downloadDir)
		err = installer.Install()
	})

	AfterEach(func() {
		server.Close()
	})

	Context("package is installed successfully", func() {
		It("succeeds", func() {
			Expect(err).ToNot(HaveOccurred())
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
	})

	Context("When an empty downloadDir is provided", func() {
		BeforeEach(func() {
			downloadDir = ""
		})

		It("should return an error", func() {
			Expect(err).To(HaveOccurred())
			Expect(err).To(MatchError(ContainSubstring("empty downloadDir provided")))
		})
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
