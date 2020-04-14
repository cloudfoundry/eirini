package bifrost_test

import (
	"context"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/pkg/errors"

	"code.cloudfoundry.org/bbs/models"
	"code.cloudfoundry.org/eirini/bifrost"
	"code.cloudfoundry.org/eirini/bifrost/bifrostfakes"
	"code.cloudfoundry.org/eirini/models/cf"
	"code.cloudfoundry.org/eirini/opi"
	"code.cloudfoundry.org/eirini/opi/opifakes"
	"code.cloudfoundry.org/lager/lagertest"
)

var _ = Describe("Staging", func() {

	var (
		err              error
		bfrstStaging     *bifrost.BuildpackStaging
		converter        *bifrostfakes.FakeConverter
		taskDesirer      *opifakes.FakeTaskDesirer
		stagingCompleter *bifrostfakes.FakeStagingCompleter
	)

	BeforeEach(func() {
		converter = new(bifrostfakes.FakeConverter)
		taskDesirer = new(opifakes.FakeTaskDesirer)
		stagingCompleter = new(bifrostfakes.FakeStagingCompleter)
	})

	JustBeforeEach(func() {
		logger := lagertest.NewTestLogger("test")
		bfrstStaging = &bifrost.BuildpackStaging{
			Converter:        converter,
			TaskDesirer:      taskDesirer,
			StagingCompleter: stagingCompleter,
			Logger:           logger,
		}
	})

	Describe("Transfer Staging", func() {
		var (
			stagingGUID    string
			stagingRequest cf.StagingRequest
			stagingTask    opi.StagingTask
		)

		BeforeEach(func() {
			stagingGUID = "staging-guid"
			stagingTask = opi.StagingTask{
				Task: &opi.Task{GUID: "some-guid"},
			}
			converter.ConvertStagingReturns(stagingTask, nil)
			stagingRequest = cf.StagingRequest{AppGUID: "app-guid"}
		})

		JustBeforeEach(func() {
			err = bfrstStaging.TransferStaging(context.Background(), stagingGUID, stagingRequest)
		})

		It("transfers the task", func() {
			Expect(err).NotTo(HaveOccurred())

			Expect(converter.ConvertStagingCallCount()).To(Equal(1))
			actualStagingGUID, actualStagingRequest := converter.ConvertStagingArgsForCall(0)
			Expect(actualStagingGUID).To(Equal(stagingGUID))
			Expect(actualStagingRequest).To(Equal(stagingRequest))

			Expect(taskDesirer.DesireStagingCallCount()).To(Equal(1))
			desiredStaging := taskDesirer.DesireStagingArgsForCall(0)
			Expect(desiredStaging.GUID).To(Equal("some-guid"))
		})

		When("converting the task fails", func() {
			BeforeEach(func() {
				converter.ConvertStagingReturns(opi.StagingTask{}, errors.New("staging-conv-err"))
			})

			It("returns the error", func() {
				Expect(err).To(MatchError(ContainSubstring("staging-conv-err")))
			})

			It("does not desire the staging task", func() {
				Expect(taskDesirer.DesireStagingCallCount()).To(Equal(0))
			})
		})

		When("desiring the staging task fails", func() {
			BeforeEach(func() {
				taskDesirer.DesireStagingReturns(errors.New("desire-staging-err"))
			})

			It("returns the error", func() {
				Expect(err).To(MatchError(ContainSubstring("desire-staging-err")))
			})
		})
	})

	Describe("Complete Staging", func() {

		var (
			task *models.TaskCallbackResponse
		)

		BeforeEach(func() {
			annotation := `{"completion_callback": "some-cc-endpoint.io/call/me/maybe"}`

			task = &models.TaskCallbackResponse{
				TaskGuid:      "our-task-guid",
				Failed:        false,
				FailureReason: "",
				Result:        `{"very": "good"}`,
				Annotation:    annotation,
				CreatedAt:     123456123,
			}
		})

		JustBeforeEach(func() {
			err = bfrstStaging.CompleteStaging(task)
		})

		It("should not return an error", func() {
			Expect(err).ToNot(HaveOccurred())
		})

		It("should complete staging", func() {
			Expect(stagingCompleter.CompleteStagingCallCount()).To(Equal(1))
			Expect(stagingCompleter.CompleteStagingArgsForCall(0)).To(Equal(task))
		})

		It("should delete the task", func() {
			Expect(taskDesirer.DeleteCallCount()).To(Equal(1))

			taskName := taskDesirer.DeleteArgsForCall(0)
			Expect(taskName).To(Equal(task.TaskGuid))
		})

		Context("and the staging completer fails", func() {
			BeforeEach(func() {
				stagingCompleter.CompleteStagingReturns(errors.New("complete boom"))
			})

			It("should return an error", func() {
				Expect(err).To(MatchError("complete boom"))
			})

			It("should delete the task", func() {
				Expect(taskDesirer.DeleteCallCount()).To(Equal(1))

				taskName := taskDesirer.DeleteArgsForCall(0)
				Expect(taskName).To(Equal(task.TaskGuid))
			})
		})

		Context("and the task deletion fails", func() {
			BeforeEach(func() {
				taskDesirer.DeleteReturns(errors.New("delete boom"))
			})

			It("should return an error", func() {
				Expect(err).To(MatchError("delete boom"))
			})
		})
	})
})
