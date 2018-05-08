package util_test

import (
	"os"

	"github.com/cloudfoundry-incubator/credhub-cli/util"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"runtime"

	credhub_errors "github.com/cloudfoundry-incubator/credhub-cli/errors"
	"github.com/cloudfoundry-incubator/credhub-cli/test"
)

var _ = Describe("Util", func() {
	Describe("#ReadFileOrStringFromField", func() {
		Context("when the file does not exist", func() {
			It("returns the original value", func() {
				readContents, err := util.ReadFileOrStringFromField("Foo")
				Expect(readContents).To(Equal("Foo"))
				Expect(err).To(BeNil())
			})

			It("handles newlines in the value", func() {
				readContents, err := util.ReadFileOrStringFromField(`foo\nbar`)
				Expect(readContents).To(Equal("foo\nbar"))
				Expect(err).To(BeNil())
			})
		})

		Context("when the file is readable", func() {
			It("reads a file into memory", func() {
				tempDir := test.CreateTempDir("filesForTesting")
				fileContents := "My Test String"
				filename := test.CreateCredentialFile(tempDir, "file.txt", fileContents)
				readContents, err := util.ReadFileOrStringFromField(filename)
				Expect(readContents).To(Equal(fileContents))
				Expect(err).To(BeNil())
				os.RemoveAll(tempDir)
			})
		})

		if runtime.GOOS != "windows" {
			Context("when the file is not readable", func() {
				It("returns an error message if a file cannot be read", func() {
					tempDir := test.CreateTempDir("filesForTesting")
					fileContents := "My Test String"
					filename := test.CreateCredentialFile(tempDir, "file.txt", fileContents)
					err := os.Chmod(filename, 0222)
					Expect(err).To(BeNil())
					readContents, err := util.ReadFileOrStringFromField(filename)
					Expect(readContents).To(Equal(""))
					Expect(err).To(MatchError(credhub_errors.NewFileLoadError()))
				})
			})
		}
	})

	Describe("#AddDefaultSchemeIfNecessary", func() {
		It("adds the default scheme (https://) to a server which has none", func() {
			transformedUrl := util.AddDefaultSchemeIfNecessary("foo.com:8080")
			Expect(transformedUrl).To(Equal("https://foo.com:8080"))
		})

		It("does not add the default scheme if one is already there", func() {
			transformedUrl := util.AddDefaultSchemeIfNecessary("ftp://foo.com:8080")
			Expect(transformedUrl).To(Equal("ftp://foo.com:8080"))
		})
	})
})
