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
	"code.cloudfoundry.org/lager/lagertest"
)

var _ = Describe("Staging", func() {

	var (
		err                     error
		buildpackStagingBifrost *bifrost.BuildpackStaging
		stagingConverter        *bifrostfakes.FakeStagingConverter
		stagingDesirer          *bifrostfakes.FakeStagingDesirer
		stagingCompleter        *bifrostfakes.FakeStagingCompleter
	)

	BeforeEach(func() {
		stagingConverter = new(bifrostfakes.FakeStagingConverter)
		stagingDesirer = new(bifrostfakes.FakeStagingDesirer)
		stagingCompleter = new(bifrostfakes.FakeStagingCompleter)
	})

	JustBeforeEach(func() {
		logger := lagertest.NewTestLogger("test")
		buildpackStagingBifrost = &bifrost.BuildpackStaging{
			Converter:        stagingConverter,
			StagingDesirer:   stagingDesirer,
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
			stagingConverter.ConvertStagingReturns(stagingTask, nil)
			stagingRequest = cf.StagingRequest{AppGUID: "app-guid"}
		})

		JustBeforeEach(func() {
			err = buildpackStagingBifrost.TransferStaging(context.Background(), stagingGUID, stagingRequest)
		})

		It("transfers the task", func() {
			Expect(err).NotTo(HaveOccurred())

			Expect(stagingConverter.ConvertStagingCallCount()).To(Equal(1))
			actualStagingGUID, actualStagingRequest := stagingConverter.ConvertStagingArgsForCall(0)
			Expect(actualStagingGUID).To(Equal(stagingGUID))
			Expect(actualStagingRequest).To(Equal(stagingRequest))

			Expect(stagingDesirer.DesireStagingCallCount()).To(Equal(1))
			desiredStaging := stagingDesirer.DesireStagingArgsForCall(0)
			Expect(desiredStaging.GUID).To(Equal("some-guid"))
		})

		When("converting the task fails", func() {
			BeforeEach(func() {
				stagingConverter.ConvertStagingReturns(opi.StagingTask{}, errors.New("staging-conv-err"))
			})

			It("returns the error", func() {
				Expect(err).To(MatchError(ContainSubstring("staging-conv-err")))
			})

			It("does not desire the staging task", func() {
				Expect(stagingDesirer.DesireStagingCallCount()).To(Equal(0))
			})
		})

		When("desiring the staging task fails", func() {
			BeforeEach(func() {
				stagingDesirer.DesireStagingReturns(errors.New("desire-staging-err"))
			})

			It("returns the error", func() {
				Expect(err).To(MatchError(ContainSubstring("desire-staging-err")))
			})
		})
	})

	Describe("Complete Staging", func() {

		var (
			taskCompletedRequest cf.TaskCompletedRequest
		)

		BeforeEach(func() {
			annotation := `{"completion_callback": "some-cc-endpoint.io/call/me/maybe"}`

			taskCompletedRequest = cf.TaskCompletedRequest{
				TaskGUID:      "our-task-guid",
				Failed:        false,
				FailureReason: "",
				Result:        `{"very": "good"}`,
				Annotation:    annotation,
			}
		})

		JustBeforeEach(func() {
			err = buildpackStagingBifrost.CompleteStaging(taskCompletedRequest)
		})

		It("should not return an error", func() {
			Expect(err).ToNot(HaveOccurred())
		})

		It("should complete staging", func() {
			Expect(stagingCompleter.CompleteStagingCallCount()).To(Equal(1))
			Expect(stagingCompleter.CompleteStagingArgsForCall(0)).To(Equal(taskCompletedRequest))
		})

		It("should delete the task", func() {
			Expect(stagingDesirer.DeleteStagingCallCount()).To(Equal(1))

			taskName := stagingDesirer.DeleteStagingArgsForCall(0)
			Expect(taskName).To(Equal(taskCompletedRequest.TaskGUID))
		})

		Context("and the staging completer fails", func() {
			BeforeEach(func() {
				stagingCompleter.CompleteStagingReturns(errors.New("complete boom"))
			})

			It("should return an error", func() {
				Expect(err).To(MatchError("complete boom"))
			})

			It("should delete the task", func() {
				Expect(stagingDesirer.DeleteStagingCallCount()).To(Equal(1))

				taskName := stagingDesirer.DeleteStagingArgsForCall(0)
				Expect(taskName).To(Equal(taskCompletedRequest.TaskGUID))
			})
		})

		Context("and the task deletion fails", func() {
			BeforeEach(func() {
				stagingDesirer.DeleteStagingReturns(errors.New("delete boom"))
			})

			It("should return an error", func() {
				Expect(err).To(MatchError("delete boom"))
			})
		})
	})
})
