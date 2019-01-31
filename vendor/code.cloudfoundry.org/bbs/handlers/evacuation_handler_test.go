package handlers_test

import (
	"net/http"
	"net/http/httptest"

	"code.cloudfoundry.org/bbs/handlers"
	"code.cloudfoundry.org/bbs/handlers/fake_controllers"
	"code.cloudfoundry.org/bbs/models"
	"code.cloudfoundry.org/bbs/models/test/model_helpers"
	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/lager/lagertest"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
)

var _ = Describe("Evacuation Handlers", func() {
	var (
		logger           lager.Logger
		responseRecorder *httptest.ResponseRecorder
		handler          *handlers.EvacuationHandler
		exitCh           chan struct{}

		key         models.ActualLRPKey
		instanceKey models.ActualLRPInstanceKey
		controller  *fake_controllers.FakeEvacuationController
	)

	BeforeEach(func() {
		logger = lagertest.NewTestLogger("test")
		logger.RegisterSink(lager.NewWriterSink(GinkgoWriter, lager.DEBUG))
		responseRecorder = httptest.NewRecorder()
		exitCh = make(chan struct{}, 1)

		controller = &fake_controllers.FakeEvacuationController{}
		handler = handlers.NewEvacuationHandler(controller, exitCh)

		key = models.ActualLRPKey{
			ProcessGuid: "some-guid",
			Index:       1,
			Domain:      "some-domain",
		}

		instanceKey = models.ActualLRPInstanceKey{
			InstanceGuid: "some-guid",
			CellId:       "some-cell",
		}
	})

	Describe("RemoveEvacuatingActualLRP", func() {
		var (
			requestBody interface{}
		)

		BeforeEach(func() {
			requestBody = &models.RemoveEvacuatingActualLRPRequest{
				ActualLrpKey:         &key,
				ActualLrpInstanceKey: &instanceKey,
			}
		})

		JustBeforeEach(func() {
			request := newTestRequest(requestBody)
			handler.RemoveEvacuatingActualLRP(logger, responseRecorder, request)
		})

		Context("when the request is valid", func() {
			Context("when the controller succeeds removing the evacuating actual lrp", func() {
				It("should respond without an error", func() {
					Expect(responseRecorder.Code).To(Equal(http.StatusOK))
					var response models.RemoveEvacuatingActualLRPResponse
					err := response.Unmarshal(responseRecorder.Body.Bytes())
					Expect(err).NotTo(HaveOccurred())
					Expect(response.Error).To(BeNil())
				})
			})

			Context("when the controller returns an error", func() {
				Context("if the error is recoverable", func() {
					BeforeEach(func() {
						controller.RemoveEvacuatingActualLRPReturns(models.ErrUnknownError)
					})

					It("should return the error in the response", func() {
						Expect(responseRecorder.Code).To(Equal(http.StatusOK))
						var response models.RemoveEvacuatingActualLRPResponse
						err := response.Unmarshal(responseRecorder.Body.Bytes())
						Expect(err).NotTo(HaveOccurred())
						Expect(response.Error).NotTo(BeNil())
						Expect(response.Error).To(Equal(models.ErrUnknownError))
					})
				})

				Context("if the error is unrecoverable", func() {
					BeforeEach(func() {
						controller.RemoveEvacuatingActualLRPReturns(models.NewUnrecoverableError(nil))
					})

					It("logs and writes to the exit channel", func() {
						Eventually(logger).Should(gbytes.Say("unrecoverable-error"))
						Eventually(exitCh).Should(Receive())
					})
				})
			})
		})

		Context("when the request is invalid", func() {
			BeforeEach(func() {
				requestBody = &models.RemoveEvacuatingActualLRPRequest{}
			})

			It("responds with an error", func() {
				Expect(responseRecorder.Code).To(Equal(http.StatusOK))

				var response models.RemoveEvacuatingActualLRPResponse
				err := response.Unmarshal(responseRecorder.Body.Bytes())
				Expect(err).NotTo(HaveOccurred())

				Expect(response.Error).NotTo(BeNil())
				Expect(response.Error.Type).To(Equal(models.Error_InvalidRequest))
			})
		})

		Context("when parsing the body fails", func() {
			BeforeEach(func() {
				requestBody = "beep boop beep boop -- i am a robot"
			})

			It("responds with an error", func() {
				Expect(responseRecorder.Code).To(Equal(http.StatusOK))

				var response models.RemoveEvacuatingActualLRPResponse
				err := response.Unmarshal(responseRecorder.Body.Bytes())
				Expect(err).NotTo(HaveOccurred())

				Expect(response.Error).NotTo(BeNil())
				Expect(response.Error).To(Equal(models.ErrBadRequest))
			})
		})

	})

	Describe("EvacuateClaimedActualLRP", func() {
		var (
			request     *http.Request
			requestBody *models.EvacuateClaimedActualLRPRequest
			actual      *models.ActualLRP
		)

		Context("when request is valid", func() {
			JustBeforeEach(func() {
				actual = model_helpers.NewValidActualLRP("process-guid", 1)
				requestBody = &models.EvacuateClaimedActualLRPRequest{
					ActualLrpKey:         &actual.ActualLRPKey,
					ActualLrpInstanceKey: &actual.ActualLRPInstanceKey,
				}
				request = newTestRequest(requestBody)
				handler.EvacuateClaimedActualLRP(logger, responseRecorder, request)
				Expect(responseRecorder.Code).To(Equal(http.StatusOK))
			})

			Context("when the controller succeeds in evacuating the actual lrp", func() {
				Context("when keepContainer is false", func() {
					BeforeEach(func() {
						controller.EvacuateClaimedActualLRPReturns(false, nil)
					})

					It("should return no error and keep the container", func() {
						Expect(responseRecorder.Code).To(Equal(http.StatusOK))
						var response models.EvacuationResponse
						err := response.Unmarshal(responseRecorder.Body.Bytes())
						Expect(err).NotTo(HaveOccurred())
						Expect(response.Error).To(BeNil())
						Expect(response.KeepContainer).To(BeFalse())
					})
				})

				Context("when keepContainer is true", func() {
					BeforeEach(func() {
						controller.EvacuateClaimedActualLRPReturns(true, nil)
					})

					It("should return no error and keep the container", func() {
						Expect(responseRecorder.Code).To(Equal(http.StatusOK))
						var response models.EvacuationResponse
						err := response.Unmarshal(responseRecorder.Body.Bytes())
						Expect(err).NotTo(HaveOccurred())
						Expect(response.Error).To(BeNil())
						Expect(response.KeepContainer).To(BeTrue())
					})
				})
			})

			Context("when the controller returns an error", func() {
				Context("if the error is recoverable and keepContainer is false", func() {
					BeforeEach(func() {
						controller.EvacuateClaimedActualLRPReturns(false, models.ErrUnknownError)
					})

					It("should return the error in the response", func() {
						Expect(responseRecorder.Code).To(Equal(http.StatusOK))
						var response models.EvacuationResponse
						err := response.Unmarshal(responseRecorder.Body.Bytes())
						Expect(err).NotTo(HaveOccurred())
						Expect(response.Error).NotTo(BeNil())
						Expect(response.Error).To(Equal(models.ErrUnknownError))
						Expect(response.KeepContainer).To(BeFalse())
					})
				})

				Context("if the error is recoverable and keepContainer is true", func() {
					BeforeEach(func() {
						controller.EvacuateClaimedActualLRPReturns(true, models.ErrUnknownError)
					})

					It("should return the error in the response", func() {
						Expect(responseRecorder.Code).To(Equal(http.StatusOK))
						var response models.EvacuationResponse
						err := response.Unmarshal(responseRecorder.Body.Bytes())
						Expect(err).NotTo(HaveOccurred())
						Expect(response.Error).NotTo(BeNil())
						Expect(response.Error).To(Equal(models.ErrUnknownError))
						Expect(response.KeepContainer).To(BeTrue())
					})
				})

				Context("if the error is unrecoverable", func() {
					BeforeEach(func() {
						controller.EvacuateClaimedActualLRPReturns(false, models.NewUnrecoverableError(nil))
					})

					It("logs and writes to the exit channel", func() {
						Eventually(logger).Should(gbytes.Say("unrecoverable-error"))
						Eventually(exitCh).Should(Receive())
					})
				})
			})
		})

		Context("when the request is invalid", func() {
			BeforeEach(func() {
				request = newTestRequest("{{")
				handler.EvacuateClaimedActualLRP(logger, responseRecorder, request)
				Expect(responseRecorder.Code).To(Equal(http.StatusOK))
			})

			It("returns an error and keeps the container", func() {
				response := models.EvacuationResponse{}
				err := response.Unmarshal(responseRecorder.Body.Bytes())
				Expect(err).NotTo(HaveOccurred())
				Expect(response.KeepContainer).To(BeTrue())
				Expect(response.Error).NotTo(BeNil())
				Expect(response.Error).To(Equal(models.ErrBadRequest))
			})
		})
	})

	Describe("EvacuateCrashedActualLRP", func() {
		var (
			request     *http.Request
			requestBody *models.EvacuateCrashedActualLRPRequest
			actual      *models.ActualLRP
		)

		Context("when request is valid", func() {
			JustBeforeEach(func() {
				actual = model_helpers.NewValidActualLRP("process-guid", 1)
				requestBody = &models.EvacuateCrashedActualLRPRequest{
					ActualLrpKey:         &actual.ActualLRPKey,
					ActualLrpInstanceKey: &actual.ActualLRPInstanceKey,
					ErrorMessage:         "i failed",
				}
				request = newTestRequest(requestBody)
				handler.EvacuateCrashedActualLRP(logger, responseRecorder, request)
				Expect(responseRecorder.Code).To(Equal(http.StatusOK))
			})

			Context("when the controller succeeds in evacuating the actual lrp", func() {
				BeforeEach(func() {
					controller.EvacuateCrashedActualLRPReturns(nil)
				})

				It("should return no error", func() {
					Expect(responseRecorder.Code).To(Equal(http.StatusOK))
					var response models.EvacuationResponse
					err := response.Unmarshal(responseRecorder.Body.Bytes())
					Expect(err).NotTo(HaveOccurred())
					Expect(response.Error).To(BeNil())
					Expect(response.KeepContainer).To(BeFalse())
				})
			})

			Context("when the controller returns an error", func() {
				Context("if the error is recoverable", func() {
					BeforeEach(func() {
						controller.EvacuateCrashedActualLRPReturns(models.ErrUnknownError)
					})

					It("should return the error in the response", func() {
						Expect(responseRecorder.Code).To(Equal(http.StatusOK))
						var response models.EvacuationResponse
						err := response.Unmarshal(responseRecorder.Body.Bytes())
						Expect(err).NotTo(HaveOccurred())
						Expect(response.Error).NotTo(BeNil())
						Expect(response.Error).To(Equal(models.ErrUnknownError))
						Expect(response.KeepContainer).To(BeFalse())
					})
				})

				Context("if the error is unrecoverable", func() {
					BeforeEach(func() {
						controller.EvacuateCrashedActualLRPReturns(models.NewUnrecoverableError(nil))
					})

					It("logs and writes to the exit channel", func() {
						Eventually(logger).Should(gbytes.Say("unrecoverable-error"))
						Eventually(exitCh).Should(Receive())
					})
				})
			})
		})

		Context("when the request is invalid", func() {
			BeforeEach(func() {
				request = newTestRequest("{{")
				handler.EvacuateCrashedActualLRP(logger, responseRecorder, request)
				Expect(responseRecorder.Code).To(Equal(http.StatusOK))
			})

			It("returns an error and keeps the container", func() {
				response := models.EvacuationResponse{}
				err := response.Unmarshal(responseRecorder.Body.Bytes())
				Expect(err).NotTo(HaveOccurred())
				Expect(response.Error).NotTo(BeNil())
				Expect(response.Error).To(Equal(models.ErrBadRequest))
			})
		})
	})

	Describe("EvacuateRunningActualLRP", func() {
		var (
			request     *http.Request
			requestBody *models.EvacuateRunningActualLRPRequest
			actual      *models.ActualLRP
		)

		Context("when request is valid", func() {
			JustBeforeEach(func() {
				actual = model_helpers.NewValidActualLRP("process-guid", 1)
				requestBody = &models.EvacuateRunningActualLRPRequest{
					ActualLrpKey:         &actual.ActualLRPKey,
					ActualLrpInstanceKey: &actual.ActualLRPInstanceKey,
					ActualLrpNetInfo:     &actual.ActualLRPNetInfo,
				}
				request = newTestRequest(requestBody)
				handler.EvacuateRunningActualLRP(logger, responseRecorder, request)
				Expect(responseRecorder.Code).To(Equal(http.StatusOK))
			})

			Context("when the controller succeeds in evacuating the actual lrp", func() {
				Context("when keepContainer is false", func() {
					BeforeEach(func() {
						controller.EvacuateRunningActualLRPReturns(false, nil)
					})

					It("should return no error and keep the container", func() {
						Expect(responseRecorder.Code).To(Equal(http.StatusOK))
						var response models.EvacuationResponse
						err := response.Unmarshal(responseRecorder.Body.Bytes())
						Expect(err).NotTo(HaveOccurred())
						Expect(response.Error).To(BeNil())
						Expect(response.KeepContainer).To(BeFalse())
					})
				})

				Context("when keepContainer is true", func() {
					BeforeEach(func() {
						controller.EvacuateRunningActualLRPReturns(true, nil)
					})

					It("should return no error and keep the container", func() {
						Expect(responseRecorder.Code).To(Equal(http.StatusOK))
						var response models.EvacuationResponse
						err := response.Unmarshal(responseRecorder.Body.Bytes())
						Expect(err).NotTo(HaveOccurred())
						Expect(response.Error).To(BeNil())
						Expect(response.KeepContainer).To(BeTrue())
					})
				})
			})

			Context("when the controller returns an error", func() {
				Context("if the error is recoverable and keepContainer is false", func() {
					BeforeEach(func() {
						controller.EvacuateRunningActualLRPReturns(false, models.ErrUnknownError)
					})

					It("should return the error in the response", func() {
						Expect(responseRecorder.Code).To(Equal(http.StatusOK))
						var response models.EvacuationResponse
						err := response.Unmarshal(responseRecorder.Body.Bytes())
						Expect(err).NotTo(HaveOccurred())
						Expect(response.Error).NotTo(BeNil())
						Expect(response.Error).To(Equal(models.ErrUnknownError))
						Expect(response.KeepContainer).To(BeFalse())
					})
				})

				Context("if the error is recoverable and keepContainer is true", func() {
					BeforeEach(func() {
						controller.EvacuateRunningActualLRPReturns(true, models.ErrUnknownError)
					})

					It("should return the error in the response", func() {
						Expect(responseRecorder.Code).To(Equal(http.StatusOK))
						var response models.EvacuationResponse
						err := response.Unmarshal(responseRecorder.Body.Bytes())
						Expect(err).NotTo(HaveOccurred())
						Expect(response.Error).NotTo(BeNil())
						Expect(response.Error).To(Equal(models.ErrUnknownError))
						Expect(response.KeepContainer).To(BeTrue())
					})
				})

				Context("if the error is unrecoverable", func() {
					BeforeEach(func() {
						controller.EvacuateRunningActualLRPReturns(false, models.NewUnrecoverableError(nil))
					})

					It("logs and writes to the exit channel", func() {
						Eventually(logger).Should(gbytes.Say("unrecoverable-error"))
						Eventually(exitCh).Should(Receive())
					})
				})
			})
		})

		Context("when the request is invalid", func() {
			BeforeEach(func() {
				request = newTestRequest("{{")
				handler.EvacuateRunningActualLRP(logger, responseRecorder, request)
				Expect(responseRecorder.Code).To(Equal(http.StatusOK))
			})

			It("returns an error and keeps the container", func() {
				response := models.EvacuationResponse{}
				err := response.Unmarshal(responseRecorder.Body.Bytes())
				Expect(err).NotTo(HaveOccurred())
				Expect(response.KeepContainer).To(BeTrue())
				Expect(response.Error).NotTo(BeNil())
				Expect(response.Error).To(Equal(models.ErrBadRequest))
			})
		})
	})

	Describe("EvacuateStoppedActualLRP", func() {
		var (
			request     *http.Request
			requestBody *models.EvacuateStoppedActualLRPRequest
			actual      *models.ActualLRP
		)

		Context("when request is valid", func() {
			JustBeforeEach(func() {
				actual = model_helpers.NewValidActualLRP("process-guid", 1)
				requestBody = &models.EvacuateStoppedActualLRPRequest{
					ActualLrpKey:         &actual.ActualLRPKey,
					ActualLrpInstanceKey: &actual.ActualLRPInstanceKey,
				}
				request = newTestRequest(requestBody)
				handler.EvacuateStoppedActualLRP(logger, responseRecorder, request)
				Expect(responseRecorder.Code).To(Equal(http.StatusOK))
			})

			Context("when the controller succeeds in evacuating the actual lrp", func() {
				BeforeEach(func() {
					controller.EvacuateStoppedActualLRPReturns(nil)
				})

				It("should return no error", func() {
					Expect(responseRecorder.Code).To(Equal(http.StatusOK))
					var response models.EvacuationResponse
					err := response.Unmarshal(responseRecorder.Body.Bytes())
					Expect(err).NotTo(HaveOccurred())
					Expect(response.Error).To(BeNil())
					Expect(response.KeepContainer).To(BeFalse())
				})
			})

			Context("when the controller returns an error", func() {
				Context("if the error is recoverable", func() {
					BeforeEach(func() {
						controller.EvacuateStoppedActualLRPReturns(models.ErrUnknownError)
					})

					It("should return the error in the response", func() {
						Expect(responseRecorder.Code).To(Equal(http.StatusOK))
						var response models.EvacuationResponse
						err := response.Unmarshal(responseRecorder.Body.Bytes())
						Expect(err).NotTo(HaveOccurred())
						Expect(response.Error).NotTo(BeNil())
						Expect(response.Error).To(Equal(models.ErrUnknownError))
						Expect(response.KeepContainer).To(BeFalse())
					})
				})

				Context("if the error is unrecoverable", func() {
					BeforeEach(func() {
						controller.EvacuateStoppedActualLRPReturns(models.NewUnrecoverableError(nil))
					})

					It("logs and writes to the exit channel", func() {
						Eventually(logger).Should(gbytes.Say("unrecoverable-error"))
						Eventually(exitCh).Should(Receive())
					})
				})
			})
		})

		Context("when the request is invalid", func() {
			BeforeEach(func() {
				request = newTestRequest("{{")
				handler.EvacuateStoppedActualLRP(logger, responseRecorder, request)
				Expect(responseRecorder.Code).To(Equal(http.StatusOK))
			})

			It("returns an error and keeps the container", func() {
				response := models.EvacuationResponse{}
				err := response.Unmarshal(responseRecorder.Body.Bytes())
				Expect(err).NotTo(HaveOccurred())
				Expect(response.Error).NotTo(BeNil())
				Expect(response.Error).To(Equal(models.ErrBadRequest))
			})
		})
	})
})
