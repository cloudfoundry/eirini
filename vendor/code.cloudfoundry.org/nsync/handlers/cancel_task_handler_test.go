package handlers_test

import (
	"net/http"
	"net/http/httptest"
	"net/url"

	"code.cloudfoundry.org/bbs/fake_bbs"
	"code.cloudfoundry.org/bbs/models"
	"code.cloudfoundry.org/lager/lagertest"
	"code.cloudfoundry.org/nsync/handlers"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
)

var _ = Describe("CancelTaskHandler", func() {
	var (
		logger        *lagertest.TestLogger
		fakeBBSClient *fake_bbs.FakeClient

		request          *http.Request
		responseRecorder *httptest.ResponseRecorder
	)

	BeforeEach(func() {
		var err error

		logger = lagertest.NewTestLogger("test")
		fakeBBSClient = new(fake_bbs.FakeClient)

		responseRecorder = httptest.NewRecorder()

		request, err = http.NewRequest("DELETE", "", nil)
		Expect(err).NotTo(HaveOccurred())
		request.Form = url.Values{
			":task_guid": []string{"some-guid"},
		}
	})

	JustBeforeEach(func() {
		handler := handlers.NewCancelTaskHandler(logger, fakeBBSClient)
		handler.CancelTask(responseRecorder, request)
	})

	It("logs the incoming and outgoing request", func() {
		Eventually(logger.TestSink.Buffer).Should(gbytes.Say("serving"))
		Eventually(logger.TestSink.Buffer).Should(gbytes.Say("canceling-task"))
	})

	Context("when the task does not exist", func() {
		BeforeEach(func() {
			fakeBBSClient.CancelTaskReturns(models.ErrResourceNotFound)
		})

		It("responds with 404 Not Found", func() {
			Expect(responseRecorder.Code).To(Equal(http.StatusNotFound))
		})
	})

	Context("when the bbs responds with an unknown error", func() {
		BeforeEach(func() {
			fakeBBSClient.CancelTaskReturns(models.ErrUnknownError)
		})

		It("responds with 500 Internal Server Error", func() {
			Expect(responseRecorder.Code).To(Equal(http.StatusInternalServerError))
		})
	})

	Context("when the task exists", func() {
		It("cancels the task", func() {
			Expect(fakeBBSClient.CancelTaskCallCount()).To(Equal(1))
			Expect(responseRecorder.Code).To(Equal(http.StatusAccepted))
		})
	})
})
