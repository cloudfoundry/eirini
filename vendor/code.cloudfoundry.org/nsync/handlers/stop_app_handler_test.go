package handlers_test

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"

	"code.cloudfoundry.org/bbs/fake_bbs"
	"code.cloudfoundry.org/bbs/models"
	"code.cloudfoundry.org/lager/lagertest"
	"code.cloudfoundry.org/nsync/handlers"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("StopAppHandler", func() {
	var (
		logger  *lagertest.TestLogger
		fakeBBS *fake_bbs.FakeClient

		request          *http.Request
		responseRecorder *httptest.ResponseRecorder
	)

	BeforeEach(func() {
		logger = lagertest.NewTestLogger("test")
		fakeBBS = new(fake_bbs.FakeClient)

		responseRecorder = httptest.NewRecorder()

		var err error
		request, err = http.NewRequest("DELETE", "", nil)
		Expect(err).NotTo(HaveOccurred())
		request.Form = url.Values{
			":process_guid": []string{"process-guid"},
		}
	})

	JustBeforeEach(func() {
		stopAppHandler := handlers.NewStopAppHandler(logger, fakeBBS)
		stopAppHandler.StopApp(responseRecorder, request)
	})

	It("invokes the bbs to delete the app", func() {
		Expect(fakeBBS.RemoveDesiredLRPCallCount()).To(Equal(1))
		_, desiredLRP := fakeBBS.RemoveDesiredLRPArgsForCall(0)
		Expect(desiredLRP).To(Equal("process-guid"))
	})

	It("responds with 202 Accepted", func() {
		Expect(responseRecorder.Code).To(Equal(http.StatusAccepted))
	})

	Context("when the bbs fails", func() {
		BeforeEach(func() {
			fakeBBS.RemoveDesiredLRPReturns(errors.New("oh no"))
		})

		It("responds with a ServiceUnavailabe error", func() {
			Expect(responseRecorder.Code).To(Equal(http.StatusServiceUnavailable))
		})
	})

	Context("when the process guid is missing", func() {
		BeforeEach(func() {
			request.Form.Del(":process_guid")
		})

		It("does not call the bbs", func() {
			Expect(fakeBBS.RemoveDesiredLRPCallCount()).To(Equal(0))
		})

		It("responds with 400 Bad Request", func() {
			Expect(responseRecorder.Code).To(Equal(http.StatusBadRequest))
		})
	})

	Context("when the lrp doesn't exist", func() {
		BeforeEach(func() {
			fakeBBS.RemoveDesiredLRPReturns(models.ErrResourceNotFound)
		})

		It("responds with a 404", func() {
			Expect(responseRecorder.Code).To(Equal(http.StatusNotFound))
		})
	})
})
