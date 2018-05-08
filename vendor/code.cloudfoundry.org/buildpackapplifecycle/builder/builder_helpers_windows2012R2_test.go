// +build windows,windows2012R2

package main_test

import (
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"

	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"
)

func buildBuilder() string {
	builder, err := gexec.Build("code.cloudfoundry.org/buildpackapplifecycle/builder", "-tags=windows2012R2")
	Expect(err).NotTo(HaveOccurred())

	return builder
}

func downloadTar() string {
	tarUrl := os.Getenv("TAR_URL")
	Expect(tarUrl).NotTo(BeEmpty(), "TAR_URL environment variable must be set")

	resp, err := http.Get(tarUrl)
	Expect(err).NotTo(HaveOccurred())

	defer resp.Body.Close()

	tmpDir, err := ioutil.TempDir("", "tar")
	Expect(err).NotTo(HaveOccurred())

	tarExePath := filepath.Join(tmpDir, "tar.exe")
	f, err := os.OpenFile(tarExePath, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0755)
	Expect(err).NotTo(HaveOccurred())
	defer f.Close()

	_, err = io.Copy(f, resp.Body)
	Expect(err).NotTo(HaveOccurred())

	return tarExePath
}

func copyTar(destDir string) {
	f, err := os.Open(tarPath)
	Expect(err).NotTo(HaveOccurred())
	defer f.Close()

	Expect(os.MkdirAll(destDir, 0755)).To(Succeed())

	d, err := os.OpenFile(filepath.Join(destDir, "tar.exe"), os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0755)
	Expect(err).NotTo(HaveOccurred())
	defer d.Close()

	_, err = io.Copy(d, f)
	Expect(err).NotTo(HaveOccurred())
	return
}
