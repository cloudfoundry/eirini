package taskworkpool_test

import (
	"encoding/json"
	"io/ioutil"
	"net/http"
	"os"
	"time"

	"code.cloudfoundry.org/bbs/db/dbfakes"
	"code.cloudfoundry.org/bbs/events/eventfakes"
	"code.cloudfoundry.org/bbs/models"
	"code.cloudfoundry.org/bbs/models/test/model_helpers"
	"code.cloudfoundry.org/bbs/taskworkpool"
	"code.cloudfoundry.org/cfhttp"
	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/lager/lagertest"
	"github.com/tedsuo/ifrit"
	"github.com/tedsuo/ifrit/ginkgomon"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/ghttp"
)

var _ = Describe("TaskWorker", func() {
	var (
		fakeServer *ghttp.Server
		logger     *lagertest.TestLogger
		timeout    time.Duration
	)

	BeforeEach(func() {
		timeout = 1 * time.Second
		cfhttp.Initialize(timeout)
		fakeServer = ghttp.NewServer()

		logger = lagertest.NewTestLogger("test")
		logger.RegisterSink(lager.NewWriterSink(GinkgoWriter, lager.INFO))
	})

	AfterEach(func() {
		fakeServer.Close()
	})

	Describe("HandleCompletedTask", func() {
		var (
			callbackURL   string
			taskDB        *dbfakes.FakeTaskDB
			statusCodes   chan int
			task          *models.Task
			before, after models.Task
			taskHub       *eventfakes.FakeHub

			httpClient *http.Client
		)

		BeforeEach(func() {
			httpClient = cfhttp.NewClient()
			statusCodes = make(chan int)
			taskHub = &eventfakes.FakeHub{}

			fakeServer.RouteToHandler("POST", "/the-callback/url", func(w http.ResponseWriter, req *http.Request) {
				w.WriteHeader(<-statusCodes)
			})

			callbackURL = fakeServer.URL() + "/the-callback/url"
			taskDB = new(dbfakes.FakeTaskDB)
			taskDB.ResolvingTaskStub = func(_ lager.Logger, taskGuidTaskGuid string) (*models.Task, *models.Task, error) {
				before = *task
				after = *task
				after.State = models.Task_Resolving
				return &before, &after, nil
			}

			taskDB.DeleteTaskStub = func(_ lager.Logger, taskGuid string) (*models.Task, error) {
				return task, nil
			}
		})

		simulateTaskCompleting := func(signals <-chan os.Signal, ready chan<- struct{}) error {
			close(ready)
			task = model_helpers.NewValidTask("the-task-guid")
			task.CompletionCallbackUrl = callbackURL
			taskworkpool.HandleCompletedTask(logger, httpClient, taskDB, taskHub, task)
			return nil
		}

		var process ifrit.Process
		JustBeforeEach(func() {
			process = ifrit.Invoke(ifrit.RunFunc(simulateTaskCompleting))
		})

		AfterEach(func() {
			ginkgomon.Kill(process)
		})

		Context("when the task has a completion callback URL", func() {
			BeforeEach(func() {
				Expect(taskDB.ResolvingTaskCallCount()).To(Equal(0))
			})

			It("marks the task as resolving", func() {
				statusCodes <- 200

				Eventually(taskDB.ResolvingTaskCallCount).Should(Equal(1))
				_, actualGuid := taskDB.ResolvingTaskArgsForCall(0)
				Expect(actualGuid).To(Equal("the-task-guid"))
			})

			It("emits a TaskChangedEvent to the hub", func() {
				// Do not emit TaskRemovedEvent in callback handler
				for i := 0; i < taskworkpool.MAX_CB_RETRIES; i++ {
					statusCodes <- http.StatusServiceUnavailable
				}

				Eventually(taskHub.EmitCallCount).Should(Equal(1))
				event := taskHub.EmitArgsForCall(0)
				changed, ok := event.(*models.TaskChangedEvent)
				Expect(ok).To(BeTrue())
				Expect(changed.Before).To(BeEquivalentTo(&before))
				Expect(changed.After).To(BeEquivalentTo(&after))
			})

			Context("when marking the task as resolving fails", func() {
				BeforeEach(func() {
					taskDB.ResolvingTaskReturns(nil, nil, models.NewError(models.Error_UnknownError, "failed to resolve task"))
				})

				It("does not make a request to the task's callback URL", func() {
					Consistently(fakeServer.ReceivedRequests, 0.25).Should(BeEmpty())
				})
			})

			Context("when marking the task as resolving succeeds", func() {
				It("POSTs to the task's callback URL", func() {
					statusCodes <- 200
					Eventually(fakeServer.ReceivedRequests).Should(HaveLen(1))
				})

				Context("when the request succeeds", func() {
					BeforeEach(func() {
						fakeServer.RouteToHandler("POST", "/the-callback/url", func(w http.ResponseWriter, req *http.Request) {
							w.WriteHeader(<-statusCodes)
							data, err := ioutil.ReadAll(req.Body)
							Expect(err).NotTo(HaveOccurred())

							var response models.TaskCallbackResponse
							err = json.Unmarshal(data, &response)
							Expect(err).NotTo(HaveOccurred())

							Expect(response.CreatedAt).To(Equal(task.CreatedAt))
							Expect(response.TaskGuid).To(Equal("the-task-guid"))
							Expect(response.CreatedAt).To(Equal(task.CreatedAt))
						})
					})

					It("resolves the task", func() {
						statusCodes <- 200

						Eventually(taskDB.DeleteTaskCallCount).Should(Equal(1))
						_, actualGuid := taskDB.DeleteTaskArgsForCall(0)
						Expect(actualGuid).To(Equal("the-task-guid"))
					})

					It("emits a TaskRemovedEvent to the hub", func() {
						statusCodes <- 200

						Eventually(taskHub.EmitCallCount).Should(Equal(2))
						event := taskHub.EmitArgsForCall(1)
						removed, ok := event.(*models.TaskRemovedEvent)
						Expect(ok).To(BeTrue())
						Expect(removed.Task).To(BeEquivalentTo(task))
					})
				})

				Context("when the request fails with a 4xx response code", func() {
					It("resolves the task", func() {
						statusCodes <- 403

						Eventually(taskDB.DeleteTaskCallCount).Should(Equal(1))
						_, actualGuid := taskDB.DeleteTaskArgsForCall(0)
						Expect(actualGuid).To(Equal("the-task-guid"))
					})

					It("emits a TaskRemovedEvent to the hub", func() {
						statusCodes <- 403

						Eventually(taskHub.EmitCallCount).Should(Equal(2))
						event := taskHub.EmitArgsForCall(1)
						removed, ok := event.(*models.TaskRemovedEvent)
						Expect(ok).To(BeTrue())
						Expect(removed.Task).To(BeEquivalentTo(task))
					})
				})

				Context("when the request fails with a 500 response code", func() {
					It("resolves the task", func() {
						statusCodes <- 500

						Eventually(taskDB.DeleteTaskCallCount).Should(Equal(1))
						_, actualGuid := taskDB.DeleteTaskArgsForCall(0)
						Expect(actualGuid).To(Equal("the-task-guid"))
					})

					It("emits a TaskRemovedEvent to the hub", func() {
						statusCodes <- 500

						Eventually(taskHub.EmitCallCount).Should(Equal(2))
						event := taskHub.EmitArgsForCall(1)
						removed, ok := event.(*models.TaskRemovedEvent)
						Expect(ok).To(BeTrue())
						Expect(removed.Task).To(BeEquivalentTo(task))
					})
				})

				Context("when the request fails with a 503 or 504 response code", func() {
					It("retries the request 2 more times", func() {
						Eventually(fakeServer.ReceivedRequests).Should(HaveLen(1))

						statusCodes <- 503

						Consistently(taskDB.DeleteTaskCallCount, 0.25).Should(Equal(0))
						Eventually(fakeServer.ReceivedRequests).Should(HaveLen(2))

						statusCodes <- 504

						Consistently(taskDB.DeleteTaskCallCount, 0.25).Should(Equal(0))
						Eventually(fakeServer.ReceivedRequests).Should(HaveLen(3))

						statusCodes <- 200

						Eventually(taskDB.DeleteTaskCallCount, 0.25).Should(Equal(1))
						_, actualGuid := taskDB.DeleteTaskArgsForCall(0)
						Expect(actualGuid).To(Equal("the-task-guid"))
					})

					Context("when the request fails every time", func() {
						It("does not resolve the task", func() {
							Eventually(fakeServer.ReceivedRequests).Should(HaveLen(1))

							statusCodes <- 503

							Consistently(taskDB.DeleteTaskCallCount, 0.25).Should(Equal(0))
							Eventually(fakeServer.ReceivedRequests).Should(HaveLen(2))

							statusCodes <- 504

							Consistently(taskDB.DeleteTaskCallCount, 0.25).Should(Equal(0))
							Eventually(fakeServer.ReceivedRequests).Should(HaveLen(3))

							statusCodes <- 503

							Consistently(taskDB.DeleteTaskCallCount, 0.25).Should(Equal(0))
							Consistently(fakeServer.ReceivedRequests, 0.25).Should(HaveLen(3))
						})
					})
				})

				Context("when DeleteTask fails", func() {
					BeforeEach(func() {
						taskDB.DeleteTaskReturns(nil, &models.Error{})
					})

					It("logs an error and returns", func() {
						Eventually(fakeServer.ReceivedRequests).Should(HaveLen(1))
						statusCodes <- 200

						Eventually(taskDB.DeleteTaskCallCount).Should(Equal(1))
						Eventually(logger.TestSink.LogMessages).Should(ContainElement("test.handle-completed-task.delete-task-failed"))
					})
				})

				Context("when the request fails with a timeout", func() {
					var sleepCh chan time.Duration

					BeforeEach(func() {
						sleepCh = make(chan time.Duration)
						fakeServer.RouteToHandler("POST", "/the-callback/url", func(w http.ResponseWriter, req *http.Request) {
							time.Sleep(<-sleepCh)
							w.WriteHeader(200)
						})
					})

					It("retries the request 2 more times", func() {
						sleepCh <- timeout + 100*time.Millisecond
						Eventually(fakeServer.ReceivedRequests).Should(HaveLen(1))

						sleepCh <- timeout + 100*time.Millisecond
						Consistently(taskDB.DeleteTaskCallCount, 0.25).Should(Equal(0))
						Eventually(fakeServer.ReceivedRequests).Should(HaveLen(2))

						sleepCh <- timeout + 100*time.Millisecond
						Consistently(taskDB.DeleteTaskCallCount, 0.25).Should(Equal(0))
						Eventually(fakeServer.ReceivedRequests).Should(HaveLen(3))

						Eventually(taskDB.DeleteTaskCallCount, 0.25).Should(Equal(0))
					})

					Context("when the request fails with timeout once and then succeeds", func() {
						It("deletes the task", func() {
							sleepCh <- (timeout + 100*time.Millisecond)

							Eventually(fakeServer.ReceivedRequests).Should(HaveLen(1))
							Consistently(taskDB.DeleteTaskCallCount, 0.25).Should(Equal(0))

							sleepCh <- 0
							Eventually(fakeServer.ReceivedRequests).Should(HaveLen(2))
							Eventually(taskDB.DeleteTaskCallCount, 0.25).Should(Equal(1))

							_, resolvedTaskGuid := taskDB.DeleteTaskArgsForCall(0)
							Expect(resolvedTaskGuid).To(Equal("the-task-guid"))
						})
					})
				})
			})
		})

		Context("when the task doesn't have a completion callback URL", func() {
			BeforeEach(func() {
				callbackURL = ""
			})

			It("does not mark the task as resolving", func() {
				Consistently(taskDB.ResolvingTaskCallCount).Should(Equal(0))
			})
		})
	})
})
