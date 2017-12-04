package metrics_test

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"code.cloudfoundry.org/clock/fakeclock"
	mfakes "code.cloudfoundry.org/diego-logging-client/testhelpers"

	"code.cloudfoundry.org/bbs/metrics"
	"code.cloudfoundry.org/lager/lagertest"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/tedsuo/ifrit"
)

var _ = Describe("FileDescriptorMetronNotifier", func() {
	var (
		fdNotifier             ifrit.Process
		fakeMetronClient       *mfakes.FakeIngressClient
		fakeProcFileSystemPath string
		fakeClock              *fakeclock.FakeClock
		reportInterval         time.Duration
		logger                 *lagertest.TestLogger
		fdCount                int
		err                    error
		symlinkedFileDir       string
	)

	BeforeEach(func() {
		fakeProcFileSystemPath, err = ioutil.TempDir("", "proc")
		Expect(err).NotTo(HaveOccurred())

		symlinkedFileDir, err = ioutil.TempDir("", "tmpdir")
		Expect(err).NotTo(HaveOccurred())

		fakeMetronClient = new(mfakes.FakeIngressClient)
		reportInterval = 100 * time.Millisecond
		fakeClock = fakeclock.NewFakeClock(time.Unix(123, 456))
		logger = lagertest.NewTestLogger("test")

		fdCount = 10
		for i := 0; i < fdCount; i++ {
			createSymlink(fakeProcFileSystemPath, symlinkedFileDir, strconv.Itoa(i))
		}
	})

	JustBeforeEach(func() {
		ticker := fakeClock.NewTicker(reportInterval)

		fdNotifier = ifrit.Invoke(
			metrics.NewFileDescriptorMetronNotifier(
				logger,
				ticker,
				fakeMetronClient,
				fakeProcFileSystemPath,
			),
		)
	})

	AfterEach(func() {
		fdNotifier.Signal(os.Interrupt)
		Eventually(fdNotifier.Wait(), 2*time.Second).Should(Receive())

		err := os.RemoveAll(fakeProcFileSystemPath)
		Expect(err).NotTo(HaveOccurred())

		err = os.RemoveAll(symlinkedFileDir)
		Expect(err).NotTo(HaveOccurred())
	})

	Context("when the file descriptor metron notifier is running", func() {
		It("periodically emits the number of open file descriptors as a metric", func() {
			fakeClock.WaitForWatcherAndIncrement(reportInterval)

			Eventually(fakeMetronClient.SendMetricCallCount).Should(Equal(1))
			name, value := fakeMetronClient.SendMetricArgsForCall(0)
			Expect(name).To(Equal("OpenFileDescriptors"))
			Expect(value).To(BeEquivalentTo(10))

			fakeClock.WaitForWatcherAndIncrement(reportInterval)

			Eventually(fakeMetronClient.SendMetricCallCount).Should(Equal(2))
			name, value = fakeMetronClient.SendMetricArgsForCall(1)
			Expect(name).To(Equal("OpenFileDescriptors"))
			Expect(value).To(BeEquivalentTo(10))

			By("creating a new symlink")
			createSymlink(fakeProcFileSystemPath, symlinkedFileDir, strconv.Itoa(11))
			fakeClock.WaitForWatcherAndIncrement(reportInterval)

			Eventually(fakeMetronClient.SendMetricCallCount).Should(Equal(3))
			name, value = fakeMetronClient.SendMetricArgsForCall(2)
			Expect(name).To(Equal("OpenFileDescriptors"))
			Expect(value).To(BeEquivalentTo(11))
		})
	})

	Context("when the notifier fails to read the proc filesystem", func() {
		BeforeEach(func() {
			fakeProcFileSystemPath = "/proc/quack/moo"
		})

		It("doesn't send a metric", func() {
			fakeClock.WaitForWatcherAndIncrement(reportInterval)
			Consistently(fakeMetronClient.SendMetricCallCount).Should(Equal(0))
		})
	})
})

func createSymlink(dir, tmpdir, symlinkId string) {
	fd, err := ioutil.TempFile(tmpdir, "socket")
	Expect(err).NotTo(HaveOccurred())
	symlink := filepath.Join(dir, symlinkId)

	err = os.Symlink(fd.Name(), symlink)
	Expect(err).NotTo(HaveOccurred())
}
