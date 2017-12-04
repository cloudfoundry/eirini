package handlers_test

import (
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"

	"code.cloudfoundry.org/auctioneer"
	"code.cloudfoundry.org/auctioneer/auctioneerfakes"
	"code.cloudfoundry.org/bbs/db/dbfakes"
	"code.cloudfoundry.org/bbs/events/eventfakes"
	"code.cloudfoundry.org/bbs/handlers"
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
		logger               lager.Logger
		fakeEvacuationDB     *dbfakes.FakeEvacuationDB
		fakeActualLRPDB      *dbfakes.FakeActualLRPDB
		fakeDesiredLRPDB     *dbfakes.FakeDesiredLRPDB
		actualHub            *eventfakes.FakeHub
		fakeAuctioneerClient *auctioneerfakes.FakeClient
		responseRecorder     *httptest.ResponseRecorder
		handler              *handlers.EvacuationHandler
		exitCh               chan struct{}
	)

	BeforeEach(func() {
		fakeEvacuationDB = new(dbfakes.FakeEvacuationDB)
		fakeActualLRPDB = new(dbfakes.FakeActualLRPDB)
		fakeDesiredLRPDB = new(dbfakes.FakeDesiredLRPDB)
		actualHub = new(eventfakes.FakeHub)
		fakeAuctioneerClient = new(auctioneerfakes.FakeClient)
		logger = lagertest.NewTestLogger("test")
		logger.RegisterSink(lager.NewWriterSink(GinkgoWriter, lager.DEBUG))
		responseRecorder = httptest.NewRecorder()
		exitCh = make(chan struct{}, 1)
		handler = handlers.NewEvacuationHandler(fakeEvacuationDB, fakeActualLRPDB, fakeDesiredLRPDB, actualHub, fakeAuctioneerClient, exitCh)
	})

	Describe("RemoveEvacuatingActualLRP", func() {
		var (
			processGuid = "process-guid"
			index       = int32(1)

			key         models.ActualLRPKey
			instanceKey models.ActualLRPInstanceKey
			actual      *models.ActualLRP

			requestBody interface{}
		)

		BeforeEach(func() {
			key = models.NewActualLRPKey(
				processGuid,
				index,
				"domain-0",
			)
			instanceKey = models.NewActualLRPInstanceKey("instance-guid", "cell-id")
			actual = &models.ActualLRP{
				ActualLRPInstanceKey: instanceKey,
			}
			requestBody = &models.RemoveEvacuatingActualLRPRequest{
				ActualLrpKey:         &key,
				ActualLrpInstanceKey: &instanceKey,
			}
			fakeActualLRPDB.ActualLRPGroupByProcessGuidAndIndexReturns(&models.ActualLRPGroup{Evacuating: actual}, nil)
		})

		JustBeforeEach(func() {
			request := newTestRequest(requestBody)
			handler.RemoveEvacuatingActualLRP(logger, responseRecorder, request)
		})

		Context("when removeEvacuatinging the actual lrp in the DB succeeds", func() {
			BeforeEach(func() {
				fakeEvacuationDB.RemoveEvacuatingActualLRPReturns(nil)
			})

			It("removeEvacuatings the actual lrp by process guid and index", func() {
				Expect(responseRecorder.Code).To(Equal(http.StatusOK))

				Expect(fakeEvacuationDB.RemoveEvacuatingActualLRPCallCount()).To(Equal(1))
				_, actualKey, actualInstanceKey := fakeEvacuationDB.RemoveEvacuatingActualLRPArgsForCall(0)
				Expect(*actualKey).To(Equal(key))
				Expect(*actualInstanceKey).To(Equal(instanceKey))
			})

			It("emits events to the hub", func() {
				Eventually(actualHub.EmitCallCount).Should(Equal(1))
				event := actualHub.EmitArgsForCall(0)
				removeEvent, ok := event.(*models.ActualLRPRemovedEvent)
				Expect(ok).To(BeTrue())
				Expect(removeEvent.ActualLrpGroup).To(Equal(&models.ActualLRPGroup{Evacuating: actual}))
			})

			Context("when the lrp has a running instance", func() {
				BeforeEach(func() {
					fakeActualLRPDB.ActualLRPGroupByProcessGuidAndIndexReturns(&models.ActualLRPGroup{
						Evacuating: actual,
						Instance:   actual,
					}, nil)
				})

				It("emits event with the evacuating instance only", func() {
					Eventually(actualHub.EmitCallCount).Should(Equal(1))
					event := actualHub.EmitArgsForCall(0)
					removeEvent, ok := event.(*models.ActualLRPRemovedEvent)
					Expect(ok).To(BeTrue())
					Expect(removeEvent.ActualLrpGroup).To(Equal(&models.ActualLRPGroup{Evacuating: actual}))
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

		Context("when the DB returns an unrecoverable error", func() {
			BeforeEach(func() {
				fakeEvacuationDB.RemoveEvacuatingActualLRPReturns(models.NewUnrecoverableError(nil))
			})

			It("logs and writes to the exit channel", func() {
				Eventually(logger).Should(gbytes.Say("unrecoverable-error"))
				Eventually(exitCh).Should(Receive())
			})
		})

		Context("when DB errors out", func() {
			BeforeEach(func() {
				fakeEvacuationDB.RemoveEvacuatingActualLRPReturns(models.ErrUnknownError)
			})

			It("responds with an error", func() {
				Expect(responseRecorder.Code).To(Equal(http.StatusOK))

				var response models.RemoveEvacuatingActualLRPResponse
				err := response.Unmarshal(responseRecorder.Body.Bytes())
				Expect(err).NotTo(HaveOccurred())

				Expect(response.Error).NotTo(BeNil())
				Expect(response.Error).To(Equal(models.ErrUnknownError))
			})
		})

		Context("when we cannot find the resource", func() {
			BeforeEach(func() {
				fakeEvacuationDB.RemoveEvacuatingActualLRPReturns(models.ErrResourceNotFound)
			})

			It("responds with an error", func() {
				Expect(responseRecorder.Code).To(Equal(http.StatusOK))

				var response models.RemoveEvacuatingActualLRPResponse
				err := response.Unmarshal(responseRecorder.Body.Bytes())
				Expect(err).NotTo(HaveOccurred())

				Expect(response.Error).NotTo(BeNil())
				Expect(response.Error).To(Equal(models.ErrResourceNotFound))
			})
		})
	})

	Describe("EvacuateClaimedActualLRP", func() {
		var (
			request     *http.Request
			requestBody *models.EvacuateClaimedActualLRPRequest
			actual      *models.ActualLRP
			afterActual *models.ActualLRP
			desiredLRP  *models.DesiredLRP
		)

		BeforeEach(func() {
			desiredLRP = model_helpers.NewValidDesiredLRP("the-guid")
			fakeDesiredLRPDB.DesiredLRPByProcessGuidReturns(desiredLRP, nil)

			actual = model_helpers.NewValidActualLRP("process-guid", 1)
			requestBody = &models.EvacuateClaimedActualLRPRequest{
				ActualLrpKey:         &actual.ActualLRPKey,
				ActualLrpInstanceKey: &actual.ActualLRPInstanceKey,
			}
			afterActual = model_helpers.NewValidActualLRP("process-guid", 1)
			afterActual.State = models.ActualLRPStateUnclaimed
			fakeActualLRPDB.ActualLRPGroupByProcessGuidAndIndexReturns(&models.ActualLRPGroup{Evacuating: actual}, nil)
			fakeActualLRPDB.UnclaimActualLRPReturns(&models.ActualLRPGroup{Instance: actual}, &models.ActualLRPGroup{Instance: afterActual}, nil)

			request = newTestRequest(requestBody)
		})

		JustBeforeEach(func() {
			handler.EvacuateClaimedActualLRP(logger, responseRecorder, request)
			Expect(responseRecorder.Code).To(Equal(http.StatusOK))
		})

		It("removes the evacuating actual lrp", func() {
			response := models.EvacuationResponse{}
			err := response.Unmarshal(responseRecorder.Body.Bytes())
			Expect(err).NotTo(HaveOccurred())
			Expect(response.KeepContainer).To(BeFalse())
			Expect(response.Error).To(BeNil())

			Expect(fakeEvacuationDB.RemoveEvacuatingActualLRPCallCount()).To(Equal(1))
			_, key, instanceKey := fakeEvacuationDB.RemoveEvacuatingActualLRPArgsForCall(0)
			Expect(*key).To(Equal(actual.ActualLRPKey))
			Expect(*instanceKey).To(Equal(actual.ActualLRPInstanceKey))
		})

		It("emits events to the hub", func() {
			Eventually(actualHub.EmitCallCount).Should(Equal(2))

			for i := 0; i < actualHub.EmitCallCount(); i++ {
				switch event := actualHub.EmitArgsForCall(i).(type) {
				case *models.ActualLRPRemovedEvent:
					Expect(event.ActualLrpGroup).To(Equal(&models.ActualLRPGroup{Evacuating: actual}))
				case *models.ActualLRPChangedEvent:
					Expect(event.Before).To(Equal(&models.ActualLRPGroup{Instance: actual}))
					Expect(event.After).To(Equal(&models.ActualLRPGroup{Instance: afterActual}))
				default:
					Fail(fmt.Sprintf("unexpected event %#v", event))
				}
			}
		})

		It("unclaims the actual lrp instance and requests an auction", func() {
			response := models.EvacuationResponse{}
			err := response.Unmarshal(responseRecorder.Body.Bytes())
			Expect(err).NotTo(HaveOccurred())
			Expect(response.KeepContainer).To(BeFalse())
			Expect(response.Error).To(BeNil())

			Expect(fakeActualLRPDB.UnclaimActualLRPCallCount()).To(Equal(1))
			_, lrpKey := fakeActualLRPDB.UnclaimActualLRPArgsForCall(0)
			Expect(lrpKey.ProcessGuid).To(Equal("process-guid"))
			Expect(lrpKey.Index).To(BeEquivalentTo(1))

			Expect(fakeDesiredLRPDB.DesiredLRPByProcessGuidCallCount()).To(Equal(1))
			_, guid := fakeDesiredLRPDB.DesiredLRPByProcessGuidArgsForCall(0)
			Expect(guid).To(Equal("process-guid"))

			expectedStartRequest := auctioneer.NewLRPStartRequestFromModel(desiredLRP, int(actual.Index))
			Expect(fakeAuctioneerClient.RequestLRPAuctionsCallCount()).To(Equal(1))
			_, startRequests := fakeAuctioneerClient.RequestLRPAuctionsArgsForCall(0)
			Expect(startRequests).To(Equal([]*auctioneer.LRPStartRequest{&expectedStartRequest}))
		})

		Context("when the DB returns an unrecoverable error", func() {
			BeforeEach(func() {
				fakeEvacuationDB.RemoveEvacuatingActualLRPReturns(models.NewUnrecoverableError(nil))
			})

			It("logs and writes to the exit channel", func() {
				Eventually(logger).Should(gbytes.Say("unrecoverable-error"))
				Eventually(exitCh).Should(Receive())
			})
		})

		Context("when removing the evacuating lrp fails", func() {
			BeforeEach(func() {
				fakeEvacuationDB.RemoveEvacuatingActualLRPReturns(errors.New("i failed"))
			})

			It("logs the error and continues", func() {
				response := models.EvacuationResponse{}
				err := response.Unmarshal(responseRecorder.Body.Bytes())
				Expect(err).NotTo(HaveOccurred())
				Expect(response.KeepContainer).To(BeFalse())
				Expect(response.Error).To(BeNil())
				Expect(logger).To(gbytes.Say("failed-removing-evacuating-actual-lrp"))
			})

			It("only emits events for unclaiming", func() {
				Eventually(actualHub.EmitCallCount).Should(Equal(1))
				event := actualHub.EmitArgsForCall(0)
				changedEvent, ok := event.(*models.ActualLRPChangedEvent)
				Expect(ok).To(BeTrue())
				Expect(changedEvent.Before).To(Equal(&models.ActualLRPGroup{Instance: actual}))
				Expect(changedEvent.After).To(Equal(&models.ActualLRPGroup{Instance: afterActual}))
			})
		})

		Context("when unclaiming the lrp instance fails", func() {
			Context("when the DB returns an unrecoverable error", func() {
				BeforeEach(func() {
					fakeActualLRPDB.UnclaimActualLRPReturns(nil, nil, models.NewUnrecoverableError(nil))
				})

				It("logs and writes to the exit channel", func() {
					Eventually(logger).Should(gbytes.Say("unrecoverable-error"))
					Eventually(exitCh).Should(Receive())
				})
			})

			Context("because the instance does not exist", func() {
				BeforeEach(func() {
					fakeActualLRPDB.UnclaimActualLRPReturns(nil, nil, models.ErrResourceNotFound)
				})

				It("does not keep the container and does not return an error", func() {
					response := models.EvacuationResponse{}
					err := response.Unmarshal(responseRecorder.Body.Bytes())
					Expect(err).NotTo(HaveOccurred())
					Expect(response.KeepContainer).To(BeFalse())
					Expect(response.Error).To(BeNil())
				})

				It("only emits events for deleting evacuating", func() {
					Eventually(actualHub.EmitCallCount).Should(Equal(1))
					event := actualHub.EmitArgsForCall(0)
					removeEvent, ok := event.(*models.ActualLRPRemovedEvent)
					Expect(ok).To(BeTrue())
					Expect(removeEvent.ActualLrpGroup).To(Equal(&models.ActualLRPGroup{Evacuating: actual}))
				})
			})

			Context("for another reason", func() {
				BeforeEach(func() {
					fakeActualLRPDB.UnclaimActualLRPReturns(nil, nil, errors.New("can't unclaim this"))
				})

				It("returns the error and keeps the container", func() {
					response := models.EvacuationResponse{}
					err := response.Unmarshal(responseRecorder.Body.Bytes())
					Expect(err).NotTo(HaveOccurred())
					Expect(response.KeepContainer).To(BeTrue())
					Expect(response.Error).NotTo(BeNil())
					Expect(response.Error.Error()).To(Equal("can't unclaim this"))
				})

				It("only emits events for deleting evacuating", func() {
					Eventually(actualHub.EmitCallCount).Should(Equal(1))
					event := actualHub.EmitArgsForCall(0)
					removeEvent, ok := event.(*models.ActualLRPRemovedEvent)
					Expect(ok).To(BeTrue())
					Expect(removeEvent.ActualLrpGroup).To(Equal(&models.ActualLRPGroup{Evacuating: actual}))
				})
			})
		})

		Context("when requesting the lrp auction fails", func() {
			BeforeEach(func() {
				fakeAuctioneerClient.RequestLRPAuctionsReturns(errors.New("boom!"))
			})

			It("does not return the error or keep the container", func() {
				response := models.EvacuationResponse{}
				err := response.Unmarshal(responseRecorder.Body.Bytes())
				Expect(err).NotTo(HaveOccurred())
				Expect(response.KeepContainer).To(BeFalse())
				Expect(response.Error).To(BeNil())
			})
		})

		Context("when the request is invalid", func() {
			BeforeEach(func() {
				request = newTestRequest("{{")
			})

			It("returns an error and keeps the container", func() {
				response := models.EvacuationResponse{}
				err := response.Unmarshal(responseRecorder.Body.Bytes())
				Expect(err).NotTo(HaveOccurred())
				Expect(response.KeepContainer).To(BeTrue())
				Expect(response.Error).NotTo(BeNil())
				Expect(response.Error).To(Equal(models.ErrBadRequest))
			})

			It("does not emit any events", func() {
				Consistently(actualHub.EmitCallCount).Should(Equal(0))
			})
		})
	})

	Describe("EvacuateCrashedActualLRP", func() {
		var (
			request     *http.Request
			requestBody *models.EvacuateCrashedActualLRPRequest
			actual      *models.ActualLRP
		)

		BeforeEach(func() {
			actual = model_helpers.NewValidActualLRP("process-guid", 1)
			requestBody = &models.EvacuateCrashedActualLRPRequest{
				ActualLrpKey:         &actual.ActualLRPKey,
				ActualLrpInstanceKey: &actual.ActualLRPInstanceKey,
				ErrorMessage:         "i failed",
			}
			fakeActualLRPDB.ActualLRPGroupByProcessGuidAndIndexReturns(&models.ActualLRPGroup{Evacuating: actual}, nil)

			request = newTestRequest(requestBody)
		})

		JustBeforeEach(func() {
			handler.EvacuateCrashedActualLRP(logger, responseRecorder, request)
			Expect(responseRecorder.Code).To(Equal(http.StatusOK))
		})

		It("does not return an error or keep the container", func() {
			response := models.EvacuationResponse{}
			err := response.Unmarshal(responseRecorder.Body.Bytes())
			Expect(err).NotTo(HaveOccurred())
			Expect(response.KeepContainer).To(BeFalse())
			Expect(response.Error).To(BeNil())
		})

		It("removes the evacuating actual lrp", func() {
			Expect(fakeEvacuationDB.RemoveEvacuatingActualLRPCallCount()).To(Equal(1))
			_, key, instanceKey := fakeEvacuationDB.RemoveEvacuatingActualLRPArgsForCall(0)
			Expect(*key).To(Equal(actual.ActualLRPKey))
			Expect(*instanceKey).To(Equal(actual.ActualLRPInstanceKey))
		})

		It("emits events to the hub", func() {
			Eventually(actualHub.EmitCallCount).Should(Equal(1))
			event := actualHub.EmitArgsForCall(0)
			removeEvent, ok := event.(*models.ActualLRPRemovedEvent)
			Expect(ok).To(BeTrue())
			Expect(removeEvent.ActualLrpGroup).To(Equal(&models.ActualLRPGroup{Evacuating: actual}))
		})

		It("crashes the actual lrp instance", func() {
			Expect(fakeActualLRPDB.CrashActualLRPCallCount()).To(Equal(1))
			_, key, instanceKey, errorMessage := fakeActualLRPDB.CrashActualLRPArgsForCall(0)
			Expect(*key).To(Equal(actual.ActualLRPKey))
			Expect(*instanceKey).To(Equal(actual.ActualLRPInstanceKey))
			Expect(errorMessage).To(Equal("i failed"))
		})

		Context("when the DB returns an unrecoverable error", func() {
			BeforeEach(func() {
				fakeEvacuationDB.RemoveEvacuatingActualLRPReturns(models.NewUnrecoverableError(nil))
			})

			It("logs and writes to the exit channel", func() {
				Eventually(logger).Should(gbytes.Say("unrecoverable-error"))
				Eventually(exitCh).Should(Receive())
			})
		})

		Context("when removing the evacuating actual lrp fails", func() {
			BeforeEach(func() {
				fakeEvacuationDB.RemoveEvacuatingActualLRPReturns(errors.New("oh no!"))
			})

			It("logs the error and continues", func() {
				response := models.EvacuationResponse{}
				err := response.Unmarshal(responseRecorder.Body.Bytes())
				Expect(err).NotTo(HaveOccurred())
				Expect(response.KeepContainer).To(BeFalse())
				Expect(response.Error).To(BeNil())

				Expect(logger).To(gbytes.Say("failed-removing-evacuating-actual-lrp"))
			})
		})

		Context("when crashing the actual lrp fails", func() {
			Context("when the DB returns an unrecoverable error", func() {
				BeforeEach(func() {
					fakeActualLRPDB.CrashActualLRPReturns(nil, nil, false, models.NewUnrecoverableError(nil))
				})

				It("logs and writes to the exit channel", func() {
					Eventually(logger).Should(gbytes.Say("unrecoverable-error"))
					Eventually(exitCh).Should(Receive())
				})
			})

			Context("because the error does not exist", func() {
				BeforeEach(func() {
					fakeActualLRPDB.CrashActualLRPReturns(nil, nil, false, models.ErrResourceNotFound)
				})

				It("does not return an error or keep the container", func() {
					response := models.EvacuationResponse{}
					err := response.Unmarshal(responseRecorder.Body.Bytes())
					Expect(err).NotTo(HaveOccurred())
					Expect(response.KeepContainer).To(BeFalse())
					Expect(response.Error).To(BeNil())
				})
			})

			Context("for another reason", func() {
				BeforeEach(func() {
					fakeActualLRPDB.CrashActualLRPReturns(nil, nil, false, errors.New("failed-crashing-dawg"))
				})

				It("returns an error and does not keep the container", func() {
					response := models.EvacuationResponse{}
					err := response.Unmarshal(responseRecorder.Body.Bytes())
					Expect(err).NotTo(HaveOccurred())
					Expect(response.KeepContainer).To(BeFalse())
					Expect(response.Error).NotTo(BeNil())
					Expect(response.Error.Error()).To(Equal("failed-crashing-dawg"))
				})
			})
		})

		Context("when the request is invalid", func() {
			BeforeEach(func() {
				request = newTestRequest("{{")
			})

			It("returns an error", func() {
				response := models.EvacuationResponse{}
				err := response.Unmarshal(responseRecorder.Body.Bytes())
				Expect(err).NotTo(HaveOccurred())
				Expect(response.KeepContainer).To(BeFalse())
				Expect(response.Error).To(Equal(models.ErrBadRequest))
			})
		})
	})

	Describe("EvacuateRunningActualLRP", func() {
		var (
			request        *http.Request
			requestBody    *models.EvacuateRunningActualLRPRequest
			actualLRPGroup *models.ActualLRPGroup
			desiredLRP     *models.DesiredLRP

			actual           *models.ActualLRP
			evacuatingActual *models.ActualLRP
			afterActual      *models.ActualLRP
		)

		BeforeEach(func() {
			request = nil
			desiredLRP = model_helpers.NewValidDesiredLRP("the-guid")
			fakeDesiredLRPDB.DesiredLRPByProcessGuidReturns(desiredLRP, nil)

			actual = model_helpers.NewValidActualLRP("the-guid", 1)
			evacuatingActual = model_helpers.NewValidActualLRP("the-guid", 1)
			afterActual = model_helpers.NewValidActualLRP("the-guid", 1)

			key := actual.ActualLRPKey
			instanceKey := actual.ActualLRPInstanceKey
			netInfo := actual.ActualLRPNetInfo
			requestBody = &models.EvacuateRunningActualLRPRequest{
				ActualLrpKey:         &key,
				ActualLrpInstanceKey: &instanceKey,
				ActualLrpNetInfo:     &netInfo,
				Ttl:                  60,
			}

			actualLRPGroup = &models.ActualLRPGroup{
				Instance:   actual,
				Evacuating: evacuatingActual,
			}

			fakeActualLRPDB.ActualLRPGroupByProcessGuidAndIndexReturns(actualLRPGroup, nil)
			fakeActualLRPDB.UnclaimActualLRPReturns(&models.ActualLRPGroup{Instance: actual}, &models.ActualLRPGroup{Instance: afterActual}, nil)
		})

		JustBeforeEach(func() {
			if request == nil {
				request = newTestRequest(requestBody)
			}
			handler.EvacuateRunningActualLRP(logger, responseRecorder, request)
			Expect(responseRecorder.Code).To(Equal(http.StatusOK))
		})

		Context("when the actual lrp group exists", func() {
			Context("when the actual lrp instance does not exist", func() {
				BeforeEach(func() {
					actualLRPGroup.Instance = nil
				})

				It("removes the evacuating lrp and does not keep the container", func() {
					response := models.EvacuationResponse{}
					err := response.Unmarshal(responseRecorder.Body.Bytes())
					Expect(err).NotTo(HaveOccurred())
					Expect(response.KeepContainer).To(BeFalse())
					Expect(response.Error).To(BeNil())

					Expect(fakeEvacuationDB.RemoveEvacuatingActualLRPCallCount()).To(Equal(1))
					_, actualLRPKey, actualLRPInstanceKey := fakeEvacuationDB.RemoveEvacuatingActualLRPArgsForCall(0)
					Expect(*actualLRPKey).To(Equal(actual.ActualLRPKey))
					Expect(*actualLRPInstanceKey).To(Equal(actual.ActualLRPInstanceKey))
				})

				It("emits events to the hub", func() {
					Eventually(actualHub.EmitCallCount).Should(Equal(1))
					event := actualHub.EmitArgsForCall(0)
					removeEvent, ok := event.(*models.ActualLRPRemovedEvent)
					Expect(ok).To(BeTrue())
					Expect(removeEvent.ActualLrpGroup).To(Equal(&models.ActualLRPGroup{Evacuating: actual}))
				})

				Context("when the evacuating lrp cannot be removed", func() {
					BeforeEach(func() {
						fakeEvacuationDB.RemoveEvacuatingActualLRPReturns(models.ErrActualLRPCannotBeRemoved)
					})

					It("returns no error and removes the container", func() {
						response := models.EvacuationResponse{}
						err := response.Unmarshal(responseRecorder.Body.Bytes())
						Expect(err).NotTo(HaveOccurred())
						Expect(response.KeepContainer).To(BeFalse())
						Expect(response.Error).To(BeNil())
					})

					It("does not emit any events", func() {
						Consistently(actualHub.EmitCallCount).Should(Equal(0))
					})
				})

				Context("when the DB returns an unrecoverable error", func() {
					BeforeEach(func() {
						fakeEvacuationDB.RemoveEvacuatingActualLRPReturns(models.NewUnrecoverableError(nil))
					})

					It("logs and writes to the exit channel", func() {
						Eventually(logger).Should(gbytes.Say("unrecoverable-error"))
						Eventually(exitCh).Should(Receive())
					})
				})

				Context("when removing the evacuating lrp fails for a different reason", func() {
					BeforeEach(func() {
						fakeEvacuationDB.RemoveEvacuatingActualLRPReturns(errors.New("didnt work"))
					})

					It("returns an error and keeps the container", func() {
						response := models.EvacuationResponse{}
						err := response.Unmarshal(responseRecorder.Body.Bytes())
						Expect(err).NotTo(HaveOccurred())
						Expect(response.KeepContainer).To(BeTrue())
						Expect(response.Error).NotTo(BeNil())
						Expect(response.Error.Error()).To(Equal("didnt work"))
					})

					It("does not emit any events", func() {
						Consistently(actualHub.EmitCallCount).Should(Equal(0))
					})
				})
			})

			Context("when the instance is unclaimed", func() {
				BeforeEach(func() {
					actual.State = models.ActualLRPStateUnclaimed
				})

				Context("without a placement error", func() {
					BeforeEach(func() {
						actual.PlacementError = ""
						group := &models.ActualLRPGroup{Evacuating: afterActual}
						fakeEvacuationDB.EvacuateActualLRPReturns(group, nil)
					})

					It("evacuates the LRP", func() {
						response := models.EvacuationResponse{}
						err := response.Unmarshal(responseRecorder.Body.Bytes())
						Expect(err).NotTo(HaveOccurred())
						Expect(response.KeepContainer).To(BeTrue())
						Expect(response.Error).To(BeNil())

						Expect(fakeEvacuationDB.EvacuateActualLRPCallCount()).To(Equal(1))
						_, actualLRPKey, actualLRPInstanceKey, actualLrpNetInfo, ttl := fakeEvacuationDB.EvacuateActualLRPArgsForCall(0)
						Expect(*actualLRPKey).To(Equal(actual.ActualLRPKey))
						Expect(*actualLRPInstanceKey).To(Equal(actual.ActualLRPInstanceKey))
						Expect(*actualLrpNetInfo).To(Equal(actual.ActualLRPNetInfo))
						Expect(ttl).To(BeEquivalentTo(60))
					})

					It("emits events to the hub", func() {
						Eventually(actualHub.EmitCallCount).Should(Equal(1))

						for i := 0; i < actualHub.EmitCallCount(); i++ {
							switch event := actualHub.EmitArgsForCall(i).(type) {
							case *models.ActualLRPCreatedEvent:
								Expect(event.ActualLrpGroup).To(Equal(&models.ActualLRPGroup{Evacuating: afterActual}))
							default:
								Fail(fmt.Sprintf("unexpected event %#v", event))
							}
						}
					})

					Context("when there's an existing evacuating on another cell", func() {
						BeforeEach(func() {
							evacuatingLRP := model_helpers.NewValidActualLRP("the-guid", 1)
							evacuatingLRP.CellId = "some-other-cell"
							actualLRPGroup.Evacuating = evacuatingLRP
						})

						It("does not error and does not keep the container", func() {
							response := models.EvacuationResponse{}
							err := response.Unmarshal(responseRecorder.Body.Bytes())
							Expect(err).NotTo(HaveOccurred())
							Expect(response.KeepContainer).To(BeFalse())
							Expect(response.Error).To(BeNil())
						})
					})

					Context("when evacuating the actual lrp fails", func() {
						BeforeEach(func() {
							fakeEvacuationDB.EvacuateActualLRPReturns(nil, errors.New("didnt work"))
						})

						It("returns an error and keeps the container", func() {
							response := models.EvacuationResponse{}
							err := response.Unmarshal(responseRecorder.Body.Bytes())
							Expect(err).NotTo(HaveOccurred())
							Expect(response.KeepContainer).To(BeTrue())
							Expect(response.Error).NotTo(BeNil())
							Expect(response.Error.Error()).To(Equal("didnt work"))
						})
					})

					Context("when the lrp cannot be evacuated", func() {
						BeforeEach(func() {
							fakeEvacuationDB.EvacuateActualLRPReturns(nil, models.ErrActualLRPCannotBeEvacuated)
						})

						It("does not error and does not keep the container", func() {
							response := models.EvacuationResponse{}
							err := response.Unmarshal(responseRecorder.Body.Bytes())
							Expect(err).NotTo(HaveOccurred())
							Expect(response.KeepContainer).To(BeFalse())
							Expect(response.Error).To(BeNil())
						})
					})
				})

				Context("with a placement error", func() {
					BeforeEach(func() {
						actual.PlacementError = "jim kinda likes cats, but loves kittens"
					})

					It("does not remove the evacuating LRP", func() {
						response := models.EvacuationResponse{}
						err := response.Unmarshal(responseRecorder.Body.Bytes())
						Expect(err).NotTo(HaveOccurred())
						Expect(response.KeepContainer).To(BeTrue())
						Expect(response.Error).To(BeNil())

						Expect(fakeEvacuationDB.RemoveEvacuatingActualLRPCallCount()).To(Equal(0))
					})

					It("does not emit events to the hub", func() {
						Consistently(actualHub.EmitCallCount).Should(Equal(0))
					})
				})
			})

			Context("when the instance is claimed", func() {
				BeforeEach(func() {
					actual.State = models.ActualLRPStateClaimed
				})

				Context("by another cell", func() {
					BeforeEach(func() {
						actual.CellId = "some-other-cell"
						group := &models.ActualLRPGroup{Evacuating: afterActual}
						fakeEvacuationDB.EvacuateActualLRPReturns(group, nil)
					})

					It("evacuates the LRP", func() {
						response := models.EvacuationResponse{}
						err := response.Unmarshal(responseRecorder.Body.Bytes())
						Expect(err).NotTo(HaveOccurred())
						Expect(response.KeepContainer).To(BeTrue())
						Expect(response.Error).To(BeNil())

						Expect(fakeEvacuationDB.EvacuateActualLRPCallCount()).To(Equal(1))
						_, actualLRPKey, actualLRPInstanceKey, actualLrpNetInfo, ttl := fakeEvacuationDB.EvacuateActualLRPArgsForCall(0)
						Expect(*actualLRPKey).To(Equal(actual.ActualLRPKey))
						Expect(*actualLRPInstanceKey).To(Equal(*requestBody.ActualLrpInstanceKey))
						Expect(*actualLrpNetInfo).To(Equal(actual.ActualLRPNetInfo))
						Expect(ttl).To(BeEquivalentTo(60))
					})

					It("emits events to the hub", func() {
						Eventually(actualHub.EmitCallCount).Should(Equal(1))

						for i := 0; i < actualHub.EmitCallCount(); i++ {
							switch event := actualHub.EmitArgsForCall(i).(type) {
							case *models.ActualLRPCreatedEvent:
								Expect(event.ActualLrpGroup).To(Equal(&models.ActualLRPGroup{Evacuating: afterActual}))
							default:
								Fail(fmt.Sprintf("unexpected event %#v", event))
							}
						}
					})

					Context("when there's an existing evacuating on another cell", func() {
						BeforeEach(func() {
							evacuatingLRP := model_helpers.NewValidActualLRP("the-guid", 1)
							evacuatingLRP.CellId = "some-other-cell"
							actualLRPGroup.Evacuating = evacuatingLRP
						})

						It("does not error and does not keep the container", func() {
							response := models.EvacuationResponse{}
							err := response.Unmarshal(responseRecorder.Body.Bytes())
							Expect(err).NotTo(HaveOccurred())
							Expect(response.KeepContainer).To(BeFalse())
							Expect(response.Error).To(BeNil())
						})
					})

					Context("when evacuating the actual lrp fails", func() {
						BeforeEach(func() {
							fakeEvacuationDB.EvacuateActualLRPReturns(nil, errors.New("didnt work"))
						})

						It("returns an error and keeps the container", func() {
							response := models.EvacuationResponse{}
							err := response.Unmarshal(responseRecorder.Body.Bytes())
							Expect(err).NotTo(HaveOccurred())
							Expect(response.KeepContainer).To(BeTrue())
							Expect(response.Error).NotTo(BeNil())
							Expect(response.Error.Error()).To(Equal("didnt work"))
						})
					})

					Context("when the lrp cannot be evacuated", func() {
						BeforeEach(func() {
							fakeEvacuationDB.EvacuateActualLRPReturns(nil, models.ErrActualLRPCannotBeEvacuated)
						})

						It("does not error and does not keep the container", func() {
							response := models.EvacuationResponse{}
							err := response.Unmarshal(responseRecorder.Body.Bytes())
							Expect(err).NotTo(HaveOccurred())
							Expect(response.KeepContainer).To(BeFalse())
							Expect(response.Error).To(BeNil())
						})
					})
				})

				Context("by the same cell", func() {
					BeforeEach(func() {
						group := &models.ActualLRPGroup{Evacuating: afterActual}
						fakeEvacuationDB.EvacuateActualLRPReturns(group, nil)
					})

					It("evacuates the lrp", func() {
						response := models.EvacuationResponse{}
						err := response.Unmarshal(responseRecorder.Body.Bytes())
						Expect(err).NotTo(HaveOccurred())
						Expect(response.KeepContainer).To(BeTrue())
						Expect(response.Error).To(BeNil())

						Expect(fakeEvacuationDB.EvacuateActualLRPCallCount()).To(Equal(1))
						_, actualLRPKey, actualLRPInstanceKey, actualLrpNetInfo, ttl := fakeEvacuationDB.EvacuateActualLRPArgsForCall(0)
						Expect(*actualLRPKey).To(Equal(actual.ActualLRPKey))
						Expect(*actualLRPInstanceKey).To(Equal(actual.ActualLRPInstanceKey))
						Expect(*actualLrpNetInfo).To(Equal(actual.ActualLRPNetInfo))
						Expect(ttl).To(BeEquivalentTo(60))
					})

					It("unclaims the lrp and requests an auction", func() {
						Expect(fakeActualLRPDB.UnclaimActualLRPCallCount()).To(Equal(1))
						_, actualLRPKey, actualLRPInstanceKey, actualLrpNetInfo, ttl := fakeEvacuationDB.EvacuateActualLRPArgsForCall(0)
						Expect(*actualLRPKey).To(Equal(actual.ActualLRPKey))
						Expect(*actualLRPInstanceKey).To(Equal(actual.ActualLRPInstanceKey))
						Expect(*actualLrpNetInfo).To(Equal(actual.ActualLRPNetInfo))
						Expect(ttl).To(BeEquivalentTo(60))

						schedulingInfo := desiredLRP.DesiredLRPSchedulingInfo()
						expectedStartRequest := auctioneer.NewLRPStartRequestFromSchedulingInfo(&schedulingInfo, int(actual.Index))

						Expect(fakeAuctioneerClient.RequestLRPAuctionsCallCount()).To(Equal(1))
						_, startRequests := fakeAuctioneerClient.RequestLRPAuctionsArgsForCall(0)
						Expect(startRequests).To(Equal([]*auctioneer.LRPStartRequest{&expectedStartRequest}))
					})

					It("emits events to the hub", func() {
						Eventually(actualHub.EmitCallCount).Should(Equal(2))

						for i := 0; i < actualHub.EmitCallCount(); i++ {
							switch event := actualHub.EmitArgsForCall(i).(type) {
							case *models.ActualLRPCreatedEvent:
								Expect(event.ActualLrpGroup).To(Equal(&models.ActualLRPGroup{Evacuating: afterActual}))
							case *models.ActualLRPChangedEvent:
								Expect(event.Before).To(Equal(&models.ActualLRPGroup{Instance: actual}))
								Expect(event.After).To(Equal(&models.ActualLRPGroup{Instance: afterActual}))
							default:
								Fail(fmt.Sprintf("unexpected event %#v", event))
							}
						}
					})

					Context("when evacuating fails", func() {
						BeforeEach(func() {
							fakeEvacuationDB.EvacuateActualLRPReturns(nil, errors.New("this is a disaster"))
						})

						It("returns an error and keep the container", func() {
							response := models.EvacuationResponse{}
							err := response.Unmarshal(responseRecorder.Body.Bytes())
							Expect(err).NotTo(HaveOccurred())
							Expect(response.KeepContainer).To(BeTrue())
							Expect(response.Error).NotTo(BeNil())
							Expect(response.Error.Error()).To(Equal("this is a disaster"))
						})
					})

					Context("when unclaiming fails", func() {
						BeforeEach(func() {
							fakeActualLRPDB.UnclaimActualLRPReturns(nil, nil, errors.New("unclaiming failed"))
						})

						It("returns an error and keeps the contianer", func() {
							response := models.EvacuationResponse{}
							err := response.Unmarshal(responseRecorder.Body.Bytes())
							Expect(err).NotTo(HaveOccurred())
							Expect(response.KeepContainer).To(BeTrue())
							Expect(response.Error).NotTo(BeNil())
							Expect(response.Error.Error()).To(Equal("unclaiming failed"))
						})
					})
				})
			})

			Context("when the instance is running", func() {
				BeforeEach(func() {
					actual.State = models.ActualLRPStateRunning
				})

				Context("on this cell", func() {
					BeforeEach(func() {
						group := &models.ActualLRPGroup{Evacuating: afterActual}
						fakeEvacuationDB.EvacuateActualLRPReturns(group, nil)
					})

					It("evacuates the lrp and keeps the container", func() {
						response := models.EvacuationResponse{}
						err := response.Unmarshal(responseRecorder.Body.Bytes())
						Expect(err).NotTo(HaveOccurred())
						Expect(response.KeepContainer).To(BeTrue())
						Expect(response.Error).To(BeNil())

						Expect(fakeEvacuationDB.EvacuateActualLRPCallCount()).To(Equal(1))
						_, actualLRPKey, actualLRPInstanceKey, actualLrpNetInfo, ttl := fakeEvacuationDB.EvacuateActualLRPArgsForCall(0)
						Expect(*actualLRPKey).To(Equal(actual.ActualLRPKey))
						Expect(*actualLRPInstanceKey).To(Equal(actual.ActualLRPInstanceKey))
						Expect(*actualLrpNetInfo).To(Equal(actual.ActualLRPNetInfo))
						Expect(ttl).To(BeEquivalentTo(60))
					})

					It("unclaims the lrp and requests an auction", func() {
						Expect(fakeActualLRPDB.UnclaimActualLRPCallCount()).To(Equal(1))
						_, lrpKey := fakeActualLRPDB.UnclaimActualLRPArgsForCall(0)
						Expect(lrpKey.ProcessGuid).To(Equal(actual.ProcessGuid))
						Expect(lrpKey.Index).To(Equal(actual.Index))

						schedulingInfo := desiredLRP.DesiredLRPSchedulingInfo()
						expectedStartRequest := auctioneer.NewLRPStartRequestFromSchedulingInfo(&schedulingInfo, int(actual.Index))

						Expect(fakeAuctioneerClient.RequestLRPAuctionsCallCount()).To(Equal(1))
						_, startRequests := fakeAuctioneerClient.RequestLRPAuctionsArgsForCall(0)
						Expect(startRequests).To(Equal([]*auctioneer.LRPStartRequest{&expectedStartRequest}))
					})

					It("emits events to the hub", func() {
						Eventually(actualHub.EmitCallCount).Should(Equal(2))

						for i := 0; i < actualHub.EmitCallCount(); i++ {
							switch event := actualHub.EmitArgsForCall(i).(type) {
							case *models.ActualLRPCreatedEvent:
								Expect(event.ActualLrpGroup).To(Equal(&models.ActualLRPGroup{Evacuating: afterActual}))
							case *models.ActualLRPChangedEvent:
								Expect(event.Before).To(Equal(&models.ActualLRPGroup{Instance: actual}))
								Expect(event.After).To(Equal(&models.ActualLRPGroup{Instance: afterActual}))
							default:
								Fail(fmt.Sprintf("unexpected event %#v", event))
							}
						}
					})

					Context("when evacuating fails", func() {
						BeforeEach(func() {
							fakeEvacuationDB.EvacuateActualLRPReturns(nil, errors.New("this is a disaster"))
						})

						It("returns an error and keep the container", func() {
							response := models.EvacuationResponse{}
							err := response.Unmarshal(responseRecorder.Body.Bytes())
							Expect(err).NotTo(HaveOccurred())
							Expect(response.KeepContainer).To(BeTrue())
							Expect(response.Error).NotTo(BeNil())
							Expect(response.Error.Error()).To(Equal("this is a disaster"))
						})
					})

					Context("when unclaiming fails", func() {
						BeforeEach(func() {
							fakeActualLRPDB.UnclaimActualLRPReturns(nil, nil, errors.New("unclaiming failed"))
						})

						It("returns an error and keeps the contianer", func() {
							response := models.EvacuationResponse{}
							err := response.Unmarshal(responseRecorder.Body.Bytes())
							Expect(err).NotTo(HaveOccurred())
							Expect(response.KeepContainer).To(BeTrue())
							Expect(response.Error).NotTo(BeNil())
							Expect(response.Error.Error()).To(Equal("unclaiming failed"))
						})
					})

					Context("when fetching the desired lrp fails", func() {
						BeforeEach(func() {
							fakeDesiredLRPDB.DesiredLRPByProcessGuidReturns(nil, errors.New("jolly rancher beer :/"))
						})

						It("does not return an error and keeps the container", func() {
							response := models.EvacuationResponse{}
							err := response.Unmarshal(responseRecorder.Body.Bytes())
							Expect(err).NotTo(HaveOccurred())
							Expect(response.KeepContainer).To(BeTrue())
							Expect(response.Error).To(BeNil())
						})
					})
				})

				Context("on another cell", func() {
					BeforeEach(func() {
						actual.CellId = "some-other-cell"
					})

					It("removes the evacuating LRP", func() {
						response := models.EvacuationResponse{}
						err := response.Unmarshal(responseRecorder.Body.Bytes())
						Expect(err).NotTo(HaveOccurred())
						Expect(response.KeepContainer).To(BeFalse())
						Expect(response.Error).To(BeNil())

						Expect(fakeEvacuationDB.RemoveEvacuatingActualLRPCallCount()).To(Equal(1))
						_, actualLRPKey, actualLRPInstanceKey := fakeEvacuationDB.RemoveEvacuatingActualLRPArgsForCall(0)
						Expect(*actualLRPKey).To(Equal(evacuatingActual.ActualLRPKey))
						Expect(*actualLRPInstanceKey).To(Equal(evacuatingActual.ActualLRPInstanceKey))
					})

					It("emits events to the hub", func() {
						Eventually(actualHub.EmitCallCount).Should(Equal(1))
						event := actualHub.EmitArgsForCall(0)
						removeEvent, ok := event.(*models.ActualLRPRemovedEvent)
						Expect(ok).To(BeTrue())
						Expect(removeEvent.ActualLrpGroup).To(Equal(&models.ActualLRPGroup{Evacuating: evacuatingActual}))
					})

					Context("when removing the evacuating LRP fails", func() {
						BeforeEach(func() {
							fakeEvacuationDB.RemoveEvacuatingActualLRPReturns(errors.New("boom!"))
						})

						It("returns an error and does keep the container", func() {
							response := models.EvacuationResponse{}
							err := response.Unmarshal(responseRecorder.Body.Bytes())
							Expect(err).NotTo(HaveOccurred())
							Expect(response.KeepContainer).To(BeTrue())
							Expect(response.Error).NotTo(BeNil())
							Expect(response.Error.Error()).To(Equal("boom!"))
						})

						Context("when the error is a ErrActualLRPCannotBeRemoved", func() {
							BeforeEach(func() {
								fakeEvacuationDB.RemoveEvacuatingActualLRPReturns(models.ErrActualLRPCannotBeRemoved)
							})

							It("does not return an error or keep the container", func() {
								response := models.EvacuationResponse{}
								err := response.Unmarshal(responseRecorder.Body.Bytes())
								Expect(err).NotTo(HaveOccurred())
								Expect(response.KeepContainer).To(BeFalse())
								Expect(response.Error).To(BeNil())
							})
						})
					})

					Context("and there is no evacuating lrp", func() {
						BeforeEach(func() {
							actualLRPGroup.Evacuating = nil
						})

						It("responds with KeepContainer set to false", func() {
							response := models.EvacuationResponse{}
							err := response.Unmarshal(responseRecorder.Body.Bytes())
							Expect(err).NotTo(HaveOccurred())
							Expect(response.KeepContainer).To(BeFalse())
							Expect(response.Error).To(BeNil())
						})
					})
				})
			})

			Context("when the instance is crashed", func() {
				BeforeEach(func() {
					actual.State = models.ActualLRPStateCrashed
				})

				It("removes the evacuating LRP", func() {
					response := models.EvacuationResponse{}
					err := response.Unmarshal(responseRecorder.Body.Bytes())
					Expect(err).NotTo(HaveOccurred())
					Expect(response.KeepContainer).To(BeFalse())
					Expect(response.Error).To(BeNil())

					Expect(fakeEvacuationDB.RemoveEvacuatingActualLRPCallCount()).To(Equal(1))
					_, actualLRPKey, actualLRPInstanceKey := fakeEvacuationDB.RemoveEvacuatingActualLRPArgsForCall(0)
					Expect(*actualLRPKey).To(Equal(actual.ActualLRPKey))
					Expect(*actualLRPInstanceKey).To(Equal(actual.ActualLRPInstanceKey))
				})

				Context("when removing the evacuating LRP fails", func() {
					BeforeEach(func() {
						fakeEvacuationDB.RemoveEvacuatingActualLRPReturns(errors.New("boom!"))
					})

					It("returns an error and does keep the container", func() {
						response := models.EvacuationResponse{}
						err := response.Unmarshal(responseRecorder.Body.Bytes())
						Expect(err).NotTo(HaveOccurred())
						Expect(response.KeepContainer).To(BeTrue())
						Expect(response.Error).NotTo(BeNil())
						Expect(response.Error.Error()).To(Equal("boom!"))
					})

					Context("when the error is a ErrActualLRPCannotBeRemoved", func() {
						BeforeEach(func() {
							fakeEvacuationDB.RemoveEvacuatingActualLRPReturns(models.ErrActualLRPCannotBeRemoved)
						})

						It("does not return an error or keep the container", func() {
							response := models.EvacuationResponse{}
							err := response.Unmarshal(responseRecorder.Body.Bytes())
							Expect(err).NotTo(HaveOccurred())
							Expect(response.KeepContainer).To(BeFalse())
							Expect(response.Error).To(BeNil())
						})
					})
				})
			})
		})

		Context("when the actual lrp group does not exist", func() {
			BeforeEach(func() {
				fakeActualLRPDB.ActualLRPGroupByProcessGuidAndIndexReturns(nil, models.ErrResourceNotFound)
			})

			It("does not return an error or keep the container", func() {
				response := models.EvacuationResponse{}
				err := response.Unmarshal(responseRecorder.Body.Bytes())
				Expect(err).NotTo(HaveOccurred())
				Expect(response.KeepContainer).To(BeFalse())
				Expect(response.Error).To(BeNil())
			})
		})

		Context("when the request body is invalid", func() {
			BeforeEach(func() {
				request = newTestRequest("{{bad: stuff}")
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
			request *http.Request
			actual  *models.ActualLRP
		)

		BeforeEach(func() {
			actual = model_helpers.NewValidActualLRP("process-guid", 1)
			requestBody := &models.EvacuateStoppedActualLRPRequest{
				ActualLrpKey:         &actual.ActualLRPKey,
				ActualLrpInstanceKey: &actual.ActualLRPInstanceKey,
			}

			fakeActualLRPDB.ActualLRPGroupByProcessGuidAndIndexReturns(&models.ActualLRPGroup{
				Instance:   actual,
				Evacuating: actual,
			}, nil)

			request = newTestRequest(requestBody)
		})

		JustBeforeEach(func() {
			handler.EvacuateStoppedActualLRP(logger, responseRecorder, request)
		})

		It("does not error and does not keep the container", func() {
			response := models.EvacuationResponse{}
			err := response.Unmarshal(responseRecorder.Body.Bytes())
			Expect(err).NotTo(HaveOccurred())
			Expect(response.KeepContainer).To(BeFalse())
			Expect(response.Error).To(BeNil())
		})

		It("removes the actual lrp", func() {
			Expect(fakeActualLRPDB.RemoveActualLRPCallCount()).To(Equal(1))
			_, guid, index, actualLRPInstanceKey := fakeActualLRPDB.RemoveActualLRPArgsForCall(0)
			Expect(guid).To(Equal("process-guid"))
			Expect(index).To(BeEquivalentTo(1))
			Expect(actualLRPInstanceKey).To(Equal(&actual.ActualLRPInstanceKey))
		})

		It("removes the evacuating actual lrp", func() {
			Expect(fakeEvacuationDB.RemoveEvacuatingActualLRPCallCount()).To(Equal(1))
			_, lrpKey, lrpInstanceKey := fakeEvacuationDB.RemoveEvacuatingActualLRPArgsForCall(0)
			Expect(*lrpKey).To(Equal(actual.ActualLRPKey))
			Expect(*lrpInstanceKey).To(Equal(actual.ActualLRPInstanceKey))
		})

		It("emits events to the hub", func() {
			Eventually(actualHub.EmitCallCount).Should(Equal(2))

			removeEventGroups := []*models.ActualLRPGroup{}
			for i := 0; i < actualHub.EmitCallCount(); i++ {
				event := actualHub.EmitArgsForCall(i)
				removeEvent, ok := event.(*models.ActualLRPRemovedEvent)
				Expect(ok).To(BeTrue())
				removeEventGroups = append(removeEventGroups, removeEvent.ActualLrpGroup)
			}

			Expect(removeEventGroups).To(ContainElement(&models.ActualLRPGroup{Instance: actual}))
			Expect(removeEventGroups).To(ContainElement(&models.ActualLRPGroup{Evacuating: actual}))
		})

		Context("when the actual lrp is on a different cell", func() {
			BeforeEach(func() {
				actual.ActualLRPInstanceKey.CellId = "different-cell"
			})

			It("returns an error but does not keep the container", func() {
				response := models.EvacuationResponse{}
				err := response.Unmarshal(responseRecorder.Body.Bytes())
				Expect(err).NotTo(HaveOccurred())
				Expect(response.KeepContainer).To(BeFalse())
				Expect(response.Error).To(Equal(models.ErrActualLRPCannotBeRemoved))
			})

			It("emits only the removal of the evacuating to the hub", func() {
				Eventually(actualHub.EmitCallCount).Should(Equal(1))
				event := actualHub.EmitArgsForCall(0)
				removeEvent, ok := event.(*models.ActualLRPRemovedEvent)
				Expect(ok).To(BeTrue())
				Expect(removeEvent.ActualLrpGroup).To(Equal(&models.ActualLRPGroup{Evacuating: actual}))
			})
		})

		Context("when the DB returns an unrecoverable error", func() {
			BeforeEach(func() {
				fakeActualLRPDB.RemoveActualLRPReturns(models.NewUnrecoverableError(nil))
			})

			It("logs and writes to the exit channel", func() {
				Eventually(logger).Should(gbytes.Say("unrecoverable-error"))
				Eventually(exitCh).Should(Receive())
			})
		})

		Context("when removing the actual lrp fails", func() {
			BeforeEach(func() {
				fakeActualLRPDB.RemoveActualLRPReturns(errors.New("boom!"))
			})

			It("returns an error but does not keep the container", func() {
				response := models.EvacuationResponse{}
				err := response.Unmarshal(responseRecorder.Body.Bytes())
				Expect(err).NotTo(HaveOccurred())
				Expect(response.KeepContainer).To(BeFalse())
				Expect(response.Error).NotTo(BeNil())
				Expect(response.Error.Error()).To(Equal("boom!"))
			})

			It("emits only the removal of the evacuating to the hub", func() {
				Eventually(actualHub.EmitCallCount).Should(Equal(1))
				event := actualHub.EmitArgsForCall(0)
				removeEvent, ok := event.(*models.ActualLRPRemovedEvent)
				Expect(ok).To(BeTrue())
				Expect(removeEvent.ActualLrpGroup).To(Equal(&models.ActualLRPGroup{Evacuating: actual}))
			})
		})

		Context("when the DB returns an unrecoverable error", func() {
			BeforeEach(func() {
				fakeActualLRPDB.ActualLRPGroupByProcessGuidAndIndexReturns(nil, models.NewUnrecoverableError(nil))
			})

			It("logs and writes to the exit channel", func() {
				Eventually(logger).Should(gbytes.Say("unrecoverable-error"))
				Eventually(exitCh).Should(Receive())
			})
		})

		Context("when fetching the actual lrp group fails", func() {
			BeforeEach(func() {
				fakeActualLRPDB.ActualLRPGroupByProcessGuidAndIndexReturns(nil, errors.New("i failed"))
			})

			It("returns an error but does not keep the container", func() {
				response := models.EvacuationResponse{}
				err := response.Unmarshal(responseRecorder.Body.Bytes())
				Expect(err).NotTo(HaveOccurred())
				Expect(response.KeepContainer).To(BeFalse())
				Expect(response.Error).NotTo(BeNil())
				Expect(response.Error.Error()).To(Equal("i failed"))
			})

			It("does not emit any events", func() {
				Consistently(actualHub.EmitCallCount).Should(Equal(0))
			})
		})

		Context("when the request is invalid", func() {
			BeforeEach(func() {
				request = newTestRequest("{{")
			})

			It("returns an error but does not keep the container", func() {
				response := models.EvacuationResponse{}
				err := response.Unmarshal(responseRecorder.Body.Bytes())
				Expect(err).NotTo(HaveOccurred())
				Expect(response.KeepContainer).To(BeFalse())
				Expect(response.Error).To(Equal(models.ErrBadRequest))
			})
		})
	})
})
