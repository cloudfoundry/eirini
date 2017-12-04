package handlers_test

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"

	"code.cloudfoundry.org/bbs/fake_bbs"
	"code.cloudfoundry.org/bbs/models"
	"code.cloudfoundry.org/bbs/models/test/model_helpers"
	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/lager/lagertest"
	"code.cloudfoundry.org/nsync/handlers"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("KillIndexHandler", func() {
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
		request, err = http.NewRequest("POST", "", nil)
		Expect(err).NotTo(HaveOccurred())
		request.Form = url.Values{
			":process_guid": []string{"process-guid-0"},
			":index":        []string{"1"},
		}

		fakeBBS.ActualLRPGroupByProcessGuidAndIndexStub = func(logger lager.Logger, processGuid string, index int) (*models.ActualLRPGroup, error) {
			return &models.ActualLRPGroup{
				Instance: model_helpers.NewValidActualLRP(processGuid, int32(index)),
			}, nil
		}
	})

	JustBeforeEach(func() {
		killHandler := handlers.NewKillIndexHandler(logger, fakeBBS)
		killHandler.KillIndex(responseRecorder, request)
	})

	It("invokes the bbs to retire", func() {
		Expect(fakeBBS.RetireActualLRPCallCount()).To(Equal(1))

		_, actualLRPKey := fakeBBS.RetireActualLRPArgsForCall(0)
		Expect(actualLRPKey.ProcessGuid).To(Equal("process-guid-0"))
		Expect(actualLRPKey.Index).To(BeEquivalentTo(1))
	})

	It("responds with 202 Accepted", func() {
		Expect(responseRecorder.Code).To(Equal(http.StatusAccepted))
	})

	Context("when the bbs fails", func() {
		BeforeEach(func() {
			fakeBBS.ActualLRPGroupByProcessGuidAndIndexReturns(nil, errors.New("oh no"))
		})

		It("responds with a ServiceUnavailable error", func() {
			Expect(responseRecorder.Code).To(Equal(http.StatusServiceUnavailable))
		})
	})

	Context("when the bbs cannot find the guid", func() {
		BeforeEach(func() {
			fakeBBS.ActualLRPGroupByProcessGuidAndIndexReturns(nil, models.ErrResourceNotFound)
		})

		It("responds with a NotFound error", func() {
			Expect(responseRecorder.Code).To(Equal(http.StatusNotFound))
		})
	})

	Context("when the process guid is missing", func() {
		BeforeEach(func() {
			request.Form.Del(":process_guid")
		})

		It("does not call the bbs", func() {
			Expect(fakeBBS.ActualLRPGroupByProcessGuidAndIndexCallCount()).To(BeZero())
		})

		It("responds with 400 Bad Request", func() {
			Expect(responseRecorder.Code).To(Equal(http.StatusBadRequest))
		})
	})

	Context("when the index is missing", func() {
		BeforeEach(func() {
			request.Form.Del(":index")
		})

		It("does not call the bbs", func() {
			Expect(fakeBBS.ActualLRPGroupByProcessGuidAndIndexCallCount()).To(BeZero())
		})

		It("responds with 400 Bad Request", func() {
			Expect(responseRecorder.Code).To(Equal(http.StatusBadRequest))
		})
	})

	Context("when the index is not a number", func() {
		BeforeEach(func() {
			request.Form.Set(":index", "not-a-number")
		})

		It("does not call the bbs", func() {
			Expect(fakeBBS.ActualLRPGroupByProcessGuidAndIndexCallCount()).To(BeZero())
		})

		It("responds with 400 Bad Request", func() {
			Expect(responseRecorder.Code).To(Equal(http.StatusBadRequest))
		})
	})

	Context("when the index is out of range", func() {
		BeforeEach(func() {
			request.Form.Set(":index", "5")
			fakeBBS.ActualLRPGroupByProcessGuidAndIndexReturns(nil, models.ErrResourceNotFound)
		})

		It("does call the bbs", func() {
			Expect(fakeBBS.ActualLRPGroupByProcessGuidAndIndexCallCount()).To(Equal(1))
		})

		It("responds with 404 Not Found", func() {
			Expect(responseRecorder.Code).To(Equal(http.StatusNotFound))
		})
	})
})
