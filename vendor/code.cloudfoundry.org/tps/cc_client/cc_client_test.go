package cc_client_test

import (
	"crypto/tls"
	"io/ioutil"
	"net/http"
	"net/url"

	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/runtimeschema/cc_messages"
	"code.cloudfoundry.org/tps/cc_client"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/ghttp"
)

var _ = Describe("CC Client", func() {
	var (
		fakeCC *ghttp.Server

		logger   lager.Logger
		ccClient cc_client.CcClient
	)

	guid := "a-guid"

	BeforeEach(func() {
		fakeCC = ghttp.NewServer()

		logger = lager.NewLogger("fakelogger")
		logger.RegisterSink(lager.NewWriterSink(GinkgoWriter, lager.DEBUG))

		tlsConfig, err := cc_client.NewTLSConfig(
			"../fixtures/watcher_cc_client.crt",
			"../fixtures/watcher_cc_client.key",
			"../fixtures/watcher_cc_ca.crt")
		Expect(err).NotTo(HaveOccurred())
		ccClient = cc_client.NewCcClient(fakeCC.URL(), tlsConfig)
	})

	AfterEach(func() {
		if fakeCC.HTTPTestServer != nil {
			fakeCC.Close()
		}
	})

	Describe("Successfully calling the Cloud Controller", func() {
		var expectedBody = []byte(`{"instance":"","index":1,"cell_id":"","reason":"","crash_count":0,"crash_timestamp":0}`)

		BeforeEach(func() {
			fakeCC.AppendHandlers(
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("POST", "/internal/v4/apps/"+guid+"/crashed"),
					ghttp.RespondWith(200, `{}`),
					func(w http.ResponseWriter, req *http.Request) {
						body, err := ioutil.ReadAll(req.Body)
						defer req.Body.Close()

						Expect(err).NotTo(HaveOccurred())
						Expect(body).To(Equal(expectedBody))
					},
				),
			)
		})

		It("sends the request payload to the CC without modification", func() {
			err := ccClient.AppCrashed(guid, cc_messages.AppCrashedRequest{
				Index: 1,
			}, logger)
			Expect(err).NotTo(HaveOccurred())
		})
	})

	Describe("Error conditions", func() {
		Context("when the request couldn't be completed", func() {
			BeforeEach(func() {
				bogusURL := "http://0.0.0.0.0:80"
				ccClient = cc_client.NewCcClient(bogusURL, &tls.Config{})
			})

			It("percolates the error", func() {
				err := ccClient.AppCrashed(guid, cc_messages.AppCrashedRequest{
					Index: 1,
				}, logger)
				Expect(err).To(HaveOccurred())
				Expect(err).To(BeAssignableToTypeOf(&url.Error{}))
			})
		})

		Context("when the response code is not StatusOK (200)", func() {
			BeforeEach(func() {
				fakeCC.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("POST", "/internal/v4/apps/"+guid+"/crashed"),
						ghttp.RespondWith(500, `{}`),
					),
				)
			})

			It("returns an error with the actual status code", func() {
				err := ccClient.AppCrashed(guid, cc_messages.AppCrashedRequest{
					Index: 1,
				}, logger)
				Expect(err).To(HaveOccurred())
				Expect(err).To(BeAssignableToTypeOf(&cc_client.BadResponseError{}))
				Expect(err.(*cc_client.BadResponseError).StatusCode).To(Equal(500))
			})
		})
	})

})
