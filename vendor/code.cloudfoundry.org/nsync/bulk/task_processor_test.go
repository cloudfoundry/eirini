package bulk_test

import (
	"errors"
	"net/http"
	"sync"
	"time"

	"code.cloudfoundry.org/bbs/fake_bbs"
	"code.cloudfoundry.org/bbs/models"
	"code.cloudfoundry.org/clock/fakeclock"
	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/lager/lagertest"
	"code.cloudfoundry.org/nsync/bulk"
	"code.cloudfoundry.org/nsync/bulk/fakes"
	"code.cloudfoundry.org/runtimeschema/cc_messages"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/tedsuo/ifrit"
	"github.com/tedsuo/ifrit/ginkgomon"
)

var _ = Describe("Task Processor", func() {
	var (
		taskStatesToFetch []cc_messages.CCTaskState
		bbsClient         *fake_bbs.FakeClient
		taskClient        *fakes.FakeTaskClient
		fetcher           *fakes.FakeFetcher

		processor ifrit.Runner

		process         ifrit.Process
		syncDuration    time.Duration
		pollingInterval time.Duration
		clock           *fakeclock.FakeClock

		logger *lagertest.TestLogger
	)

	BeforeEach(func() {
		fetcher = new(fakes.FakeFetcher)
		fetcher.FetchTaskStatesStub = func(
			logger lager.Logger,
			cancel <-chan struct{},
			httpClient *http.Client,
		) (<-chan []cc_messages.CCTaskState, <-chan error) {
			results := make(chan []cc_messages.CCTaskState, 1)
			errors := make(chan error, 1)

			results <- taskStatesToFetch
			close(results)
			close(errors)

			return results, errors
		}

		bbsClient = new(fake_bbs.FakeClient)
		taskClient = new(fakes.FakeTaskClient)
		clock = fakeclock.NewFakeClock(time.Now())
		logger = lagertest.NewTestLogger("test")

		bbsClient.UpsertDomainStub = func(lager.Logger, string, time.Duration) error {
			clock.Increment(syncDuration)
			return nil
		}

		pollingInterval = 500 * time.Millisecond
		processor = bulk.NewTaskProcessor(
			logger,
			bbsClient,
			taskClient,
			pollingInterval,
			time.Second,
			50,
			50,
			false,
			fetcher,
			clock,
		)
	})

	JustBeforeEach(func() {
		process = ifrit.Invoke(processor)
	})

	AfterEach(func() {
		ginkgomon.Interrupt(process)
	})

	Context("when bbs does not know about a running task", func() {
		BeforeEach(func() {
			taskStatesToFetch = []cc_messages.CCTaskState{
				{TaskGuid: "task-guid-1", State: cc_messages.TaskStateRunning, CompletionCallbackUrl: "asdf"},
			}
		})

		It("fails the task", func() {
			Eventually(taskClient.FailTaskCallCount).Should(Equal(1))
			_, taskState, _ := taskClient.FailTaskArgsForCall(0)
			Expect(taskState.TaskGuid).Should(Equal("task-guid-1"))
			Expect(taskState.CompletionCallbackUrl).Should(Equal("asdf"))
		})

		It("updates the domain", func() {
			Eventually(bbsClient.UpsertDomainCallCount).Should(Equal(1))
		})

		Context("and failing the task fails", func() {
			BeforeEach(func() {
				taskClient.FailTaskReturns(errors.New("nope"))
			})

			It("does not update the domain", func() {
				Consistently(bbsClient.UpsertDomainCallCount).Should(Equal(0))
			})
		})
	})

	Context("when bbs does not know about a pending task", func() {
		BeforeEach(func() {
			taskStatesToFetch = []cc_messages.CCTaskState{
				{TaskGuid: "task-guid-1", State: cc_messages.TaskStatePending},
			}
		})

		It("does not fail the task", func() {
			Consistently(taskClient.FailTaskCallCount).Should(Equal(0))
		})
	})

	Context("when bbs does not know about a completed task", func() {
		BeforeEach(func() {
			taskStatesToFetch = []cc_messages.CCTaskState{
				{TaskGuid: "task-guid-1", State: cc_messages.TaskStateSucceeded},
			}
		})

		It("does not fail the task", func() {
			Consistently(taskClient.FailTaskCallCount).Should(Equal(0))
		})
	})

	Context("when bbs does not know about a canceling task", func() {
		BeforeEach(func() {
			taskStatesToFetch = []cc_messages.CCTaskState{
				{TaskGuid: "task-guid-1", State: cc_messages.TaskStateRunning, CompletionCallbackUrl: "asdf"},
			}
		})

		It("fails the task", func() {
			Eventually(taskClient.FailTaskCallCount).Should(Equal(1))
			_, taskState, _ := taskClient.FailTaskArgsForCall(0)
			Expect(taskState.TaskGuid).Should(Equal("task-guid-1"))
			Expect(taskState.CompletionCallbackUrl).Should(Equal("asdf"))
		})

		It("updates the domain", func() {
			Eventually(bbsClient.UpsertDomainCallCount).Should(Equal(1))
		})

		Context("and failing the task fails", func() {
			BeforeEach(func() {
				taskClient.FailTaskReturns(errors.New("nope"))
			})

			It("does not update the domain", func() {
				Consistently(bbsClient.UpsertDomainCallCount).Should(Equal(0))
			})
		})
	})

	Context("when bbs knows about a running task", func() {
		BeforeEach(func() {
			taskStatesToFetch = []cc_messages.CCTaskState{
				{TaskGuid: "task-guid-1", State: cc_messages.TaskStateRunning},
			}

			bbsClient.TasksByDomainReturns([]*models.Task{{TaskGuid: "task-guid-1"}}, nil)
		})

		It("does not fail the task", func() {
			Consistently(taskClient.FailTaskCallCount).Should(Equal(0))
		})
	})

	Context("when bbs has a running task cc does not know about", func() {
		BeforeEach(func() {
			taskStatesToFetch = []cc_messages.CCTaskState{}
			bbsClient.TasksByDomainReturns([]*models.Task{{TaskGuid: "task-guid-1", State: models.Task_Running}}, nil)
		})

		It("cancels the task", func() {
			Eventually(bbsClient.CancelTaskCallCount).Should(Equal(1))
		})

		It("updates the domain", func() {
			Eventually(bbsClient.UpsertDomainCallCount).Should(Equal(1))
		})
	})

	Context("when bbs has a running task cc wants to cancel", func() {
		BeforeEach(func() {
			taskStatesToFetch = []cc_messages.CCTaskState{
				{TaskGuid: "task-guid-1", State: cc_messages.TaskStateCanceling},
			}
			bbsClient.TasksByDomainReturns([]*models.Task{{TaskGuid: "task-guid-1", State: models.Task_Running}}, nil)
		})

		It("cancels the task", func() {
			Eventually(bbsClient.CancelTaskCallCount).Should(Equal(1))
		})

		It("updates the domain", func() {
			Eventually(bbsClient.UpsertDomainCallCount).Should(Equal(1))
		})
	})

	Context("when canceling a task fails", func() {
		BeforeEach(func() {
			taskStatesToFetch = []cc_messages.CCTaskState{
				{TaskGuid: "task-guid-1", State: cc_messages.TaskStateCanceling},
				{TaskGuid: "task-guid-2", State: cc_messages.TaskStateCanceling},
			}
			bbsClient.TasksByDomainReturns([]*models.Task{
				{TaskGuid: "task-guid-1", State: models.Task_Running},
				{TaskGuid: "task-guid-2", State: models.Task_Running},
			}, nil)

			lock := sync.Mutex{}
			count := 0
			bbsClient.CancelTaskStub = func(logger lager.Logger, guid string) error {
				lock.Lock()
				defer lock.Unlock()
				if count == 0 {
					count++
					return errors.New("oh no!")
				}
				return nil
			}
		})

		It("does not update the domain", func() {
			Consistently(bbsClient.UpsertDomainCallCount).Should(Equal(0))
		})

		It("sends all the other updates", func() {
			Eventually(bbsClient.CancelTaskCallCount).Should(Equal(2))
		})
	})

	Context("when fetching task states fails", func() {
		BeforeEach(func() {
			fetcher.FetchTaskStatesStub = func(
				logger lager.Logger,
				cancel <-chan struct{},
				httpClient *http.Client,
			) (<-chan []cc_messages.CCTaskState, <-chan error) {
				results := make(chan []cc_messages.CCTaskState, 1)
				errorsChan := make(chan error, 1)

				close(results)

				errorsChan <- errors.New("uh oh")
				close(errorsChan)

				return results, errorsChan
			}
		})

		It("keeps calm and carries on", func() {
			Consistently(process.Wait()).ShouldNot(Receive())
		})

		It("does not update the domain", func() {
			Consistently(bbsClient.UpsertDomainCallCount).Should(Equal(0))
		})

		It("doesn't fail any tasks", func() {
			Consistently(taskClient.FailTaskCallCount).Should(Equal(0))
		})
	})

	Context("when getting all tasks fails", func() {
		BeforeEach(func() {
			bbsClient.TasksByDomainReturns(nil, errors.New("oh no!"))
		})

		It("keeps calm and carries on", func() {
			Consistently(process.Wait()).ShouldNot(Receive())
		})

		It("tries again after the polling interval", func() {
			Eventually(bbsClient.TasksByDomainCallCount).Should(Equal(1))
			clock.Increment(pollingInterval / 2)
			Consistently(bbsClient.TasksByDomainCallCount).Should(Equal(1))

			clock.Increment(pollingInterval)
			Eventually(bbsClient.TasksByDomainCallCount).Should(Equal(2))
		})

		It("does not call the the fetcher, or the task client for updates", func() {
			Consistently(fetcher.FetchTaskStatesCallCount).Should(Equal(0))
			Consistently(taskClient.FailTaskCallCount).Should(Equal(0))
		})
	})
})
