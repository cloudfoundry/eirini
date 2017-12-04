package sqldb_test

import (
	"time"

	"code.cloudfoundry.org/auctioneer"
	"code.cloudfoundry.org/bbs/models"
	"code.cloudfoundry.org/bbs/models/test/model_helpers"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

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
		var (
			domain          string
			tasksToAuction  []*auctioneer.TaskStartRequest
			tasksToComplete []*models.Task
			taskEvents      []models.Event
			cellSet         models.CellSet

			taskDef *models.TaskDefinition

			pendingTask, anotherPendingTask, runningTask, runningTaskNoCell, resolvingKickableTask, expiredCompletedTask *models.Task
		)

		BeforeEach(func() {
			var err error
			domain = "my-domain"
			cellSet = models.NewCellSetFromList([]*models.CellPresence{
				{CellId: "existing-cell"},
			})
			taskDef = model_helpers.NewValidTaskDefinition()

			fakeClock.IncrementBySeconds(-expirePendingTaskDurationInSeconds)
			pendingTask, err = sqlDB.DesireTask(logger, taskDef, "pending-expired-task", domain)
			Expect(err).NotTo(HaveOccurred())
			anotherPendingTask, err = sqlDB.DesireTask(logger, taskDef, "another-pending-expired-task", domain)
			Expect(err).NotTo(HaveOccurred())
			_, err = sqlDB.DesireTask(logger, taskDef, "pending-invalid-task", domain)
			Expect(err).NotTo(HaveOccurred())
			_, err = db.Exec("UPDATE tasks SET task_definition = 'garbage' WHERE guid = 'pending-invalid-task'")
			Expect(err).NotTo(HaveOccurred())
			fakeClock.IncrementBySeconds(expirePendingTaskDurationInSeconds)

			fakeClock.IncrementBySeconds(-kickTasksDurationInSeconds)
			_, err = sqlDB.DesireTask(logger, taskDef, "pending-kickable-task", domain)
			Expect(err).NotTo(HaveOccurred())
			fakeClock.IncrementBySeconds(kickTasksDurationInSeconds)

			fakeClock.IncrementBySeconds(-kickTasksDurationInSeconds)
			_, err = sqlDB.DesireTask(logger, taskDef, "pending-kickable-invalid-task", domain)
			Expect(err).NotTo(HaveOccurred())
			_, err = db.Exec("UPDATE tasks SET task_definition = 'garbage' WHERE guid = 'pending-kickable-invalid-task'")
			Expect(err).NotTo(HaveOccurred())
			fakeClock.IncrementBySeconds(kickTasksDurationInSeconds)

			_, err = sqlDB.DesireTask(logger, taskDef, "pending-task", domain)
			Expect(err).NotTo(HaveOccurred())

			_, err = sqlDB.DesireTask(logger, taskDef, "running-task-no-cell", domain)
			Expect(err).NotTo(HaveOccurred())
			_, runningTaskNoCell, _, err = sqlDB.StartTask(logger, "running-task-no-cell", "non-existant-cell")
			Expect(err).NotTo(HaveOccurred())

			_, err = sqlDB.DesireTask(logger, taskDef, "invalid-running-task-no-cell", domain)
			Expect(err).NotTo(HaveOccurred())
			_, _, _, err = sqlDB.StartTask(logger, "invalid-running-task-no-cell", "non-existant-cell")
			Expect(err).NotTo(HaveOccurred())
			_, err = db.Exec("UPDATE tasks SET task_definition = 'garbage' WHERE guid = 'invalid-running-task-no-cell'")
			Expect(err).NotTo(HaveOccurred())

			_, err = sqlDB.DesireTask(logger, taskDef, "running-task", domain)
			Expect(err).NotTo(HaveOccurred())
			_, runningTask, _, err = sqlDB.StartTask(logger, "running-task", "existing-cell")
			Expect(err).NotTo(HaveOccurred())

			fakeClock.Increment(-expireCompletedTaskDuration)
			_, err = sqlDB.DesireTask(logger, taskDef, "completed-expired-task", domain)
			Expect(err).NotTo(HaveOccurred())
			_, _, _, err = sqlDB.StartTask(logger, "completed-expired-task", "existing-cell")
			Expect(err).NotTo(HaveOccurred())
			_, expiredCompletedTask, err = sqlDB.CompleteTask(logger, "completed-expired-task", "existing-cell", false, "", "")
			Expect(err).NotTo(HaveOccurred())

			_, err = sqlDB.DesireTask(logger, taskDef, "invalid-completed-expired-task", domain)
			Expect(err).NotTo(HaveOccurred())
			_, _, _, err = sqlDB.StartTask(logger, "invalid-completed-expired-task", "existing-cell")
			Expect(err).NotTo(HaveOccurred())
			_, _, err = sqlDB.CompleteTask(logger, "invalid-completed-expired-task", "existing-cell", false, "", "")
			Expect(err).NotTo(HaveOccurred())
			_, err = db.Exec("UPDATE tasks SET task_definition = 'garbage' WHERE guid = 'invalid-completed-expired-task'")
			Expect(err).NotTo(HaveOccurred())
			fakeClock.Increment(expireCompletedTaskDuration)

			fakeClock.IncrementBySeconds(-kickTasksDurationInSeconds)
			_, err = sqlDB.DesireTask(logger, taskDef, "completed-kickable-task", domain)
			Expect(err).NotTo(HaveOccurred())
			_, _, _, err = sqlDB.StartTask(logger, "completed-kickable-task", "existing-cell")
			Expect(err).NotTo(HaveOccurred())
			_, _, err = sqlDB.CompleteTask(logger, "completed-kickable-task", "existing-cell", false, "", "")
			Expect(err).NotTo(HaveOccurred())
			fakeClock.IncrementBySeconds(kickTasksDurationInSeconds)

			fakeClock.IncrementBySeconds(-kickTasksDurationInSeconds)
			_, err = sqlDB.DesireTask(logger, taskDef, "completed-kickable-invalid-task", domain)
			Expect(err).NotTo(HaveOccurred())
			_, _, _, err = sqlDB.StartTask(logger, "completed-kickable-invalid-task", "existing-cell")
			Expect(err).NotTo(HaveOccurred())
			_, _, err = sqlDB.CompleteTask(logger, "completed-kickable-invalid-task", "existing-cell", false, "", "")
			Expect(err).NotTo(HaveOccurred())
			_, err = db.Exec("UPDATE tasks SET task_definition = 'garbage' WHERE guid = 'completed-kickable-invalid-task'")
			Expect(err).NotTo(HaveOccurred())
			fakeClock.IncrementBySeconds(kickTasksDurationInSeconds)

			_, err = sqlDB.DesireTask(logger, taskDef, "completed-task", domain)
			Expect(err).NotTo(HaveOccurred())
			_, _, _, err = sqlDB.StartTask(logger, "completed-task", "existing-cell")
			Expect(err).NotTo(HaveOccurred())
			_, _, err = sqlDB.CompleteTask(logger, "completed-task", "existing-cell", false, "", "")
			Expect(err).NotTo(HaveOccurred())

			fakeClock.Increment(-expireCompletedTaskDuration)
			_, err = sqlDB.DesireTask(logger, taskDef, "resolving-expired-task", domain)
			Expect(err).NotTo(HaveOccurred())
			_, _, _, err = sqlDB.StartTask(logger, "resolving-expired-task", "existing-cell")
			Expect(err).NotTo(HaveOccurred())
			_, _, err = sqlDB.CompleteTask(logger, "resolving-expired-task", "existing-cell", false, "", "")
			Expect(err).NotTo(HaveOccurred())
			_, _, err = sqlDB.ResolvingTask(logger, "resolving-expired-task")
			Expect(err).NotTo(HaveOccurred())
			fakeClock.Increment(expireCompletedTaskDuration)

			fakeClock.IncrementBySeconds(-kickTasksDurationInSeconds)
			_, err = sqlDB.DesireTask(logger, taskDef, "resolving-kickable-task", domain)
			Expect(err).NotTo(HaveOccurred())
			_, _, _, err = sqlDB.StartTask(logger, "resolving-kickable-task", "existing-cell")
			Expect(err).NotTo(HaveOccurred())
			_, _, err = sqlDB.CompleteTask(logger, "resolving-kickable-task", "existing-cell", false, "", "")
			Expect(err).NotTo(HaveOccurred())
			_, resolvingKickableTask, err = sqlDB.ResolvingTask(logger, "resolving-kickable-task")
			Expect(err).NotTo(HaveOccurred())

			_, err = sqlDB.DesireTask(logger, taskDef, "invalid-resolving-kickable-task", domain)
			Expect(err).NotTo(HaveOccurred())
			_, _, _, err = sqlDB.StartTask(logger, "invalid-resolving-kickable-task", "existing-cell")
			Expect(err).NotTo(HaveOccurred())
			_, _, err = sqlDB.CompleteTask(logger, "invalid-resolving-kickable-task", "existing-cell", false, "", "")
			Expect(err).NotTo(HaveOccurred())
			_, _, err = sqlDB.ResolvingTask(logger, "invalid-resolving-kickable-task")
			Expect(err).NotTo(HaveOccurred())
			_, err = db.Exec("UPDATE tasks SET task_definition = 'garbage' WHERE guid = 'invalid-resolving-kickable-task'")
			Expect(err).NotTo(HaveOccurred())
			fakeClock.IncrementBySeconds(kickTasksDurationInSeconds)

			_, err = sqlDB.DesireTask(logger, taskDef, "resolving-task", domain)
			Expect(err).NotTo(HaveOccurred())
			_, _, _, err = sqlDB.StartTask(logger, "resolving-task", "existing-cell")
			Expect(err).NotTo(HaveOccurred())
			_, _, err = sqlDB.CompleteTask(logger, "resolving-task", "existing-cell", false, "", "")
			Expect(err).NotTo(HaveOccurred())
			_, _, err = sqlDB.ResolvingTask(logger, "resolving-task")
			Expect(err).NotTo(HaveOccurred())

			fakeClock.IncrementBySeconds(1)
		})

		JustBeforeEach(func() {
			tasksToAuction, tasksToComplete, taskEvents = sqlDB.ConvergeTasks(logger, cellSet, kickTasksDuration, expirePendingTaskDuration, expireCompletedTaskDuration)
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

		It("emits task status count metrics", func() {
			Expect(fakeMetronClient.SendMetricCallCount()).To(Equal(4))

			name, value := fakeMetronClient.SendMetricArgsForCall(0)
			Expect(name).To(Equal("TasksPending"))
			Expect(value).To(Equal(2))

			name, value = fakeMetronClient.SendMetricArgsForCall(1)
			Expect(name).To(Equal("TasksRunning"))
			Expect(value).To(Equal(1))

			name, value = fakeMetronClient.SendMetricArgsForCall(2)
			Expect(name).To(Equal("TasksCompleted"))
			Expect(value).To(Equal(6))

			name, value = fakeMetronClient.SendMetricArgsForCall(3)
			Expect(name).To(Equal("TasksResolving"))
			Expect(value).To(Equal(1))

			Expect(fakeMetronClient.IncrementCounterWithDeltaCallCount()).To(Equal(2))

			name, value64 := fakeMetronClient.IncrementCounterWithDeltaArgsForCall(0)
			Expect(name).To(Equal("ConvergenceTasksKicked"))
			Expect(value64).To(Equal(uint64(6)))

			name, value64 = fakeMetronClient.IncrementCounterWithDeltaArgsForCall(1)
			Expect(name).To(Equal("ConvergenceTasksPruned"))
			Expect(value64).To(Equal(uint64(8)))
		})

		Context("pending tasks", func() {
			It("fails expired tasks", func() {
				task, err := sqlDB.TaskByGuid(logger, "pending-expired-task")
				Expect(err).NotTo(HaveOccurred())
				Expect(task.FailureReason).To(Equal("not started within time limit"))
				Expect(task.Failed).To(BeTrue())
				Expect(task.Result).To(Equal(""))
				Expect(task.State).To(Equal(models.Task_Completed))
				Expect(task.UpdatedAt).To(Equal(fakeClock.Now().UnixNano()))
				Expect(task.FirstCompletedAt).To(Equal(fakeClock.Now().UnixNano()))

				taskRequest := auctioneer.NewTaskStartRequestFromModel("pending-expired-task", domain, taskDef)
				Expect(tasksToAuction).NotTo(ContainElement(&taskRequest))
			})

			It("returns TaskChangedEvents for all failed pending tasks", func() {
				afterPending, err := sqlDB.TaskByGuid(logger, "pending-expired-task")
				Expect(err).NotTo(HaveOccurred())
				afterAnotherPending, err := sqlDB.TaskByGuid(logger, "another-pending-expired-task")
				Expect(err).NotTo(HaveOccurred())

				event1 := models.NewTaskChangedEvent(pendingTask, afterPending)
				event2 := models.NewTaskChangedEvent(anotherPendingTask, afterAnotherPending)

				Expect(taskEvents).To(ContainElement(event1))
				Expect(taskEvents).To(ContainElement(event2))
			})

			It("returns tasks that should be kicked for auctioning", func() {
				task, err := sqlDB.TaskByGuid(logger, "pending-kickable-task")
				Expect(err).NotTo(HaveOccurred())
				Expect(task.FailureReason).NotTo(Equal("not started within time limit"))
				Expect(task.Failed).NotTo(BeTrue())

				taskRequest := auctioneer.NewTaskStartRequestFromModel("pending-kickable-task", domain, taskDef)
				Expect(tasksToAuction).To(ContainElement(&taskRequest))
			})

			It("delete tasks that should be kicked if they're invalid", func() {
				_, err := sqlDB.TaskByGuid(logger, "pending-kickable-invalid-task")
				Expect(err).To(Equal(models.ErrResourceNotFound))
			})

			It("doesn't do anything with unexpired tasks that should not be kicked", func() {
				taskRequest := auctioneer.NewTaskStartRequestFromModel("pending-task", domain, taskDef)
				Expect(tasksToAuction).NotTo(ContainElement(&taskRequest))

				task, err := sqlDB.TaskByGuid(logger, "pending-task")
				Expect(err).NotTo(HaveOccurred())
				Expect(task.FailureReason).NotTo(Equal("not started within time limit"))
				Expect(task.Failed).NotTo(BeTrue())
			})
		})

		Context("running tasks", func() {
			It("fails them when their cells are not present", func() {
				task, err := sqlDB.TaskByGuid(logger, "running-task-no-cell")
				Expect(err).NotTo(HaveOccurred())
				Expect(task.FailureReason).To(Equal("cell disappeared before completion"))
				Expect(task.Failed).To(BeTrue())
				Expect(task.Result).To(Equal(""))
				Expect(task.State).To(Equal(models.Task_Completed))
				Expect(task.UpdatedAt).To(Equal(fakeClock.Now().UnixNano()))
				Expect(task.FirstCompletedAt).To(Equal(fakeClock.Now().UnixNano()))
			})

			It("doesn't do anything when their cells are present", func() {
				taskRequest := auctioneer.NewTaskStartRequestFromModel("running-task", domain, taskDef)
				Expect(tasksToAuction).NotTo(ContainElement(taskRequest))

				task, err := sqlDB.TaskByGuid(logger, "running-task")
				Expect(err).NotTo(HaveOccurred())
				Expect(task.FailureReason).NotTo(Equal("cell disappeared before completion"))
				Expect(task.Failed).NotTo(BeTrue())
				Expect(task.State).To(Equal(models.Task_Running))
			})
		})

		Context("completed tasks", func() {
			It("deletes expired tasks", func() {
				_, err := sqlDB.TaskByGuid(logger, "completed-expired-task")
				Expect(err).To(Equal(models.ErrResourceNotFound))
			})

			It("returns tasks that should be kicked for completion", func() {
				task, err := sqlDB.TaskByGuid(logger, "completed-kickable-task")
				Expect(err).NotTo(HaveOccurred())
				Expect(tasksToComplete).To(ContainElement(task))
			})

			It("doesn't do anything with unexpired tasks that should not be kicked", func() {
				task, err := sqlDB.TaskByGuid(logger, "completed-task")
				Expect(err).NotTo(HaveOccurred())
				Expect(tasksToComplete).NotTo(ContainElement(task))
			})

			It("delete tasks that should be kicked if they're invalid", func() {
				_, err := sqlDB.TaskByGuid(logger, "completed-kickable-invalid-task")
				Expect(err).To(Equal(models.ErrResourceNotFound))
			})

			It("returns TaskRemovedEvents for all deleted tasks", func() {
				event1 := models.NewTaskRemovedEvent(expiredCompletedTask)
				Expect(taskEvents).To(ContainElement(event1))
			})
		})

		Context("resolving tasks", func() {
			It("deletes expired tasks", func() {
				_, err := sqlDB.TaskByGuid(logger, "resolving-expired-task")
				Expect(err).To(Equal(models.ErrResourceNotFound))
			})

			It("transitions the task back to the completed state if it should be kicked", func() {
				task, err := sqlDB.TaskByGuid(logger, "resolving-kickable-task")
				Expect(err).NotTo(HaveOccurred())
				Expect(task.State).To(Equal(models.Task_Completed))
			})

			It("returns tasks that should be kicked for completion", func() {
				task, err := sqlDB.TaskByGuid(logger, "resolving-kickable-task")
				Expect(err).NotTo(HaveOccurred())
				Expect(tasksToComplete).To(ContainElement(task))
			})

			It("doesn't do anything with unexpired tasks that should not be kicked", func() {
				task, err := sqlDB.TaskByGuid(logger, "resolving-task")
				Expect(err).NotTo(HaveOccurred())
				Expect(task.State).To(Equal(models.Task_Resolving))
				Expect(tasksToComplete).NotTo(ContainElement(task))
			})

			It("returns TaskChangedEvents for all kicked resolved tasks", func() {
				after, err := sqlDB.TaskByGuid(logger, "resolving-kickable-task")
				Expect(err).NotTo(HaveOccurred())

				event1 := models.NewTaskChangedEvent(resolvingKickableTask, after)

				Expect(taskEvents).To(ContainElement(event1))
			})
		})

		Context("when the cell state list is empty", func() {
			BeforeEach(func() {
				cellSet = models.NewCellSetFromList([]*models.CellPresence{})
			})

			It("fails the running task", func() {
				task, err := sqlDB.TaskByGuid(logger, "running-task")
				Expect(err).NotTo(HaveOccurred())
				Expect(task.Failed).To(BeTrue())
				Expect(task.FailureReason).To(Equal("cell disappeared before completion"))
				Expect(task.Result).To(Equal(""))
			})

			It("returns TaskChangedEvents for all failed tasks", func() {
				after1, err := sqlDB.TaskByGuid(logger, "running-task")
				Expect(err).NotTo(HaveOccurred())
				after2, err := sqlDB.TaskByGuid(logger, "running-task-no-cell")
				Expect(err).NotTo(HaveOccurred())

				event1 := models.NewTaskChangedEvent(runningTask, after1)
				event2 := models.NewTaskChangedEvent(runningTaskNoCell, after2)
				Expect(taskEvents).To(ContainElement(event1))
				Expect(taskEvents).To(ContainElement(event2))
			})
		})
	})
})
