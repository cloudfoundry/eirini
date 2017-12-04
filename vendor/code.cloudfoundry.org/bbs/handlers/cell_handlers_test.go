package handlers_test

import (
	"net/http"
	"net/http/httptest"

	"code.cloudfoundry.org/bbs/handlers"
	"code.cloudfoundry.org/bbs/models"
	"code.cloudfoundry.org/bbs/serviceclient/serviceclientfakes"
	"code.cloudfoundry.org/lager/lagertest"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
)

var _ = Describe("Cell Handlers", func() {
	var (
		logger            *lagertest.TestLogger
		responseRecorder  *httptest.ResponseRecorder
		handler           *handlers.CellHandler
		fakeServiceClient *serviceclientfakes.FakeServiceClient
		exitCh            chan struct{}
		cells             []*models.CellPresence
		cellSet           models.CellSet
	)

	BeforeEach(func() {
		fakeServiceClient = new(serviceclientfakes.FakeServiceClient)
		logger = lagertest.NewTestLogger("test")
		responseRecorder = httptest.NewRecorder()
		exitCh = make(chan struct{}, 1)
		handler = handlers.NewCellHandler(fakeServiceClient, exitCh)
		cells = []*models.CellPresence{
			{
				CellId:     "cell-1",
				RepAddress: "1.1.1.1",
				Zone:       "z1",
				Capacity: &models.CellCapacity{
					MemoryMb:   1000,
					DiskMb:     1000,
					Containers: 50,
				},
				RootfsProviders: []*models.Provider{
					&models.Provider{"preloaded", []string{"provider-1", "provider-2"}},
					&models.Provider{"provider-3", nil},
				},
				PlacementTags: []string{"test1", "test2"},
			},
			{
				CellId:     "cell-2",
				RepAddress: "2.2.2.2",
				Zone:       "z2",
				Capacity: &models.CellCapacity{
					MemoryMb:   2000,
					DiskMb:     2000,
					Containers: 20,
				},
				RootfsProviders: []*models.Provider{
					&models.Provider{"preloaded", []string{"provider-1"}},
				},
				PlacementTags: []string{"test3", "test4"},
			},
		}
		cellSet = models.NewCellSet()
		cellSet.Add(cells[0])
		cellSet.Add(cells[1])
	})

	Describe("Cells", func() {
		JustBeforeEach(func() {
			handler.Cells(logger, responseRecorder, newTestRequest(""))
		})

		Context("when reading cells succeeds", func() {
			BeforeEach(func() {
				fakeServiceClient.CellsReturns(cellSet, nil)
			})

			It("returns a list of cells", func() {
				Expect(responseRecorder.Code).To(Equal(http.StatusOK))

				response := &models.CellsResponse{}
				err := response.Unmarshal(responseRecorder.Body.Bytes())
				Expect(err).NotTo(HaveOccurred())

				Expect(response.Error).To(BeNil())
				Expect(response.Cells).To(ConsistOf(cells))
			})
		})

		Context("when the serviceClient returns no cells", func() {
			BeforeEach(func() {
				fakeServiceClient.CellsReturns(nil, nil)
			})

			It("returns an empty list", func() {
				Expect(responseRecorder.Code).To(Equal(http.StatusOK))

				response := &models.CellsResponse{}
				err := response.Unmarshal(responseRecorder.Body.Bytes())
				Expect(err).NotTo(HaveOccurred())

				Expect(response.Error).To(BeNil())
				Expect(response.Cells).To(BeNil())
			})
		})

		Context("when the DB returns an unrecoverable error", func() {
			BeforeEach(func() {
				fakeServiceClient.CellsReturns(nil, models.NewUnrecoverableError(nil))
			})

			It("logs and writes to the exit channel", func() {
				Eventually(logger).Should(gbytes.Say("unrecoverable-error"))
				Eventually(exitCh).Should(Receive())
			})
		})

		Context("when the serviceClient errors out", func() {
			BeforeEach(func() {
				fakeServiceClient.CellsReturns(nil, models.ErrUnknownError)
			})

			It("provides relevant error information", func() {
				Expect(responseRecorder.Code).To(Equal(http.StatusOK))

				response := &models.CellsResponse{}
				err := response.Unmarshal(responseRecorder.Body.Bytes())
				Expect(err).NotTo(HaveOccurred())

				Expect(response.Error).To(Equal(models.ErrUnknownError))
				Expect(response.Cells).To(BeNil())
			})
		})
	})
})
