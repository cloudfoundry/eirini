package handlers_test

import (
	"fmt"
	"net/http"
	"net/http/httptest"

	. "code.cloudfoundry.org/cfhttp/handlers"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var w *httptest.ResponseRecorder

type Response struct {
	Key string `json:"key"`
}

var _ = Describe("cf_http handlers", func() {
	BeforeEach(func() {
		w = httptest.NewRecorder()
	})

	Context("with valid http response writer", func() {
		Context("with valid json structure", func() {
			It("should succeed", func() {
				var structure Response
				structure.Key = "val"
				WriteJSONResponse(w, 200, structure)
				Expect(w.HeaderMap["Content-Length"]).To(Equal([]string{"13"}))
				Expect(w.Body.String()).To(Equal("{\"key\":\"val\"}"))
				Expect(w.Code).To(Equal(200))
			})
		})
		Context("with invalid json structure", func() {
			It("should fail", func() {
				var garbage map[float64]string
				defer func() {
					r := recover()
					Expect(r).NotTo(BeNil())
				}()
				WriteJSONResponse(w, 200, garbage)
			})
		})
		Context("with accepted response", func() {
			It("should succeed", func() {
				WriteStatusAcceptedResponse(w)
				Expect(w.Body.String()).To(Equal("{}"))
				Expect(w.Code).To(Equal(http.StatusAccepted))

			})
		})

		Context("with created response", func() {
			It("should succeed", func() {
				WriteStatusCreatedResponse(w)
				Expect(w.Body.String()).To(Equal("{}"))
				Expect(w.Code).To(Equal(http.StatusCreated))

			})

		})
		Context("with Internal Error response", func() {
			It("should succeed", func() {
				WriteInternalErrorJSONResponse(w, fmt.Errorf("test error"))
				Expect(w.Body.String()).To(Equal("{\"error\":\"test error\"}"))
				Expect(w.Code).To(Equal(http.StatusInternalServerError))

			})
		})

		Context("with Invalid JSON response", func() {
			It("should succeed", func() {
				WriteInvalidJSONResponse(w, fmt.Errorf("test error"))
				Expect(w.Body.String()).To(Equal("{\"error\":\"test error\"}"))
				Expect(w.Code).To(Equal(http.StatusBadRequest))

			})
		})
	})

})
