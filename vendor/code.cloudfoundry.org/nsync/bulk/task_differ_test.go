package bulk_test

import (
	"code.cloudfoundry.org/bbs/models"
	"code.cloudfoundry.org/lager/lagertest"
	"code.cloudfoundry.org/nsync/bulk"
	"code.cloudfoundry.org/runtimeschema/cc_messages"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("TaskDiffer", func() {

	var (
		bbsTasks map[string]*models.Task
		ccTasks  chan []cc_messages.CCTaskState
		cancelCh chan struct{}
		logger   *lagertest.TestLogger
		differ   bulk.TaskDiffer
	)

	BeforeEach(func() {
		logger = lagertest.NewTestLogger("test")
		cancelCh = make(chan struct{})
		ccTasks = make(chan []cc_messages.CCTaskState, 1)
		bbsTasks = map[string]*models.Task{}
	})

	JustBeforeEach(func() {
		differ = bulk.NewTaskDiffer(bbsTasks)
	})

	AfterEach(func() {
		Eventually(differ.TasksToFail()).Should(BeClosed())
		Eventually(differ.TasksToCancel()).Should(BeClosed())
	})

	Context("tasks found in cc but not diego", func() {
		Context("when bbs does not know about a running task", func() {
			expectedTask := cc_messages.CCTaskState{TaskGuid: "task-guid-1", State: cc_messages.TaskStateRunning, CompletionCallbackUrl: "asdf"}

			BeforeEach(func() {
				ccTasks <- []cc_messages.CCTaskState{expectedTask}
				close(ccTasks)
			})

			It("includes it in TasksToFail", func() {
				differ.Diff(logger, ccTasks, cancelCh)

				Eventually(differ.TasksToFail()).Should(Receive(ConsistOf(expectedTask)))
			})
		})

		Context("when bbs does not know about a pending task", func() {
			BeforeEach(func() {
				ccTasks <- []cc_messages.CCTaskState{
					{TaskGuid: "task-guid-1", State: cc_messages.TaskStatePending},
				}
				close(ccTasks)
			})

			It("is not included in TasksToFail", func() {
				differ.Diff(logger, ccTasks, cancelCh)

				Consistently(differ.TasksToFail()).Should(Not(Receive()))
			})
		})

		Context("when bbs does not know about a completed task", func() {
			BeforeEach(func() {
				ccTasks <- []cc_messages.CCTaskState{
					{TaskGuid: "task-guid-1", State: cc_messages.TaskStateSucceeded},
				}
				close(ccTasks)
			})

			It("is not included in TasksToFail", func() {
				differ.Diff(logger, ccTasks, cancelCh)

				Consistently(differ.TasksToFail()).Should(Not(Receive()))
			})
		})

		Context("when bbs does not know about a canceling task", func() {
			expectedTask := cc_messages.CCTaskState{TaskGuid: "task-guid-1", State: cc_messages.TaskStateCanceling, CompletionCallbackUrl: "asdf"}

			BeforeEach(func() {
				ccTasks <- []cc_messages.CCTaskState{expectedTask}
				close(ccTasks)
			})

			It("includes it in TasksToFail", func() {
				differ.Diff(logger, ccTasks, cancelCh)

				Eventually(differ.TasksToFail()).Should(Receive(ConsistOf(expectedTask)))
			})
		})

		Context("when bbs knows about a running task", func() {
			BeforeEach(func() {
				bbsTasks = map[string]*models.Task{"task-guid-1": {}}
				ccTasks <- []cc_messages.CCTaskState{
					{TaskGuid: "task-guid-1", State: cc_messages.TaskStateRunning},
				}
				close(ccTasks)
			})

			It("is not included in TasksToFail", func() {
				differ.Diff(logger, ccTasks, cancelCh)

				Consistently(differ.TasksToFail()).Should(Not(Receive()))
			})
		})
	})

	Context("tasks unknown or canceling to cc but running in diego", func() {
		Context("when cc does not know about a task", func() {
			Context("it is running in diego", func() {
				expectedTask := &models.Task{TaskGuid: "task-guid-1", State: models.Task_Running}

				BeforeEach(func() {
					bbsTasks = map[string]*models.Task{"task-guid-1": expectedTask}
					close(ccTasks)
				})

				It("is included in TasksToCancel", func() {
					differ.Diff(logger, ccTasks, cancelCh)

					Eventually(differ.TasksToCancel()).Should(Receive(ConsistOf("task-guid-1")))
				})

			})

			Context("it is pending in diego", func() {
				expectedTask := &models.Task{TaskGuid: "task-guid-1", State: models.Task_Pending}

				BeforeEach(func() {
					bbsTasks = map[string]*models.Task{"task-guid-1": expectedTask}
					close(ccTasks)
				})

				It("is included in TasksToCancel", func() {
					differ.Diff(logger, ccTasks, cancelCh)

					Eventually(differ.TasksToCancel()).Should(Receive(ConsistOf("task-guid-1")))
				})
			})

			Context("it is completed in diego", func() {
				BeforeEach(func() {
					bbsTasks = map[string]*models.Task{"task-guid-1": {TaskGuid: "task-guid-1", State: models.Task_Completed}}
					close(ccTasks)
				})

				It("is not included in TasksToCancel", func() {
					differ.Diff(logger, ccTasks, cancelCh)

					Consistently(differ.TasksToCancel()).Should(Not(Receive()))
				})
			})

			Context("it is resolving in diego", func() {
				BeforeEach(func() {
					bbsTasks = map[string]*models.Task{"task-guid-1": {TaskGuid: "task-guid-1", State: models.Task_Resolving}}
					close(ccTasks)
				})

				It("is not included in TasksToCancel", func() {
					differ.Diff(logger, ccTasks, cancelCh)

					Consistently(differ.TasksToCancel()).Should(Not(Receive()))
				})
			})
		})

		Context("when cc knows about a task", func() {
			Context("cc state is canceling", func() {
				BeforeEach(func() {
					ccTasks <- []cc_messages.CCTaskState{cc_messages.CCTaskState{TaskGuid: "task-guid-1", State: cc_messages.TaskStateCanceling, CompletionCallbackUrl: "asdf"}}
					close(ccTasks)
				})

				Context("it is running in diego", func() {
					BeforeEach(func() {
						bbsTasks = map[string]*models.Task{"task-guid-1": &models.Task{TaskGuid: "task-guid-1", State: models.Task_Running}}
					})

					It("is included in TasksToCancel", func() {
						differ.Diff(logger, ccTasks, cancelCh)

						Eventually(differ.TasksToCancel()).Should(Receive(ConsistOf("task-guid-1")))
					})
				})

				Context("it is pending in diego", func() {
					BeforeEach(func() {
						bbsTasks = map[string]*models.Task{"task-guid-1": &models.Task{TaskGuid: "task-guid-1", State: models.Task_Pending}}
					})

					It("is included in TasksToCancel", func() {
						differ.Diff(logger, ccTasks, cancelCh)

						Eventually(differ.TasksToCancel()).Should(Receive(ConsistOf("task-guid-1")))
					})
				})

				Context("it is completed in diego", func() {
					BeforeEach(func() {
						bbsTasks = map[string]*models.Task{"task-guid-1": &models.Task{TaskGuid: "task-guid-1", State: models.Task_Completed}}
					})

					It("is not included in TasksToCancel", func() {
						differ.Diff(logger, ccTasks, cancelCh)

						Consistently(differ.TasksToCancel()).Should(Not(Receive()))
					})
				})

				Context("it is resolving in diego", func() {
					BeforeEach(func() {
						bbsTasks = map[string]*models.Task{"task-guid-1": &models.Task{TaskGuid: "task-guid-1", State: models.Task_Resolving}}
					})

					It("is not included in TasksToCancel", func() {
						differ.Diff(logger, ccTasks, cancelCh)

						Consistently(differ.TasksToCancel()).Should(Not(Receive()))
					})
				})
			})

			Context("cc state is anything else", func() {
				BeforeEach(func() {
					ccTasks <- []cc_messages.CCTaskState{cc_messages.CCTaskState{TaskGuid: "task-guid-1", State: cc_messages.TaskStateRunning, CompletionCallbackUrl: "asdf"}}
					bbsTasks = map[string]*models.Task{"task-guid-1": &models.Task{TaskGuid: "task-guid-1", State: models.Task_Running}}
					close(ccTasks)
				})

				It("is not included in TasksToCancel", func() {
					differ.Diff(logger, ccTasks, cancelCh)

					Consistently(differ.TasksToCancel()).Should(Not(Receive()))
				})
			})
		})
	})

	Context("canceling", func() {
		Context("when it is receiving tasks", func() {
			BeforeEach(func() {
				ccTasks <- []cc_messages.CCTaskState{
					{TaskGuid: "task-guid-1", State: cc_messages.TaskStateRunning},
				}
				close(ccTasks)
			})

			It("closes the output channels", func() {
				close(cancelCh)
				differ.Diff(logger, ccTasks, cancelCh)

				Eventually(differ.TasksToFail()).Should(BeClosed())
				Eventually(differ.TasksToCancel()).Should(BeClosed())
			})
		})

		Context("when it is not receiving tasks", func() {
			It("closes the output channels", func() {
				close(cancelCh)
				differ.Diff(logger, ccTasks, cancelCh)

				Eventually(differ.TasksToFail()).Should(BeClosed())
				Eventually(differ.TasksToCancel()).Should(BeClosed())
			})
		})
	})
})
