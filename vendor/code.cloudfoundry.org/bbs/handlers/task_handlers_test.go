package handlers_test

import (
	"net/http"
	"net/http/httptest"

	"code.cloudfoundry.org/bbs/handlers"
	"code.cloudfoundry.org/bbs/handlers/fake_controllers"
	"code.cloudfoundry.org/bbs/models"
	"code.cloudfoundry.org/bbs/models/test/model_helpers"
	"code.cloudfoundry.org/lager/lagertest"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
)

var _ = Describe("Task Handlers", func() {
	var (
		logger     *lagertest.TestLogger
		controller *fake_controllers.FakeTaskController

		responseRecorder *httptest.ResponseRecorder

		handler *handlers.TaskHandler
		exitCh  chan struct{}

		requestBody interface{}

		request *http.Request
	)

	BeforeEach(func() {
		logger = lagertest.NewTestLogger("test")
		responseRecorder = httptest.NewRecorder()
		exitCh = make(chan struct{}, 1)
		controller = &fake_controllers.FakeTaskController{}
		handler = handlers.NewTaskHandler(controller, exitCh)
	})

	Describe("Tasks", func() {
		var (
			task1          models.Task
			task2          models.Task
			cellId, domain string
		)

		BeforeEach(func() {
			task1 = models.Task{Domain: "domain-1"}
			task2 = models.Task{CellId: "cell-id"}
			requestBody = &models.TasksRequest{}
		})

		JustBeforeEach(func() {
			requestBody = &models.TasksRequest{
				Domain: domain,
				CellId: cellId,
			}
			request = newTestRequest(requestBody)
			handler.Tasks(logger, responseRecorder, request)
		})

		Context("when reading tasks from controller succeeds", func() {
			var tasks []*models.Task

			BeforeEach(func() {
				tasks = []*models.Task{&task1, &task2}
				controller.TasksReturns(tasks, nil)
			})

			It("returns a list of task", func() {
				Expect(responseRecorder.Code).To(Equal(http.StatusOK))
				response := models.TasksResponse{}
				err := response.Unmarshal(responseRecorder.Body.Bytes())
				Expect(err).NotTo(HaveOccurred())

				Expect(response.Error).To(BeNil())
				Expect(response.Tasks).To(Equal(tasks))
			})

			It("calls the controller with no filter", func() {
				Expect(controller.TasksCallCount()).To(Equal(1))
				_, actualDomain, actualCellId := controller.TasksArgsForCall(0)
				Expect(actualDomain).To(Equal(domain))
				Expect(actualCellId).To(Equal(cellId))
			})

			Context("and filtering by domain", func() {
				BeforeEach(func() {
					domain = "domain-1"
				})

				It("calls the controller with a domain filter", func() {
					Expect(controller.TasksCallCount()).To(Equal(1))
					_, actualDomain, actualCellId := controller.TasksArgsForCall(0)
					Expect(actualDomain).To(Equal(domain))
					Expect(actualCellId).To(Equal(cellId))
				})
			})

			Context("and filtering by cell id", func() {
				BeforeEach(func() {
					cellId = "cell-id"
				})

				It("calls the controller with a cell filter", func() {
					Expect(controller.TasksCallCount()).To(Equal(1))
					_, actualDomain, actualCellId := controller.TasksArgsForCall(0)
					Expect(actualDomain).To(Equal(domain))
					Expect(actualCellId).To(Equal(cellId))
				})
			})
		})

		Context("when the controller returns an unrecoverable error", func() {
			BeforeEach(func() {
				controller.TasksReturns(nil, models.NewUnrecoverableError(nil))
			})

			It("logs and writes to the exit channel", func() {
				Eventually(logger).Should(gbytes.Say("unrecoverable-error"))
				Eventually(exitCh).Should(Receive())
			})
		})

		Context("when the controller errors out", func() {
			BeforeEach(func() {
				controller.TasksReturns(nil, models.ErrUnknownError)
			})

			It("provides relevant error information", func() {
				Expect(responseRecorder.Code).To(Equal(http.StatusOK))
				response := models.TasksResponse{}
				err := response.Unmarshal(responseRecorder.Body.Bytes())
				Expect(err).NotTo(HaveOccurred())

				Expect(response.Error).To(Equal(models.ErrUnknownError))
			})
		})
	})

	Describe("TaskByGuid", func() {
		var taskGuid = "task-guid"

		BeforeEach(func() {
			requestBody = &models.TaskByGuidRequest{
				TaskGuid: taskGuid,
			}
		})

		JustBeforeEach(func() {
			request := newTestRequest(requestBody)
			handler.TaskByGuid(logger, responseRecorder, request)
		})

		Context("when reading a task from the controller succeeds", func() {
			var task *models.Task

			BeforeEach(func() {
				task = &models.Task{TaskGuid: taskGuid}
				controller.TaskByGuidReturns(task, nil)
			})

			It("fetches task by guid", func() {
				Expect(controller.TaskByGuidCallCount()).To(Equal(1))
				_, actualGuid := controller.TaskByGuidArgsForCall(0)
				Expect(actualGuid).To(Equal(taskGuid))
			})

			It("returns the task", func() {
				Expect(responseRecorder.Code).To(Equal(http.StatusOK))
				response := models.TaskResponse{}
				err := response.Unmarshal(responseRecorder.Body.Bytes())
				Expect(err).NotTo(HaveOccurred())

				Expect(response.Error).To(BeNil())
				Expect(response.Task).To(Equal(task))
			})
		})

		Context("when the controller returns no task", func() {
			BeforeEach(func() {
				controller.TaskByGuidReturns(nil, models.ErrResourceNotFound)
			})

			It("returns a resource not found error", func() {
				Expect(responseRecorder.Code).To(Equal(http.StatusOK))
				response := models.TaskResponse{}
				err := response.Unmarshal(responseRecorder.Body.Bytes())
				Expect(err).NotTo(HaveOccurred())

				Expect(response.Error).To(Equal(models.ErrResourceNotFound))
			})
		})

		Context("when the controller returns an unrecoverable error", func() {
			BeforeEach(func() {
				controller.TaskByGuidReturns(nil, models.NewUnrecoverableError(nil))
			})

			It("logs and writes to the exit channel", func() {
				Eventually(logger).Should(gbytes.Say("unrecoverable-error"))
				Eventually(exitCh).Should(Receive())
			})
		})

		Context("when the controller errors out", func() {
			BeforeEach(func() {
				controller.TaskByGuidReturns(nil, models.ErrUnknownError)
			})

			It("provides relevant error information", func() {
				Expect(responseRecorder.Code).To(Equal(http.StatusOK))
				response := models.TaskResponse{}
				err := response.Unmarshal(responseRecorder.Body.Bytes())
				Expect(err).NotTo(HaveOccurred())

				Expect(response.Error).To(Equal(models.ErrUnknownError))
			})
		})
	})

	Describe("DesireTask", func() {
		var (
			taskGuid = "task-guid"
			domain   = "domain"
			taskDef  *models.TaskDefinition
		)

		BeforeEach(func() {
			taskDef = model_helpers.NewValidTaskDefinition()
			requestBody = &models.DesireTaskRequest{
				TaskGuid:       taskGuid,
				Domain:         domain,
				TaskDefinition: taskDef,
			}
		})

		JustBeforeEach(func() {
			request := newTestRequest(requestBody)
			handler.DesireTask(logger, responseRecorder, request)
		})

		Context("when the desire is successful", func() {
			It("desires the task with the requested definitions", func() {
				Expect(controller.DesireTaskCallCount()).To(Equal(1))
				_, actualTaskDef, actualTaskGuid, actualDomain := controller.DesireTaskArgsForCall(0)
				Expect(actualTaskDef).To(Equal(taskDef))
				Expect(actualTaskGuid).To(Equal(taskGuid))
				Expect(actualDomain).To(Equal(domain))

				Expect(responseRecorder.Code).To(Equal(http.StatusOK))
				response := &models.TaskLifecycleResponse{}
				err := response.Unmarshal(responseRecorder.Body.Bytes())
				Expect(err).NotTo(HaveOccurred())

				Expect(response.Error).To(BeNil())
			})
		})

		Context("when the controller returns an unrecoverable error", func() {
			BeforeEach(func() {
				controller.DesireTaskReturns(models.NewUnrecoverableError(nil))
			})

			It("logs and writes to the exit channel", func() {
				Eventually(logger).Should(gbytes.Say("unrecoverable-error"))
				Eventually(exitCh).Should(Receive())
			})
		})

		Context("when desiring the task fails", func() {
			BeforeEach(func() {
				controller.DesireTaskReturns(models.ErrUnknownError)
			})

			It("responds with an error", func() {
				Expect(responseRecorder.Code).To(Equal(http.StatusOK))
				response := &models.TaskLifecycleResponse{}
				err := response.Unmarshal(responseRecorder.Body.Bytes())
				Expect(err).NotTo(HaveOccurred())

				Expect(response.Error).To(Equal(models.ErrUnknownError))
			})
		})
	})

	Describe("StartTask", func() {
		Context("when the start is successful", func() {
			BeforeEach(func() {
				requestBody = &models.StartTaskRequest{
					TaskGuid: "task-guid",
					CellId:   "cell-id",
				}
			})

			JustBeforeEach(func() {
				request := newTestRequest(requestBody)
				handler.StartTask(logger, responseRecorder, request)
			})

			It("calls StartTask", func() {
				Expect(controller.StartTaskCallCount()).To(Equal(1))
				taskLogger, taskGuid, cellId := controller.StartTaskArgsForCall(0)
				Expect(taskLogger.SessionName()).To(ContainSubstring("start-task"))
				Expect(taskGuid).To(Equal("task-guid"))
				Expect(cellId).To(Equal("cell-id"))
			})

			Context("when the task should start", func() {
				BeforeEach(func() {
					controller.StartTaskReturns(true, nil)
				})

				It("responds with true", func() {
					Expect(responseRecorder.Code).To(Equal(http.StatusOK))
					response := &models.StartTaskResponse{}
					err := response.Unmarshal(responseRecorder.Body.Bytes())
					Expect(err).NotTo(HaveOccurred())

					Expect(response.Error).To(BeNil())
					Expect(response.ShouldStart).To(BeTrue())
				})
			})

			Context("when the task should not start", func() {
				BeforeEach(func() {
					controller.StartTaskReturns(false, nil)
				})

				It("responds with false", func() {
					Expect(responseRecorder.Code).To(Equal(http.StatusOK))
					response := &models.StartTaskResponse{}
					err := response.Unmarshal(responseRecorder.Body.Bytes())
					Expect(err).NotTo(HaveOccurred())

					Expect(response.Error).To(BeNil())
					Expect(response.ShouldStart).To(BeFalse())
				})
			})

			Context("when the controller returns an unrecoverable error", func() {
				BeforeEach(func() {
					controller.StartTaskReturns(false, models.NewUnrecoverableError(nil))
				})

				It("logs and writes to the exit channel", func() {
					Eventually(logger).Should(gbytes.Say("unrecoverable-error"))
					Eventually(exitCh).Should(Receive())
				})
			})

			Context("when the controller fails", func() {
				BeforeEach(func() {
					controller.StartTaskReturns(false, models.ErrResourceExists)
				})

				It("bubbles up the underlying model error", func() {
					Expect(responseRecorder.Code).To(Equal(http.StatusOK))
					response := &models.StartTaskResponse{}
					err := response.Unmarshal(responseRecorder.Body.Bytes())
					Expect(err).NotTo(HaveOccurred())

					Expect(response.Error).To(Equal(models.ErrResourceExists))
				})
			})
		})
	})

	Describe("CancelTask", func() {
		var (
			request *http.Request
		)

		BeforeEach(func() {
			requestBody = &models.TaskGuidRequest{
				TaskGuid: "task-guid",
			}

			controller.CancelTaskReturns(nil)

			request = newTestRequest(requestBody)
		})

		JustBeforeEach(func() {
			handler.CancelTask(logger, responseRecorder, request)
			Expect(responseRecorder.Code).To(Equal(http.StatusOK))
		})

		Context("when the cancel request is normal", func() {
			Context("when canceling the task in the controller succeeds", func() {
				BeforeEach(func() {
					cellPresence := models.CellPresence{CellId: "cell-id"}
					fakeServiceClient.CellByIdReturns(&cellPresence, nil)
				})

				It("returns no error", func() {
					Expect(controller.CancelTaskCallCount()).To(Equal(1))
					taskLogger, taskGuid := controller.CancelTaskArgsForCall(0)
					Expect(taskLogger.SessionName()).To(ContainSubstring("cancel-task"))
					Expect(taskGuid).To(Equal("task-guid"))

					response := &models.TaskLifecycleResponse{}
					err := response.Unmarshal(responseRecorder.Body.Bytes())
					Expect(err).NotTo(HaveOccurred())

					Expect(response.Error).To(BeNil())
				})
			})

			Context("when cancelling the task fails", func() {
				BeforeEach(func() {
					controller.CancelTaskReturns(models.ErrUnknownError)
				})

				It("responds with an error", func() {
					response := &models.TaskLifecycleResponse{}
					err := response.Unmarshal(responseRecorder.Body.Bytes())
					Expect(err).NotTo(HaveOccurred())

					Expect(response.Error).To(Equal(models.ErrUnknownError))
				})
			})
			Context("when the controller returns an unrecoverable error", func() {
				BeforeEach(func() {
					controller.CancelTaskReturns(models.NewUnrecoverableError(nil))
				})

				It("logs and writes to the exit channel", func() {
					Eventually(logger).Should(gbytes.Say("unrecoverable-error"))
					Eventually(exitCh).Should(Receive())
				})
			})
		})

		Context("when the cancel task request is not valid", func() {
			BeforeEach(func() {
				request = newTestRequest("{{")
			})

			It("returns an BadRequest error", func() {
				response := &models.TaskLifecycleResponse{}
				err := response.Unmarshal(responseRecorder.Body.Bytes())
				Expect(err).NotTo(HaveOccurred())

				Expect(response.Error).To(Equal(models.ErrBadRequest))
			})
		})
	})

	Describe("FailTask", func() {
		var (
			taskGuid      string
			failureReason string
		)

		BeforeEach(func() {
			taskGuid = "task-guid"
			failureReason = "just cuz ;)"

			controller.FailTaskReturns(nil)

			requestBody = &models.FailTaskRequest{
				TaskGuid:      taskGuid,
				FailureReason: failureReason,
			}
		})

		JustBeforeEach(func() {
			request = newTestRequest(requestBody)
			handler.FailTask(logger, responseRecorder, request)
		})

		Context("when failing the task succeeds", func() {
			It("returns no error", func() {
				_, actualTaskGuid, actualFailureReason := controller.FailTaskArgsForCall(0)
				Expect(actualTaskGuid).To(Equal(taskGuid))
				Expect(actualFailureReason).To(Equal(failureReason))

				Expect(responseRecorder.Code).To(Equal(http.StatusOK))
				response := &models.TaskLifecycleResponse{}
				err := response.Unmarshal(responseRecorder.Body.Bytes())
				Expect(err).NotTo(HaveOccurred())

				Expect(response.Error).To(BeNil())
			})
		})

		Context("when the controller returns an unrecoverable error", func() {
			BeforeEach(func() {
				controller.FailTaskReturns(models.NewUnrecoverableError(nil))
			})

			It("logs and writes to the exit channel", func() {
				Eventually(logger).Should(gbytes.Say("unrecoverable-error"))
				Eventually(exitCh).Should(Receive())
			})
		})

		Context("when failing the task fails", func() {
			BeforeEach(func() {
				controller.FailTaskReturns(models.ErrUnknownError)
			})

			It("responds with an error", func() {
				Expect(responseRecorder.Code).To(Equal(http.StatusOK))
				response := &models.TaskLifecycleResponse{}
				err := response.Unmarshal(responseRecorder.Body.Bytes())
				Expect(err).NotTo(HaveOccurred())

				Expect(response.Error).To(Equal(models.ErrUnknownError))
			})
		})
	})

	Describe("CompleteTask", func() {
		var (
			taskGuid      string
			cellId        string
			failed        bool
			failureReason string
			result        string
		)

		BeforeEach(func() {
			taskGuid = "t-guid"
			cellId = "c-id"
			failed = true
			failureReason = "some-error"
			result = "yeah"

			controller.CompleteTaskReturns(nil)

			requestBody = &models.CompleteTaskRequest{
				TaskGuid:      taskGuid,
				CellId:        cellId,
				Failed:        failed,
				FailureReason: failureReason,
				Result:        result,
			}
		})

		JustBeforeEach(func() {
			request := newTestRequest(requestBody)
			handler.CompleteTask(logger, responseRecorder, request)
		})

		Context("when completing the task succeeds", func() {
			It("returns no error", func() {
				Expect(controller.CompleteTaskCallCount()).To(Equal(1))
				_, actualTaskGuid, actualCellId, actualFailed, actualFailureReason, actualResult := controller.CompleteTaskArgsForCall(0)
				Expect(actualTaskGuid).To(Equal(taskGuid))
				Expect(actualCellId).To(Equal(cellId))
				Expect(actualFailed).To(Equal(failed))
				Expect(actualFailureReason).To(Equal(failureReason))
				Expect(actualResult).To(Equal(result))

				Expect(responseRecorder.Code).To(Equal(http.StatusOK))
				response := &models.TaskLifecycleResponse{}
				err := response.Unmarshal(responseRecorder.Body.Bytes())
				Expect(err).NotTo(HaveOccurred())

				Expect(response.Error).To(BeNil())
			})
		})

		Context("when the controller returns an unrecoverable error", func() {
			BeforeEach(func() {
				controller.CompleteTaskReturns(models.NewUnrecoverableError(nil))
			})

			It("logs and writes to the exit channel", func() {
				Eventually(logger).Should(gbytes.Say("unrecoverable-error"))
				Eventually(exitCh).Should(Receive())
			})
		})

		Context("when completing the task fails", func() {
			BeforeEach(func() {
				controller.CompleteTaskReturns(models.ErrUnknownError)
			})

			It("responds with an error", func() {
				Expect(responseRecorder.Code).To(Equal(http.StatusOK))
				response := &models.TaskLifecycleResponse{}
				err := response.Unmarshal(responseRecorder.Body.Bytes())
				Expect(err).NotTo(HaveOccurred())

				Expect(response.Error).To(Equal(models.ErrUnknownError))
			})
		})
	})

	Describe("ResolvingTask", func() {
		Context("when the resolving request is normal", func() {
			BeforeEach(func() {
				requestBody = &models.TaskGuidRequest{
					TaskGuid: "task-guid",
				}
			})

			JustBeforeEach(func() {
				request := newTestRequest(requestBody)
				handler.ResolvingTask(logger, responseRecorder, request)
			})

			Context("when resolvinging the task succeeds", func() {
				It("returns no error", func() {
					Expect(controller.ResolvingTaskCallCount()).To(Equal(1))
					_, taskGuid := controller.ResolvingTaskArgsForCall(0)
					Expect(taskGuid).To(Equal("task-guid"))

					Expect(responseRecorder.Code).To(Equal(http.StatusOK))
					response := &models.TaskLifecycleResponse{}
					err := response.Unmarshal(responseRecorder.Body.Bytes())
					Expect(err).NotTo(HaveOccurred())

					Expect(response.Error).To(BeNil())
				})
			})

			Context("when the controller returns an unrecoverable error", func() {
				BeforeEach(func() {
					controller.ResolvingTaskReturns(models.NewUnrecoverableError(nil))
				})

				It("logs and writes to the exit channel", func() {
					Eventually(logger).Should(gbytes.Say("unrecoverable-error"))
					Eventually(exitCh).Should(Receive())
				})
			})

			Context("when desiring the task fails", func() {
				BeforeEach(func() {
					controller.ResolvingTaskReturns(models.ErrUnknownError)
				})

				It("responds with an error", func() {
					Expect(responseRecorder.Code).To(Equal(http.StatusOK))
					response := &models.TaskLifecycleResponse{}
					err := response.Unmarshal(responseRecorder.Body.Bytes())
					Expect(err).NotTo(HaveOccurred())

					Expect(response.Error).To(Equal(models.ErrUnknownError))
				})
			})
		})
	})

	Describe("DeleteTask", func() {
		Context("when the delete request is normal", func() {
			BeforeEach(func() {
				requestBody = &models.TaskGuidRequest{
					TaskGuid: "task-guid",
				}
			})
			JustBeforeEach(func() {
				request := newTestRequest(requestBody)
				handler.DeleteTask(logger, responseRecorder, request)
			})

			Context("when deleting the task succeeds", func() {
				It("returns no error", func() {
					Expect(controller.DeleteTaskCallCount()).To(Equal(1))
					_, taskGuid := controller.DeleteTaskArgsForCall(0)
					Expect(taskGuid).To(Equal("task-guid"))

					Expect(responseRecorder.Code).To(Equal(http.StatusOK))
					response := &models.TaskLifecycleResponse{}
					err := response.Unmarshal(responseRecorder.Body.Bytes())
					Expect(err).NotTo(HaveOccurred())

					Expect(response.Error).To(BeNil())
				})
			})

			Context("when the controller returns an unrecoverable error", func() {
				BeforeEach(func() {
					controller.DeleteTaskReturns(models.NewUnrecoverableError(nil))
				})

				It("logs and writes to the exit channel", func() {
					Eventually(logger).Should(gbytes.Say("unrecoverable-error"))
					Eventually(exitCh).Should(Receive())
				})
			})

			Context("when desiring the task fails", func() {
				BeforeEach(func() {
					controller.DeleteTaskReturns(models.ErrUnknownError)
				})

				It("responds with an error", func() {
					Expect(responseRecorder.Code).To(Equal(http.StatusOK))
					response := &models.TaskLifecycleResponse{}
					err := response.Unmarshal(responseRecorder.Body.Bytes())
					Expect(err).NotTo(HaveOccurred())

					Expect(response.Error).To(Equal(models.ErrUnknownError))
				})
			})
		})
	})
})
