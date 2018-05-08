package config_test

import (
	"errors"

	"github.com/cloudfoundry-incubator/credhub-cli/config"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Config validation", func() {
	It("returns nil if the config is valid", func() {
		cfg := config.Config{
			ApiURL:      "http://api.example.com",
			AccessToken: "non-revoked",
		}

		Expect(config.ValidateConfig(cfg)).To(BeNil())
	})

	It("requires an API URL", func() {
		cfg := config.Config{}
		cfg.AccessToken = "non-revoked"

		Expect(config.ValidateConfig(cfg)).To(Equal(errors.New("An API target is not set. Please target the location of your server with `credhub api --server api.example.com` to continue.")))
	})

	It("requires a non-revoked token", func() {
		cfg := config.Config{}
		cfg.ApiURL = "http://api.example.com"
		cfg.AccessToken = "revoked"

		Expect(config.ValidateConfig(cfg)).To(Equal(errors.New("You are not currently authenticated. Please log in to continue.")))
	})

	It("requires a non-empty token", func() {
		cfg := config.Config{}
		cfg.ApiURL = "http://api.example.com"

		Expect(config.ValidateConfig(cfg)).To(Equal(errors.New("You are not currently authenticated. Please log in to continue.")))

	})
})
