package util_test

import (
	"context"
	"net/http"
	"time"

	"code.cloudfoundry.org/eirini/util"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/ghttp"
)

type TestData struct {
	Value string `json:"value"`
}

var _ = Describe("RetryableJSONClient", func() {
	var (
		server              *ghttp.Server
		retryableJSONClient *util.RetryableJSONClient
		data                interface{}
	)

	BeforeEach(func() {
		data = TestData{Value: "foo"}
		server = ghttp.NewServer()
		retryableJSONClient = util.NewRetryableJSONClientWithConfig(http.DefaultClient, 3, time.Millisecond)
	})

	AfterEach(func() {
		server.Close()
	})

	Describe("POST", func() {
		var err error

		BeforeEach(func() {
			server.AppendHandlers(ghttp.CombineHandlers(
				ghttp.VerifyRequest("POST", "/"),
				ghttp.VerifyJSONRepresenting(data),
				ghttp.VerifyHeaderKV("Content-Type", "application/json"),
			))
		})

		JustBeforeEach(func() {
			err = retryableJSONClient.Post(ctx, server.URL(), data)
		})

		It("succeeds", func() {
			Expect(err).NotTo(HaveOccurred())
		})

		It("performs a POST request with a serialised body", func() {
			Expect(server.ReceivedRequests()).To(HaveLen(1))
		})

		When("the context is cancelled prior to calling POST", func() {
			BeforeEach(func() {
				var cancel context.CancelFunc
				ctx, cancel = context.WithCancel(ctx)
				cancel()
			})

			It("returns a cancelled context error", func() {
				Expect(err).To(MatchError(ContainSubstring("context canceled")))
			})
		})

		When("the server fails to handle the request", func() {
			BeforeEach(func() {
				server.RouteToHandler("POST", "/", ghttp.RespondWith(500, nil))
			})

			It("retries", func() {
				Expect(server.ReceivedRequests()).To(HaveLen(4))
			})

			It("fails", func() {
				Expect(err).To(HaveOccurred())
			})
		})

		When("the request is not ok", func() {
			BeforeEach(func() {
				server.RouteToHandler("POST", "/", ghttp.RespondWith(400, nil))
			})

			It("doesn't retry", func() {
				Expect(server.ReceivedRequests()).To(HaveLen(1))
			})

			It("fails", func() {
				Expect(err).To(HaveOccurred())
			})
		})
	})
})
