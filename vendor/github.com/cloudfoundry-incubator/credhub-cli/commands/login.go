package commands

import (
	"fmt"

	"os"

	"code.cloudfoundry.org/credhub-cli/config"
	"code.cloudfoundry.org/credhub-cli/credhub"
	"code.cloudfoundry.org/credhub-cli/credhub/auth"
	"code.cloudfoundry.org/credhub-cli/credhub/auth/uaa"
	"code.cloudfoundry.org/credhub-cli/errors"
	"code.cloudfoundry.org/credhub-cli/util"
	"github.com/howeyc/gopass"
)

type LoginCommand struct {
	Username          string   `short:"u" long:"username" description:"Authentication username"`
	Password          string   `short:"p" long:"password" description:"Authentication password"`
	ClientName        string   `long:"client-name" description:"Client name for UAA client grant" env:"CREDHUB_CLIENT"`
	ClientSecret      string   `long:"client-secret" description:"Client secret for UAA client grant" env:"CREDHUB_SECRET"`
	ServerUrl         string   `short:"s" long:"server" description:"URI of API server to target" env:"CREDHUB_SERVER"`
	CaCerts           []string `long:"ca-cert" description:"Trusted CA for API and UAA TLS connections" env:"CREDHUB_CA_CERT"`
	SkipTlsValidation bool     `long:"skip-tls-validation" description:"Skip certificate validation of the API endpoint. Not recommended!"`
	SSO               bool     `long:"sso" description:"Prompt for a one-time passcode to login"`
	SSOPasscode       string   `long:"sso-passcode" description:"One-time passcode"`
	ConfigCommand
}

func (c *LoginCommand) Execute([]string) error {
	var (
		accessToken  string
		refreshToken string
		err          error
	)

	if c.config.ApiURL == "" && c.ServerUrl == "" {
		return errors.NewNoApiUrlSetError()
	}

	if c.ServerUrl != "" {
		c.config.InsecureSkipVerify = c.SkipTlsValidation

		serverUrl := util.AddDefaultSchemeIfNecessary(c.ServerUrl)
		c.config.ApiURL = serverUrl

		err := c.config.UpdateTrustedCAs(c.CaCerts)
		if err != nil {
			return err
		}

		credhubInfo, err := GetApiInfo(serverUrl, c.config.CaCerts, c.SkipTlsValidation)
		if err != nil {
			return errors.NewNetworkError(err)
		}
		c.config.AuthURL = credhubInfo.AuthServer.URL

		err = verifyAuthServerConnection(c.config, c.SkipTlsValidation)
		if err != nil {
			return errors.NewNetworkError(err)
		}
	}

	err = validateParameters(c)

	if err != nil {
		return err
	}
	credhubClient, err := credhub.New(c.config.ApiURL, credhub.CaCerts(c.config.CaCerts...), credhub.SkipTLSValidation(c.config.InsecureSkipVerify))
	if err != nil {
		return err
	}

	uaaClient := uaa.Client{
		AuthURL: c.config.AuthURL,
		Client:  credhubClient.Client(),
	}

	if c.ClientName != "" || c.ClientSecret != "" {
		accessToken, err = uaaClient.ClientCredentialGrant(c.ClientName, c.ClientSecret)
	} else {
		err = promptForMissingCredentials(c, &uaaClient)
		if err == nil {
			if c.SSOPasscode != "" {
				accessToken, refreshToken, err = uaaClient.PasscodeGrant(config.AuthClient, config.AuthPassword, c.SSOPasscode)
			} else {
				accessToken, refreshToken, err = uaaClient.PasswordGrant(config.AuthClient, config.AuthPassword, c.Username, c.Password)
			}
		}
	}

	if err != nil {
		RevokeTokenIfNecessary(c.config)
		MarkTokensAsRevokedInConfig(&c.config)
		config.WriteConfig(c.config)
		return errors.NewUAAError(err)
	}

	if os.Getenv("CREDHUB_CLIENT") == "" || os.Getenv("CREDHUB_SECRET") == "" {
		c.config.AccessToken = accessToken
	} else {
		c.config.AccessToken = ""
	}

	c.config.RefreshToken = refreshToken

	credhubClient, err = credhub.New(c.config.ApiURL,
		credhub.CaCerts(c.config.CaCerts...),
		credhub.SkipTLSValidation(c.config.InsecureSkipVerify),
		credhub.AuthURL(c.config.AuthURL),
		credhub.Auth(auth.Uaa(c.ClientName, c.ClientSecret, "", "", c.config.AccessToken, "", true)),
	)

	if err != nil {
		return err
	}

	version, err := credhubClient.ServerVersion()
	if err != nil {
		return err
	}

	c.config.ServerVersion = version.String()

	if err := config.WriteConfig(c.config); err != nil {
		return err
	}

	if c.ServerUrl != "" {
		PrintWarnings(c.ServerUrl, c.SkipTlsValidation)
		fmt.Println("Setting the target url:", c.config.ApiURL)
	}

	fmt.Println("Login Successful")

	return nil
}

func validateParameters(cmd *LoginCommand) error {
	switch {
	// Intent is client credentials
	case cmd.ClientName != "" || cmd.ClientSecret != "":
		// Make sure nothing else is specified
		if cmd.Username != "" || cmd.Password != "" || cmd.SSO || cmd.SSOPasscode != "" {
			return errors.NewMixedAuthorizationParametersError()
		}

		// Make sure all required fields are specified
		if cmd.ClientName == "" || cmd.ClientSecret == "" {
			return errors.NewClientAuthorizationParametersError()
		}

		return nil

		// Intent is SSO passcode
	case cmd.SSOPasscode != "":
		// Make sure nothing else is specified
		if cmd.ClientName != "" || cmd.ClientSecret != "" || cmd.Username != "" || cmd.Password != "" || cmd.SSO {
			return errors.NewMixedAuthorizationParametersError()
		}

		return nil

		// Intent is to be prompted for token
	case cmd.SSO:
		// Make sure nothing else is specified
		if cmd.ClientName != "" || cmd.ClientSecret != "" || cmd.Username != "" || cmd.Password != "" || cmd.SSOPasscode != "" {
			return errors.NewMixedAuthorizationParametersError()
		}

		return nil

		// Intent is username/password
	default:
		// Make sure nothing else is specified
		if cmd.ClientName != "" || cmd.ClientSecret != "" || cmd.SSO || cmd.SSOPasscode != "" {
			return errors.NewMixedAuthorizationParametersError()
		}

		// Make sure all required fields are specified
		if cmd.Username == "" && cmd.Password != "" {
			return errors.NewPasswordAuthorizationParametersError()
		}

		return nil
	}
}

func promptForMissingCredentials(cmd *LoginCommand, uaa *uaa.Client) error {
	if cmd.SSO || cmd.SSOPasscode != "" {
		if cmd.SSOPasscode == "" {
			md, err := uaa.Metadata()
			if err != nil {
				return err
			}
			fmt.Printf("%s : ", md.PasscodePrompt())
			code, err := gopass.GetPasswdMasked()
			if err != nil {
				return err
			}
			cmd.SSOPasscode = string(code)
		}
		return nil
	}
	if cmd.Username == "" {
		fmt.Printf("username: ")
		fmt.Scanln(&cmd.Username)
	}
	if cmd.Password == "" {
		fmt.Printf("password: ")
		pass, _ := gopass.GetPasswdMasked()
		cmd.Password = string(pass)
	}
	return nil
}
