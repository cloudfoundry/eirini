package util_test

import (
	"code.cloudfoundry.org/eirini/util"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("URL", func() {
	It("should make NATS url from password and IP", func() {
		Expect(util.GenerateNatsURL("p@ssword", "10.0.0.1", 4222)).To(Equal("nats://nats:p%40ssword@10.0.0.1:4222"))
	})

})
