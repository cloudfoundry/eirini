package main_test

import (
	"code.cloudfoundry.org/bbs/cmd/bbs/testrunner"
	"code.cloudfoundry.org/bbs/models"
	. "code.cloudfoundry.org/bbs/models/test/matchers"
	"code.cloudfoundry.org/bbs/models/test/model_helpers"
	"github.com/tedsuo/ifrit/ginkgomon"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Task API", func() {
	var expectedTasks []*models.Task

	BeforeEach(func() {
		bbsRunner = testrunner.New(bbsBinPath, bbsConfig)
		bbsProcess = ginkgomon.Invoke(bbsRunner)
		expectedTasks = []*models.Task{model_helpers.NewValidTask("a-guid"), model_helpers.NewValidTask("b-guid")}
		expectedTasks[1].Domain = "b-domain"
		for _, t := range expectedTasks {
			err := client.DesireTask(logger, t.TaskGuid, t.Domain, t.TaskDefinition)
			Expect(err).NotTo(HaveOccurred())
		}
		client.StartTask(logger, expectedTasks[1].TaskGuid, "b-cell")
	})

	AfterEach(func() {
		ginkgomon.Kill(bbsProcess)
	})

	Describe("Tasks", func() {
		It("has the correct number of responses", func() {
			actualTasks, err := client.Tasks(logger)
			Expect(err).NotTo(HaveOccurred())
			Expect(actualTasks).To(MatchTasks(expectedTasks))
		})
	})

	Describe("TasksByDomain", func() {
		It("has the correct number of responses", func() {
			domain := expectedTasks[0].Domain
			actualTasks, err := client.TasksByDomain(logger, domain)
			Expect(err).NotTo(HaveOccurred())
			Expect(actualTasks).To(MatchTasks([]*models.Task{expectedTasks[0]}))
		})
	})

	Describe("TasksByCellID", func() {
		It("has the correct number of responses", func() {
			actualTasks, err := client.TasksByCellID(logger, "b-cell")
			Expect(err).NotTo(HaveOccurred())
			Expect(actualTasks).To(MatchTasks([]*models.Task{expectedTasks[1]}))
		})
	})

	Describe("TaskByGuid", func() {
		It("returns the task", func() {
			task, err := client.TaskByGuid(logger, expectedTasks[0].TaskGuid)
			Expect(err).NotTo(HaveOccurred())
			Expect(task).To(MatchTask(expectedTasks[0]))
		})
	})

	Describe("TaskWithFilter", func() {
		It("returns the task with filters on domain", func() {
			tasks, err := client.TasksWithFilter(logger, models.TaskFilter{Domain: "b-domain"})
			Expect(err).NotTo(HaveOccurred())
			Expect(len(tasks)).To(Equal(1))
			Expect(tasks[0]).To(MatchTask(expectedTasks[1]))
		})

		It("returns the task with filters on cell-id", func() {
			tasks, err := client.TasksWithFilter(logger, models.TaskFilter{CellID: "b-cell"})
			Expect(err).NotTo(HaveOccurred())
			Expect(len(tasks)).To(Equal(1))
			Expect(tasks[0]).To(MatchTask(expectedTasks[1]))
		})

		It("returns the task with filters on domain and cell-id", func() {
			tasks, err := client.TasksWithFilter(logger, models.TaskFilter{Domain: "b-domain", CellID: "b-cell"})
			Expect(err).NotTo(HaveOccurred())
			Expect(len(tasks)).To(Equal(1))
			Expect(tasks[0]).To(MatchTask(expectedTasks[1]))
		})
	})

	Describe("DesireTask", func() {
		It("adds the desired task", func() {
			expectedTask := model_helpers.NewValidTask("task-1")
			err := client.DesireTask(logger, expectedTask.TaskGuid, expectedTask.Domain, expectedTask.TaskDefinition)
			Expect(err).NotTo(HaveOccurred())

			task, err := client.TaskByGuid(logger, expectedTask.TaskGuid)
			Expect(err).NotTo(HaveOccurred())
			Expect(task).To(MatchTask(expectedTask))
		})
	})

	Describe("Task Lifecycle", func() {
		var taskDef = model_helpers.NewValidTaskDefinition()
		const taskGuid = "task-1"
		const cellId = "cell-1"

		BeforeEach(func() {
			err := client.DesireTask(logger, taskGuid, "test", taskDef)
			Expect(err).NotTo(HaveOccurred())
		})

		Describe("StartTask", func() {
			It("changes the task state from pending to running", func() {
				task, err := client.TaskByGuid(logger, taskGuid)
				Expect(err).NotTo(HaveOccurred())
				Expect(task.State).To(Equal(models.Task_Pending))

				_, err = client.StartTask(logger, taskGuid, cellId)
				Expect(err).NotTo(HaveOccurred())

				task, err = client.TaskByGuid(logger, taskGuid)
				Expect(err).NotTo(HaveOccurred())
				Expect(task.State).To(Equal(models.Task_Running))
			})

			It("shouldStart is true", func() {
				shouldStart, err := client.StartTask(logger, taskGuid, cellId)
				Expect(err).NotTo(HaveOccurred())
				Expect(shouldStart).To(BeTrue())
			})
		})

		Describe("CancelTask", func() {
			It("cancel the desired task", func() {
				err := client.CancelTask(logger, taskGuid)
				Expect(err).NotTo(HaveOccurred())

				task, err := client.TaskByGuid(logger, taskGuid)
				Expect(err).NotTo(HaveOccurred())
				Expect(task.FailureReason).To(Equal("task was cancelled"))
			})
		})

		Context("task has been started", func() {
			BeforeEach(func() {
				_, err := client.StartTask(logger, taskGuid, cellId)
				Expect(err).NotTo(HaveOccurred())
			})

			Describe("FailTask", func() {
				It("marks the task completed and sets FailureReason", func() {
					err := client.FailTask(logger, taskGuid, "some failure happened")
					Expect(err).NotTo(HaveOccurred())

					task, err := client.TaskByGuid(logger, taskGuid)
					Expect(err).NotTo(HaveOccurred())
					Expect(task.State).To(Equal(models.Task_Completed))
					Expect(task.FailureReason).To(Equal("some failure happened"))
				})
			})

			Describe("CompleteTask", func() {
				It("changes the task state from running to completed", func() {
					task, err := client.TaskByGuid(logger, taskGuid)
					Expect(err).NotTo(HaveOccurred())
					Expect(task.State).To(Equal(models.Task_Running))

					err = client.CompleteTask(logger, taskGuid, cellId, false, "", "result")
					Expect(err).NotTo(HaveOccurred())

					task, err = client.TaskByGuid(logger, taskGuid)
					Expect(err).NotTo(HaveOccurred())
					Expect(task.State).To(Equal(models.Task_Completed))
				})
			})

			Context("task has been completed", func() {
				BeforeEach(func() {
					err := client.CompleteTask(logger, taskGuid, cellId, false, "", "result")
					Expect(err).NotTo(HaveOccurred())
				})

				Describe("ResolvingTask", func() {
					It("changes the task state from completed to resolving", func() {
						err := client.ResolvingTask(logger, taskGuid)
						Expect(err).NotTo(HaveOccurred())

						task, err := client.TaskByGuid(logger, taskGuid)
						Expect(err).NotTo(HaveOccurred())
						Expect(task.State).To(Equal(models.Task_Resolving))
					})
				})

				Context("task is resolving", func() {
					BeforeEach(func() {
						err := client.ResolvingTask(logger, taskGuid)
						Expect(err).NotTo(HaveOccurred())
					})

					Describe("DeleteTask", func() {
						It("deletes the task", func() {
							err := client.DeleteTask(logger, taskGuid)
							Expect(err).NotTo(HaveOccurred())

							_, err = client.TaskByGuid(logger, taskGuid)
							Expect(err).To(Equal(models.ErrResourceNotFound))
						})
					})
				})
			})
		})
	})
})
