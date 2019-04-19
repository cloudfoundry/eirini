package util_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "code.cloudfoundry.org/eirini/util"
)

var _ = Describe("Hash", func() {

	It("should hash the passed string", func() {
		hasher := TruncatedSHA256Hasher{}
		Expect(hasher.Hash("whatisthis")).To(HaveLen(10))
	})

})
