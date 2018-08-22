package config

import "code.cloudfoundry.org/credhub-cli/errors"

func ValidateConfig(c Config) error {
	err := ValidateConfigApi(c)
	if err != nil {
		return err
	} else if (c.AccessToken == "" || c.AccessToken == "revoked") && c.ClientID == "" {
		return errors.NewRevokedTokenError()
	}

	return nil
}

func ValidateConfigApi(c Config) error {
	if c.ApiURL == "" {
		return errors.NewNoApiUrlSetError()
	}

	return nil
}
