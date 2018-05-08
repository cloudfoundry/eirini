package main_test

import (
	"io/ioutil"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"
)

var launcher string
var hello string

const defaultTimeout = time.Second * 5
const defaultInterval = time.Millisecond * 100

func TestBuildpackLifecycleLauncher(t *testing.T) {
	SetDefaultEventuallyTimeout(defaultTimeout)
	SetDefaultEventuallyPollingInterval(defaultInterval)

	RegisterFailHandler(Fail)
	RunSpecs(t, "Buildpack-Lifecycle-Launcher Suite")
}

var _ = SynchronizedBeforeSuite(func() []byte {
	helloPath, err := gexec.Build("code.cloudfoundry.org/buildpackapplifecycle/launcher/fixtures/hello")
	Expect(err).NotTo(HaveOccurred())

	launcherPath := buildLauncher()

	getenvPath, err := gexec.Build("code.cloudfoundry.org/buildpackapplifecycle/getenv")
	Expect(err).NotTo(HaveOccurred())

	return []byte(helloPath + "^" + launcherPath + "^" + getenvPath)
}, func(exePaths []byte) {
	paths := strings.Split(string(exePaths), "^")
	hello = paths[0]
	launcher = paths[1]

	if runtime.GOOS == "windows" {
		getenv := paths[2]

		launcherDir := filepath.Dir(launcher)

		getenvContents, err := ioutil.ReadFile(getenv)
		Expect(err).NotTo(HaveOccurred())

		err = ioutil.WriteFile(filepath.Join(launcherDir, "getenv.exe"), getenvContents, 0644)
		Expect(err).NotTo(HaveOccurred())
	}
})

var _ = SynchronizedAfterSuite(func() {
	//noop
}, func() {
	gexec.CleanupBuildArtifacts()
})
