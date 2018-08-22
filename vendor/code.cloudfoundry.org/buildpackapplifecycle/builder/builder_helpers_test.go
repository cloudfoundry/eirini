// +build !windows2012R2

package main_test

import (
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"
)

func buildBuilder() string {
	builder, err := gexec.Build("code.cloudfoundry.org/buildpackapplifecycle/builder")
	Expect(err).NotTo(HaveOccurred())

	return builder
}

func downloadTar() string {
	return ""
}

func copyTar(destDir string) {
	return
}
