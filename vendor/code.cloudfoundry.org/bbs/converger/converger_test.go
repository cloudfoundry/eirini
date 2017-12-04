package converger_test

import (
	"errors"
	"time"

	"code.cloudfoundry.org/lager/lagertest"
	"github.com/tedsuo/ifrit"
	"github.com/tedsuo/ifrit/ginkgomon"

	"code.cloudfoundry.org/bbs/models"
	"code.cloudfoundry.org/bbs/serviceclient/serviceclientfakes"
	"code.cloudfoundry.org/clock/fakeclock"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"code.cloudfoundry.org/bbs/converger"
	"code.cloudfoundry.org/bbs/converger/fake_controllers"
)

const aBit = 100 * time.Millisecond

var _ = Describe("ConvergerProcess", func() {
	var (
		fakeLrpConvergenceController *fake_controllers.FakeLrpConvergenceController
		fakeTaskController           *fake_controllers.FakeTaskController
		fakeBBSServiceClient         *serviceclientfakes.FakeServiceClient
		logger                       *lagertest.TestLogger
		fakeClock                    *fakeclock.FakeClock
		convergeRepeatInterval       time.Duration
		kickTaskDuration             time.Duration
		expirePendingTaskDuration    time.Duration
		expireCompletedTaskDuration  time.Duration

		process ifrit.Process

		waitEvents chan<- models.CellEvent
		waitErrs   chan<- error
	)

	BeforeEach(func() {
		fakeLrpConvergenceController = new(fake_controllers.FakeLrpConvergenceController)
		fakeTaskController = new(fake_controllers.FakeTaskController)
		fakeBBSServiceClient = new(serviceclientfakes.FakeServiceClient)
		logger = lagertest.NewTestLogger("test")
		fakeClock = fakeclock.NewFakeClock(time.Now())

		convergeRepeatInterval = 1 * time.Second

		kickTaskDuration = 10 * time.Millisecond
		expirePendingTaskDuration = 30 * time.Second
		expireCompletedTaskDuration = 60 * time.Minute

		cellEvents := make(chan models.CellEvent, 100)
		errs := make(chan error, 100)

		waitEvents = cellEvents
		waitErrs = errs

		fakeBBSServiceClient.CellEventsReturns(cellEvents)
	})

	JustBeforeEach(func() {
		process = ifrit.Invoke(
			converger.New(
				logger,
				fakeClock,
				fakeLrpConvergenceController,
				fakeTaskController,
				fakeBBSServiceClient,
				convergeRepeatInterval,
				kickTaskDuration,
				expirePendingTaskDuration,
				expireCompletedTaskDuration,
			),
		)
	})

	AfterEach(func() {
		ginkgomon.Interrupt(process)
		Eventually(process.Wait()).Should(Receive())
	})

	Describe("converging over time", func() {
		It("converges tasks, LRPs, and auctions when the lock is periodically reestablished", func() {
			fakeClock.WaitForWatcherAndIncrement(convergeRepeatInterval + aBit)

			Eventually(fakeTaskController.ConvergeTasksCallCount).Should(Equal(1))
			Eventually(fakeLrpConvergenceController.ConvergeLRPsCallCount).Should(Equal(1))

			_, actualKickTaskDuration, actualExpirePendingTaskDuration, actualExpireCompletedTaskDuration := fakeTaskController.ConvergeTasksArgsForCall(0)
			Expect(actualKickTaskDuration).To(Equal(kickTaskDuration))
			Expect(actualExpirePendingTaskDuration).To(Equal(expirePendingTaskDuration))
			Expect(actualExpireCompletedTaskDuration).To(Equal(expireCompletedTaskDuration))

			fakeClock.WaitForWatcherAndIncrement(convergeRepeatInterval + aBit)

			Eventually(fakeTaskController.ConvergeTasksCallCount).Should(Equal(2))
			Eventually(fakeLrpConvergenceController.ConvergeLRPsCallCount).Should(Equal(2))

			_, actualKickTaskDuration, actualExpirePendingTaskDuration, actualExpireCompletedTaskDuration = fakeTaskController.ConvergeTasksArgsForCall(1)
			Expect(actualKickTaskDuration).To(Equal(kickTaskDuration))
			Expect(actualExpirePendingTaskDuration).To(Equal(expirePendingTaskDuration))
			Expect(actualExpireCompletedTaskDuration).To(Equal(expireCompletedTaskDuration))
		})
	})

	Describe("converging when cells disappear", func() {
		It("converges tasks and LRPs immediately", func() {
			Consistently(fakeTaskController.ConvergeTasksCallCount).Should(Equal(0))
			Consistently(fakeLrpConvergenceController.ConvergeLRPsCallCount).Should(Equal(0))

			waitEvents <- models.CellDisappearedEvent{
				IDs: []string{"some-cell-id"},
			}

			Eventually(fakeTaskController.ConvergeTasksCallCount).Should(Equal(1))
			Eventually(fakeLrpConvergenceController.ConvergeLRPsCallCount).Should(Equal(1))

			_, actualKickTaskDuration, actualExpirePendingTaskDuration, actualExpireCompletedTaskDuration := fakeTaskController.ConvergeTasksArgsForCall(0)
			Expect(actualKickTaskDuration).To(Equal(kickTaskDuration))
			Expect(actualExpirePendingTaskDuration).To(Equal(expirePendingTaskDuration))
			Expect(actualExpireCompletedTaskDuration).To(Equal(expireCompletedTaskDuration))

			waitErrs <- errors.New("whoopsie")

			waitEvents <- models.CellDisappearedEvent{
				IDs: []string{"some-cell-id"},
			}

			Eventually(fakeTaskController.ConvergeTasksCallCount).Should(Equal(2))
			Eventually(fakeLrpConvergenceController.ConvergeLRPsCallCount).Should(Equal(2))
		})

		It("defers convergence to one full interval later", func() {
			fakeClock.WaitForWatcherAndIncrement(convergeRepeatInterval - aBit)

			waitEvents <- models.CellDisappearedEvent{
				IDs: []string{"some-cell-id"},
			}

			Eventually(fakeTaskController.ConvergeTasksCallCount).Should(Equal(1))
			Eventually(fakeLrpConvergenceController.ConvergeLRPsCallCount).Should(Equal(1))

			fakeClock.WaitForWatcherAndIncrement(2 * aBit)

			Consistently(fakeTaskController.ConvergeTasksCallCount).Should(Equal(1))
			Consistently(fakeLrpConvergenceController.ConvergeLRPsCallCount).Should(Equal(1))

			fakeClock.WaitForWatcherAndIncrement(convergeRepeatInterval + aBit)
			Eventually(fakeTaskController.ConvergeTasksCallCount).Should(Equal(2))
			Eventually(fakeLrpConvergenceController.ConvergeLRPsCallCount).Should(Equal(2))
		})
	})
})
