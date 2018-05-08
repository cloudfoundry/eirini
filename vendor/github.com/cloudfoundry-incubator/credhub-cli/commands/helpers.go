package commands

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"

	"github.com/cloudfoundry-incubator/credhub-cli/config"
	"github.com/cloudfoundry-incubator/credhub-cli/credhub"
	"github.com/cloudfoundry-incubator/credhub-cli/credhub/auth"
	"github.com/cloudfoundry-incubator/credhub-cli/errors"
	"gopkg.in/yaml.v2"
)

func initializeCredhubClient(cfg config.Config) (*credhub.CredHub, error) {
	var credhubClient *credhub.CredHub

	readConfigFromEnvironmentVariables(&cfg)

	err := config.ValidateConfig(cfg)
	if err != nil {
		if !clientCredentialsInEnvironment() || config.ValidateConfigApi(cfg) != nil {
			return nil, err
		}
	}

	if clientCredentialsInEnvironment() {
		credhubClient, err = newCredhubClient(&cfg, os.Getenv("CREDHUB_CLIENT"), os.Getenv("CREDHUB_SECRET"), true)
	} else {
		credhubClient, err = newCredhubClient(&cfg, config.AuthClient, config.AuthPassword, false)
	}

	return credhubClient, err
}

func printCredential(outputJson bool, v interface{}) {
	if outputJson {
		s, _ := json.MarshalIndent(v, "", "\t")
		fmt.Println(string(s))
	} else {
		s, _ := yaml.Marshal(v)
		fmt.Println(string(s))
	}
}

func readConfigFromEnvironmentVariables(cfg *config.Config) error {
	if cfg.CaCerts == nil && os.Getenv("CREDHUB_CA_CERT") != "" {
		caCerts, err := ReadOrGetCaCerts([]string{os.Getenv("CREDHUB_CA_CERT")})
		if err != nil {
			return err
		}

		cfg.CaCerts = caCerts
	}

	if cfg.ApiURL == "" && os.Getenv("CREDHUB_SERVER") != "" {
		cfg.ApiURL = os.Getenv("CREDHUB_SERVER")
	}

	if cfg.AuthURL == "" && cfg.ApiURL != "" {
		credhubInfo, err := GetApiInfo(cfg.ApiURL, cfg.CaCerts, cfg.InsecureSkipVerify)
		if err != nil {
			return errors.NewNetworkError(err)
		}

		cfg.AuthURL = credhubInfo.AuthServer.URL
	}

	return config.WriteConfig(*cfg)
}

func newCredhubClient(cfg *config.Config, clientId string, clientSecret string, usingClientCredentials bool) (*credhub.CredHub, error) {
	credhubClient, err := credhub.New(cfg.ApiURL, credhub.CaCerts(cfg.CaCerts...), credhub.SkipTLSValidation(cfg.InsecureSkipVerify), credhub.Auth(auth.Uaa(
		clientId,
		clientSecret,
		"",
		"",
		cfg.AccessToken,
		cfg.RefreshToken,
		usingClientCredentials,
	)),
		credhub.AuthURL(cfg.AuthURL))
	return credhubClient, err
}

func clientCredentialsInEnvironment() bool {
	return os.Getenv("CREDHUB_CLIENT") != "" || os.Getenv("CREDHUB_SECRET") != ""
}

func verifyAuthServerConnection(cfg config.Config, skipTlsValidation bool) error {
	credhubClient, err := credhub.New(cfg.ApiURL, credhub.CaCerts(cfg.CaCerts...), credhub.SkipTLSValidation(skipTlsValidation))
	if err != nil {
		return err
	}
	if !skipTlsValidation {
		request, _ := http.NewRequest("GET", cfg.AuthURL+"/info", nil)
		request.Header.Add("Accept", "application/json")
		_, err = credhubClient.Client().Do(request)
	}

	return err
}
