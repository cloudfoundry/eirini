package commands

import (
	"fmt"

	"net/url"

	"github.com/cloudfoundry-incubator/credhub-cli/config"
	"github.com/cloudfoundry-incubator/credhub-cli/credhub"
	"github.com/cloudfoundry-incubator/credhub-cli/credhub/server"
	"github.com/cloudfoundry-incubator/credhub-cli/errors"
	"github.com/cloudfoundry-incubator/credhub-cli/util"
	"github.com/fatih/color"
)

var warning = color.New(color.Bold, color.FgYellow).PrintlnFunc()
var deprecation = color.New(color.Bold, color.FgRed).PrintlnFunc()

type ApiCommand struct {
	Server            ApiPositionalArgs `positional-args:"yes" env:"CREDHUB_SERVER"`
	ServerFlagUrl     string            `short:"s" long:"server" description:"URI of API server to target" env:"CREDHUB_SERVER"`
	CaCerts           []string          `long:"ca-cert" description:"Trusted CA for API and UAA TLS connections. Multiple flags may be provided." env:"CREDHUB_CA_CERT"`
	SkipTlsValidation bool              `long:"skip-tls-validation" description:"Skip certificate validation of the API endpoint. Not recommended!"`
	ConfigCommand
}

type ApiPositionalArgs struct {
	ServerUrl string `positional-arg-name:"SERVER" description:"URI of API server to target"`
}

func (c *ApiCommand) Execute([]string) error {
	var newConfig config.Config
	if c.Server.ServerUrl != "" {
		newConfig.ApiURL = c.Server.ServerUrl
	} else if c.ServerFlagUrl != "" {
		newConfig.ApiURL = c.ServerFlagUrl
	} else if c.config.ApiURL != "" {
		fmt.Println(c.config.ApiURL)
		return nil
	} else {
		return errors.NewNoApiUrlSetError()
	}
	newConfig.ApiURL = util.AddDefaultSchemeIfNecessary(newConfig.ApiURL)
	fmt.Println("Setting the target url:", newConfig.ApiURL)

	caCerts, err := ReadOrGetCaCerts(c.CaCerts)
	if err != nil {
		return err
	}
	newConfig.CaCerts = caCerts
	newConfig.InsecureSkipVerify = c.SkipTlsValidation

	credhubInfo, err := GetApiInfo(newConfig.ApiURL, newConfig.CaCerts, newConfig.InsecureSkipVerify)
	if err != nil {
		return errors.NewNetworkError(err)
	}
	newConfig.AuthURL = credhubInfo.AuthServer.URL

	if newConfig.AuthURL != c.config.AuthURL {
		RevokeTokenIfNecessary(c.config)
		MarkTokensAsRevokedInConfig(&c.config)
	}
	newConfig.AccessToken = c.config.AccessToken
	newConfig.RefreshToken = c.config.RefreshToken

	err = verifyAuthServerConnection(newConfig, newConfig.InsecureSkipVerify)
	if err != nil {
		return errors.NewAuthServerNetworkError(err)
	}

	err = PrintWarnings(newConfig.ApiURL, newConfig.InsecureSkipVerify)
	if err != nil {
		return err
	}

	return config.WriteConfig(newConfig)
}

func GetApiInfo(serverUrl string, caCerts []string, skipTlsValidation bool) (*server.Info, error) {
	credhubClient, err := credhub.New(serverUrl, credhub.CaCerts(caCerts...), credhub.SkipTLSValidation(skipTlsValidation))
	if err != nil {
		return nil, err
	}

	credhubInfo, err := credhubClient.Info()
	return credhubInfo, err
}

func PrintWarnings(serverUrl string, skipTlsValidation bool) error {
	parsedUrl, err := url.Parse(serverUrl)
	if err != nil {
		return err
	}

	if parsedUrl.Scheme != "https" {
		warning("Warning: Insecure HTTP API detected. Data sent to this API could be intercepted" +
			" in transit by third parties. Secure HTTPS API endpoints are recommended.")
	} else {
		if skipTlsValidation {
			warning("Warning: The targeted TLS certificate has not been verified for this connection.")
			deprecation("Warning: The --skip-tls-validation flag is deprecated. Please use --ca-cert instead.")
		}
	}
	return nil
}

func ReadOrGetCaCerts(caCerts []string) ([]string, error) {
	certs := []string{}

	for _, cert := range caCerts {
		certContents, err := util.ReadFileOrStringFromField(cert)
		if err != nil {
			return certs, err
		}
		certs = append(certs, certContents)
	}

	return certs, nil
}
