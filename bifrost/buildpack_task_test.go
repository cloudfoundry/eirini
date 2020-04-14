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
	"code.cloudfoundry.org/eirini/opi/opifakes"
)

var _ = Describe("Transfer Task", func() {

	var (
		err         error
		bfrstTask   *bifrost.BuildpackTask
		converter   *bifrostfakes.FakeConverter
		taskDesirer *opifakes.FakeTaskDesirer
		taskGUID    string
		taskRequest cf.TaskRequest
		task        opi.Task
	)

	BeforeEach(func() {
		converter = new(bifrostfakes.FakeConverter)
		taskDesirer = new(opifakes.FakeTaskDesirer)
		taskGUID = "task-guid"
		task = opi.Task{GUID: "my-guid"}
		converter.ConvertTaskReturns(task, nil)
		taskRequest = cf.TaskRequest{AppGUID: "app-guid"}
		bfrstTask = &bifrost.BuildpackTask{
			Converter:   converter,
			TaskDesirer: taskDesirer,
		}
	})

	JustBeforeEach(func() {
		err = bfrstTask.TransferTask(context.Background(), taskGUID, taskRequest)
	})

	It("transfers the task", func() {
		Expect(err).NotTo(HaveOccurred())

		Expect(converter.ConvertTaskCallCount()).To(Equal(1))
		actualTaskGUID, actualTaskRequest := converter.ConvertTaskArgsForCall(0)
		Expect(actualTaskGUID).To(Equal(taskGUID))
		Expect(actualTaskRequest).To(Equal(taskRequest))

		Expect(taskDesirer.DesireCallCount()).To(Equal(1))
		desiredTask := taskDesirer.DesireArgsForCall(0)
		Expect(desiredTask.GUID).To(Equal("my-guid"))
	})

	When("converting the task fails", func() {
		BeforeEach(func() {
			converter.ConvertTaskReturns(opi.Task{}, errors.New("task-conv-err"))
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
