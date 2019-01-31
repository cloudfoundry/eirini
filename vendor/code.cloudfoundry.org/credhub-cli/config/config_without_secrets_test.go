// +build !windows

package config_test

import (
	"code.cloudfoundry.org/credhub-cli/config"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("ConfigWithoutSecrets", func() {
	Describe("#ConvertConfigToConfigWithoutSecrets", func() {
		It("converts config to configWithoutSecrets", func() {
			cliConfig := config.Config{
				ConfigWithoutSecrets: config.ConfigWithoutSecrets{
					ApiURL:             "apiURL",
					AuthURL:            "authURL",
					AccessToken:        "accessToken",
					RefreshToken:       "refreshToken",
					InsecureSkipVerify: true,
					CaCerts:            []string{"cert1", "cert2"},
					ServerVersion:      "version",
				},
				ClientID:     "clientID",
				ClientSecret: "clientSecret",
			}

			expectedState := config.ConfigWithoutSecrets{
				ApiURL:             "apiURL",
				AuthURL:            "authURL",
				AccessToken:        "accessToken",
				RefreshToken:       "refreshToken",
				InsecureSkipVerify: true,
				CaCerts:            []string{"cert1", "cert2"},
				ServerVersion:      "version",
			}

			actualState := config.ConvertConfigToConfigWithoutSecrets(cliConfig)
			Expect(actualState).To(Equal(expectedState))
		})
	})
})
