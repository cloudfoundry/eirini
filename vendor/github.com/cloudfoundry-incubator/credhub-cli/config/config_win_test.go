// +build windows

package config_test

import (
	"syscall"

	"github.com/cloudfoundry-incubator/credhub-cli/config"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Config (windows specific)", func() {
	var cfg config.Config

	BeforeEach(func() {
		cfg = config.Config{
			ApiURL:  "http://api.example.com",
			AuthURL: "http://auth.example.com",
		}
	})

	It("hides the config directory", func() {
		err := config.WriteConfig(cfg)
		Expect(err).NotTo(HaveOccurred())

		p, err := syscall.UTF16PtrFromString(config.ConfigDir())
		Expect(err).ToNot(HaveOccurred())

		attrs, err := syscall.GetFileAttributes(p)
		Expect(err).ToNot(HaveOccurred())

		Expect(attrs & syscall.FILE_ATTRIBUTE_HIDDEN).To(Equal(uint32(syscall.FILE_ATTRIBUTE_HIDDEN)))
	})
})
