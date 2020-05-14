package utils_test

import (
	"net/http"

	"code.cloudfoundry.org/eirini/k8s/utils"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/ghttp"
)

var _ = Describe("Utils/Http", func() {
	var (
		server  *ghttp.Server
		client  *http.Client
		handler http.HandlerFunc
	)

	BeforeEach(func() {
		handler = ghttp.CombineHandlers(
			ghttp.VerifyRequest("PUT", "/this/is/POTATO"),
			ghttp.VerifyJSON(`{"foo": "bar"}`),
			ghttp.VerifyHeaderKV("Content-Type", "application/json"),
		)
	})

	JustBeforeEach(func() {
		server = ghttp.NewServer()
		server.AppendHandlers(handler)
		client = &http.Client{}
	})

	AfterEach(func() {
		server.Close()
	})

	It("succeeds", func() {
		url := server.URL() + "/this/is/POTATO"
		body := struct {
			Foo string `json:"foo"`
		}{Foo: "bar"}
		Expect(utils.Put(client, url, body)).NotTo(HaveOccurred())
	})

	When("creating the request fails", func() {
		It("errors", func() {
			Expect(utils.Put(client, "\t", nil)).To(MatchError(ContainSubstring("failed to create request")))
		})
	})

	When("performing the request fails", func() {
		It("erorrs", func() {
			Expect(utils.Put(client, "foo-url", nil)).To(MatchError(ContainSubstring("request failed")))
		})
	})

	When("the server fails", func() {
		BeforeEach(func() {
			handler = ghttp.RespondWith(http.StatusInternalServerError, "")
		})

		It("erorrs", func() {
			Expect(utils.Put(client, server.URL(), nil)).To(MatchError(ContainSubstring("request not successful: status=500")))
		})
	})
})
