package etcd_test

import (
	"time"

	"code.cloudfoundry.org/auctioneer"
	"code.cloudfoundry.org/bbs/models"
	"code.cloudfoundry.org/bbs/models/test/model_helpers"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

const TTL = 0

var _ = Describe("Convergence of Tasks", func() {
	var (
		kickTasksDurationInSeconds, expirePendingTaskDurationInSeconds            uint64
		kickTasksDuration, expirePendingTaskDuration, expireCompletedTaskDuration time.Duration
	)

	BeforeEach(func() {
		kickTasksDurationInSeconds = 10
		kickTasksDuration = time.Duration(kickTasksDurationInSeconds) * time.Second
		expirePendingTaskDurationInSeconds = 30
		expirePendingTaskDuration = time.Duration(expirePendingTaskDurationInSeconds) * time.Second
		expireCompletedTaskDuration = time.Hour
	})

	Describe("ConvergeTasks", func() {
		const (
			taskGuid  = "some-guid"
			taskGuid2 = "some-other-guid"
			domain    = "some-domain"
			cellId    = "cell-id"
		)

		var (
			tasksToAuction  []*auctioneer.TaskStartRequest
			tasksToComplete []*models.Task
			cells           models.CellSet
		)

		BeforeEach(func() {
			cells = models.CellSet{}
		})

		JustBeforeEach(func() {
			tasksToAuction, tasksToComplete, _ = etcdDB.ConvergeTasks(logger, cells, kickTasksDuration, expirePendingTaskDuration, expireCompletedTaskDuration)
		})

		It("bumps the convergence counter", func() {
			Expect(fakeMetronClient.IncrementCounterCallCount()).To(Equal(1))
			name := fakeMetronClient.IncrementCounterArgsForCall(0)
			Expect(name).To(Equal("ConvergenceTaskRuns"))
		})

		It("reports the duration that it took to converge", func() {
			Expect(fakeMetronClient.SendDurationCallCount()).To(Equal(1))
			name, value := fakeMetronClient.SendDurationArgsForCall(0)
			Expect(name).To(Equal("ConvergenceTaskDuration"))
			Expect(value).NotTo(BeZero())
		})

		It("emits -1 metrics", func() {
			Expect(fakeMetronClient.SendMetricCallCount()).To(Equal(4))
			name, value := fakeMetronClient.SendMetricArgsForCall(0)
			Expect(name).To(Equal("TasksPending"))
			Expect(value).To(Equal(-1))

			name, value = fakeMetronClient.SendMetricArgsForCall(1)
			Expect(name).To(Equal("TasksRunning"))
			Expect(value).To(Equal(-1))

			name, value = fakeMetronClient.SendMetricArgsForCall(2)
			Expect(name).To(Equal("TasksCompleted"))
			Expect(value).To(Equal(-1))

			name, value = fakeMetronClient.SendMetricArgsForCall(3)
			Expect(name).To(Equal("TasksResolving"))
			Expect(value).To(Equal(-1))
		})

		Context("when a Task is malformed", func() {
			BeforeEach(func() {
				etcdHelper.CreateMalformedTask(taskGuid)
			})

			It("should delete it", func() {
				_, modelErr := etcdDB.TaskByGuid(logger, taskGuid)
				Expect(modelErr).To(BeEquivalentTo(models.ErrResourceNotFound))
			})

			It("bumps the pruned counter", func() {
				Expect(fakeMetronClient.IncrementCounterWithDeltaCallCount()).To(Equal(2))
				name, value := fakeMetronClient.IncrementCounterWithDeltaArgsForCall(1)

				Expect(name).To(Equal("ConvergenceTasksPruned"))
				Expect(value).To(Equal(uint64(1)))
			})
		})

		Context("when a Task is invalid", func() {
			BeforeEach(func() {
				task := model_helpers.NewValidTask(taskGuid)
				task.Domain = ""
				etcdHelper.SetRawTask(task)
			})

			It("should delete it", func() {
				_, modelErr := etcdDB.TaskByGuid(logger, taskGuid)
				Expect(modelErr).To(BeEquivalentTo(models.ErrResourceNotFound))
			})

			It("bumps the pruned counter", func() {
				Expect(fakeMetronClient.IncrementCounterWithDeltaCallCount()).To(Equal(2))
				name, value := fakeMetronClient.IncrementCounterWithDeltaArgsForCall(1)

				Expect(name).To(Equal("ConvergenceTasksPruned"))
				Expect(value).To(Equal(uint64(1)))
			})
		})

		Context("when Tasks are pending", func() {
			BeforeEach(func() {
				expectedTasks := []*models.Task{
					model_helpers.NewValidTask(taskGuid), model_helpers.NewValidTask(taskGuid2),
				}

				for _, t := range expectedTasks {
					t.CreatedAt = clock.Now().UnixNano()
					t.UpdatedAt = clock.Now().UnixNano()
					t.FirstCompletedAt = 0
					etcdHelper.SetRawTask(t)
				}
			})

			It("emits a pending metric", func() {
				Expect(fakeMetronClient.SendMetricCallCount()).To(Equal(4))
				name, value := fakeMetronClient.SendMetricArgsForCall(0)
				Expect(name).To(Equal("TasksPending"))
				Expect(value).To(Equal(2))
			})

			Context("when the Task has NOT been pending for too long", func() {
				BeforeEach(func() {
					clock.IncrementBySeconds(kickTasksDurationInSeconds - 1)
				})

				It("returns no tasks to auction", func() {
					Expect(tasksToAuction).To(BeEmpty())
				})
			})

			Context("when the Tasks have been pending for longer than the kick interval", func() {
				BeforeEach(func() {
					clock.IncrementBySeconds(kickTasksDurationInSeconds + 1)
				})

				It("bumps the compare-and-swap counter", func() {
					Expect(fakeMetronClient.IncrementCounterWithDeltaCallCount()).To(Equal(2))
					name, value := fakeMetronClient.IncrementCounterWithDeltaArgsForCall(0)

					Expect(name).To(Equal("ConvergenceTasksKicked"))
					Expect(value).To(Equal(uint64(2)))
				})

				It("returns the tasks to be auctioned", func() {
					Expect(tasksToAuction).To(HaveLen(2))
					Expect([]string{tasksToAuction[0].TaskGuid, tasksToAuction[1].TaskGuid}).To(ConsistOf(taskGuid, taskGuid2))
				})
			})

			Context("when the Task has been pending for longer than the expirePendingTasksDuration", func() {
				BeforeEach(func() {
					clock.IncrementBySeconds(expirePendingTaskDurationInSeconds + 1)
				})

				It("should mark the Task as completed & failed", func() {
					returnedTask, err := etcdDB.TaskByGuid(logger, taskGuid)
					Expect(err).NotTo(HaveOccurred())
					Expect(returnedTask.State).To(Equal(models.Task_Completed))

					Expect(returnedTask.Failed).To(Equal(true))
					Expect(returnedTask.FailureReason).To(ContainSubstring("time limit"))
				})

				It("bumps the compare-and-swap counter", func() {
					Expect(fakeMetronClient.IncrementCounterWithDeltaCallCount()).To(Equal(2))
					name, value := fakeMetronClient.IncrementCounterWithDeltaArgsForCall(0)

					Expect(name).To(Equal("ConvergenceTasksKicked"))
					Expect(value).To(Equal(uint64(2)))
				})
			})
		})

		Context("when a Task is running", func() {
			BeforeEach(func() {
				_, err := etcdDB.DesireTask(logger, model_helpers.NewValidTaskDefinition(), taskGuid, domain)
				Expect(err).NotTo(HaveOccurred())

				_, _, _, err = etcdDB.StartTask(logger, taskGuid, "cell-id")
				Expect(err).NotTo(HaveOccurred())
			})

			It("emits a running metric", func() {
				Expect(fakeMetronClient.SendMetricCallCount()).To(Equal(4))
				name, value := fakeMetronClient.SendMetricArgsForCall(1)
				Expect(name).To(Equal("TasksRunning"))
				Expect(value).To(Equal(1))
			})

			Context("when the associated cell is present", func() {
				BeforeEach(func() {
					cellPresence := models.NewCellPresence(
						"cell-id",
						"1.2.3.4",
						"",
						"the-zone",
						models.NewCellCapacity(128, 1024, 3),
						[]string{},
						[]string{},
						[]string{},
						[]string{},
					)
					cells["cell-id"] = &cellPresence
				})

				It("leaves the task running", func() {
					returnedTask, err := etcdDB.TaskByGuid(logger, taskGuid)
					Expect(err).NotTo(HaveOccurred())
					Expect(returnedTask.State).To(Equal(models.Task_Running))
				})
			})

			Context("when the associated cell is missing", func() {
				It("should mark the Task as completed & failed", func() {
					returnedTask, err := etcdDB.TaskByGuid(logger, taskGuid)
					Expect(err).NotTo(HaveOccurred())
					Expect(returnedTask.State).To(Equal(models.Task_Completed))

					Expect(returnedTask.Failed).To(Equal(true))
					Expect(returnedTask.FailureReason).To(ContainSubstring("cell"))
				})

				It("bumps the compare-and-swap counter", func() {
					Expect(fakeMetronClient.IncrementCounterWithDeltaCallCount()).To(Equal(2))
					name, value := fakeMetronClient.IncrementCounterWithDeltaArgsForCall(0)

					Expect(name).To(Equal("ConvergenceTasksKicked"))
					Expect(value).To(Equal(uint64(1)))
				})
			})
		})

		Describe("Completed tasks", func() {
			Context("when Tasks with a complete URL are completed", func() {
				BeforeEach(func() {
					taskDef := model_helpers.NewValidTaskDefinition()
					taskDef.CompletionCallbackUrl = "blah"

					_, err := etcdDB.DesireTask(logger, taskDef, taskGuid, domain)
					Expect(err).NotTo(HaveOccurred())

					_, _, _, err = etcdDB.StartTask(logger, taskGuid, cellId)
					Expect(err).NotTo(HaveOccurred())

					_, task, err := etcdDB.CompleteTask(logger, taskGuid, cellId, true, "'cause I said so", "a magical result")
					Expect(err).NotTo(HaveOccurred())
					Expect(task.TaskGuid).To(Equal(taskGuid))

					_, err = etcdDB.DesireTask(logger, taskDef, taskGuid2, domain)

					_, _, _, err = etcdDB.StartTask(logger, taskGuid2, cellId)
					Expect(err).NotTo(HaveOccurred())

					_, task, err = etcdDB.CompleteTask(logger, taskGuid2, cellId, true, "'cause I said so", "a magical result")
					Expect(err).NotTo(HaveOccurred())
					Expect(task.TaskGuid).To(Equal(taskGuid2))
				})

				It("emits a completed metric", func() {
					name, value := fakeMetronClient.SendMetricArgsForCall(2)
					Expect(name).To(Equal("TasksCompleted"))
					Expect(value).To(Equal(2))
				})

				Context("for longer than the convergence interval", func() {
					BeforeEach(func() {
						clock.IncrementBySeconds(expirePendingTaskDurationInSeconds + 1)
					})

					It("returns the tasks to be completed", func() {
						Expect(tasksToComplete).To(HaveLen(2))
						Expect([]string{tasksToComplete[0].TaskGuid, tasksToComplete[1].TaskGuid}).To(ConsistOf(taskGuid, taskGuid2))
					})

					It("bumps the compare-and-swap counter", func() {
						Expect(fakeMetronClient.IncrementCounterWithDeltaCallCount()).To(Equal(2))
						name, value := fakeMetronClient.IncrementCounterWithDeltaArgsForCall(0)

						Expect(name).To(Equal("ConvergenceTasksKicked"))
						Expect(value).To(Equal(uint64(2)))
					})
				})

				Context("when the task has been completed for longer than the time-to-resolve interval", func() {
					BeforeEach(func() {
						clock.IncrementBySeconds(uint64(expireCompletedTaskDuration.Seconds()) + 1)
					})

					It("should delete the task", func() {
						_, modelErr := etcdDB.TaskByGuid(logger, taskGuid)
						Expect(modelErr).To(Equal(models.ErrResourceNotFound))
					})
				})

				Context("when the task has been completed for less than the convergence interval", func() {
					var previousTime int64

					BeforeEach(func() {
						previousTime = clock.Now().UnixNano()
						clock.IncrementBySeconds(1)
					})

					It("should NOT kick the Task", func() {
						returnedTask, err := etcdDB.TaskByGuid(logger, taskGuid)
						Expect(err).NotTo(HaveOccurred())
						Expect(returnedTask.State).To(Equal(models.Task_Completed))
						Expect(returnedTask.UpdatedAt).To(Equal(previousTime))
					})
				})
			})
		})

		Context("when a Task is resolving", func() {
			BeforeEach(func() {
				taskDef := model_helpers.NewValidTaskDefinition()
				taskDef.CompletionCallbackUrl = "blah"

				_, err := etcdDB.DesireTask(logger, taskDef, taskGuid, domain)
				Expect(err).NotTo(HaveOccurred())

				_, _, _, err = etcdDB.StartTask(logger, taskGuid, cellId)
				Expect(err).NotTo(HaveOccurred())

				_, task, err := etcdDB.CompleteTask(logger, taskGuid, cellId, true, "'cause I said so", "a magical result")
				Expect(err).NotTo(HaveOccurred())
				Expect(task.TaskGuid).To(Equal(taskGuid))

				_, _, err = etcdDB.ResolvingTask(logger, taskGuid)
				Expect(err).NotTo(HaveOccurred())
			})

			It("emits a resolving metric", func() {
				Expect(fakeMetronClient.SendMetricCallCount()).To(Equal(4))
				name, value := fakeMetronClient.SendMetricArgsForCall(3)
				Expect(name).To(Equal("TasksResolving"))
				Expect(value).To(Equal(1))
			})

			Context("when the task is in resolving state for less than the convergence interval", func() {
				BeforeEach(func() {
					clock.IncrementBySeconds(1)
				})

				It("should return no tasks to complete", func() {
					Expect(tasksToComplete).To(BeEmpty())
				})
			})

			Context("when the task has been resolving for longer than a convergence interval", func() {
				BeforeEach(func() {
					clock.IncrementBySeconds(expirePendingTaskDurationInSeconds)
				})

				It("should put the Task back into the completed state", func() {
					returnedTask, err := etcdDB.TaskByGuid(logger, taskGuid)
					Expect(err).NotTo(HaveOccurred())
					Expect(returnedTask.State).To(Equal(models.Task_Completed))
					Expect(returnedTask.UpdatedAt).To(Equal(clock.Now().UnixNano()))
				})

				It("returns the task to complete", func() {
					Expect(tasksToComplete).To(HaveLen(1))
					Expect(tasksToComplete[0].TaskGuid).To(Equal(taskGuid))
				})

				It("bumps the compare-and-swap counter", func() {
					Expect(fakeMetronClient.IncrementCounterWithDeltaCallCount()).To(Equal(2))
					name, value := fakeMetronClient.IncrementCounterWithDeltaArgsForCall(0)

					Expect(name).To(Equal("ConvergenceTasksKicked"))
					Expect(value).To(Equal(uint64(1)))
				})
			})

			Context("when the resolving task has been completed for longer than the time-to-resolve interval", func() {
				BeforeEach(func() {
					clock.IncrementBySeconds(uint64(expireCompletedTaskDuration.Seconds()) + 1)
				})

				It("should delete the task", func() {
					_, err := etcdDB.TaskByGuid(logger, taskGuid)
					Expect(err).To(Equal(models.ErrResourceNotFound))
				})
			})
		})
	})
})
