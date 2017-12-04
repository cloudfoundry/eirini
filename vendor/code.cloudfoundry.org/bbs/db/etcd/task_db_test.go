package etcd_test

import (
	"code.cloudfoundry.org/bbs/models"
	"code.cloudfoundry.org/bbs/models/test/model_helpers"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("TaskDB", func() {
	const (
		taskGuid = "some-guid"
		domain   = "some-domain"
		cellId   = "cell-id"
	)
	var (
		taskDef *models.TaskDefinition
	)

	filterByState := func(state models.Task_State) []*models.Task {
		allTasks, err := etcdDB.Tasks(logger, models.TaskFilter{})
		Expect(err).NotTo(HaveOccurred())
		tasks := []*models.Task{}
		for _, task := range allTasks {
			if task.State == state {
				tasks = append(tasks, task)
			}
		}
		return tasks
	}

	Describe("Tasks", func() {
		Context("when there are tasks", func() {
			var expectedTasks []*models.Task

			BeforeEach(func() {
				task1 := model_helpers.NewValidTask("a-guid")
				task1.Domain = "domain-1"
				task1.CellId = "cell-1"
				task2 := model_helpers.NewValidTask("b-guid")
				task2.Domain = "domain-2"
				task2.CellId = "cell-2"
				expectedTasks = []*models.Task{task1, task2}

				for _, t := range expectedTasks {
					etcdHelper.SetRawTask(t)
				}
			})

			It("returns all the tasks", func() {
				tasks, err := etcdDB.Tasks(logger, models.TaskFilter{})
				Expect(err).NotTo(HaveOccurred())
				Expect(tasks).To(ConsistOf(expectedTasks))
			})

			It("can filter by domain", func() {
				tasks, err := etcdDB.Tasks(logger, models.TaskFilter{Domain: "domain-1"})
				Expect(err).NotTo(HaveOccurred())
				Expect(tasks).To(HaveLen(1))
				Expect(tasks[0]).To(Equal(expectedTasks[0]))
			})

			It("can filter by cell id", func() {
				tasks, err := etcdDB.Tasks(logger, models.TaskFilter{CellID: "cell-2"})
				Expect(err).NotTo(HaveOccurred())
				Expect(tasks).To(HaveLen(1))
				Expect(tasks[0]).To(Equal(expectedTasks[1]))
			})
		})

		Context("when there are no tasks", func() {
			It("returns an empty list", func() {
				tasks, err := etcdDB.Tasks(logger, models.TaskFilter{})
				Expect(err).NotTo(HaveOccurred())
				Expect(tasks).NotTo(BeNil())
				Expect(tasks).To(BeEmpty())
			})
		})

		Context("when there is invalid data", func() {
			BeforeEach(func() {
				etcdHelper.CreateValidTask("some-guid")
				etcdHelper.CreateMalformedTask("some-other-guid")
				etcdHelper.CreateValidTask("some-third-guid")
			})

			It("errors", func() {
				_, err := etcdDB.Tasks(logger, models.TaskFilter{})
				Expect(err).To(HaveOccurred())
			})
		})

		Context("when etcd is not there", func() {
			BeforeEach(func() {
				etcdRunner.Stop()
			})

			AfterEach(func() {
				etcdRunner.Start()
			})

			It("errors", func() {
				_, err := etcdDB.Tasks(logger, models.TaskFilter{})
				Expect(err).To(HaveOccurred())
			})
		})
	})

	Describe("TaskByGuid", func() {
		Context("when there is a task", func() {
			var expectedTask *models.Task

			BeforeEach(func() {
				expectedTask = model_helpers.NewValidTask("task-guid")
				etcdHelper.SetRawTask(expectedTask)
			})

			It("returns the task", func() {
				task, err := etcdDB.TaskByGuid(logger, "task-guid")
				Expect(err).NotTo(HaveOccurred())
				Expect(task).To(Equal(expectedTask))
			})
		})

		Context("when there is no task", func() {
			It("returns a ResourceNotFound", func() {
				_, err := etcdDB.TaskByGuid(logger, "nota-guid")
				Expect(err).To(Equal(models.ErrResourceNotFound))
			})
		})

		Context("when there is invalid data", func() {
			BeforeEach(func() {
				etcdHelper.CreateMalformedTask("some-other-guid")
			})

			It("errors", func() {
				_, err := etcdDB.TaskByGuid(logger, "some-other-guid")
				Expect(err).To(Equal(models.ErrDeserialize))
			})
		})

		Context("when etcd is not there", func() {
			BeforeEach(func() {
				etcdRunner.Stop()
			})

			AfterEach(func() {
				etcdRunner.Start()
			})

			It("errors", func() {
				_, err := etcdDB.TaskByGuid(logger, "some-other-guid")
				Expect(err).To(Equal(models.ErrUnknownError))
			})
		})
	})

	Describe("DesireTask", func() {
		var errDesire error
		var task *models.Task

		JustBeforeEach(func() {
			_, errDesire = etcdDB.DesireTask(logger, taskDef, taskGuid, domain)
		})

		BeforeEach(func() {
			task = model_helpers.NewValidTask(taskGuid)
			taskDef = task.TaskDefinition
		})

		Context("when a task is not already present at the desired key", func() {
			It("does not error", func() {
				Expect(errDesire).NotTo(HaveOccurred())
			})

			It("persists the task", func() {
				persistedTask, err := etcdDB.TaskByGuid(logger, taskGuid)
				Expect(err).NotTo(HaveOccurred())

				Expect(persistedTask.Domain).To(Equal(domain))
				Expect(*persistedTask.TaskDefinition).To(Equal(*taskDef))
			})

			It("provides a CreatedAt time", func() {
				persistedTask, err := etcdDB.TaskByGuid(logger, taskGuid)
				Expect(err).NotTo(HaveOccurred())
				Expect(persistedTask.CreatedAt).To(Equal(clock.Now().UnixNano()))
			})

			It("sets the UpdatedAt time", func() {
				persistedTask, err := etcdDB.TaskByGuid(logger, taskGuid)
				Expect(err).NotTo(HaveOccurred())
				Expect(persistedTask.UpdatedAt).To(Equal(clock.Now().UnixNano()))
			})
		})

		Context("when a task is already present at the desired key", func() {
			const initialDomain = "other-domain"

			BeforeEach(func() {
				_, err := etcdDB.DesireTask(logger, taskDef, taskGuid, initialDomain)
				Expect(err).NotTo(HaveOccurred())
			})

			It("does not persist a second task", func() {
				tasks, err := etcdDB.Tasks(logger, models.TaskFilter{})
				Expect(err).NotTo(HaveOccurred())
				Expect(tasks).To(HaveLen(1))
				Expect(tasks[0].Domain).To(Equal(initialDomain))
			})

			It("returns an error", func() {
				Expect(errDesire).To(Equal(models.ErrResourceExists))
			})
		})
	})

	Describe("StartTask", func() {
		BeforeEach(func() {
			taskDef = model_helpers.NewValidTaskDefinition()
		})

		Context("when starting a pending Task", func() {
			BeforeEach(func() {
				_, err := etcdDB.DesireTask(logger, taskDef, taskGuid, domain)
				Expect(err).NotTo(HaveOccurred())
			})

			It("returns shouldStart as true", func() {
				_, _, started, err := etcdDB.StartTask(logger, taskGuid, cellId)
				Expect(err).NotTo(HaveOccurred())
				Expect(started).To(BeTrue())
			})

			It("correctly updates the task record", func() {
				clock.IncrementBySeconds(1)

				_, _, _, err := etcdDB.StartTask(logger, taskGuid, cellId)
				Expect(err).NotTo(HaveOccurred())

				task, err := etcdDB.TaskByGuid(logger, taskGuid)
				Expect(err).NotTo(HaveOccurred())

				Expect(task.TaskGuid).To(Equal(taskGuid))
				Expect(task.State).To(Equal(models.Task_Running))
				Expect(*task.TaskDefinition).To(Equal(*taskDef))
				Expect(task.UpdatedAt).To(Equal(clock.Now().UnixNano()))
			})
		})

		Context("When starting a Task that is already started", func() {
			BeforeEach(func() {
				_, err := etcdDB.DesireTask(logger, taskDef, taskGuid, "domain")
				Expect(err).NotTo(HaveOccurred())

				_, _, _, err = etcdDB.StartTask(logger, taskGuid, cellId)
				Expect(err).NotTo(HaveOccurred())
			})

			Context("on the same cell", func() {
				It("returns shouldStart as false", func() {
					_, _, changed, err := etcdDB.StartTask(logger, taskGuid, cellId)
					Expect(err).NotTo(HaveOccurred())
					Expect(changed).To(BeFalse())
				})

				It("does not change the Task in the store", func() {
					previousTime := clock.Now().UnixNano()
					clock.IncrementBySeconds(1)

					_, _, _, err := etcdDB.StartTask(logger, taskGuid, cellId)
					Expect(err).NotTo(HaveOccurred())

					task, err := etcdDB.TaskByGuid(logger, taskGuid)
					Expect(err).NotTo(HaveOccurred())

					Expect(task.UpdatedAt).To(Equal(previousTime))
				})
			})

			Context("on another cell", func() {
				It("returns an error", func() {
					_, _, _, err := etcdDB.StartTask(logger, taskGuid, "some-other-cell")
					modelErr := models.ConvertError(err)
					Expect(modelErr).NotTo(BeNil())
					Expect(modelErr.Type).To(Equal(models.Error_InvalidStateTransition))
				})

				It("does not change the Task in the store", func() {
					previousTime := clock.Now().UnixNano()
					clock.IncrementBySeconds(1)

					_, _, _, err := etcdDB.StartTask(logger, taskGuid, cellId)
					Expect(err).NotTo(HaveOccurred())

					task, err := etcdDB.TaskByGuid(logger, taskGuid)
					Expect(err).NotTo(HaveOccurred())

					Expect(task.UpdatedAt).To(Equal(previousTime))
				})
			})
		})
	})

	Describe("CancelTask", func() {
		var (
			cancelError     error
			taskAfterCancel *models.Task
			taskReturned    *models.Task
			cellIDReturned  string
		)

		JustBeforeEach(func() {
			_, taskReturned, cellIDReturned, cancelError = etcdDB.CancelTask(logger, taskGuid)
			taskAfterCancel, _ = etcdDB.TaskByGuid(logger, taskGuid)
		})

		itMarksTaskAsCancelled := func() {
			It("does not error", func() {
				Expect(cancelError).NotTo(HaveOccurred())
			})

			It("returns the right task", func() {
				Expect(taskReturned).To(Equal(taskAfterCancel))
			})

			It("marks the task as completed", func() {
				Expect(taskAfterCancel.State).To(Equal(models.Task_Completed))
			})

			It("marks the task as failed", func() {
				Expect(taskAfterCancel.Failed).To(BeTrue())
			})

			It("sets the failure reason to cancelled", func() {
				Expect(taskAfterCancel.FailureReason).To(Equal("task was cancelled"))
			})

			It("bumps UpdatedAt", func() {
				Expect(taskAfterCancel.UpdatedAt).To(Equal(clock.Now().UnixNano()))
			})
		}

		Context("when the task is in pending state", func() {
			BeforeEach(func() {
				taskDef = model_helpers.NewValidTaskDefinition()
				_, err := etcdDB.DesireTask(logger, taskDef, taskGuid, domain)
				Expect(err).NotTo(HaveOccurred())
			})

			itMarksTaskAsCancelled()

			It("does not return a cellID", func() {
				Expect(cellIDReturned).To(Equal(""))
			})
		})

		Context("when the task is in running state", func() {
			BeforeEach(func() {
				taskDef = model_helpers.NewValidTaskDefinition()
				_, err := etcdDB.DesireTask(logger, taskDef, taskGuid, domain)
				Expect(err).NotTo(HaveOccurred())

				_, _, _, err = etcdDB.StartTask(logger, taskGuid, cellId)
				Expect(err).NotTo(HaveOccurred())
			})

			itMarksTaskAsCancelled()

			It("returns the cellID on which the task was running", func() {
				Expect(cellIDReturned).To(Equal(cellId))
			})
		})

		Context("when the task is in completed state", func() {
			BeforeEach(func() {
				taskDef = model_helpers.NewValidTaskDefinition()
				_, err := etcdDB.DesireTask(logger, taskDef, taskGuid, domain)
				Expect(err).NotTo(HaveOccurred())

				_, _, _, err = etcdDB.StartTask(logger, taskGuid, cellId)
				Expect(err).NotTo(HaveOccurred())

				_, task, err := etcdDB.CompleteTask(logger, taskGuid, cellId, false, "", "")
				Expect(err).NotTo(HaveOccurred())
				Expect(task.TaskGuid).To(Equal(taskGuid))
			})

			It("returns an error", func() {
				Expect(cancelError).To(HaveOccurred())
				Expect(cancelError).To(Equal(models.NewTaskTransitionError(models.Task_Completed, models.Task_Completed)))
			})
		})

		Context("when the task is in resolving state", func() {
			BeforeEach(func() {
				taskDef = model_helpers.NewValidTaskDefinition()
				_, err := etcdDB.DesireTask(logger, taskDef, taskGuid, domain)
				Expect(err).NotTo(HaveOccurred())

				_, _, _, err = etcdDB.StartTask(logger, taskGuid, cellId)
				Expect(err).NotTo(HaveOccurred())

				_, task, err := etcdDB.CompleteTask(logger, taskGuid, cellId, false, "", "")
				Expect(err).NotTo(HaveOccurred())
				Expect(task.TaskGuid).To(Equal(taskGuid))

				_, _, err = etcdDB.ResolvingTask(logger, taskGuid)
				Expect(err).NotTo(HaveOccurred())
			})

			It("returns an error", func() {
				Expect(cancelError).To(HaveOccurred())
				Expect(cancelError).To(Equal(models.NewTaskTransitionError(models.Task_Resolving, models.Task_Completed)))
			})
		})

		Context("when the task does not exist", func() {
			It("returns an error", func() {
				Expect(cancelError).To(HaveOccurred())
				Expect(cancelError).To(Equal(models.ErrResourceNotFound))
			})
		})

		Context("when the store returns some error other than key not found or timeout", func() {
			BeforeEach(func() {
				etcdRunner.Stop()
			})

			AfterEach(func() {
				etcdRunner.Start()
			})

			It("returns an error", func() {
				Expect(cancelError).To(HaveOccurred())
				Expect(cancelError).To(Equal(models.ErrUnknownError))
			})
		})
	})

	Describe("CompleteTask", func() {
		Context("when completing a pending Task", func() {
			JustBeforeEach(func() {
				taskDef = model_helpers.NewValidTaskDefinition()
				_, err := etcdDB.DesireTask(logger, taskDef, taskGuid, domain)
				Expect(err).NotTo(HaveOccurred())
			})

			It("returns an error", func() {
				_, task, err := etcdDB.CompleteTask(logger, taskGuid, cellId, true, "another failure reason", "")
				Expect(err).To(HaveOccurred())
				Expect(task).To(BeNil())
			})
		})

		Context("when completing a running Task", func() {
			BeforeEach(func() {
				taskDef = model_helpers.NewValidTaskDefinition()
			})

			JustBeforeEach(func() {
				_, err := etcdDB.DesireTask(logger, taskDef, taskGuid, domain)
				Expect(err).NotTo(HaveOccurred())

				_, _, _, err = etcdDB.StartTask(logger, taskGuid, cellId)
				Expect(err).NotTo(HaveOccurred())
			})

			Context("when the cell id is not the same", func() {
				It("returns an error", func() {
					_, task, err := etcdDB.CompleteTask(logger, taskGuid, "another-cell", true, "another failure reason", "")
					Expect(err).To(Equal(models.NewRunningOnDifferentCellError("another-cell", cellId)))
					Expect(task).To(BeNil())
				})
			})

			Context("when the cell id is the same", func() {
				It("sets the Task in the completed state", func() {
					clock.IncrementBySeconds(1)

					_, returnedTask, err := etcdDB.CompleteTask(logger, taskGuid, cellId, true, "because i said so", "a result")
					Expect(err).NotTo(HaveOccurred())
					Expect(returnedTask.TaskGuid).To(Equal(taskGuid))

					tasks := filterByState(models.Task_Completed)

					task := tasks[0]
					Expect(task.Failed).To(BeTrue())
					Expect(task.FailureReason).To(Equal("because i said so"))
					Expect(task.UpdatedAt).To(Equal(clock.Now().UnixNano()))
					Expect(task.FirstCompletedAt).To(Equal(clock.Now().UnixNano()))
					Expect(task.CellId).To(BeEmpty())
				})
			})
		})

		Context("When completing a Task that is already completed", func() {
			BeforeEach(func() {
				taskDef = model_helpers.NewValidTaskDefinition()
				_, err := etcdDB.DesireTask(logger, taskDef, taskGuid, domain)
				Expect(err).NotTo(HaveOccurred())

				_, _, _, err = etcdDB.StartTask(logger, taskGuid, cellId)
				Expect(err).NotTo(HaveOccurred())

				_, task, err := etcdDB.CompleteTask(logger, taskGuid, cellId, true, "some failure reason", "")
				Expect(err).NotTo(HaveOccurred())
				Expect(task.TaskGuid).To(Equal(taskGuid))
			})

			It("returns an error", func() {
				_, task, err := etcdDB.CompleteTask(logger, taskGuid, cellId, true, "another failure reason", "")
				Expect(err).To(HaveOccurred())
				Expect(task).To(BeNil())
			})
		})

		Context("When completing a Task that is resolving", func() {
			BeforeEach(func() {
				taskDef = model_helpers.NewValidTaskDefinition()
				_, err := etcdDB.DesireTask(logger, taskDef, taskGuid, domain)
				Expect(err).NotTo(HaveOccurred())

				_, _, _, err = etcdDB.StartTask(logger, taskGuid, cellId)
				Expect(err).NotTo(HaveOccurred())

				_, task, err := etcdDB.CompleteTask(logger, taskGuid, cellId, false, "", "")
				Expect(err).NotTo(HaveOccurred())
				Expect(task.TaskGuid).To(Equal(taskGuid))

				_, _, err = etcdDB.ResolvingTask(logger, taskGuid)
				Expect(err).NotTo(HaveOccurred())
			})

			It("returns an error", func() {
				_, task, err := etcdDB.CompleteTask(logger, taskGuid, cellId, false, "", "")
				Expect(err).To(HaveOccurred())
				Expect(task).To(BeNil())
			})
		})
	})

	Describe("FailTask", func() {
		BeforeEach(func() {
			taskDef = model_helpers.NewValidTaskDefinition()
		})

		Context("when failing a Task", func() {
			Context("when the task is pending", func() {
				JustBeforeEach(func() {
					_, err := etcdDB.DesireTask(logger, taskDef, taskGuid, domain)
					Expect(err).NotTo(HaveOccurred())
				})

				It("sets the Task in the completed state", func() {
					clock.IncrementBySeconds(1)

					_, returnedTask, err := etcdDB.FailTask(logger, taskGuid, "because i said so")
					Expect(err).NotTo(HaveOccurred())
					Expect(returnedTask.TaskGuid).To(Equal(taskGuid))

					tasks := filterByState(models.Task_Completed)

					task := tasks[0]

					Expect(task.Failed).To(BeTrue())
					Expect(task.FailureReason).To(Equal("because i said so"))
					Expect(task.UpdatedAt).To(Equal(clock.Now().UnixNano()))
					Expect(task.FirstCompletedAt).To(Equal(clock.Now().UnixNano()))
				})
			})

			Context("when the task is completed", func() {
				JustBeforeEach(func() {
					taskDef = model_helpers.NewValidTaskDefinition()
					_, err := etcdDB.DesireTask(logger, taskDef, taskGuid, domain)
					Expect(err).NotTo(HaveOccurred())

					_, _, _, err = etcdDB.StartTask(logger, taskGuid, cellId)
					Expect(err).NotTo(HaveOccurred())

					_, task, err := etcdDB.CompleteTask(logger, taskGuid, cellId, true, "some failure reason", "")
					Expect(err).NotTo(HaveOccurred())
					Expect(task.TaskGuid).To(Equal(taskGuid))
				})

				It("fails", func() {
					_, task, err := etcdDB.FailTask(logger,
						taskGuid,
						"because i said so",
					)
					Expect(err).To(HaveOccurred())
					Expect(task).To(BeNil())
				})
			})

			Context("when the task is resolving", func() {
				JustBeforeEach(func() {
					taskDef = model_helpers.NewValidTaskDefinition()
					_, err := etcdDB.DesireTask(logger, taskDef, taskGuid, domain)
					Expect(err).NotTo(HaveOccurred())

					_, _, _, err = etcdDB.StartTask(logger, taskGuid, cellId)
					Expect(err).NotTo(HaveOccurred())

					_, task, err := etcdDB.CompleteTask(logger, taskGuid, cellId, true, "some failure reason", "")
					Expect(err).NotTo(HaveOccurred())
					Expect(task.TaskGuid).To(Equal(taskGuid))

					_, _, err = etcdDB.ResolvingTask(logger, taskGuid)
					Expect(err).NotTo(HaveOccurred())
				})

				It("fails", func() {
					_, task, err := etcdDB.FailTask(logger,
						taskGuid,
						"because i said so",
					)
					Expect(err).To(HaveOccurred())
					Expect(task).To(BeNil())
				})
			})
		})
	})

	Describe("ResolvingTask", func() {
		BeforeEach(func() {
			taskDef = model_helpers.NewValidTaskDefinition()
			_, err := etcdDB.DesireTask(logger, taskDef, taskGuid, domain)
			Expect(err).NotTo(HaveOccurred())

			_, _, _, err = etcdDB.StartTask(logger, taskGuid, cellId)
			Expect(err).NotTo(HaveOccurred())
		})

		Context("when the task is complete", func() {
			BeforeEach(func() {
				_, task, err := etcdDB.CompleteTask(logger, taskGuid, cellId, true, "because i said so", "a result")
				Expect(err).NotTo(HaveOccurred())
				Expect(task.TaskGuid).To(Equal(taskGuid))
			})

			It("swaps /task/<guid>'s state to resolving", func() {
				_, _, err := etcdDB.ResolvingTask(logger, taskGuid)
				Expect(err).NotTo(HaveOccurred())

				tasks := filterByState(models.Task_Resolving)
				Expect(tasks[0].TaskGuid).To(Equal(taskGuid))
				Expect(tasks[0].State).To(Equal(models.Task_Resolving))
			})

			It("bumps UpdatedAt", func() {
				clock.IncrementBySeconds(1)

				_, _, err := etcdDB.ResolvingTask(logger, taskGuid)
				Expect(err).NotTo(HaveOccurred())

				tasks := filterByState(models.Task_Resolving)
				Expect(tasks[0].UpdatedAt).To(Equal(clock.Now().UnixNano()))
			})

			Context("when the Task is already resolving", func() {
				BeforeEach(func() {
					_, _, err := etcdDB.ResolvingTask(logger, taskGuid)
					Expect(err).NotTo(HaveOccurred())
				})

				It("fails", func() {
					_, _, err := etcdDB.ResolvingTask(logger, taskGuid)
					Expect(err).To(HaveOccurred())
				})
			})
		})

		Context("when the task is not complete", func() {
			It("should fail", func() {
				_, _, err := etcdDB.ResolvingTask(logger, taskGuid)
				Expect(err).To(Equal(models.NewTaskTransitionError(models.Task_Running, models.Task_Resolving)))
			})
		})
	})

	Describe("DeleteTask", func() {
		BeforeEach(func() {
			taskDef = model_helpers.NewValidTaskDefinition()
			_, err := etcdDB.DesireTask(logger, taskDef, taskGuid, domain)
			Expect(err).NotTo(HaveOccurred())

			_, _, _, err = etcdDB.StartTask(logger, taskGuid, cellId)
			Expect(err).NotTo(HaveOccurred())

			_, task, err := etcdDB.CompleteTask(logger, taskGuid, cellId, true, "because i said so", "a result")
			Expect(err).NotTo(HaveOccurred())
			Expect(task.TaskGuid).To(Equal(taskGuid))
		})

		Context("when the task is resolving", func() {
			BeforeEach(func() {
				_, _, err := etcdDB.ResolvingTask(logger, taskGuid)
				Expect(err).NotTo(HaveOccurred())
			})

			It("should remove /task/<guid>", func() {
				_, err := etcdDB.DeleteTask(logger, taskGuid)
				Expect(err).NotTo(HaveOccurred())

				tasks, err := etcdDB.Tasks(logger, models.TaskFilter{})
				Expect(err).NotTo(HaveOccurred())
				Expect(tasks).To(BeEmpty())
			})
		})

		Context("when the task is not resolving", func() {
			It("should fail", func() {
				_, err := etcdDB.DeleteTask(logger, taskGuid)
				Expect(err).To(HaveOccurred())
			})
		})
	})
})
