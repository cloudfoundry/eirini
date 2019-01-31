package handlers_test

import (
	"net/http"
	"net/http/httptest"

	"code.cloudfoundry.org/bbs/db/dbfakes"
	"code.cloudfoundry.org/bbs/handlers"
	"code.cloudfoundry.org/bbs/models"
	"code.cloudfoundry.org/lager/lagertest"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
)

var _ = Describe("ActualLRP Handlers", func() {
	var (
		logger           *lagertest.TestLogger
		fakeActualLRPDB  *dbfakes.FakeActualLRPDB
		responseRecorder *httptest.ResponseRecorder
		handler          *handlers.ActualLRPHandler
		exitCh           chan struct{}

		actualLRP1     models.ActualLRP
		actualLRP2     models.ActualLRP
		evacuatingLRP2 models.ActualLRP
	)

	BeforeEach(func() {
		actualLRP1 = models.ActualLRP{
			ActualLRPKey: models.NewActualLRPKey(
				"process-guid-0",
				1,
				"domain-0",
			),
			ActualLRPInstanceKey: models.NewActualLRPInstanceKey(
				"instance-guid-0",
				"cell-id-0",
			),
			State: models.ActualLRPStateRunning,
			Since: 1138,
		}

		actualLRP2 = models.ActualLRP{
			ActualLRPKey: models.NewActualLRPKey(
				"process-guid-1",
				2,
				"domain-1",
			),
			ActualLRPInstanceKey: models.NewActualLRPInstanceKey(
				"instance-guid-1",
				"cell-id-1",
			),
			State: models.ActualLRPStateClaimed,
			Since: 4444,
		}

		evacuatingLRP2 = actualLRP2
		evacuatingLRP2.Presence = models.ActualLRP_Evacuating
		evacuatingLRP2.State = models.ActualLRPStateRunning
		evacuatingLRP2.Since = 3417
		evacuatingLRP2.ActualLRPInstanceKey = models.NewActualLRPInstanceKey(
			"instance-guid-1",
			"cell-id-0",
		)

		fakeActualLRPDB = new(dbfakes.FakeActualLRPDB)
		logger = lagertest.NewTestLogger("test")
		responseRecorder = httptest.NewRecorder()
		exitCh = make(chan struct{}, 1)
		handler = handlers.NewActualLRPHandler(fakeActualLRPDB, exitCh)
	})

	Describe("ActualLRPs", func() {
		var requestBody interface{}

		BeforeEach(func() {
			requestBody = &models.ActualLRPsRequest{}
		})

		JustBeforeEach(func() {
			request := newTestRequest(requestBody)
			handler.ActualLRPs(logger, responseRecorder, request)
		})

		Context("when reading actual lrps from DB succeeds", func() {
			var (
				actualLRPs  []*models.ActualLRP
				suspectLRP1 models.ActualLRP
			)

			BeforeEach(func() {
				actualLRP1.State = models.ActualLRPStateUnclaimed

				suspectLRP1 = models.ActualLRP{
					ActualLRPKey: models.NewActualLRPKey(
						"process-guid-0",
						1,
						"domain-0",
					),
					ActualLRPInstanceKey: models.NewActualLRPInstanceKey(
						"instance-guid-0",
						"cell-id-2",
					),
					State:    models.ActualLRPStateRunning,
					Since:    2626,
					Presence: models.ActualLRP_Suspect,
				}
				actualLRPs =
					[]*models.ActualLRP{
						&suspectLRP1, &actualLRP1, &actualLRP2, &evacuatingLRP2,
					}
				fakeActualLRPDB.ActualLRPsReturns(actualLRPs, nil)
			})

			It("returns a list of actual lrps", func() {
				Expect(responseRecorder.Code).To(Equal(http.StatusOK))
				response := models.ActualLRPsResponse{}
				err := response.Unmarshal(responseRecorder.Body.Bytes())
				Expect(err).NotTo(HaveOccurred())

				Expect(response.Error).To(BeNil())
				Expect(response.ActualLrps).To(Equal(actualLRPs))
			})

			Context("and no filter is provided", func() {
				It("calls the DB with no filters to retrieve the actual lrp groups", func() {
					Expect(fakeActualLRPDB.ActualLRPsCallCount()).To(Equal(1))
					_, filter := fakeActualLRPDB.ActualLRPsArgsForCall(0)
					Expect(filter).To(Equal(models.ActualLRPFilter{}))
				})
			})

			Context("and filtering by domain", func() {
				BeforeEach(func() {
					requestBody = &models.ActualLRPsRequest{Domain: "domain-1"}
				})

				It("calls the DB with the domain filter to retrieve the actual lrps", func() {
					Expect(fakeActualLRPDB.ActualLRPsCallCount()).To(Equal(1))
					_, filter := fakeActualLRPDB.ActualLRPsArgsForCall(0)
					Expect(filter).To(Equal(models.ActualLRPFilter{Domain: "domain-1"}))
				})
			})

			Context("and filtering by cellId", func() {
				BeforeEach(func() {
					requestBody = &models.ActualLRPsRequest{CellId: "cellid-1"}
				})

				It("calls the DB with the cell id filter to retrieve the actual lrps ", func() {
					Expect(fakeActualLRPDB.ActualLRPsCallCount()).To(Equal(1))
					_, filter := fakeActualLRPDB.ActualLRPsArgsForCall(0)
					Expect(filter).To(Equal(models.ActualLRPFilter{CellID: "cellid-1"}))
				})
			})

			Context("and filtering by processGuid", func() {
				BeforeEach(func() {
					requestBody = &models.ActualLRPsRequest{ProcessGuid: "process-guid-1"}
				})

				It("calls the DB with the process guid filter to retrieve the actual lrps", func() {
					Expect(fakeActualLRPDB.ActualLRPsCallCount()).To(Equal(1))
					_, filter := fakeActualLRPDB.ActualLRPsArgsForCall(0)
					Expect(filter).To(Equal(models.ActualLRPFilter{ProcessGuid: "process-guid-1"}))
				})
			})

			Context("and filtering by instance index", func() {
				BeforeEach(func() {
					index := int32(1)
					requestBody = &models.ActualLRPsRequest{Index: &index}
				})

				It("calls the DB with the index filter to retrieve the actual lrps", func() {
					Expect(fakeActualLRPDB.ActualLRPsCallCount()).To(Equal(1))
					_, filter := fakeActualLRPDB.ActualLRPsArgsForCall(0)
					Expect(filter.Index).NotTo(BeNil())
					Expect(*filter.Index).To(Equal(int32(1)))
				})
			})

			Context("and filtering by multiple fields", func() {
				BeforeEach(func() {
					index := int32(2)
					requestBody = &models.ActualLRPsRequest{Domain: "potato", CellId: "cellid-1", Index: &index, ProcessGuid: "process-guid-0"}
				})

				It("call the DB with all provided filters to retrieve the actual lrps", func() {
					Expect(fakeActualLRPDB.ActualLRPsCallCount()).To(Equal(1))
					_, filter := fakeActualLRPDB.ActualLRPsArgsForCall(0)
					Expect(filter.Domain).To(Equal("potato"))
					Expect(filter.CellID).To(Equal("cellid-1"))
					Expect(filter.ProcessGuid).To(Equal("process-guid-0"))
					Expect(filter.Index).NotTo(BeNil())
					Expect(*filter.Index).To(Equal(int32(2)))
				})
			})
		})

		Context("when the DB returns no actual lrps", func() {
			BeforeEach(func() {
				fakeActualLRPDB.ActualLRPsReturns([]*models.ActualLRP{}, nil)
			})

			It("returns an empty list", func() {
				Expect(responseRecorder.Code).To(Equal(http.StatusOK))
				response := &models.ActualLRPsResponse{}
				err := response.Unmarshal(responseRecorder.Body.Bytes())
				Expect(err).NotTo(HaveOccurred())

				Expect(response.Error).To(BeNil())
				Expect(response.ActualLrps).To(BeNil())
			})
		})

		Context("when the DB returns an unrecoverable error", func() {
			BeforeEach(func() {
				fakeActualLRPDB.ActualLRPsReturns([]*models.ActualLRP{}, models.NewUnrecoverableError(nil))
			})

			It("logs and writes to the exit channel", func() {
				Eventually(logger).Should(gbytes.Say("unrecoverable-error"))
				Eventually(exitCh).Should(Receive())
			})
		})

		Context("when the DB errors out", func() {
			BeforeEach(func() {
				fakeActualLRPDB.ActualLRPsReturns([]*models.ActualLRP{}, models.ErrUnknownError)
			})

			It("provides relevant error information", func() {
				Expect(responseRecorder.Code).To(Equal(http.StatusOK))
				response := &models.ActualLRPsResponse{}
				err := response.Unmarshal(responseRecorder.Body.Bytes())
				Expect(err).NotTo(HaveOccurred())

				Expect(response.Error).To(Equal(models.ErrUnknownError))
			})
		})
	})

	Describe("ActualLRPGroups", func() {
		var requestBody interface{}

		BeforeEach(func() {
			requestBody = &models.ActualLRPGroupsRequest{}
		})

		JustBeforeEach(func() {
			request := newTestRequest(requestBody)
			handler.ActualLRPGroups(logger, responseRecorder, request)
		})

		Context("when reading actual lrps from DB succeeds", func() {
			var actualLRPGroups []*models.ActualLRPGroup

			BeforeEach(func() {
				actualLRPs :=
					[]*models.ActualLRP{
						&actualLRP1,
						&actualLRP2,
						&evacuatingLRP2,
					}
				fakeActualLRPDB.ActualLRPsReturns(actualLRPs, nil)

				actualLRPGroups = []*models.ActualLRPGroup{
					&models.ActualLRPGroup{Instance: &actualLRP1},
					&models.ActualLRPGroup{Instance: &actualLRP2, Evacuating: &evacuatingLRP2},
				}
			})

			It("returns a list of actual lrp groups", func() {
				Expect(responseRecorder.Code).To(Equal(http.StatusOK))
				response := models.ActualLRPGroupsResponse{}
				err := response.Unmarshal(responseRecorder.Body.Bytes())
				Expect(err).NotTo(HaveOccurred())

				Expect(response.Error).To(BeNil())
				Expect(response.ActualLrpGroups).To(Equal(actualLRPGroups))
			})

			Context("and no filter is provided", func() {
				It("call the DB with no filters to retrieve the actual lrp groups", func() {
					Expect(fakeActualLRPDB.ActualLRPsCallCount()).To(Equal(1))
					_, filter := fakeActualLRPDB.ActualLRPsArgsForCall(0)
					Expect(filter).To(Equal(models.ActualLRPFilter{}))
				})
			})

			Context("and filtering by domain", func() {
				BeforeEach(func() {
					requestBody = &models.ActualLRPGroupsRequest{Domain: "domain-1"}
				})

				It("call the DB with the domain filter to retrieve the actual lrp groups", func() {
					Expect(fakeActualLRPDB.ActualLRPsCallCount()).To(Equal(1))
					_, filter := fakeActualLRPDB.ActualLRPsArgsForCall(0)
					Expect(filter.Domain).To(Equal("domain-1"))
				})
			})

			Context("and filtering by cellId", func() {
				BeforeEach(func() {
					requestBody = &models.ActualLRPGroupsRequest{CellId: "cellid-1"}
				})

				It("call the DB with the cell id filter to retrieve the actual lrp groups", func() {
					Expect(fakeActualLRPDB.ActualLRPsCallCount()).To(Equal(1))
					_, filter := fakeActualLRPDB.ActualLRPsArgsForCall(0)
					Expect(filter.CellID).To(Equal("cellid-1"))
				})
			})

			Context("and filtering by cellId and domain", func() {
				BeforeEach(func() {
					requestBody = &models.ActualLRPGroupsRequest{Domain: "potato", CellId: "cellid-1"}
				})

				It("call the DB with the both filters to retrieve the actual lrp groups", func() {
					Expect(fakeActualLRPDB.ActualLRPsCallCount()).To(Equal(1))
					_, filter := fakeActualLRPDB.ActualLRPsArgsForCall(0)
					Expect(filter.CellID).To(Equal("cellid-1"))
					Expect(filter.Domain).To(Equal("potato"))
				})
			})
		})

		Context("when the DB returns no actual lrp groups", func() {
			BeforeEach(func() {
				fakeActualLRPDB.ActualLRPsReturns([]*models.ActualLRP{}, nil)
			})

			It("returns an empty list", func() {
				Expect(responseRecorder.Code).To(Equal(http.StatusOK))
				response := &models.ActualLRPGroupsResponse{}
				err := response.Unmarshal(responseRecorder.Body.Bytes())
				Expect(err).NotTo(HaveOccurred())

				Expect(response.Error).To(BeNil())
				Expect(response.ActualLrpGroups).To(BeNil())
			})
		})

		Context("when the DB returns an unrecoverable error", func() {
			BeforeEach(func() {
				fakeActualLRPDB.ActualLRPsReturns([]*models.ActualLRP{}, models.NewUnrecoverableError(nil))
			})

			It("logs and writes to the exit channel", func() {
				Eventually(logger).Should(gbytes.Say("unrecoverable-error"))
				Eventually(exitCh).Should(Receive())
			})
		})

		Context("when the DB errors out", func() {
			BeforeEach(func() {
				fakeActualLRPDB.ActualLRPsReturns([]*models.ActualLRP{}, models.ErrUnknownError)
			})

			It("provides relevant error information", func() {
				Expect(responseRecorder.Code).To(Equal(http.StatusOK))
				response := &models.ActualLRPGroupsResponse{}
				err := response.Unmarshal(responseRecorder.Body.Bytes())
				Expect(err).NotTo(HaveOccurred())

				Expect(response.Error).To(Equal(models.ErrUnknownError))
			})
		})
	})

	Describe("ActualLRPGroupsByProcessGuid", func() {
		var (
			processGuid = "process-guid"
			requestBody interface{}
		)

		BeforeEach(func() {
			requestBody = &models.ActualLRPGroupsByProcessGuidRequest{
				ProcessGuid: processGuid,
			}
		})

		JustBeforeEach(func() {
			request := newTestRequest(requestBody)
			handler.ActualLRPGroupsByProcessGuid(logger, responseRecorder, request)
		})

		Context("when reading actual lrps from DB succeeds", func() {
			var actualLRPs []*models.ActualLRP
			var actualLRPGroups []*models.ActualLRPGroup

			BeforeEach(func() {
				actualLRPGroups =
					[]*models.ActualLRPGroup{
						{Instance: &actualLRP1},
						{Instance: &actualLRP2, Evacuating: &evacuatingLRP2},
					}

				actualLRPs =
					[]*models.ActualLRP{
						&actualLRP1,
						&actualLRP2,
						&evacuatingLRP2,
					}
				fakeActualLRPDB.ActualLRPsReturns(actualLRPs, nil)
			})

			It("fetches actual lrp groups by process guid", func() {
				Expect(fakeActualLRPDB.ActualLRPsCallCount()).To(Equal(1))
				_, filter := fakeActualLRPDB.ActualLRPsArgsForCall(0)
				Expect(filter.ProcessGuid).To(Equal(processGuid))
				Expect(responseRecorder.Code).To(Equal(http.StatusOK))
			})

			It("returns a list of actual lrp groups", func() {
				Expect(responseRecorder.Code).To(Equal(http.StatusOK))

				response := &models.ActualLRPGroupsResponse{}
				err := response.Unmarshal(responseRecorder.Body.Bytes())
				Expect(err).NotTo(HaveOccurred())

				Expect(response.ActualLrpGroups).To(Equal(actualLRPGroups))
			})
		})

		Context("when the DB returns no actual lrp groups", func() {
			BeforeEach(func() {
				fakeActualLRPDB.ActualLRPsReturns([]*models.ActualLRP{}, nil)
			})

			It("returns an empty list", func() {
				Expect(responseRecorder.Code).To(Equal(http.StatusOK))

				response := &models.ActualLRPGroupsResponse{}
				err := response.Unmarshal(responseRecorder.Body.Bytes())
				Expect(err).NotTo(HaveOccurred())

				Expect(response.ActualLrpGroups).To(BeNil())
			})
		})

		Context("when the DB returns an unrecoverable error", func() {
			BeforeEach(func() {
				fakeActualLRPDB.ActualLRPsReturns([]*models.ActualLRP{}, models.NewUnrecoverableError(nil))
			})

			It("logs and writes to the exit channel", func() {
				Eventually(logger).Should(gbytes.Say("unrecoverable-error"))
				Eventually(exitCh).Should(Receive())
			})
		})

		Context("when the DB errors out", func() {
			BeforeEach(func() {
				fakeActualLRPDB.ActualLRPsReturns([]*models.ActualLRP{}, models.ErrUnknownError)
			})

			It("provides relevant error information", func() {
				Expect(responseRecorder.Code).To(Equal(http.StatusOK))

				response := &models.ActualLRPGroupsResponse{}
				err := response.Unmarshal(responseRecorder.Body.Bytes())
				Expect(err).NotTo(HaveOccurred())

				Expect(response.Error).To(Equal(models.ErrUnknownError))
			})
		})
	})

	Describe("ActualLRPGroupByProcessGuidAndIndex", func() {
		var (
			processGuid       = "process-guid"
			index       int32 = 1

			requestBody interface{}
		)

		BeforeEach(func() {
			requestBody = &models.ActualLRPGroupByProcessGuidAndIndexRequest{
				ProcessGuid: processGuid,
				Index:       index,
			}
		})

		JustBeforeEach(func() {
			request := newTestRequest(requestBody)
			handler.ActualLRPGroupByProcessGuidAndIndex(logger, responseRecorder, request)
		})

		Context("when reading actual lrps from DB succeeds", func() {
			var actualLRPGroup *models.ActualLRPGroup
			var actualLRPs []*models.ActualLRP

			BeforeEach(func() {
				actualLRPGroup = &models.ActualLRPGroup{Instance: &actualLRP1}
				actualLRPs = []*models.ActualLRP{&actualLRP1}
				fakeActualLRPDB.ActualLRPsReturns(actualLRPs, nil)
			})

			It("fetches actual lrp group by process guid and index", func() {
				Expect(fakeActualLRPDB.ActualLRPsCallCount()).To(Equal(1))
				_, filter := fakeActualLRPDB.ActualLRPsArgsForCall(0)
				Expect(filter.ProcessGuid).To(Equal(processGuid))
				Expect(*filter.Index).To(BeEquivalentTo(index))
			})

			It("returns an actual lrp group", func() {
				Expect(responseRecorder.Code).To(Equal(http.StatusOK))

				response := &models.ActualLRPGroupResponse{}
				err := response.Unmarshal(responseRecorder.Body.Bytes())
				Expect(err).NotTo(HaveOccurred())

				Expect(response.Error).To(BeNil())
				Expect(response.ActualLrpGroup).To(Equal(actualLRPGroup))
			})

			Context("when there is also an evacuating LRP", func() {
				BeforeEach(func() {
					actualLRPGroup = &models.ActualLRPGroup{Instance: &actualLRP2, Evacuating: &evacuatingLRP2}
					actualLRPs = []*models.ActualLRP{&actualLRP2, &evacuatingLRP2}
					fakeActualLRPDB.ActualLRPsReturns(actualLRPs, nil)
				})

				It("returns both LRPs in the group", func() {
					response := &models.ActualLRPGroupResponse{}
					err := response.Unmarshal(responseRecorder.Body.Bytes())
					Expect(err).NotTo(HaveOccurred())

					Expect(response.Error).To(BeNil())
					Expect(response.ActualLrpGroup).To(Equal(actualLRPGroup))
				})
			})
		})

		Context("when we cannot find the resource", func() {
			BeforeEach(func() {
				fakeActualLRPDB.ActualLRPsReturns([]*models.ActualLRP{}, nil)
			})

			It("provides relevant error information", func() {
				Expect(responseRecorder.Code).To(Equal(http.StatusOK))
				response := &models.ActualLRPGroupResponse{}
				err := response.Unmarshal(responseRecorder.Body.Bytes())
				Expect(err).NotTo(HaveOccurred())

				Expect(response.Error).To(Equal(models.ErrResourceNotFound))
			})
		})

		Context("when the DB returns an unrecoverable error", func() {
			BeforeEach(func() {
				fakeActualLRPDB.ActualLRPsReturns(nil, models.NewUnrecoverableError(nil))
			})

			It("logs and writes to the exit channel", func() {
				Eventually(logger).Should(gbytes.Say("unrecoverable-error"))
				Eventually(exitCh).Should(Receive())
			})
		})

		Context("when the DB errors out", func() {
			BeforeEach(func() {
				fakeActualLRPDB.ActualLRPsReturns(nil, models.ErrUnknownError)
			})

			It("provides relevant error information", func() {
				Expect(responseRecorder.Code).To(Equal(http.StatusOK))
				response := &models.ActualLRPGroupResponse{}
				err := response.Unmarshal(responseRecorder.Body.Bytes())
				Expect(err).NotTo(HaveOccurred())

				Expect(response.Error).To(Equal(models.ErrUnknownError))
			})
		})
	})
})
