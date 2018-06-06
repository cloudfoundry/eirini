package handler_test

import (
	"bytes"
	"net/http"
	"net/http/httptest"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"code.cloudfoundry.org/eirini/eirinifakes"
	. "code.cloudfoundry.org/eirini/handler"
	"code.cloudfoundry.org/lager/lagertest"
)

var _ = Describe("Handler", func() {

	var (
		ts            *httptest.Server
		client        *http.Client
		handlerClient http.Handler
	)

	BeforeEach(func() {
		client = &http.Client{}
		bifrost := new(eirinifakes.FakeBifrost)
		lager := lagertest.NewTestLogger("handler-test")
		handlerClient = New(bifrost, lager)
	})

	JustBeforeEach(func() {
		ts = httptest.NewServer(handlerClient)
	})

	Context("Routes", func() {
		It("serves a apps/:process_guid endpoint", func() {
			req, err := http.NewRequest("PUT", ts.URL+"/apps/myguid", bytes.NewReader([]byte(`{"process_guid": "myguid", "num_instances": 5}`)))
			Expect(err).ToNot(HaveOccurred())
			res, err := client.Do(req)
			Expect(err).ToNot(HaveOccurred())
			Expect(res.StatusCode).To(Equal(http.StatusAccepted))
		})

		It("serves a /apps endpoint", func() {
			resp, err := http.Get(ts.URL + "/apps")
			Expect(err).ToNot(HaveOccurred())
			Expect(resp.StatusCode).To(Equal(http.StatusOK))
		})
	})

})
