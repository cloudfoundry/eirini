package handlers_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"time"

	"code.cloudfoundry.org/bbs/format"
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

		task1 models.Task
		task2 models.Task

		requestBody interface{}
	)

	BeforeEach(func() {
		logger = lagertest.NewTestLogger("test")
		responseRecorder = httptest.NewRecorder()
		exitCh = make(chan struct{}, 1)
		controller = &fake_controllers.FakeTaskController{}
		handler = handlers.NewTaskHandler(controller, exitCh)
	})

	Describe("DesireTask", func() {
		var (
			taskGuid   = "task-guid"
			domain     = "domain"
			oldTaskDef *models.TaskDefinition
		)

		BeforeEach(func() {
			config, err := json.Marshal(map[string]string{"foo": "bar"})
			Expect(err).NotTo(HaveOccurred())

			oldTaskDef = model_helpers.NewValidTaskDefinition()
			oldTaskDef.VolumeMounts = []*models.VolumeMount{{
				Driver:             "my-driver",
				ContainerDir:       "/mnt/mypath",
				DeprecatedMode:     models.DeprecatedBindMountMode_RO,
				DeprecatedConfig:   config,
				DeprecatedVolumeId: "my-volume",
			}}

			requestBody = &models.DesireTaskRequest{
				TaskGuid:       taskGuid,
				Domain:         domain,
				TaskDefinition: oldTaskDef,
			}

		})

		JustBeforeEach(func() {
			request := newTestRequest(requestBody)
			handler.DesireTask_r1(logger, responseRecorder, request)
		})

		Context("when the desire is successful", func() {
			It("upconverts the deprecated volume mounts", func() {
				expectedTaskDef := model_helpers.NewValidTaskDefinition()

				Expect(controller.DesireTaskCallCount()).To(Equal(1))
				_, actualTaskDef, _, _ := controller.DesireTaskArgsForCall(0)
				Expect(actualTaskDef.VolumeMounts).To(Equal(expectedTaskDef.VolumeMounts))
				Expect(actualTaskDef).To(Equal(expectedTaskDef))

				Expect(responseRecorder.Code).To(Equal(http.StatusOK))
				response := &models.TaskLifecycleResponse{}
				err := response.Unmarshal(responseRecorder.Body.Bytes())
				Expect(err).NotTo(HaveOccurred())

				Expect(response.Error).To(BeNil())
			})
		})
	})

	Describe("Tasks_r0", func() {
		BeforeEach(func() {
			task1 = models.Task{Domain: "domain-1"}
			task2 = models.Task{CellId: "cell-id"}
			requestBody = &models.TasksRequest{}
		})

		JustBeforeEach(func() {
			request := newTestRequest(requestBody)
			handler.Tasks_r0(logger, responseRecorder, request)
		})

		Context("when reading tasks from DB succeeds", func() {
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

			It("calls the DB with no filter", func() {
				Expect(controller.TasksCallCount()).To(Equal(1))
				_, domain, cellID := controller.TasksArgsForCall(0)
				Expect(domain).To(Equal(""))
				Expect(cellID).To(Equal(""))
			})

			Context("and filtering by domain", func() {
				BeforeEach(func() {
					requestBody = &models.TasksRequest{
						Domain: "domain-1",
					}
				})

				It("calls the DB with a domain filter", func() {
					Expect(controller.TasksCallCount()).To(Equal(1))
					_, domain, cellID := controller.TasksArgsForCall(0)
					Expect(domain).To(Equal("domain-1"))
					Expect(cellID).To(Equal(""))
				})
			})

			Context("and filtering by cell id", func() {
				BeforeEach(func() {
					requestBody = &models.TasksRequest{
						CellId: "cell-id",
					}
				})

				It("calls the DB with a cell filter", func() {
					Expect(controller.TasksCallCount()).To(Equal(1))
					_, domain, cellID := controller.TasksArgsForCall(0)
					Expect(domain).To(Equal(""))
					Expect(cellID).To(Equal("cell-id"))
				})
			})

			Context("and the returned tasks have cache dependencies", func() {
				BeforeEach(func() {
					task1.TaskDefinition = &models.TaskDefinition{}
					task2.TaskDefinition = &models.TaskDefinition{}

					task1.Action = &models.Action{
						UploadAction: &models.UploadAction{
							From: "web_location",
						},
					}

					task1.CachedDependencies = []*models.CachedDependency{
						{Name: "name-1", From: "from-1", To: "to-1", CacheKey: "cache-key-1", LogSource: "log-source-1"},
					}

					task2.CachedDependencies = []*models.CachedDependency{
						{Name: "name-2", From: "from-2", To: "to-2", CacheKey: "cache-key-2", LogSource: "log-source-2"},
						{Name: "name-3", From: "from-3", To: "to-3", CacheKey: "cache-key-3", LogSource: "log-source-3"},
					}
				})

				It("translates the cache dependencies into download actions", func() {
					Expect(responseRecorder.Code).To(Equal(http.StatusOK))
					response := models.TasksResponse{}
					err := response.Unmarshal(responseRecorder.Body.Bytes())
					Expect(err).NotTo(HaveOccurred())

					Expect(response.Error).To(BeNil())
					Expect(response.Tasks).To(HaveLen(2))
					Expect(response.Tasks[0]).To(Equal(task1.VersionDownTo(format.V0)))
					Expect(response.Tasks[1]).To(Equal(task2.VersionDownTo(format.V0)))
				})
			})
		})

		Context("when the DB returns an unrecoverable error", func() {
			BeforeEach(func() {
				controller.TasksReturns(nil, models.NewUnrecoverableError(nil))
			})

			It("logs and writes to the exit channel", func() {
				Eventually(logger).Should(gbytes.Say("unrecoverable-error"))
				Eventually(exitCh).Should(Receive())
			})
		})

		Context("when the DB errors out", func() {
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

	Describe("Tasks_r1", func() {
		BeforeEach(func() {
			task1 = models.Task{Domain: "domain-1"}
			task2 = models.Task{CellId: "cell-id"}
			requestBody = &models.TasksRequest{}
		})

		JustBeforeEach(func() {
			request := newTestRequest(requestBody)
			handler.Tasks_r1(logger, responseRecorder, request)
		})

		Context("when reading tasks from DB succeeds", func() {
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

			It("calls the DB with no filter", func() {
				Expect(controller.TasksCallCount()).To(Equal(1))
				_, domain, cellID := controller.TasksArgsForCall(0)
				Expect(domain).To(Equal(""))
				Expect(cellID).To(Equal(""))
			})

			Context("and filtering by domain", func() {
				BeforeEach(func() {
					requestBody = &models.TasksRequest{
						Domain: "domain-1",
					}
				})

				It("calls the DB with a domain filter", func() {
					Expect(controller.TasksCallCount()).To(Equal(1))
					_, domain, cellID := controller.TasksArgsForCall(0)
					Expect(domain).To(Equal("domain-1"))
					Expect(cellID).To(Equal(""))
				})
			})

			Context("and filtering by cell id", func() {
				BeforeEach(func() {
					requestBody = &models.TasksRequest{
						CellId: "cell-id",
					}
				})

				It("calls the DB with a cell filter", func() {
					Expect(controller.TasksCallCount()).To(Equal(1))
					_, domain, cellID := controller.TasksArgsForCall(0)
					Expect(domain).To(Equal(""))
					Expect(cellID).To(Equal("cell-id"))
				})
			})

			Context("and the tasks have timeout not timeout_ms", func() {
				BeforeEach(func() {
					task1.TaskDefinition = &models.TaskDefinition{}
					task2.TaskDefinition = &models.TaskDefinition{}

					task1.Action = &models.Action{
						TimeoutAction: &models.TimeoutAction{
							Action: models.WrapAction(&models.UploadAction{
								From: "web_location",
							}),
							TimeoutMs: 10000,
						},
					}
				})

				It("translates the timeoutMs to timeout", func() {
					Expect(responseRecorder.Code).To(Equal(http.StatusOK))
					response := models.TasksResponse{}
					err := response.Unmarshal(responseRecorder.Body.Bytes())
					Expect(err).NotTo(HaveOccurred())

					Expect(response.Error).To(BeNil())
					Expect(response.Tasks).To(HaveLen(2))
					Expect(response.Tasks[0]).To(Equal(task1.VersionDownTo(format.V1)))
					Expect(response.Tasks[1]).To(Equal(task2.VersionDownTo(format.V1)))
				})
			})
		})

		Context("when the DB returns an unrecoverable error", func() {
			BeforeEach(func() {
				controller.TasksReturns(nil, models.NewUnrecoverableError(nil))
			})

			It("logs and writes to the exit channel", func() {
				Eventually(logger).Should(gbytes.Say("unrecoverable-error"))
				Eventually(exitCh).Should(Receive())
			})
		})

		Context("when the DB errors out", func() {
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

	Describe("TaskByGuid_r0", func() {
		var taskGuid = "task-guid"

		BeforeEach(func() {
			requestBody = &models.TaskByGuidRequest{
				TaskGuid: taskGuid,
			}
		})

		JustBeforeEach(func() {
			request := newTestRequest(requestBody)
			handler.TaskByGuid_r0(logger, responseRecorder, request)
		})

		Context("when reading a task from the DB succeeds", func() {
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

			Context("when the task has cache dependencies", func() {
				BeforeEach(func() {
					task.TaskDefinition = &models.TaskDefinition{}
					task.CachedDependencies = []*models.CachedDependency{
						{Name: "name-2", From: "from-2", To: "to-2", CacheKey: "cache-key-2", LogSource: "log-source-2"},
						{Name: "name-3", From: "from-3", To: "to-3", CacheKey: "cache-key-3", LogSource: "log-source-3"},
					}
				})

				It("moves them to the actions", func() {
					Expect(responseRecorder.Code).To(Equal(http.StatusOK))
					response := models.TaskResponse{}
					err := response.Unmarshal(responseRecorder.Body.Bytes())
					Expect(err).NotTo(HaveOccurred())

					Expect(response.Error).To(BeNil())
					Expect(response.Task).To(Equal(task.VersionDownTo(format.V0)))
				})
			})

			Context("and the tasks have timeout not timeout_ms", func() {
				BeforeEach(func() {
					task.TaskDefinition = &models.TaskDefinition{}

					task.Action = &models.Action{
						TimeoutAction: &models.TimeoutAction{
							Action: models.WrapAction(&models.UploadAction{
								From: "web_location",
							}),
							TimeoutMs: 10000,
						},
					}
				})

				It("translates the timeoutMs to timeout", func() {
					Expect(responseRecorder.Code).To(Equal(http.StatusOK))
					response := models.TaskResponse{}
					err := response.Unmarshal(responseRecorder.Body.Bytes())
					Expect(err).NotTo(HaveOccurred())

					Expect(response.Error).To(BeNil())
					Expect(response.Task).To(Equal(task.VersionDownTo(format.V1)))
				})
			})
		})

		Context("when the DB returns an unrecoverable error", func() {
			BeforeEach(func() {
				controller.TaskByGuidReturns(nil, models.NewUnrecoverableError(nil))
			})

			It("logs and writes to the exit channel", func() {
				Eventually(logger).Should(gbytes.Say("unrecoverable-error"))
				Eventually(exitCh).Should(Receive())
			})
		})

		Context("when the DB returns no task", func() {
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

		Context("when the DB errors out", func() {
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

	Describe("TaskByGuid_r1", func() {
		var taskGuid = "task-guid"

		BeforeEach(func() {
			requestBody = &models.TaskByGuidRequest{
				TaskGuid: taskGuid,
			}
		})

		JustBeforeEach(func() {
			request := newTestRequest(requestBody)
			handler.TaskByGuid_r1(logger, responseRecorder, request)
		})

		Context("when reading a task from the DB succeeds", func() {
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

			Context("when the task has cache dependencies", func() {
				BeforeEach(func() {
					task.TaskDefinition = &models.TaskDefinition{}
					task.CachedDependencies = []*models.CachedDependency{
						{Name: "name-2", From: "from-2", To: "to-2", CacheKey: "cache-key-2", LogSource: "log-source-2"},
						{Name: "name-3", From: "from-3", To: "to-3", CacheKey: "cache-key-3", LogSource: "log-source-3"},
					}
				})

				It("moves them to the actions", func() {
					Expect(responseRecorder.Code).To(Equal(http.StatusOK))
					response := models.TaskResponse{}
					err := response.Unmarshal(responseRecorder.Body.Bytes())
					Expect(err).NotTo(HaveOccurred())

					Expect(response.Error).To(BeNil())
					Expect(response.Task).To(Equal(task.VersionDownTo(format.V1)))
				})
			})

			Context("and the tasks have timeout not timeout_ms", func() {
				BeforeEach(func() {
					task.TaskDefinition = &models.TaskDefinition{}

					task.Action = &models.Action{
						TimeoutAction: &models.TimeoutAction{
							Action: models.WrapAction(&models.UploadAction{
								From: "web_location",
							}),
							TimeoutMs: 10000,
						},
					}
				})

				It("translates the timeoutMs to timeout", func() {
					Expect(responseRecorder.Code).To(Equal(http.StatusOK))
					response := models.TaskResponse{}
					err := response.Unmarshal(responseRecorder.Body.Bytes())
					Expect(err).NotTo(HaveOccurred())

					Expect(response.Error).To(BeNil())
					Expect(response.Task).To(Equal(task.VersionDownTo(format.V1)))
				})
			})
		})

		Context("when the DB returns an unrecoverable error", func() {
			BeforeEach(func() {
				controller.TaskByGuidReturns(nil, models.NewUnrecoverableError(nil))
			})

			It("logs and writes to the exit channel", func() {
				Eventually(logger).Should(gbytes.Say("unrecoverable-error"))
				Eventually(exitCh).Should(Receive())
			})
		})

		Context("when the DB returns no task", func() {
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

		Context("when the DB errors out", func() {
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

	Describe("DesireTask_r0", func() {
		var (
			taskGuid = "task-guid"
			domain   = "domain"
			taskDef  *models.TaskDefinition
		)

		BeforeEach(func() {
			taskDef = model_helpers.NewValidTaskDefinition()
			taskDef.Action = &models.Action{
				TimeoutAction: &models.TimeoutAction{
					Action: models.WrapAction(&models.UploadAction{
						From: "web_location",
						To:   "potato",
						User: "face",
					}),
					DeprecatedTimeoutNs: int64(time.Second),
				},
			}
			requestBody = &models.DesireTaskRequest{
				TaskGuid:       taskGuid,
				Domain:         domain,
				TaskDefinition: taskDef,
			}
		})

		JustBeforeEach(func() {
			request := newTestRequest(requestBody)
			handler.DesireTask_r0(logger, responseRecorder, request)
		})

		Context("when the desire is successful", func() {
			BeforeEach(func() {
				controller.DesireTaskReturns(nil)
			})

			It("desires the task with the requested definitions", func() {
				Expect(controller.DesireTaskCallCount()).To(Equal(1))
				_, actualTaskDef, actualTaskGuid, actualDomain := controller.DesireTaskArgsForCall(0)
				taskDef.Action = &models.Action{
					TimeoutAction: &models.TimeoutAction{
						Action: models.WrapAction(&models.UploadAction{
							From: "web_location",
							To:   "potato",
							User: "face",
						}),
						DeprecatedTimeoutNs: int64(time.Second),
						TimeoutMs:           1000,
					},
				}
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

		Context("when the DB returns an unrecoverable error", func() {
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
})
