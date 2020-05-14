package bifrost_test

import (
	"context"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/pkg/errors"

	"code.cloudfoundry.org/eirini/bifrost"
	"code.cloudfoundry.org/eirini/bifrost/bifrostfakes"
	"code.cloudfoundry.org/eirini/models/cf"
	"code.cloudfoundry.org/eirini/opi"
)

var _ = Describe("Buildpack task", func() {
	var (
		err           error
		taskBifrost   *bifrost.Task
		taskConverter *bifrostfakes.FakeTaskConverter
		taskDesirer   *bifrostfakes.FakeTaskDesirer
		taskGUID      string
		task          opi.Task
	)

	BeforeEach(func() {
		taskConverter = new(bifrostfakes.FakeTaskConverter)
		taskDesirer = new(bifrostfakes.FakeTaskDesirer)
		taskGUID = "task-guid"
		task = opi.Task{GUID: "my-guid"}
		taskConverter.ConvertTaskReturns(task, nil)
		taskBifrost = &bifrost.Task{
			Converter:   taskConverter,
			TaskDesirer: taskDesirer,
		}
	})

	Describe("Transfer Task", func() {
		var (
			taskRequest cf.TaskRequest
		)

		BeforeEach(func() {
			taskRequest = cf.TaskRequest{
				Name:               "cake",
				AppGUID:            "app-guid",
				AppName:            "foo",
				OrgName:            "my-org",
				OrgGUID:            "asdf123",
				SpaceName:          "my-space",
				SpaceGUID:          "fdsa4321",
				CompletionCallback: "my-callback",
				Environment:        nil,
				Lifecycle: cf.Lifecycle{
					BuildpackLifecycle: &cf.BuildpackLifecycle{
						DropletHash:  "h123jhh",
						DropletGUID:  "fds1234",
						StartCommand: "run",
					},
				},
			}
			task := opi.Task{GUID: "my-guid"}
			taskConverter.ConvertTaskReturns(task, nil)

		})

		JustBeforeEach(func() {
			err = taskBifrost.TransferTask(context.Background(), taskGUID, taskRequest)
		})

		It("transfers the task", func() {
			Expect(err).NotTo(HaveOccurred())

			Expect(taskConverter.ConvertTaskCallCount()).To(Equal(1))
			actualTaskGUID, actualTaskRequest := taskConverter.ConvertTaskArgsForCall(0)
			Expect(actualTaskGUID).To(Equal(taskGUID))
			Expect(actualTaskRequest).To(Equal(taskRequest))

			Expect(taskDesirer.DesireCallCount()).To(Equal(1))
			desiredTask := taskDesirer.DesireArgsForCall(0)
			Expect(desiredTask.GUID).To(Equal("my-guid"))
		})

		When("converting the task fails", func() {
			BeforeEach(func() {
				taskConverter.ConvertTaskReturns(opi.Task{}, errors.New("task-conv-err"))
			})

			It("returns the error", func() {
				Expect(err).To(MatchError(ContainSubstring("task-conv-err")))
			})

			It("does not desire the task", func() {
				Expect(taskDesirer.DesireCallCount()).To(Equal(0))
			})
		})

		When("desiring the task fails", func() {
			BeforeEach(func() {
				taskDesirer.DesireReturns(errors.New("desire-task-err"))
			})

			It("returns the error", func() {
				Expect(err).To(MatchError(ContainSubstring("desire-task-err")))
			})
		})
	})

	Describe("Complete Task", func() {
		JustBeforeEach(func() {
			err = taskBifrost.CompleteTask(taskGUID)
		})

		It("succeeds", func() {
			Expect(err).NotTo(HaveOccurred())
		})

		It("deletes the task", func() {
			Expect(taskDesirer.DeleteCallCount()).To(Equal(1))
			Expect(taskDesirer.DeleteArgsForCall(0)).To(Equal(taskGUID))
		})

		When("deleting the task fails", func() {
			BeforeEach(func() {
				taskDesirer.DeleteReturns(errors.New("delete-task-err"))
			})

			It("returns the error", func() {
				Expect(err).To(MatchError(ContainSubstring("delete-task-err")))
			})
		})
	})
})
