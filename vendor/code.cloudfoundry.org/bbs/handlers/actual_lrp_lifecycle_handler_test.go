package handlers_test

import (
	"errors"
	"net/http"
	"net/http/httptest"

	"code.cloudfoundry.org/bbs/handlers"
	"code.cloudfoundry.org/bbs/handlers/fake_controllers"
	"code.cloudfoundry.org/bbs/models"
	"code.cloudfoundry.org/bbs/serviceclient/serviceclientfakes"
	"code.cloudfoundry.org/lager/lagertest"
	"code.cloudfoundry.org/rep/repfakes"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
)

var _ = Describe("ActualLRP Lifecycle Handlers", func() {
	var (
		logger           *lagertest.TestLogger
		responseRecorder *httptest.ResponseRecorder
		handler          *handlers.ActualLRPLifecycleHandler
		fakeController   *fake_controllers.FakeActualLRPLifecycleController
		exitCh           chan struct{}
	)

	BeforeEach(func() {
		logger = lagertest.NewTestLogger("test")
		responseRecorder = httptest.NewRecorder()

		fakeServiceClient = new(serviceclientfakes.FakeServiceClient)
		fakeRepClientFactory = new(repfakes.FakeClientFactory)
		fakeRepClient = new(repfakes.FakeClient)
		fakeRepClientFactory.CreateClientReturns(fakeRepClient, nil)

		exitCh = make(chan struct{}, 1)
		fakeController = &fake_controllers.FakeActualLRPLifecycleController{}
		handler = handlers.NewActualLRPLifecycleHandler(fakeController, exitCh)
	})

	Describe("ClaimActualLRP", func() {
		var (
			processGuid       = "process-guid"
			index       int32 = 1
			instanceKey models.ActualLRPInstanceKey
			requestBody interface{}
		)

		BeforeEach(func() {
			instanceKey = models.NewActualLRPInstanceKey(
				"instance-guid-0",
				"cell-id-0",
			)
			requestBody = &instanceKey
			requestBody = &models.ClaimActualLRPRequest{
				ProcessGuid:          processGuid,
				Index:                index,
				ActualLrpInstanceKey: &instanceKey,
			}
		})

		JustBeforeEach(func() {
			request := newTestRequest(requestBody)
			handler.ClaimActualLRP(logger, responseRecorder, request)
		})

		It("calls the controller", func() {
			Expect(fakeController.ClaimActualLRPCallCount()).To(Equal(1))
			_, actualProcessGuid, actualIndex, actualInstanceKey := fakeController.ClaimActualLRPArgsForCall(0)
			Expect(actualProcessGuid).To(Equal(processGuid))
			Expect(actualIndex).To(Equal(index))
			Expect(actualInstanceKey).To(Equal(&instanceKey))
		})

		Context("when the controller call succeeds", func() {
			BeforeEach(func() {
				fakeController.ClaimActualLRPReturns(nil)
			})

			It("response with no error", func() {
				Expect(responseRecorder.Code).To(Equal(http.StatusOK))
				response := &models.ActualLRPLifecycleResponse{}
				err := response.Unmarshal(responseRecorder.Body.Bytes())
				Expect(err).NotTo(HaveOccurred())

				Expect(response.Error).To(BeNil())
			})
		})

		Context("when the DB returns an unrecoverable error", func() {
			BeforeEach(func() {
				fakeController.ClaimActualLRPReturns(models.NewUnrecoverableError(nil))
			})

			It("logs and writes to the exit channel", func() {
				Eventually(logger).Should(gbytes.Say("unrecoverable-error"))
				Eventually(exitCh).Should(Receive())
			})
		})

		Context("when claiming the actual lrp fails", func() {
			BeforeEach(func() {
				fakeController.ClaimActualLRPReturns(models.ErrUnknownError)
			})

			It("responds with an error", func() {
				Expect(responseRecorder.Code).To(Equal(http.StatusOK))
				response := &models.ActualLRPLifecycleResponse{}
				err := response.Unmarshal(responseRecorder.Body.Bytes())
				Expect(err).NotTo(HaveOccurred())

				Expect(response.Error).To(Equal(models.ErrUnknownError))
			})
		})
	})

	Describe("StartActualLRP", func() {
		var (
			processGuid = "process-guid"
			index       = int32(1)

			key         models.ActualLRPKey
			instanceKey models.ActualLRPInstanceKey
			netInfo     models.ActualLRPNetInfo

			requestBody interface{}
		)

		BeforeEach(func() {
			key = models.NewActualLRPKey(
				processGuid,
				index,
				"domain-0",
			)
			instanceKey = models.NewActualLRPInstanceKey(
				"instance-guid-0",
				"cell-id-0",
			)
			netInfo = models.NewActualLRPNetInfo("1.1.1.1", "2.2.2.2", models.NewPortMapping(10, 20))
			requestBody = &models.StartActualLRPRequest{
				ActualLrpKey:         &key,
				ActualLrpInstanceKey: &instanceKey,
				ActualLrpNetInfo:     &netInfo,
			}
		})

		JustBeforeEach(func() {
			request := newTestRequest(requestBody)
			handler.StartActualLRP(logger, responseRecorder, request)
		})

		It("calls the controller", func() {
			Expect(fakeController.StartActualLRPCallCount()).To(Equal(1))
			_, actualKey, actualInstanceKey, actualNetInfo := fakeController.StartActualLRPArgsForCall(0)
			Expect(actualKey).To(Equal(&key))
			Expect(actualInstanceKey).To(Equal(&instanceKey))
			Expect(actualNetInfo).To(Equal(&netInfo))
		})

		Context("when starting the actual lrp in the DB succeeds", func() {
			BeforeEach(func() {
				fakeController.StartActualLRPReturns(nil)
			})

			It("responds with no error", func() {
				Expect(responseRecorder.Code).To(Equal(http.StatusOK))
				response := &models.ActualLRPLifecycleResponse{}
				err := response.Unmarshal(responseRecorder.Body.Bytes())
				Expect(err).NotTo(HaveOccurred())

				Expect(response.Error).To(BeNil())
			})
		})

		Context("when an unrecoverable error is returned", func() {
			BeforeEach(func() {
				fakeController.StartActualLRPReturns(models.NewUnrecoverableError(nil))
			})

			It("logs and writes to the exit channel", func() {
				Eventually(logger).Should(gbytes.Say("unrecoverable-error"))
				Eventually(exitCh).Should(Receive())
			})
		})

		Context("when a recoverable error is returned", func() {
			BeforeEach(func() {
				fakeController.StartActualLRPReturns(models.ErrUnknownError)
			})

			It("responds with an error", func() {
				Expect(responseRecorder.Code).To(Equal(http.StatusOK))
				response := &models.ActualLRPLifecycleResponse{}
				err := response.Unmarshal(responseRecorder.Body.Bytes())
				Expect(err).NotTo(HaveOccurred())

				Expect(response.Error).To(Equal(models.ErrUnknownError))
			})
		})
	})

	Describe("CrashActualLRP", func() {
		var (
			processGuid  = "process-guid"
			index        = int32(1)
			instanceGuid = "instance-guid"
			cellId       = "cell-id"

			key          models.ActualLRPKey
			instanceKey  models.ActualLRPInstanceKey
			errorMessage string

			requestBody interface{}
		)

		BeforeEach(func() {
			key = models.NewActualLRPKey(
				processGuid,
				index,
				"domain-0",
			)
			instanceKey = models.NewActualLRPInstanceKey(instanceGuid, cellId)
			errorMessage = "something went wrong"
			requestBody = &models.CrashActualLRPRequest{
				ActualLrpKey:         &key,
				ActualLrpInstanceKey: &instanceKey,
				ErrorMessage:         errorMessage,
			}
		})

		JustBeforeEach(func() {
			request := newTestRequest(requestBody)
			handler.CrashActualLRP(logger, responseRecorder, request)
		})

		It("calls the controller", func() {
			Expect(fakeController.CrashActualLRPCallCount()).To(Equal(1))
			_, actualKey, actualInstanceKey, actualErrorMessage := fakeController.CrashActualLRPArgsForCall(0)
			Expect(actualKey).To(Equal(&key))
			Expect(actualInstanceKey).To(Equal(&instanceKey))
			Expect(actualErrorMessage).To(Equal(errorMessage))
		})

		Context("when crashing the actual lrp succeeds", func() {
			BeforeEach(func() {
				fakeController.CrashActualLRPReturns(nil)
			})

			It("response with no error", func() {
				Expect(responseRecorder.Code).To(Equal(http.StatusOK))
				response := &models.ActualLRPLifecycleResponse{}
				err := response.Unmarshal(responseRecorder.Body.Bytes())
				Expect(err).NotTo(HaveOccurred())

				Expect(response.Error).To(BeNil())
			})
		})

		Context("when an unrecoverable error is returned", func() {
			BeforeEach(func() {
				fakeController.CrashActualLRPReturns(models.NewUnrecoverableError(nil))
			})

			It("logs and writes to the exit channel", func() {
				Eventually(logger).Should(gbytes.Say("unrecoverable-error"))
				Eventually(exitCh).Should(Receive())
			})
		})

		Context("when a recoverable error is returned", func() {
			BeforeEach(func() {
				fakeController.CrashActualLRPReturns(models.ErrUnknownError)
			})

			It("responds with an error", func() {
				Expect(responseRecorder.Code).To(Equal(http.StatusOK))
				response := &models.ActualLRPLifecycleResponse{}
				err := response.Unmarshal(responseRecorder.Body.Bytes())
				Expect(err).NotTo(HaveOccurred())

				Expect(response.Error).To(Equal(models.ErrUnknownError))
			})
		})
	})

	Describe("RetireActualLRP", func() {
		var (
			request     *http.Request
			response    *models.ActualLRPLifecycleResponse
			processGuid = "process-guid"
			index       = int32(1)

			key models.ActualLRPKey

			requestBody interface{}
		)

		BeforeEach(func() {
			key = models.NewActualLRPKey(
				processGuid,
				index,
				"domain-0",
			)

			requestBody = &models.RetireActualLRPRequest{
				ActualLrpKey: &key,
			}
		})

		JustBeforeEach(func() {
			request = newTestRequest(requestBody)
			handler.RetireActualLRP(logger, responseRecorder, request)

			response = &models.ActualLRPLifecycleResponse{}
			err := response.Unmarshal(responseRecorder.Body.Bytes())
			Expect(err).NotTo(HaveOccurred())
		})

		It("calls the controller", func() {
			Expect(fakeController.RetireActualLRPCallCount()).To(Equal(1))
			_, actualKey := fakeController.RetireActualLRPArgsForCall(0)
			Expect(actualKey).To(Equal(&key))
		})

		Context("when an unrecoverable error is returned", func() {
			BeforeEach(func() {
				fakeController.RetireActualLRPReturns(models.NewUnrecoverableError(nil))
			})

			It("logs and writes to the exit channel", func() {
				Eventually(logger).Should(gbytes.Say("unrecoverable-error"))
				Eventually(exitCh).Should(Receive())
			})
		})

		Context("when a recoverable error is returned", func() {
			BeforeEach(func() {
				fakeController.RetireActualLRPReturns(errors.New("could not find lrp"))
			})

			It("returns an error and does not retry", func() {
				Expect(response.Error.Message).To(Equal("could not find lrp"))
			})
		})
	})

	Describe("FailActualLRP", func() {
		var (
			request     *http.Request
			processGuid = "process-guid"
			index       = int32(1)

			key          models.ActualLRPKey
			errorMessage string

			requestBody interface{}
		)

		BeforeEach(func() {
			key = models.NewActualLRPKey(
				processGuid,
				index,
				"domain-0",
			)
			errorMessage = "something went wrong"
			requestBody = &models.FailActualLRPRequest{
				ActualLrpKey: &key,
				ErrorMessage: errorMessage,
			}
		})

		JustBeforeEach(func() {
			request = newTestRequest(requestBody)
			handler.FailActualLRP(logger, responseRecorder, request)
		})

		It("calls the controller", func() {
			Expect(fakeController.FailActualLRPCallCount()).To(Equal(1))
			_, actualKey, actualErrorMessage := fakeController.FailActualLRPArgsForCall(0)
			Expect(actualKey).To(Equal(&key))
			Expect(actualErrorMessage).To(Equal(errorMessage))
		})

		Context("when it succeeds", func() {
			BeforeEach(func() {
				fakeController.FailActualLRPReturns(nil)
			})

			It("fails the actual lrp by process guid and index", func() {
				Expect(responseRecorder.Code).To(Equal(http.StatusOK))

				response := &models.ActualLRPLifecycleResponse{}
				err := response.Unmarshal(responseRecorder.Body.Bytes())
				Expect(err).NotTo(HaveOccurred())

				Expect(response.Error).To(BeNil())
			})
		})

		Context("when an unrecoverable error is returned", func() {
			BeforeEach(func() {
				fakeController.FailActualLRPReturns(models.NewUnrecoverableError(nil))
			})

			It("logs and writes to the exit channel", func() {
				Eventually(logger).Should(gbytes.Say("unrecoverable-error"))
				Eventually(exitCh).Should(Receive())
			})
		})

		Context("when failing the actual lrp fails", func() {
			BeforeEach(func() {
				fakeController.FailActualLRPReturns(models.ErrUnknownError)
			})

			It("responds with an error", func() {
				Expect(responseRecorder.Code).To(Equal(http.StatusOK))
				response := &models.ActualLRPLifecycleResponse{}
				err := response.Unmarshal(responseRecorder.Body.Bytes())
				Expect(err).NotTo(HaveOccurred())

				Expect(response.Error).To(Equal(models.ErrUnknownError))
			})
		})
	})

	Describe("RemoveActualLRP", func() {
		var (
			processGuid       = "process-guid"
			index       int32 = 1

			instanceKey models.ActualLRPInstanceKey

			requestBody interface{}
		)

		BeforeEach(func() {
			instanceKey = models.NewActualLRPInstanceKey(
				"instance-guid-0",
				"cell-id-0",
			)

			requestBody = &models.RemoveActualLRPRequest{
				ProcessGuid:          processGuid,
				Index:                index,
				ActualLrpInstanceKey: &instanceKey,
			}
		})

		JustBeforeEach(func() {
			request := newTestRequest(requestBody)
			handler.RemoveActualLRP(logger, responseRecorder, request)
		})

		It("calls the controller", func() {
			Expect(fakeController.RemoveActualLRPCallCount()).To(Equal(1))
			_, actualProcessGuid, actualIndex, actualInstanceKey := fakeController.RemoveActualLRPArgsForCall(0)
			Expect(actualProcessGuid).To(Equal(processGuid))
			Expect(actualIndex).To(Equal(index))
			Expect(actualInstanceKey).To(Equal(&instanceKey))
		})

		Context("when it succeeds", func() {
			BeforeEach(func() {
				fakeController.RemoveActualLRPReturns(nil)
			})

			It("responds with no error", func() {
				Expect(responseRecorder.Code).To(Equal(http.StatusOK))
				response := &models.ActualLRPLifecycleResponse{}

				err := response.Unmarshal(responseRecorder.Body.Bytes())
				Expect(err).NotTo(HaveOccurred())
				Expect(response.Error).To(BeNil())
			})
		})

		Context("when an unrecoverable error is returned", func() {
			BeforeEach(func() {
				fakeController.RemoveActualLRPReturns(models.NewUnrecoverableError(nil))
			})

			It("logs and writes to the exit channel", func() {
				Eventually(logger).Should(gbytes.Say("unrecoverable-error"))
				Eventually(exitCh).Should(Receive())
			})
		})

		Context("when a recoverable error is returned", func() {
			BeforeEach(func() {
				fakeController.RemoveActualLRPReturns(models.ErrUnknownError)
			})

			It("responds with an error", func() {
				Expect(responseRecorder.Code).To(Equal(http.StatusOK))
				response := &models.ActualLRPLifecycleResponse{}
				err := response.Unmarshal(responseRecorder.Body.Bytes())
				Expect(err).NotTo(HaveOccurred())

				Expect(response.Error).To(Equal(models.ErrUnknownError))
			})
		})
	})
})
