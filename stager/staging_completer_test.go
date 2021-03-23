package stager_test

import (
	"context"

	"code.cloudfoundry.org/eirini/models/cf"
	"code.cloudfoundry.org/eirini/stager"
	"code.cloudfoundry.org/eirini/stager/stagerfakes"
	"code.cloudfoundry.org/lager/lagertest"
	"code.cloudfoundry.org/runtimeschema/cc_messages"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/pkg/errors"
)

var _ = Describe("StagingCompleter", func() {
	var (
		taskCompletedRequest cf.StagingCompletedRequest
		callbackClient       *stagerfakes.FakeCallbackClient
		stagingCompleter     *stager.CallbackStagingCompleter
		err                  error
		ctx                  context.Context
	)

	BeforeEach(func() {
		ctx = context.Background()
		annotation := `{"completion_callback": "call/me/maybe"}`
		taskCompletedRequest = cf.StagingCompletedRequest{
			TaskGUID:      "our-task-guid",
			Failed:        false,
			FailureReason: "",
			Result:        `{"very": "good"}`,
			Annotation:    annotation,
		}

		callbackClient = new(stagerfakes.FakeCallbackClient)
		logger := lagertest.NewTestLogger("test")
		stagingCompleter = stager.NewCallbackStagingCompleter(logger, callbackClient)
	})

	JustBeforeEach(func() {
		err = stagingCompleter.CompleteStaging(ctx, taskCompletedRequest)
	})

	It("should not return an error", func() {
		Expect(err).ToNot(HaveOccurred())
	})

	It("should post the response", func() {
		Expect(callbackClient.PostCallCount()).To(Equal(1))
		_, url, data := callbackClient.PostArgsForCall(0)
		Expect(url).To(Equal("call/me/maybe"))

		response, ok := data.(cc_messages.StagingResponseForCC)
		Expect(ok).To(BeTrue())

		result, jsonErr := response.Result.MarshalJSON()
		Expect(jsonErr).NotTo(HaveOccurred())
		Expect(string(result)).To(Equal(`{"very": "good"}`))
	})

	Context("and the staging failed", func() {
		BeforeEach(func() {
			taskCompletedRequest.Failed = true
			taskCompletedRequest.FailureReason = "u broke my boy"
			taskCompletedRequest.Result = ""
		})

		It("should not return an error", func() {
			Expect(err).ToNot(HaveOccurred())
		})

		It("should post the response", func() {
			Expect(callbackClient.PostCallCount()).To(Equal(1))
			_, url, data := callbackClient.PostArgsForCall(0)
			Expect(url).To(Equal("call/me/maybe"))

			response, ok := data.(cc_messages.StagingResponseForCC)
			Expect(ok).To(BeTrue())
			Expect(response.Error.Id).To(Equal(cc_messages.STAGING_ERROR))
			Expect(response.Error.Message).To(Equal("u broke my boy"))
		})
	})

	Context("and the annotation is not a valid json", func() {
		BeforeEach(func() {
			taskCompletedRequest.Annotation = "{ !(valid json)"
		})

		It("should return an error", func() {
			Expect(err).To(HaveOccurred())
		})

		It("should not post the response", func() {
			Expect(callbackClient.PostCallCount()).To(BeZero())
		})
	})

	Context("and the callback response is an error", func() {
		BeforeEach(func() {
			callbackClient.PostReturns(errors.New("FAIL"))
		})

		It("should return an error", func() {
			Expect(err).To(MatchError(ContainSubstring("callback-response-unsuccessful")))
			Expect(err).To(MatchError(ContainSubstring("FAIL")))
		})
	})
})
