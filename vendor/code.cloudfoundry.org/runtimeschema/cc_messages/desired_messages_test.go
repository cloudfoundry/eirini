package cc_messages_test

import (
	"encoding/json"

	"code.cloudfoundry.org/runtimeschema/cc_messages"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("CC Messages", func() {
	Describe("CCHTTPRoutes", func() {
		It("can convert itself into a CCRouteInfo struct", func() {
			httpRoutes := cc_messages.CCHTTPRoutes{
				{Hostname: "route1"},
				{Hostname: "route2"},
			}

			expectedJson, err := json.Marshal(httpRoutes)
			Expect(err).ToNot(HaveOccurred())

			ccRouteInfo, err := httpRoutes.CCRouteInfo()
			Expect(err).NotTo(HaveOccurred())

			json := ccRouteInfo[cc_messages.CC_HTTP_ROUTES]
			Expect(string(*json)).To(MatchJSON(expectedJson))
		})
	})
})
