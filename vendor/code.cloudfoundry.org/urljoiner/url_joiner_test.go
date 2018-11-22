package urljoiner_test

import (
	. "code.cloudfoundry.org/urljoiner"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("UrlJoiner", func() {
	It("should join URLs", func() {
		Expect(Join("")).To(Equal(""))
		Expect(Join("", "bar")).To(Equal("/bar"))
		Expect(Join("http://foo.com")).To(Equal("http://foo.com"))
		Expect(Join("http://foo.com/")).To(Equal("http://foo.com/"))
		Expect(Join("http://foo.com", "bar")).To(Equal("http://foo.com/bar"))
		Expect(Join("http://foo.com", "bar", "baz")).To(Equal("http://foo.com/bar/baz"))
		Expect(Join("http://foo.com/", "bar", "/baz")).To(Equal("http://foo.com/bar/baz"))
		Expect(Join("http://foo.com/", "/bar")).To(Equal("http://foo.com/bar"))
		Expect(Join("http://foo.com", "")).To(Equal("http://foo.com"))
		Expect(Join("http://foo.com/", "")).To(Equal("http://foo.com/"))
	})
})
