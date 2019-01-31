package sqldb_test

import (
	"time"

	"code.cloudfoundry.org/auctioneer"
	dbpkg "code.cloudfoundry.org/bbs/db"
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
			domain         string
			existingCellID string
			cellSet        models.CellSet
			taskDef        *models.TaskDefinition

			convergenceResult dbpkg.TaskConvergenceResult
		)

		BeforeEach(func() {
			domain = "my-domain"
			existingCellID = "existingCellID"
			cellSet = models.NewCellSetFromList([]*models.CellPresence{
				{CellId: existingCellID},
			})
			taskDef = model_helpers.NewValidTaskDefinition()
		})

		JustBeforeEach(func() {
			convergenceResult = sqlDB.ConvergeTasks(logger, cellSet, kickTasksDuration, expirePendingTaskDuration, expireCompletedTaskDuration)
		})

		Context("pending tasks", func() {
			var pendingTask, anotherPendingTask *models.Task

			BeforeEach(func() {
				var err error
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
				_, err = sqlDB.DesireTask(logger, taskDef, "pending-kickable-invalid-task", domain)
				Expect(err).NotTo(HaveOccurred())
				_, err = db.Exec("UPDATE tasks SET task_definition = 'garbage' WHERE guid = 'pending-kickable-invalid-task'")
				Expect(err).NotTo(HaveOccurred())
				fakeClock.IncrementBySeconds(kickTasksDurationInSeconds)

				_, err = sqlDB.DesireTask(logger, taskDef, "pending-task", domain)
				Expect(err).NotTo(HaveOccurred())

				fakeClock.IncrementBySeconds(1)
			})

			It("returns the correct task metrics", func() {
				Expect(convergenceResult.Metrics.TasksPending).To(Equal(int(2)))
				Expect(convergenceResult.Metrics.TasksRunning).To(Equal(int(0)))
				Expect(convergenceResult.Metrics.TasksCompleted).To(Equal(int(2)))
				Expect(convergenceResult.Metrics.TasksResolving).To(Equal(int(0)))
				Expect(convergenceResult.Metrics.TasksPruned).To(Equal(uint64(2)))
				Expect(convergenceResult.Metrics.TasksKicked).To(Equal(uint64(4)))
			})

			It("fails expired tasks that have exceeded the expirePendingTaskDuration", func() {
				task, err := sqlDB.TaskByGuid(logger, "pending-expired-task")
				Expect(err).NotTo(HaveOccurred())
				Expect(task.FailureReason).To(Equal("not started within time limit"))
				Expect(task.Failed).To(BeTrue())
				Expect(task.Result).To(Equal(""))
				Expect(task.State).To(Equal(models.Task_Completed))
				Expect(task.UpdatedAt).To(Equal(fakeClock.Now().UnixNano()))
				Expect(task.FirstCompletedAt).To(Equal(fakeClock.Now().UnixNano()))

				taskRequest := auctioneer.NewTaskStartRequestFromModel("pending-expired-task", domain, taskDef)
				Expect(convergenceResult.TasksToAuction).NotTo(ContainElement(&taskRequest))
			})

			It("returns TaskChangedEvents for all expired pending tasks", func() {
				Expect(convergenceResult.Events).To(HaveLen(2))

				afterPending, err := sqlDB.TaskByGuid(logger, "pending-expired-task")
				Expect(err).NotTo(HaveOccurred())
				afterAnotherPending, err := sqlDB.TaskByGuid(logger, "another-pending-expired-task")
				Expect(err).NotTo(HaveOccurred())

				event1 := models.NewTaskChangedEvent(pendingTask, afterPending)
				event2 := models.NewTaskChangedEvent(anotherPendingTask, afterAnotherPending)

				Expect(convergenceResult.Events).To(ContainElement(event1))
				Expect(convergenceResult.Events).To(ContainElement(event2))
			})

			It("returns tasks that have not expired and should be kicked for auctioning", func() {
				pendingTask, err := sqlDB.TaskByGuid(logger, "pending-kickable-task")
				Expect(err).NotTo(HaveOccurred())
				Expect(pendingTask.FailureReason).NotTo(Equal("not started within time limit"))
				Expect(pendingTask.Failed).NotTo(BeTrue())

				pendingTaskRequest := auctioneer.NewTaskStartRequestFromModel("pending-kickable-task", domain, taskDef)
				taskRequest := auctioneer.NewTaskStartRequestFromModel("pending-task", domain, taskDef)
				Expect(convergenceResult.TasksToAuction).To(ConsistOf(&pendingTaskRequest, &taskRequest))
			})

			It("delete tasks that are invalid", func() {
				_, err := sqlDB.TaskByGuid(logger, "pending-invalid-task")
				Expect(err).To(Equal(models.ErrResourceNotFound))
				_, err = sqlDB.TaskByGuid(logger, "pending-kickable-invalid-task")
				Expect(err).To(Equal(models.ErrResourceNotFound))
			})
		})

		Context("running tasks", func() {
			var runningTaskNoCell *models.Task

			BeforeEach(func() {
				var err error
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
				_, _, _, err = sqlDB.StartTask(logger, "running-task", existingCellID)
				Expect(err).NotTo(HaveOccurred())

				fakeClock.IncrementBySeconds(1)
			})

			It("returns the correct task metrics", func() {
				Expect(convergenceResult.Metrics.TasksPending).To(Equal(int(0)))
				Expect(convergenceResult.Metrics.TasksRunning).To(Equal(int(1)))
				Expect(convergenceResult.Metrics.TasksCompleted).To(Equal(int(1)))
				Expect(convergenceResult.Metrics.TasksResolving).To(Equal(int(0)))
				Expect(convergenceResult.Metrics.TasksPruned).To(Equal(uint64(1)))
				Expect(convergenceResult.Metrics.TasksKicked).To(Equal(uint64(1)))
			})

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
				Expect(convergenceResult.TasksToAuction).NotTo(ContainElement(taskRequest))

				task, err := sqlDB.TaskByGuid(logger, "running-task")
				Expect(err).NotTo(HaveOccurred())
				Expect(task.FailureReason).NotTo(Equal("cell disappeared before completion"))
				Expect(task.Failed).NotTo(BeTrue())
				Expect(task.State).To(Equal(models.Task_Running))
			})

			It("returns TaskChangedEvents for all running tasks with dissappeared cells", func() {
				Expect(convergenceResult.Events).To(HaveLen(1))

				afterRunning, err := sqlDB.TaskByGuid(logger, "running-task-no-cell")
				Expect(err).NotTo(HaveOccurred())

				event := models.NewTaskChangedEvent(runningTaskNoCell, afterRunning)

				Expect(convergenceResult.Events).To(ContainElement(event))
			})
		})

		Context("completed tasks", func() {
			var expiredCompletedTask *models.Task

			BeforeEach(func() {
				var err error
				fakeClock.Increment(-expireCompletedTaskDuration)
				_, err = sqlDB.DesireTask(logger, taskDef, "completed-expired-task", domain)
				Expect(err).NotTo(HaveOccurred())
				_, _, _, err = sqlDB.StartTask(logger, "completed-expired-task", existingCellID)
				Expect(err).NotTo(HaveOccurred())
				_, expiredCompletedTask, err = sqlDB.CompleteTask(logger, "completed-expired-task", existingCellID, false, "", "")
				Expect(err).NotTo(HaveOccurred())
				fakeClock.Increment(expireCompletedTaskDuration)

				fakeClock.IncrementBySeconds(-kickTasksDurationInSeconds)
				_, err = sqlDB.DesireTask(logger, taskDef, "completed-kickable-task", domain)
				Expect(err).NotTo(HaveOccurred())
				_, _, _, err = sqlDB.StartTask(logger, "completed-kickable-task", existingCellID)
				Expect(err).NotTo(HaveOccurred())
				_, _, err = sqlDB.CompleteTask(logger, "completed-kickable-task", existingCellID, false, "", "")
				Expect(err).NotTo(HaveOccurred())

				_, err = sqlDB.DesireTask(logger, taskDef, "completed-kickable-invalid-task", domain)
				Expect(err).NotTo(HaveOccurred())
				_, _, _, err = sqlDB.StartTask(logger, "completed-kickable-invalid-task", existingCellID)
				Expect(err).NotTo(HaveOccurred())
				_, _, err = sqlDB.CompleteTask(logger, "completed-kickable-invalid-task", existingCellID, false, "", "")
				Expect(err).NotTo(HaveOccurred())
				_, err = db.Exec("UPDATE tasks SET task_definition = 'garbage' WHERE guid = 'completed-kickable-invalid-task'")
				Expect(err).NotTo(HaveOccurred())
				fakeClock.IncrementBySeconds(kickTasksDurationInSeconds)

				_, err = sqlDB.DesireTask(logger, taskDef, "completed-task", domain)
				Expect(err).NotTo(HaveOccurred())
				_, _, _, err = sqlDB.StartTask(logger, "completed-task", existingCellID)
				Expect(err).NotTo(HaveOccurred())
				_, _, err = sqlDB.CompleteTask(logger, "completed-task", existingCellID, false, "", "")
				Expect(err).NotTo(HaveOccurred())

				fakeClock.IncrementBySeconds(1)
			})

			It("returns the correct task metrics", func() {
				Expect(convergenceResult.Metrics.TasksPending).To(Equal(int(0)))
				Expect(convergenceResult.Metrics.TasksRunning).To(Equal(int(0)))
				Expect(convergenceResult.Metrics.TasksCompleted).To(Equal(int(2)))
				Expect(convergenceResult.Metrics.TasksResolving).To(Equal(int(0)))
				Expect(convergenceResult.Metrics.TasksPruned).To(Equal(uint64(2)))
				Expect(convergenceResult.Metrics.TasksKicked).To(Equal(uint64(1)))
			})

			It("deletes tasks that have exceeded expireCompleteTaskDuration", func() {
				_, err := sqlDB.TaskByGuid(logger, "completed-expired-task")
				Expect(err).To(Equal(models.ErrResourceNotFound))
			})

			Context("when there are invalid tasks", func() {
				BeforeEach(func() {
					fakeClock.Increment(-expireCompletedTaskDuration)
					_, err := sqlDB.DesireTask(logger, taskDef, "another-completed-task", domain)
					Expect(err).NotTo(HaveOccurred())
					_, _, _, err = sqlDB.StartTask(logger, "another-completed-task", existingCellID)
					Expect(err).NotTo(HaveOccurred())
					_, _, err = sqlDB.CompleteTask(logger, "another-completed-task", existingCellID, false, "", "")
					Expect(err).NotTo(HaveOccurred())
					fakeClock.Increment(expireCompletedTaskDuration)

					updateTaskToInvalid(db, serializer, expiredCompletedTask)
					fakeClock.IncrementBySeconds(1)
				})

				It("deletes tasks that have exceeded expireCompleteTaskDuration", func() {
					_, err := sqlDB.TaskByGuid(logger, "another-completed-task")
					Expect(err).To(Equal(models.ErrResourceNotFound))
				})
			})

			It("returns tasks that should be kicked for completion", func() {
				task, err := sqlDB.TaskByGuid(logger, "completed-kickable-task")
				Expect(err).NotTo(HaveOccurred())
				Expect(convergenceResult.TasksToComplete).To(ContainElement(task))
			})

			It("doesn't do anything with unexpired tasks that should not be kicked", func() {
				task, err := sqlDB.TaskByGuid(logger, "completed-task")
				Expect(err).NotTo(HaveOccurred())
				Expect(convergenceResult.TasksToComplete).NotTo(ContainElement(task))
			})

			It("delete tasks that are invalid", func() {
				_, err := sqlDB.TaskByGuid(logger, "invalid-completed-expired-task")
				Expect(err).To(Equal(models.ErrResourceNotFound))
				_, err = sqlDB.TaskByGuid(logger, "completed-kickable-invalid-task")
				Expect(err).To(Equal(models.ErrResourceNotFound))
			})

			It("returns TaskRemovedEvents for tasks deleted due to expiration", func() {
				Expect(convergenceResult.Events).To(HaveLen(1))
				event := models.NewTaskRemovedEvent(expiredCompletedTask)
				Expect(convergenceResult.Events).To(ContainElement(event))
			})
		})

		Context("resolving tasks", func() {
			var resolvingExpiredTask, resolvingKickableTask *models.Task

			BeforeEach(func() {
				var err error
				fakeClock.Increment(-expireCompletedTaskDuration)
				// resolving-expired-task will first get demoted to the completed state and then be deleted for exceeding the expiredCompletedTaskDuration
				_, err = sqlDB.DesireTask(logger, taskDef, "resolving-expired-task", domain)
				Expect(err).NotTo(HaveOccurred())
				_, _, _, err = sqlDB.StartTask(logger, "resolving-expired-task", existingCellID)
				Expect(err).NotTo(HaveOccurred())
				_, _, err = sqlDB.CompleteTask(logger, "resolving-expired-task", existingCellID, false, "", "")
				Expect(err).NotTo(HaveOccurred())
				_, resolvingExpiredTask, err = sqlDB.ResolvingTask(logger, "resolving-expired-task")
				Expect(err).NotTo(HaveOccurred())
				fakeClock.Increment(expireCompletedTaskDuration)

				fakeClock.IncrementBySeconds(-kickTasksDurationInSeconds)
				_, err = sqlDB.DesireTask(logger, taskDef, "resolving-kickable-task", domain)
				Expect(err).NotTo(HaveOccurred())
				_, _, _, err = sqlDB.StartTask(logger, "resolving-kickable-task", existingCellID)
				Expect(err).NotTo(HaveOccurred())
				_, _, err = sqlDB.CompleteTask(logger, "resolving-kickable-task", existingCellID, false, "", "")
				Expect(err).NotTo(HaveOccurred())
				_, resolvingKickableTask, err = sqlDB.ResolvingTask(logger, "resolving-kickable-task")
				Expect(err).NotTo(HaveOccurred())

				_, err = sqlDB.DesireTask(logger, taskDef, "invalid-resolving-kickable-task", domain)
				Expect(err).NotTo(HaveOccurred())
				_, _, _, err = sqlDB.StartTask(logger, "invalid-resolving-kickable-task", existingCellID)
				Expect(err).NotTo(HaveOccurred())
				_, _, err = sqlDB.CompleteTask(logger, "invalid-resolving-kickable-task", existingCellID, false, "", "")
				Expect(err).NotTo(HaveOccurred())
				_, _, err = sqlDB.ResolvingTask(logger, "invalid-resolving-kickable-task")
				Expect(err).NotTo(HaveOccurred())
				_, err = db.Exec("UPDATE tasks SET task_definition = 'garbage' WHERE guid = 'invalid-resolving-kickable-task'")
				Expect(err).NotTo(HaveOccurred())
				fakeClock.IncrementBySeconds(kickTasksDurationInSeconds)

				_, err = sqlDB.DesireTask(logger, taskDef, "resolving-task", domain)
				Expect(err).NotTo(HaveOccurred())
				_, _, _, err = sqlDB.StartTask(logger, "resolving-task", existingCellID)
				Expect(err).NotTo(HaveOccurred())
				_, _, err = sqlDB.CompleteTask(logger, "resolving-task", existingCellID, false, "", "")
				Expect(err).NotTo(HaveOccurred())
				_, _, err = sqlDB.ResolvingTask(logger, "resolving-task")
				Expect(err).NotTo(HaveOccurred())

				fakeClock.IncrementBySeconds(1)
			})

			It("returns the correct task metrics", func() {
				Expect(convergenceResult.Metrics.TasksPending).To(Equal(int(0)))
				Expect(convergenceResult.Metrics.TasksRunning).To(Equal(int(0)))
				Expect(convergenceResult.Metrics.TasksCompleted).To(Equal(int(1)))
				Expect(convergenceResult.Metrics.TasksResolving).To(Equal(int(1)))
				Expect(convergenceResult.Metrics.TasksPruned).To(Equal(uint64(2)))
				Expect(convergenceResult.Metrics.TasksKicked).To(Equal(uint64(1)))
			})

			It("deletes expired resolving tasks bc they were demoted and also exceeded the expireCompletedTaskDuration", func() {
				_, err := sqlDB.TaskByGuid(logger, "resolving-expired-task")
				Expect(err).To(Equal(models.ErrResourceNotFound))
			})

			It("transitions the task back to the completed state if it exceed kickTasksDuration and then it is kicked", func() {
				task, err := sqlDB.TaskByGuid(logger, "resolving-kickable-task")
				Expect(err).NotTo(HaveOccurred())
				Expect(task.State).To(Equal(models.Task_Completed))
			})

			It("returns tasks that should be kicked for completion", func() {
				Expect(convergenceResult.TasksToComplete).To(HaveLen(1))
				task, err := sqlDB.TaskByGuid(logger, "resolving-kickable-task")
				Expect(err).NotTo(HaveOccurred())
				Expect(convergenceResult.TasksToComplete).To(ContainElement(task))
			})

			It("doesn't do anything with unexpired tasks that should not be kicked", func() {
				task, err := sqlDB.TaskByGuid(logger, "resolving-task")
				Expect(err).NotTo(HaveOccurred())
				Expect(task.State).To(Equal(models.Task_Resolving))
				Expect(convergenceResult.TasksToComplete).NotTo(ContainElement(task))
			})

			It("delete tasks that are invalid", func() {
				_, err := sqlDB.TaskByGuid(logger, "invalid-resolving-kickable-task")
				Expect(err).To(Equal(models.ErrResourceNotFound))
			})

			It("returns TaskChangedEvents for all kicked resolved tasks", func() {
				afterResolvingKickedTask, err := sqlDB.TaskByGuid(logger, "resolving-kickable-task")
				Expect(err).NotTo(HaveOccurred())

				afterResolvingExpiredTask := &models.Task{}
				*afterResolvingExpiredTask = *resolvingExpiredTask
				afterResolvingExpiredTask.State = models.Task_Completed

				event1 := models.NewTaskChangedEvent(resolvingKickableTask, afterResolvingKickedTask)
				event2 := models.NewTaskChangedEvent(resolvingExpiredTask, afterResolvingExpiredTask)
				event3 := models.NewTaskRemovedEvent(afterResolvingExpiredTask)

				Expect(convergenceResult.Events).To(ConsistOf(event1, event2, event3))
			})
		})
	})
})
