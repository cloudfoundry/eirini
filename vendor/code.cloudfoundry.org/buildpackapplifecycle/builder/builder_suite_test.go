package main_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"
)

var (
	builderPath string
	tarPath     string
)

func TestBuildpackLifecycleBuilder(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Buildpack-Lifecycle-Builder Suite")
}

var _ = SynchronizedBeforeSuite(func() []byte {
	builder := buildBuilder()
	tar := downloadTar()
	return []byte(builder + "^" + tar)
}, func(exePaths []byte) {
	paths := strings.Split(string(exePaths), "^")
	builderPath = paths[0]
	tarPath = paths[1]
})

var _ = SynchronizedAfterSuite(func() {
	//noop
}, func() {
	gexec.CleanupBuildArtifacts()
	if tarPath != "" {
		Expect(os.RemoveAll(filepath.Dir(tarPath))).To(Succeed())
	}
})
